package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"

	"gs-cli/internal/cache"
	"gs-cli/internal/metastore"
	"gs-cli/internal/pipeline"
	"gs-cli/internal/rocks"
)

func main() {
	var (
		dbPath    string
		nodesFile string
		edgesFile string
		verbose   bool
	)

	log.SetFlags(0)

	var ingestCmd = &cobra.Command{
		Use:   "ingest",
		Short: "Ingesta masiva de archivos .pgdf en RocksDB",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()

			if verbose {
				log.Printf("INFO: Iniciando ingesta en base de datos: %s\n", dbPath)
				if nodesFile != "" {
					log.Printf("INFO: Archivo de nodos: %s\n", nodesFile)
				}
				if edgesFile != "" {
					log.Printf("INFO: Archivo de aristas: %s\n", edgesFile)
				}
			}

			// Inicializar BD
			dbStore, err := rocks.Open(dbPath)
			if err != nil {
				return fmt.Errorf("error al abrir RocksDB: %w", err)
			}
			defer dbStore.Close()

			// Inicializar Caché y Metastore
			meta := metastore.NewMetaStore()
			nodeCache := cache.NewNodeCache()
			defer nodeCache.Clear()

			ingestor := pipeline.NewIngestor(dbStore, meta, nodeCache)

			// Procesar Nodos
			if nodesFile != "" {
				if verbose {
					log.Println("INFO: Procesando nodos...")
				}
				t0 := time.Now()
				if err := ingestor.IngestNodes(nodesFile); err != nil {
					return fmt.Errorf("error procesando nodos: %w", err)
				}
				if verbose {
					log.Printf("INFO: Nodos procesados en %s\n", time.Since(t0))
				}
			}

			// Procesar Aristas
			if edgesFile != "" {
				if verbose {
					log.Println("INFO: Procesando aristas...")
				}
				t0 := time.Now()
				if err := ingestor.IngestEdges(edgesFile); err != nil {
					return fmt.Errorf("error procesando aristas: %w", err)
				}
				if verbose {
					log.Printf("INFO: Aristas procesadas en %s\n", time.Since(t0))
				}
			}

			// Guardar Metadatos
			if err := meta.Save(dbPath); err != nil {
				return fmt.Errorf("error guardando metadatos: %w", err)
			}

			if verbose {
				log.Printf("INFO: Ingesta completada exitosamente en %s\n", time.Since(start))
			} else {
				// Salida limpia para scripts
				fmt.Println("Ingesta completada.")
			}

			return nil
		},
	}

	ingestCmd.Flags().StringVarP(&dbPath, "db", "d", "", "DB output name")
	ingestCmd.Flags().StringVarP(&nodesFile, "nodes", "n", "", "Nodes .pgdf file")
	ingestCmd.Flags().StringVarP(&edgesFile, "edges", "e", "", "Edge .pgdf file")
	ingestCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	ingestCmd.MarkFlagRequired("db")

	var rootCmd = &cobra.Command{
		Use:          "gs-cli",
		Short:        "GraphStorage CLI - RocksDB graph databases manager",
		SilenceUsage: true,
	}

	rootCmd.AddCommand(ingestCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
