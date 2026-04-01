package main

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"

	"gs-cli/internal/cache"
	"gs-cli/internal/metastore"
	"gs-cli/internal/pipeline"
	"gs-cli/internal/rocks"
)

var (
	ingestDbPath    string
	ingestNodesFile string
	ingestEdgesFile string
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Massive ingestion of .pgdf files into RocksDB",
	RunE: func(cmd *cobra.Command, args []string) error {
		start := time.Now()

		if verboseCount >= 1 {
			log.Printf("INFO: Starting ingestion into database: %s\n", ingestDbPath)
			if ingestNodesFile != "" {
				log.Printf("INFO: Nodes file: %s\n", ingestNodesFile)
			}
			if ingestEdgesFile != "" {
				log.Printf("INFO: Edges file: %s\n", ingestEdgesFile)
			}
		}

		// Initialize DB
		dbStore, err := rocks.Open(ingestDbPath, false)
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
		if ingestNodesFile != "" {
			if verboseCount >= 1 {
				log.Println("INFO: Processing nodes...")
			}
			t0 := time.Now()
			if err := ingestor.IngestNodes(ingestNodesFile); err != nil {
				return fmt.Errorf("error processing nodes: %w", err)
			}
			if verboseCount >= 1 {
				log.Printf("INFO: Nodes processed in %s\n", time.Since(t0))
			}
		}

		// Process Edges
		if ingestEdgesFile != "" {
			if verboseCount >= 1 {
				log.Println("INFO: Processing edges...")
			}
			t0 := time.Now()
			if err := ingestor.IngestEdges(ingestEdgesFile); err != nil {
				return fmt.Errorf("error processing edges: %w", err)
			}
			if verboseCount >= 1 {
				log.Printf("INFO: Edges processed in %s\n", time.Since(t0))
			}
		}

		// Save Metadata
		if err := meta.Save(ingestDbPath); err != nil {
			return fmt.Errorf("error saving metadata: %w", err)
		}

		if verboseCount >= 1 {
			log.Printf("INFO: Ingestion completed successfully in %s\n", time.Since(start))
		} else {
			fmt.Println("Ingestion completed.")
		}

		return nil
	},
}

func init() {
	ingestCmd.Flags().StringVarP(&ingestDbPath, "db", "d", "", "DB output name")
	ingestCmd.Flags().StringVarP(&ingestNodesFile, "nodes", "n", "", "Nodes .pgdf file")
	ingestCmd.Flags().StringVarP(&ingestEdgesFile, "edges", "e", "", "Edge .pgdf file")
	ingestCmd.MarkFlagRequired("db")
	rootCmd.AddCommand(ingestCmd)
}
