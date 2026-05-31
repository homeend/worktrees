package cmd

import (
	"github.com/spf13/cobra"
)

// rootCmd is the base command. Subcommands are registered in init() funcs
// across the package. Bare invocation will later launch the TUI (Phase 7).
var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "Fast git worktree management with lifecycle hooks",
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		return 1
	}
	return 0
}
