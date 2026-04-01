package main

import (
	"github.com/spf13/cobra"
)

var verboseCount int

var rootCmd = &cobra.Command{
	Use:          "gs-cli",
	Short:        "GraphStorage CLI - RocksDB graph databases manager",
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().CountVarP(&verboseCount, "verbose", "v", "Verbose output (use -vvv for debug)")
}
