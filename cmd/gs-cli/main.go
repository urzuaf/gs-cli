package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gs-cli/internal/cache"
	"gs-cli/internal/metastore"
	"gs-cli/internal/pipeline"
	"gs-cli/internal/rocks"
)

func main() {
	var (
		dbPath       string
		nodesFile    string
		edgesFile    string
		verboseCount int
	)

	log.SetFlags(0)

	var ingestCmd = &cobra.Command{
		Use:   "ingest",
		Short: "Massive ingestion of .pgdf files into RocksDB",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()

			if verboseCount >= 1 {
				log.Printf("INFO: Starting ingestion into database: %s\n", dbPath)
				if nodesFile != "" {
					log.Printf("INFO: Nodes file: %s\n", nodesFile)
				}
				if edgesFile != "" {
					log.Printf("INFO: Edges file: %s\n", edgesFile)
				}
			}

			// Initialize DB
			dbStore, err := rocks.Open(dbPath, false)
			if err != nil {
				return fmt.Errorf("error opening RocksDB: %w", err)
			}
			defer dbStore.Close()

			// Initialize Cache and Metastore
			meta := metastore.NewMetaStore()
			nodeCache := cache.NewNodeCache()
			defer nodeCache.Clear()

			ingestor := pipeline.NewIngestor(dbStore, meta, nodeCache, verboseCount)

			// Process Nodes
			if nodesFile != "" {
				if verboseCount >= 1 {
					log.Println("INFO: Processing nodes...")
				}
				t0 := time.Now()
				if err := ingestor.IngestNodes(nodesFile); err != nil {
					return fmt.Errorf("error processing nodes: %w", err)
				}
				if verboseCount >= 1 {
					log.Printf("INFO: Nodes processed in %s\n", time.Since(t0))
				}
			}

			// Process Edges
			if edgesFile != "" {
				if verboseCount >= 1 {
					log.Println("INFO: Processing edges...")
				}
				t0 := time.Now()
				if err := ingestor.IngestEdges(edgesFile); err != nil {
					return fmt.Errorf("error processing edges: %w", err)
				}
				if verboseCount >= 1 {
					log.Printf("INFO: Edges processed in %s\n", time.Since(t0))
				}
			}

			// Save Metadata
			if err := meta.Save(dbPath); err != nil {
				return fmt.Errorf("error saving metadata: %w", err)
			}

			if verboseCount >= 1 {
				log.Printf("INFO: Ingestion completed successfully in %s\n", time.Since(start))
			} else {
				// Clean output for scripts
				fmt.Println("Ingestion completed.")
			}

			return nil
		},
	}

	ingestCmd.Flags().StringVarP(&dbPath, "db", "d", "", "DB output name")
	ingestCmd.Flags().StringVarP(&nodesFile, "nodes", "n", "", "Nodes .pgdf file")
	ingestCmd.Flags().StringVarP(&edgesFile, "edges", "e", "", "Edge .pgdf file")
	ingestCmd.Flags().CountVarP(&verboseCount, "verbose", "v", "Verbose output (use -vvv for debug)")
	ingestCmd.MarkFlagRequired("db")

	// Stats Parent Command
	var statsCmd = &cobra.Command{
		Use:   "stats",
		Short: "Inspection tools for the database",
	}

	// Subcommand: Graph Metadata
	var graphStatsCmd = &cobra.Command{
		Use:   "graph",
		Short: "Show graph-level metadata (counts, labels, schema)",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := metastore.Load(dbPath)
			if err != nil {
				return fmt.Errorf("could not load metadata.json: %w", err)
			}

			fmt.Println("\n┌────────────────────────────────────────────────────────┐")
			fmt.Println("│                GRAPH METADATA SUMMARY                  │")
			fmt.Println("└────────────────────────────────────────────────────────┘")
			fmt.Printf("  Total Nodes : %d\n", meta.NodeCount)
			fmt.Printf("  Total Edges : %d\n", meta.EdgeCount)

			fmt.Println("\n[ NODE TYPES & SCHEMA ]")
			for label, count := range meta.NodeCountByLabel {
				props := strings.Join(meta.NodeSchema[label], ", ")
				fmt.Printf("  • %-15s [%d nodes]\n", label, count)
				fmt.Printf("    Props: %s\n", props)
			}

			fmt.Println("\n[ EDGE TYPES & SCHEMA ]")
			for label, count := range meta.EdgeCountByLabel {
				props := strings.Join(meta.EdgeSchema[label], ", ")
				fmt.Printf("  • %-15s [%d edges]\n", label, count)
				fmt.Printf("    Props: %s\n", props)
			}

			fmt.Println("\n[ TOPOLOGY (Label Connections) ]")
			for edgeLabel, conns := range meta.EdgeConnections {
				fmt.Printf("  • %s:\n", edgeLabel)
				for _, c := range conns {
					fmt.Printf("    %-15s -> %-15s [%d]\n", c.SrcLabel, c.DstLabel, c.Count)
				}
			}
			fmt.Println("──────────────────────────────────────────────────────────")
			return nil
		},
	}

	// Subcommand: DB Internal Stats
	var dbStatsCmd = &cobra.Command{
		Use:   "db",
		Short: "Show RocksDB internal engine statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbStore, err := rocks.Open(dbPath, true)
			if err != nil {
				return fmt.Errorf("error opening RocksDB for stats: %w", err)
			}
			defer dbStore.Close()

			fmt.Println("\n[ LSM TREE STRUCTURE ]")
			fmt.Println("CF Nodes Stats:")
			fmt.Println(dbStore.GetProperty("rocksdb.levelstats", dbStore.CFNodes))

			fmt.Println("\n[ RESOURCE USAGE ]")
			fmt.Printf("  MemTable Total   : %s\n", dbStore.GetProperty("rocksdb.cur-size-all-mem-tables", nil))
			fmt.Printf("  Table Readers    : %s\n", dbStore.GetProperty("rocksdb.estimate-table-readers-mem", nil))
			fmt.Printf("  Block Cache      : %s\n", dbStore.GetProperty("rocksdb.block-cache-usage", nil))

			fmt.Println("\n[ ENGINE HEALTH ]")
			fmt.Printf("  Pending Compactions : %s\n", dbStore.GetProperty("rocksdb.compaction-pending", nil))
			fmt.Printf("  Num Live Versions   : %s\n", dbStore.GetProperty("rocksdb.num-live-versions", nil))
			fmt.Printf("  Num Immutable Mem   : %s\n", dbStore.GetProperty("rocksdb.num-immutable-mem-table", nil))

			return nil
		},
	}

	statsCmd.PersistentFlags().StringVarP(&dbPath, "db", "d", "", "DB path")
	statsCmd.MarkPersistentFlagRequired("db")
	statsCmd.AddCommand(graphStatsCmd, dbStatsCmd)

	var rootCmd = &cobra.Command{
		Use:          "gs-cli",
		Short:        "GraphStorage CLI - RocksDB graph databases manager",
		SilenceUsage: true,
	}

	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(statsCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
