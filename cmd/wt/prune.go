package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/code-drill/wt/internal/git"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune stale worktree administrative state",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		r := git.New()
		if err := r.Prune(cwd); err != nil {
			return err
		}
		fmt.Println("Pruned stale worktree entries.")
		return nil
	},
}

func init() { rootCmd.AddCommand(pruneCmd) }
