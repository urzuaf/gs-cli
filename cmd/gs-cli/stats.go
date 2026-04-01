package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"gs-cli/internal/metastore"
	"gs-cli/internal/rocks"
)

var statsDbPath string

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Inspection tools for the database",
}

var graphStatsCmd = &cobra.Command{
	Use:   "graph",
	Short: "Show graph-level metadata (counts, labels, schema)",
	RunE: func(cmd *cobra.Command, args []string) error {
		meta, err := metastore.Load(statsDbPath)
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

var dbStatsCmd = &cobra.Command{
	Use:   "db",
	Short: "Show RocksDB internal engine statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		dbStore, err := rocks.Open(statsDbPath, true)
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

func init() {
	statsCmd.PersistentFlags().StringVarP(&statsDbPath, "db", "d", "", "DB path")
	statsCmd.MarkPersistentFlagRequired("db")
	statsCmd.AddCommand(graphStatsCmd, dbStatsCmd)
	rootCmd.AddCommand(statsCmd)
}
