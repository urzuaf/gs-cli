package repl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gs-cli/internal/cache"
	"gs-cli/internal/metastore"
	"gs-cli/internal/pipeline"
	"gs-cli/internal/rocks"
	"gs-cli/internal/server"
	"gs-cli/internal/session"
	"gs-cli/internal/validator"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/linxGnu/grocksdb"
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
	parts := parseArgs(in)
	if len(parts) == 0 {
		return
	}
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
			fmt.Println("Usage: \\benchmark <filepath> [--fast] [--cold] [--restart-cmd \"<cmd>\"] [--name <name>]")
			return
		}
		filePath := parts[1]
		fast := false
		cold := false
		restartCmd := ""
		customName := ""

		for i := 2; i < len(parts); i++ {
			arg := parts[i]
			if arg == "--fast" || arg == "-fast" {
				fast = true
			} else if arg == "--cold" || arg == "-cold" {
				cold = true
			} else if (arg == "--restart-cmd" || arg == "-restart-cmd") && i+1 < len(parts) {
				restartCmd = parts[i+1]
				i++
			} else if (arg == "--name" || arg == "-name") && i+1 < len(parts) {
				customName = parts[i+1]
				i++
			}
		}

		if cold && restartCmd == "" {
			fmt.Println("Error: --restart-cmd is required when using --cold")
			return
		}

		e.runBenchmark(filePath, fast, cold, restartCmd, customName)
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
	result, err := e.State.Client.Query(e.State.ActiveDB, query, false)
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

func (e *Executor) runBenchmark(filepath string, fast bool, cold bool, restartCmd string, customName string) {
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

	var rawFile, finalFile *os.File
	if !fast {
		baseName := filepath[strings.LastIndex(filepath, "/")+1:]
		if i := strings.LastIndex(baseName, "."); i > 0 {
			baseName = baseName[:i]
		}

		outputName := baseName
		if customName != "" {
			outputName = customName
		}

		rawFile, err = os.Create(fmt.Sprintf("raw_results_%s.txt", outputName))
		if err != nil {
			fmt.Printf("Error creating raw results file: %v\n", err)
			return
		}
		defer rawFile.Close()

		finalFile, err = os.Create(fmt.Sprintf("final_results_%s.txt", outputName))
		if err != nil {
			fmt.Printf("Error creating final results file: %v\n", err)
			return
		}
		defer finalFile.Close()

		rawFile.WriteString(fmt.Sprintf("Benchmark on DB: %s\n", e.State.ActiveDB))
		rawFile.WriteString("--------------------------------------------------\n")
		finalFile.WriteString(fmt.Sprintf("Benchmark Averages on DB: %s\n", e.State.ActiveDB))
		finalFile.WriteString("--------------------------------------------------\n")
	}

	numRuns := 5
	if fast {
		numRuns = 1
		fmt.Printf("Starting FAST benchmark (1 run per query) for %d queries...\n", len(queries))
	} else {
		fmt.Printf("Starting benchmark (5 runs per query) for %d queries...\n", len(queries))
	}

	for i, query := range queries {
		fmt.Printf("Running Query %d/%d: %s\n", i+1, len(queries), query)
		if !fast {
			rawFile.WriteString(fmt.Sprintf("\nQuery %d: %s\n", i+1, query))
		}

		if cold {
			fmt.Println("Cold run activated. Performing preventive restart...")
			if err := e.restartServer(restartCmd); err != nil {
				fmt.Printf("Error restarting server: %v\n", err)
				if !fast {
					rawFile.WriteString(fmt.Sprintf("Query %d FAILED: restart server error: %v\n", i+1, err))
					finalFile.WriteString(fmt.Sprintf("Q%d: FAILED (restart server error: %v)\n", i+1, err))
				}
				continue
			}
		}

		var times []float64
		var peakRAMs []float64
		var pathCounts []int
		var errors []string
		serverCrashed := false

		for run := 1; run <= numRuns; run++ {
			if !fast {
				fmt.Printf("  Run %d... ", run)
			}
			// In benchmark, we use dryRun = true to measure pure engine performance
			result, err := e.State.Client.Query(e.State.ActiveDB, query, true)

			if err != nil {
				errMsg := err.Error()

				standardizedErr := errMsg
				isOOMorCrash := false

				lowerErr := strings.ToLower(errMsg)
				if strings.Contains(lowerErr, "out of memory") ||
					strings.Contains(lowerErr, "oom") ||
					strings.Contains(lowerErr, "memory") ||
					strings.Contains(lowerErr, "heap") ||
					strings.Contains(lowerErr, "connection refused") ||
					strings.Contains(lowerErr, "eof") ||
					strings.Contains(lowerErr, "broken pipe") ||
					strings.Contains(lowerErr, "connection reset") ||
					strings.Contains(lowerErr, "not reachable") {
					standardizedErr = "FAILED (OOM)"
					isOOMorCrash = true
				} else if strings.Contains(lowerErr, "timed out") ||
					strings.Contains(lowerErr, "timeout") ||
					strings.Contains(lowerErr, "deadline exceeded") {
					standardizedErr = "FAILED (TIMEOUT after 300s)"
				} else {
					standardizedErr = fmt.Sprintf("FAILED (%s)", errMsg)
				}

				fmt.Println(standardizedErr)
				if !fast {
					rawFile.WriteString(fmt.Sprintf("  Run %d: %s\n", run, standardizedErr))
				}
				errors = append(errors, standardizedErr)

				if isOOMorCrash || strings.Contains(lowerErr, "connection refused") || strings.Contains(lowerErr, "eof") || strings.Contains(lowerErr, "not reachable") {
					fmt.Println("  CRITICAL ERROR: Server crashed or is unreachable. Skipping remaining runs for this query.")
					serverCrashed = true
					break
				}

				if strings.Contains(standardizedErr, "TIMEOUT") {
					if !fast {
						fmt.Println("  Skipping remaining runs for this query due to timeout.")
					}
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

			if fast {
				fmt.Printf("  Summary: [Time: %.3fs] [Peak RAM: %.2f MB] [Paths: %d]\n", t, ram, result.Metadata.TotalPaths)
			} else {
				fmt.Printf("OK (%.3fs)\n", t)
				rawFile.WriteString(fmt.Sprintf("  Run %d: [Time: %.3fs] [Peak RAM: %.2f MB] [Paths: %d]\n", run, t, ram, result.Metadata.TotalPaths))
			}
		}

		if serverCrashed && cold {
			fmt.Println("Server crashed. Performing autonomous recovery restart...")
			if err := e.restartServer(restartCmd); err != nil {
				fmt.Printf("Error recovering server: %v\n", err)
			}
		}

		if !fast {
			if len(times) == 0 {
				finalErr := "FAILED"
				if len(errors) > 0 {
					finalErr = errors[0]
				}
				finalFile.WriteString(fmt.Sprintf("Q%d: %s\n", i+1, finalErr))
				continue
			}

			var avgTime, avgRam float64
			paths := 0
			if len(pathCounts) > 0 {
				paths = pathCounts[0]
			}

			if len(times) == 5 {
				sort.Float64s(times)
				sort.Float64s(peakRAMs)

				validTimes := times[1:4]
				validRams := peakRAMs[1:4]

				var sumTime, sumRam float64
				for _, v := range validTimes {
					sumTime += v
				}
				for _, v := range validRams {
					sumRam += v
				}

				avgTime = sumTime / 3.0
				avgRam = sumRam / 3.0
			} else if len(times) == 1 {
				avgTime = times[0]
				avgRam = peakRAMs[0]
			} else {
				var sumTime, sumRam float64
				for _, v := range times {
					sumTime += v
				}
				for _, v := range peakRAMs {
					sumRam += v
				}
				avgTime = sumTime / float64(len(times))
				avgRam = sumRam / float64(len(peakRAMs))
			}

			finalFile.WriteString(fmt.Sprintf("Q%d: [Avg Time: %.3fs] [Avg Peak RAM: %.2f MB] [Paths: %d]\n", i+1, avgTime, avgRam, paths))
		}
	}
	if fast {
		fmt.Println("Fast Benchmark complete.")
	} else {
		fmt.Println("Benchmark complete. Results saved.")
	}
}

func (e *Executor) restartServer(restartCmd string) error {
	fmt.Printf("Executing restart command: %s\n", restartCmd)
	cmd := exec.Command("sh", "-c", restartCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Warning: restart command returned error: %v, output: %s\n", err, string(output))
	}

	fmt.Println("Waiting for server to start (polling)...")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-ticker.C:
			if ok, err := e.State.Client.CheckStatus(); err == nil && ok {
				fmt.Println("Server is up and running.")
				if e.State.ActiveDB != "" {
					fmt.Printf("Restoring active database: %s...\n", e.State.ActiveDB)
					_, err := e.State.Client.UseDatabase(e.State.ActiveDB)
					if err != nil {
						fmt.Printf("Warning: failed to restore database: %v\n", err)
					} else {
						fmt.Println("Database restored successfully.")
					}
				}
				return nil
			}
		case <-timeout:
			return fmt.Errorf("timeout waiting for server to start (60s)")
		}
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

	// List of Column Families
	type CFInfo struct {
		Name   string
		Handle *grocksdb.ColumnFamilyHandle
	}
	cfs := []CFInfo{
		{"Nodes", dbStore.CFNodes},
		{"Edges", dbStore.CFEdges},
		{"Node Index (Prop)", dbStore.CFIdxNodeProp},
		{"Edge Index (Prop)", dbStore.CFIdxEdgeProp},
		{"Edge Index (Src)", dbStore.CFIdxEdgeSrc},
		{"Edge Index (Dst)", dbStore.CFIdxEdgeDst},
		{"Label Index", dbStore.CFIdxLabel},
	}

	fmt.Println("\n==========================================================================")
	fmt.Println("   ROCKSDB PHYSICAL STORAGE STATISTICS (LOCAL)")
	fmt.Println("==========================================================================")

	// Table header
	fmt.Printf("%-20s | %-12s | %-12s | %-12s\n", "Column Family", "Est. Keys", "MemTable Size", "Index/Filter Mem")
	fmt.Println("--------------------------------------------------------------------------")

	var totalKeys int64
	var totalMemTable int64
	var totalIndexMem int64

	for _, cf := range cfs {
		keys := getIntProperty(dbStore, "rocksdb.estimate-num-keys", cf.Handle)
		memTable := getIntProperty(dbStore, "rocksdb.cur-size-all-mem-tables", cf.Handle)
		indexMem := getIntProperty(dbStore, "rocksdb.estimate-table-readers-mem", cf.Handle)

		totalKeys += keys
		totalMemTable += memTable
		totalIndexMem += indexMem

		fmt.Printf("%-20s | %-12s | %-12s | %-12s\n",
			cf.Name,
			formatWithCommas(keys),
			formatBytes(memTable),
			formatBytes(indexMem),
		)
	}
	fmt.Println("--------------------------------------------------------------------------")
	fmt.Printf("%-20s | %-12s | %-12s | %-12s\n",
		"TOTAL",
		formatWithCommas(totalKeys),
		formatBytes(totalMemTable),
		formatBytes(totalIndexMem),
	)
	fmt.Println("==========================================================================")

	// Detailed Level stats for main CFs
	fmt.Println("\n[ Detailed Level Stats for Nodes & Edges ]")
	fmt.Println("--------------------------------------------------------------------------")
	fmt.Println("--- Nodes (CFNodes) Stats ---")
	fmt.Println(strings.TrimSpace(dbStore.GetProperty("rocksdb.levelstats", dbStore.CFNodes)))
	fmt.Println("\n--- Edges (CFEdges) Stats ---")
	fmt.Println(strings.TrimSpace(dbStore.GetProperty("rocksdb.levelstats", dbStore.CFEdges)))
	fmt.Println("==========================================================================")
}

func (e *Executor) describe() {
	if e.State.CurrentMode == session.ModeLocal && e.State.ActiveDB != "" {
		dbPath := filepath.Join(e.State.LocalRoot, e.State.ActiveDB)
		meta, err := metastore.Load(dbPath)
		if err != nil {
			fmt.Printf("Error loading metadata: %v\n", err)
			return
		}
		printMetadata(meta)
	} else if e.State.CurrentMode == session.ModeRemote && e.State.ActiveDB != "" {
		fmt.Printf("Fetching remote metadata for database '%s'...\n", e.State.ActiveDB)
		jsonStr, err := e.State.Client.DescribeDatabase(e.State.ActiveDB)
		if err != nil {
			fmt.Printf("Error describing remote database: %v\n", err)
			return
		}
		var meta metastore.MetaData
		if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
			fmt.Printf("Error parsing remote metadata: %v\n", err)
			return
		}
		printMetadata(&meta)
	} else {
		fmt.Println("Select a database first.")
	}
}

func getIntProperty(store *rocks.Store, prop string, cf *grocksdb.ColumnFamilyHandle) int64 {
	valStr := store.GetProperty(prop, cf)
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatWithCommas(num int64) string {
	str := strconv.FormatInt(num, 10)
	var result []byte
	length := len(str)
	for i, rune := range str {
		if i > 0 && (length-i)%3 == 0 && str[i-1] != '-' {
			result = append(result, ',')
		}
		result = append(result, byte(rune))
	}
	return string(result)
}

func printMetadata(meta *metastore.MetaData) {
	fmt.Println("\n==========================================================================")
	fmt.Println("   DATABASE SCHEMA & METADATA")
	fmt.Println("==========================================================================")
	fmt.Printf("General Info:\n")
	fmt.Printf("  Total Nodes : %s\n", formatWithCommas(meta.NodeCount))
	fmt.Printf("  Total Edges : %s\n", formatWithCommas(meta.EdgeCount))
	fmt.Println()

	fmt.Printf("Node Labels:\n")
	if len(meta.NodeCountByLabel) == 0 {
		fmt.Printf("  (No nodes)\n")
	} else {
		nodeLabels := make([]string, 0, len(meta.NodeCountByLabel))
		for label := range meta.NodeCountByLabel {
			nodeLabels = append(nodeLabels, label)
		}
		sort.Strings(nodeLabels)

		for _, label := range nodeLabels {
			count := meta.NodeCountByLabel[label]
			props := meta.NodeSchema[label]
			sort.Strings(props)

			propStr := "None"
			if len(props) > 0 {
				propStr = strings.Join(props, ", ")
			}
			fmt.Printf("  - %s: %s nodes\n", label, formatWithCommas(count))
			fmt.Printf("    Properties: %s\n", propStr)
		}
	}
	fmt.Println()

	fmt.Printf("Edge Labels & Connections:\n")
	if len(meta.EdgeCountByLabel) == 0 {
		fmt.Printf("  (No edges)\n")
	} else {
		edgeLabels := make([]string, 0, len(meta.EdgeCountByLabel))
		for label := range meta.EdgeCountByLabel {
			edgeLabels = append(edgeLabels, label)
		}
		sort.Strings(edgeLabels)

		for _, label := range edgeLabels {
			count := meta.EdgeCountByLabel[label]
			props := meta.EdgeSchema[label]
			sort.Strings(props)

			propStr := "None"
			if len(props) > 0 {
				propStr = strings.Join(props, ", ")
			}
			fmt.Printf("  - %s: %s edges\n", label, formatWithCommas(count))

			conns := meta.EdgeConnections[label]
			if len(conns) > 0 {
				fmt.Printf("    Connections:\n")
				sort.Slice(conns, func(i, j int) bool {
					if conns[i].SrcLabel != conns[j].SrcLabel {
						return conns[i].SrcLabel < conns[j].SrcLabel
					}
					return conns[i].DstLabel < conns[j].DstLabel
				})
				for _, conn := range conns {
					fmt.Printf("      • %s -> %s (%s)\n", conn.SrcLabel, conn.DstLabel, formatWithCommas(conn.Count))
				}
			}
			fmt.Printf("    Properties: %s\n", propStr)
		}
	}
	fmt.Println("==========================================================================")
}

func parseArgs(in string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	var quoteChar rune

	runes := []rune(in)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if inQuote {
			if r == quoteChar {
				inQuote = false
			} else if r == '\\' && i+1 < len(runes) {
				i++
				current.WriteRune(runes[i])
			} else {
				current.WriteRune(r)
			}
		} else {
			if r == ' ' || r == '\t' {
				if current.Len() > 0 {
					args = append(args, current.String())
					current.Reset()
				}
			} else if r == '"' || r == '\'' {
				inQuote = true
				quoteChar = r
			} else {
				current.WriteRune(r)
			}
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
