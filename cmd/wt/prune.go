package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/internal/git"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune stale worktree administrative state",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := workdir()
		if err != nil {
			return err
		}
		r := git.New()
		if err := r.EnsureMinVersion(2, 30); err != nil {
			return err
		}
		root, err := r.MainRoot(cwd)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrNotARepo, err)
		}
		if err := r.Prune(root); err != nil {
			return err
		}
		fmt.Println("Pruned stale worktree entries.")
		return nil
	},
}

func init() { rootCmd.AddCommand(pruneCmd) }
