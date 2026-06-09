package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "shim-mcp %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
