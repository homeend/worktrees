package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/code-drill/wt/pkg/worktree"
)

var (
	rmForce       bool
	rmForceBranch bool
	rmKeepBranch  bool
	rmNoHooks     bool
)

func worktreeRemoveOptions(name string, force, forceBranch, keepBranch, noHooks bool) worktree.RemoveOptions {
	return worktree.RemoveOptions{
		Name: name, Force: force, ForceBranch: forceBranch,
		KeepBranch: keepBranch, NoHooks: noHooks,
	}
}

var rmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a worktree and its branch",
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
		res, err := m.Remove(cwd, worktreeRemoveOptions(args[0], rmForce, rmForceBranch, rmKeepBranch, rmNoHooks))
		if err != nil {
			return err
		}
		fmt.Printf("Removed worktree %q (%s)\n", res.Name, res.Path)
		switch {
		case res.BranchDeleted:
			fmt.Printf("Deleted branch %s\n", res.Branch)
		case res.BranchKept:
			fmt.Printf("Kept branch %s (unmerged). Delete with: wt rm %s --force-branch, or git branch -D %s\n",
				res.Branch, res.Name, res.Branch)
		}
		return nil
	},
}

func init() {
	rmCmd.Flags().BoolVar(&rmForce, "force", false, "force-remove a dirty worktree")
	rmCmd.Flags().BoolVarP(&rmForceBranch, "force-branch", "D", false, "force-delete an unmerged branch")
	rmCmd.Flags().BoolVar(&rmKeepBranch, "keep-branch", false, "do not delete the branch")
	rmCmd.Flags().BoolVar(&rmNoHooks, "no-hooks", false, "skip lifecycle hooks")
	rootCmd.AddCommand(rmCmd)
}
