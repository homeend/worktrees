package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/code-drill/wt/pkg/worktree"
)

var killYes bool

// killer is the subset of *worktree.Manager kill-em-all needs.
type killer interface {
	PlanRemoveAll(dir string) (worktree.RemoveAllPlan, error)
	RemoveAll(dir string) (worktree.RemoveAllResult, error)
}

type killOpts struct {
	yes   bool
	isTTY bool
	in    io.Reader // confirmation input (defaults to os.Stdin in the command)
}

// runKillEmAll drives the destructive cleanup with a confirmation gate.
// Extracted from the cobra command for testing. It writes user-facing output to
// out and returns ErrPartialCleanup when any item failed.
func runKillEmAll(k killer, repoRoot string, opts killOpts, out io.Writer) error {
	plan, err := k.PlanRemoveAll(repoRoot)
	if err != nil {
		return err
	}
	if len(plan.Worktrees) == 0 && len(plan.Branches) == 0 {
		fmt.Fprintln(out, "nothing to remove")
		return nil
	}

	fmt.Fprintln(out, "note: lifecycle hooks are skipped for kill-em-all")
	fmt.Fprintf(out, "This will force-remove %d worktree(s) and delete %d branch(es):\n",
		len(plan.Worktrees), len(plan.Branches))
	for _, w := range plan.Worktrees {
		fmt.Fprintf(out, "  worktree %s\n", w.Path)
	}
	for _, b := range plan.Branches {
		fmt.Fprintf(out, "  branch   %s\n", b)
	}

	if !opts.yes {
		if !opts.isTTY {
			return fmt.Errorf("refusing to run without --yes (no TTY for confirmation)")
		}
		fmt.Fprint(out, "Remove everything? [y/N]: ")
		reader := bufio.NewReader(opts.in)
		line, _ := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			// proceed
		default:
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	res, err := k.RemoveAll(repoRoot)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Removed %d worktree(s), deleted %d branch(es) (%d failed)\n",
		res.WorktreesRemoved, res.BranchesDeleted, len(res.Failures))
	for _, f := range res.Failures {
		fmt.Fprintf(out, "  FAILED %s %s: %s\n", f.Kind, f.Ref, f.Err)
	}
	if len(res.Failures) > 0 {
		return fmt.Errorf("%w: %d item(s)", ErrPartialCleanup, len(res.Failures))
	}
	return nil
}

var killEmAllCmd = &cobra.Command{
	Use:   "kill-em-all",
	Short: "Remove ALL worktrees and prefixed branches (destructive)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}
		opts := killOpts{
			yes:   killYes,
			isTTY: term.IsTerminal(int(os.Stdout.Fd())),
			in:    os.Stdin,
		}
		return runKillEmAll(m, cwd, opts, os.Stdout)
	},
}

func init() {
	killEmAllCmd.Flags().BoolVar(&killYes, "yes", false, "skip the confirmation prompt")
	rootCmd.AddCommand(killEmAllCmd)
}
