package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"gs-cli/internal/server"
)

var (
	serverPort        int
	serverDB          string
	serverShowResults bool
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Interact with a running PathDB server",
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the PathDB server is alive",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := server.NewClient(serverPort)
		ok, err := c.CheckStatus()
		if err != nil || !ok {
			fmt.Printf("PathDB server on port %d is NOT running or unreachable.\n", serverPort)
			return nil
		}
		fmt.Printf("PathDB server on port %d is UP and running.\n", serverPort)
		return nil
	},
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available databases on the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := server.NewClient(serverPort)
		dbs, err := c.ListDatabases()
		if err != nil {
			return err
		}
		fmt.Printf("Available databases on server (port %d):\n", serverPort)
		for _, db := range dbs {
			fmt.Printf("  - %s\n", db)
		}
		return nil
	},
}

var serverUseCmd = &cobra.Command{
	Use:   "use",
	Short: "Switch the active database on the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if serverDB == "" {
			return fmt.Errorf("database name (-d) is required")
		}
		c := server.NewClient(serverPort)
		msg, err := c.UseDatabase(serverDB)
		if err != nil {
			return err
		}
		fmt.Println(msg)
		return nil
	},
}

var serverQueryCmd = &cobra.Command{
	Use:   "query [query_string]",
	Short: "Execute a query on the server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if serverDB == "" {
			return fmt.Errorf("database name (-d) is required")
		}
		query := args[0]
		c := server.NewClient(serverPort)
		result, err := c.Query(serverDB, query)
		if err != nil {
			return err
		}

		fmt.Printf("Query results for DB '%s' (Time: %s, Total Paths: %d):\n",
			serverDB, result.Metadata.Time, result.Metadata.TotalPaths)

		if serverShowResults {
			for i, path := range result.Data {
				fmt.Printf("Path %d:\n", i+1)
				for _, step := range path.Content {
					fmt.Printf("  %s\n", step)
				}
			}
		} else {
			fmt.Println("Use -r to see detailed path results.")
		}
		return nil
	},
}

func init() {
	serverCmd.PersistentFlags().IntVarP(&serverPort, "port", "p", 8080, "PathDB server port")
	serverCmd.PersistentFlags().StringVarP(&serverDB, "db", "d", "", "Database name")

	serverQueryCmd.Flags().BoolVarP(&serverShowResults, "results", "r", false, "Show detailed query results (paths)")

	serverCmd.AddCommand(serverStatusCmd, serverListCmd, serverUseCmd, serverQueryCmd)
	rootCmd.AddCommand(serverCmd)
}
