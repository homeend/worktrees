package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/code-drill/wt/pkg/worktree"
)

// resolveWorktreePath returns the on-disk path for a named worktree.
func resolveWorktreePath(m *worktree.Manager, dir, name string) (string, error) {
	w, err := m.Find(dir, name)
	if err != nil {
		return "", err
	}
	return w.Path, nil
}

var pathCmd = &cobra.Command{
	Use:   "path <name>",
	Short: "Print the path of a worktree (for shell cd)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		m, err := buildManager(cwd)
		if err != nil {
			return err
		}
		p, err := resolveWorktreePath(m, cwd, args[0])
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	},
}

func init() { rootCmd.AddCommand(pathCmd) }
