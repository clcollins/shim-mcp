package cli

import (
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "shim-mcp",
	Short: "Lightweight MCP server for authenticated HTTP request proxying",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
}

func Execute() error {
	return rootCmd.Execute()
}
