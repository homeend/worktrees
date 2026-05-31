package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/code-drill/wt/pkg/worktree"
)

var (
	newBranch  string
	newBase    string
	newNoHooks bool
)

// worktreeAddOptions builds AddOptions from flag values (extracted for testing).
func worktreeAddOptions(name, branch, base string, noHooks bool) worktree.AddOptions {
	return worktree.AddOptions{Name: name, Branch: branch, BaseRef: base, NoHooks: noHooks}
}

var newCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new worktree",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		res, err := m.Add(cwd, worktreeAddOptions(name, newBranch, newBase, newNoHooks))
		if err != nil {
			return err
		}
		fmt.Printf("Created worktree %q\n  branch: %s\n  path:   %s\n", res.Name, res.Branch, res.Path)
		return nil
	},
}

func init() {
	newCmd.Flags().StringVarP(&newBranch, "branch", "b", "", "branch name (default: derived from name)")
	newCmd.Flags().StringVar(&newBase, "base", "", "base ref to branch from (default: config base_ref / HEAD)")
	newCmd.Flags().BoolVar(&newNoHooks, "no-hooks", false, "skip lifecycle hooks")
	rootCmd.AddCommand(newCmd)
}
