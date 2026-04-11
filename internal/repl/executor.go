package repl

import (
	"fmt"
	"gs-cli/internal/cache"
	"gs-cli/internal/metastore"
	"gs-cli/internal/pipeline"
	"gs-cli/internal/rocks"
	"gs-cli/internal/server"
	"gs-cli/internal/session"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Executor struct {
	State *session.State
}

func (e *Executor) Execute(in string) {
	in = strings.TrimSpace(in)
	if in == "" {
		return
	}

	if strings.HasPrefix(in, "\\") {
		e.handleMetaCommand(in)
		return
	}

	// Default: Run query (Remote Only)
	if e.State.CurrentMode == session.ModeRemote {
		if e.State.ActiveDB == "" {
			fmt.Println("Error: No active database selected. Use \\c <dbname>")
			return
		}
		e.runQuery(in)
	} else if e.State.CurrentMode == session.ModeLocal {
		fmt.Println("Error: Local mode is for management only. Queries are not available here.")
		fmt.Println("Connect to a server with \\connect <host:port> to run queries.")
	} else {
		fmt.Println("Error: Not connected. Use \\connect <path|port>")
	}
}

func (e *Executor) handleMetaCommand(in string) {
	parts := strings.Fields(in)
	cmd := parts[0]

	switch cmd {
	case "\\q":
		fmt.Println("Goodbye!")
		os.Exit(0)
	case "\\connect":
		if len(parts) < 2 {
			fmt.Println("Usage: \\connect <path|port>")
			return
		}
		e.connect(parts[1])
	case "\\l":
		e.listDBs()
	case "\\c":
		if len(parts) < 2 {
			fmt.Println("Usage: \\c <dbname>")
			return
		}
		dbName := parts[1]
		if e.State.CurrentMode == session.ModeRemote {
			fmt.Printf("Requesting server to use database: %s...\n", dbName)
			msg, err := e.State.Client.UseDatabase(dbName)
			if err != nil {
				fmt.Printf("Error: server failed to switch database: %v\n", err)
				return
			}
			fmt.Println(msg)
		}
		e.State.ActiveDB = dbName
		fmt.Printf("Active database set to: %s\n", e.State.ActiveDB)
	case "\\i":
		e.ingest(parts[1:])
	case "\\create":
		if len(parts) < 2 {
			fmt.Println("Usage: \\create <dbname>")
			return
		}
		e.createDB(parts[1])
	case "\\drop":
		if len(parts) < 2 {
			fmt.Println("Usage: \\drop <dbname>")
			return
		}
		e.dropDB(parts[1])
	case "\\s":
		e.showStats()
	case "\\d":
		e.describe()
	case "\\results":
		e.State.ShowResults = !e.State.ShowResults
		status := "OFF"
		if e.State.ShowResults {
			status = "ON"
		}
		fmt.Printf("Display of detailed query results is now: %s\n", status)
	default:
		fmt.Printf("Unknown meta-command: %s. Use \\q to exit.\n", cmd)
	}
}

func (e *Executor) connect(target string) {
	if strings.ContainsAny(target, "/.") || (!strings.Contains(target, ":") && !isNumber(target)) {
		absPath, err := filepath.Abs(target)
		if err != nil {
			fmt.Printf("Error: invalid path: %v\n", err)
			return
		}
		e.State.CurrentMode = session.ModeLocal
		e.State.LocalRoot = absPath
		e.State.ActiveDB = ""
		fmt.Printf("Switched to LOCAL management at: %s\n", absPath)
	} else {
		portStr := target
		if strings.Contains(target, ":") {
			parts := strings.Split(target, ":")
			portStr = parts[len(parts)-1]
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			fmt.Printf("Error: invalid port: %v\n", err)
			return
		}
		e.State.CurrentMode = session.ModeRemote
		e.State.RemotePort = port
		e.State.Client = server.NewClient(port)
		e.State.ActiveDB = ""
		fmt.Printf("Connected to REMOTE server at port: %d\n", port)
	}
}

func (e *Executor) listDBs() {
	if e.State.CurrentMode == session.ModeLocal {
		entries, err := os.ReadDir(e.State.LocalRoot)
		if err != nil {
			fmt.Printf("Error reading local directory: %v\n", err)
			return
		}
		fmt.Printf("Local databases in %s:\n", e.State.LocalRoot)
		for _, entry := range entries {
			if entry.IsDir() {
				fmt.Printf("  - %s\n", entry.Name())
			}
		}
	} else if e.State.CurrentMode == session.ModeRemote {
		dbs, err := e.State.Client.ListDatabases()
		if err != nil {
			fmt.Printf("Error listing remote databases: %v\n", err)
			return
		}
		fmt.Printf("Remote databases on port %d:\n", e.State.RemotePort)
		for _, db := range dbs {
			fmt.Printf("  - %s\n", db)
		}
	} else {
		fmt.Println("Error: Not connected.")
	}
}

func (e *Executor) ingest(args []string) {
	if e.State.CurrentMode != session.ModeLocal {
		fmt.Println("Error: Ingestion is only available in LOCAL mode.")
		return
	}
	if e.State.ActiveDB == "" {
		fmt.Println("Error: No active database selected for ingestion. Use \\c <dbname>")
		return
	}

	var nodesPath, edgesPath string
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			nodesPath = args[i+1]
			i++
		} else if args[i] == "-e" && i+1 < len(args) {
			edgesPath = args[i+1]
			i++
		}
	}

	if nodesPath == "" || edgesPath == "" {
		fmt.Println("Error: Both -n <nodes.pgdf> and -e <edges.pgdf> are required for ingestion.")
		return
	}

	dbPath := filepath.Join(e.State.LocalRoot, e.State.ActiveDB)
	fmt.Printf("Ingesting data into %s...\n", dbPath)

	dbStore, err := rocks.Open(dbPath, false)
	if err != nil {
		fmt.Printf("Error opening DB: %v\n", err)
		return
	}
	defer dbStore.Close()

	meta := metastore.NewMetaStore()
	nodeCache := cache.NewNodeCache()
	defer nodeCache.Clear()

	ingestor := pipeline.NewIngestor(dbStore, meta, nodeCache, 1)

	if err := ingestor.IngestNodes(nodesPath); err != nil {
		fmt.Printf("Error ingesting nodes: %v\n", err)
		return
	}
	if err := ingestor.IngestEdges(edgesPath); err != nil {
		fmt.Printf("Error ingesting edges: %v\n", err)
		return
	}

	if err := meta.Save(dbPath); err != nil {
		fmt.Printf("Error saving metadata: %v\n", err)
		return
	}

	fmt.Println("Ingestion completed successfully.")
}

func (e *Executor) createDB(name string) {
	if e.State.CurrentMode != session.ModeLocal {
		fmt.Println("Error: DB creation is only available in LOCAL mode.")
		return
	}
	path := filepath.Join(e.State.LocalRoot, name)
	if err := os.MkdirAll(path, 0755); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}
	fmt.Printf("Database '%s' created at %s\n", name, path)
}

func (e *Executor) dropDB(name string) {
	if e.State.CurrentMode != session.ModeLocal {
		fmt.Println("Error: DB deletion is only available in LOCAL mode.")
		return
	}
	path := filepath.Join(e.State.LocalRoot, name)
	if err := os.RemoveAll(path); err != nil {
		fmt.Printf("Error deleting directory: %v\n", err)
		return
	}
	fmt.Printf("Database '%s' deleted.\n", name)
}

func (e *Executor) runQuery(query string) {
	fmt.Printf("Executing query on remote DB '%s'...\n", e.State.ActiveDB)
	result, err := e.State.Client.Query(e.State.ActiveDB, query)
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
		return
	}

	fmt.Printf("Summary: [Time: %s] [Total Paths: %d]\n", result.Metadata.Time, result.Metadata.TotalPaths)

	if e.State.ShowResults {
		for i, path := range result.Data {
			fmt.Printf("Path %d: %s\n", i+1, strings.Join(path.Content, " -> "))
		}
	} else {
		fmt.Println("Use \\results to toggle detailed path output.")
	}
}

func (e *Executor) showStats() {
	if e.State.CurrentMode != session.ModeLocal || e.State.ActiveDB == "" {
		fmt.Println("Error: Statistics only available for active LOCAL database.")
		return
	}
	dbPath := filepath.Join(e.State.LocalRoot, e.State.ActiveDB)
	dbStore, err := rocks.Open(dbPath, true)
	if err != nil {
		fmt.Printf("Error opening DB: %v\n", err)
		return
	}
	defer dbStore.Close()

	fmt.Println("\n[ LSM TREE STRUCTURE ]")
	fmt.Println(dbStore.GetProperty("rocksdb.levelstats", dbStore.CFNodes))
}

func (e *Executor) describe() {
	if e.State.CurrentMode == session.ModeLocal && e.State.ActiveDB != "" {
		dbPath := filepath.Join(e.State.LocalRoot, e.State.ActiveDB)
		meta, err := metastore.Load(dbPath)
		if err != nil {
			fmt.Printf("Error loading metadata: %v\n", err)
			return
		}
		fmt.Printf("Nodes: %d, Edges: %d\n", meta.NodeCount, meta.EdgeCount)
	} else if e.State.CurrentMode == session.ModeRemote && e.State.ActiveDB != "" {
		fmt.Println("Describe remote not fully implemented yet.")
	} else {
		fmt.Println("Select a database first.")
	}
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
