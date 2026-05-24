package repl

import (
	"bufio"
	"fmt"
	"gs-cli/internal/cache"
	"gs-cli/internal/metastore"
	"gs-cli/internal/pipeline"
	"gs-cli/internal/rocks"
	"gs-cli/internal/server"
	"gs-cli/internal/session"
	"gs-cli/internal/validator"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
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
	case "\\validate":
		e.validate(parts[1:])
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
	case "\\benchmark":
		if len(parts) < 2 {
			fmt.Println("Usage: \\benchmark <filepath>")
			return
		}
		e.runBenchmark(parts[1])
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

func (e *Executor) validate(args []string) {
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
		fmt.Println("Usage: \\validate -n <nodes.pgdf> -e <edges.pgdf>")
		return
	}

	v := validator.NewPGDFValidator()
	if err := v.Validate(nodesPath, edgesPath); err != nil {
		fmt.Printf("Validation error: %v\n", err)
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
	skipValidation := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			nodesPath = args[i+1]
			i++
		} else if args[i] == "-e" && i+1 < len(args) {
			edgesPath = args[i+1]
			i++
		} else if args[i] == "-yolo" || args[i] == "--yolo" {
			skipValidation = true
		}
	}

	if nodesPath == "" || edgesPath == "" {
		fmt.Println("Error: Both -n <nodes.pgdf> and -e <edges.pgdf> are required for ingestion.")
		return
	}

	// Phase 0: Validation
	if !skipValidation {
		fmt.Println("Phase 0: Validating PGDF files...")
		v := validator.NewPGDFValidator()
		if err := v.Validate(nodesPath, edgesPath); err != nil {
			fmt.Printf("Aborting ingestion due to validation errors: %v\n", err)
			return
		}
	} else {
		fmt.Println("Phase 0: Skipping validation (YOLO mode activated)...")
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

	fmt.Printf("Summary: [Total: %ss] [Peak RAM: %s] [Max RAM: %s] [Paths: %d]\n",
		result.Metadata.Time,
		result.Metadata.PeakMemory,
		result.Metadata.MaxMemory,
		result.Metadata.TotalPaths)

	if e.State.ShowResults {
		for i, path := range result.Data {
			fmt.Printf("Path %d: %s\n", i+1, strings.Join(path.Content, " -> "))
		}
	} else {
		fmt.Println("Use \\results to toggle detailed path output.")
	}
}

func (e *Executor) runBenchmark(filepath string) {
	if e.State.CurrentMode != session.ModeRemote {
		fmt.Println("Error: Benchmark must be run in remote mode.")
		return
	}
	if e.State.ActiveDB == "" {
		fmt.Println("Error: No active database selected.")
		return
	}

	file, err := os.Open(filepath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	var queries []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "--") {
			queries = append(queries, line)
		}
	}

	if len(queries) == 0 {
		fmt.Println("No queries found in file.")
		return
	}

	baseName := filepath[strings.LastIndex(filepath, "/")+1:]
	if i := strings.LastIndex(baseName, "."); i > 0 {
		baseName = baseName[:i]
	}

	rawFile, err := os.Create(fmt.Sprintf("raw_results_%s.txt", baseName))
	if err != nil {
		fmt.Printf("Error creating raw results file: %v\n", err)
		return
	}
	defer rawFile.Close()

	finalFile, err := os.Create(fmt.Sprintf("final_results_%s.txt", baseName))
	if err != nil {
		fmt.Printf("Error creating final results file: %v\n", err)
		return
	}
	defer finalFile.Close()

	rawFile.WriteString(fmt.Sprintf("Benchmark on DB: %s\n", e.State.ActiveDB))
	rawFile.WriteString("--------------------------------------------------\n")
	finalFile.WriteString(fmt.Sprintf("Benchmark Averages on DB: %s\n", e.State.ActiveDB))
	finalFile.WriteString("--------------------------------------------------\n")

	fmt.Printf("Starting benchmark for %d queries...\n", len(queries))

	for i, query := range queries {
		fmt.Printf("Running Query %d/%d: %s\n", i+1, len(queries), query)
		rawFile.WriteString(fmt.Sprintf("\nQuery %d: %s\n", i+1, query))
		
		var times []float64
		var peakRAMs []float64
		var pathCounts []int
		var errors []string

		for run := 1; run <= 5; run++ {
			fmt.Printf("  Run %d... ", run)
			result, err := e.State.Client.Query(e.State.ActiveDB, query)
			
			if err != nil {
				errMsg := fmt.Sprintf("Error: %v", err)
				fmt.Println(errMsg)
				rawFile.WriteString(fmt.Sprintf("  Run %d: %s\n", run, errMsg))
				errors = append(errors, errMsg)

				if strings.Contains(errMsg, "timed out") {
					fmt.Println("  Skipping remaining runs for this query due to timeout.")
					break
				}
				
				if strings.Contains(errMsg, "server is not reachable") || strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "EOF") {
					fmt.Println("  CRITICAL ERROR: Server seems to have crashed. Skipping remaining runs for this query.")
					// Wait a few seconds to let the user restart the server if they are testing manually
					time.Sleep(2 * time.Second)
					break
				}
				continue
			}

			// Parse time (handle both comma and dot)
			tStr := strings.ReplaceAll(result.Metadata.Time, ",", ".")
			t, err := strconv.ParseFloat(tStr, 64)
			if err != nil {
				fmt.Println("failed to parse time")
				continue
			}
			
			// Parse Peak RAM
			ramStr := strings.TrimSuffix(result.Metadata.PeakMemory, " MB")
			ramStr = strings.ReplaceAll(ramStr, ",", ".")
			ram, err := strconv.ParseFloat(ramStr, 64)
			if err != nil {
				fmt.Println("failed to parse ram")
				continue
			}

			times = append(times, t)
			peakRAMs = append(peakRAMs, ram)
			pathCounts = append(pathCounts, result.Metadata.TotalPaths)

			fmt.Printf("OK (%.3fs)\n", t)
			rawFile.WriteString(fmt.Sprintf("  Run %d: [Time: %.3fs] [Peak RAM: %.2f MB] [Paths: %d]\n", run, t, ram, result.Metadata.TotalPaths))
		}

		if len(errors) == 5 || (len(errors) > 0 && len(times) == 0) {
			finalFile.WriteString(fmt.Sprintf("Q%d: FAILED (%s)\n", i+1, errors[0]))
			continue
		}

		if len(times) >= 3 {
			// Sort to drop highest and lowest
			sort.Float64s(times)
			sort.Float64s(peakRAMs)
			
			// Drop 1 highest, 1 lowest
			validTimes := times[1 : len(times)-1]
			validRams := peakRAMs[1 : len(peakRAMs)-1]

			var sumTime, sumRam float64
			for _, v := range validTimes { sumTime += v }
			for _, v := range validRams { sumRam += v }
			
			avgTime := sumTime / float64(len(validTimes))
			avgRam := sumRam / float64(len(validRams))
			
			// Take the path count from the first successful run
			paths := 0
			if len(pathCounts) > 0 {
				paths = pathCounts[0]
			}

			finalFile.WriteString(fmt.Sprintf("Q%d: [Avg Time: %.3fs] [Avg Peak RAM: %.2f MB] [Paths: %d]\n", i+1, avgTime, avgRam, paths))
		} else {
			finalFile.WriteString(fmt.Sprintf("Q%d: INCOMPLETE (Not enough successful runs to average)\n", i+1))
		}
	}
	fmt.Println("Benchmark complete. Results saved.")
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
