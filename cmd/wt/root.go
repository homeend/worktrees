package cmd

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/code-drill/wt/internal/config"
	"github.com/code-drill/wt/internal/git"
	"github.com/code-drill/wt/internal/hooks"
	"github.com/code-drill/wt/internal/tui"
	"github.com/code-drill/wt/pkg/worktree"
)

var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "Fast git worktree management with lifecycle hooks",
}

var repoFlag string

func init() {
	rootCmd.PersistentFlags().StringVarP(&repoFlag, "repo", "r", "", "source repo (default: current dir)")

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		if !shouldLaunchTUI(isTTY) {
			return cmd.Help()
		}
		m, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}
		return tui.Run(m, cwd)
	}
}

// shouldLaunchTUI reports whether the bare command should open the TUI.
func shouldLaunchTUI(isTTY bool) bool { return isTTY }

// workdir returns the directory wt should operate from: the --repo flag if set,
// otherwise the current working directory.
func workdir() (string, error) {
	if repoFlag != "" {
		return repoFlag, nil
	}
	return os.Getwd()
}

// managerForWorkdir resolves the workdir and builds a Manager for it.
func managerForWorkdir() (*worktree.Manager, string, error) {
	cwd, err := workdir()
	if err != nil {
		return nil, "", err
	}
	m, err := buildManager(cwd)
	if err != nil {
		return nil, "", err
	}
	return m, cwd, nil
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		err = classify(err)
		fmt.Fprintln(os.Stderr, "error:", err)
		return exitCodeFor(err)
	}
	return 0
}

// gitAdapter bridges *git.Runner to worktree.GitRunner.
type gitAdapter struct{ r *git.Runner }

func (a gitAdapter) MainRoot(d string) (string, error)              { return a.r.MainRoot(d) }
func (a gitAdapter) VerifyRef(d, ref string) error                  { return a.r.VerifyRef(d, ref) }
func (a gitAdapter) CheckRefFormat(b string) error                  { return a.r.CheckRefFormat(b) }
func (a gitAdapter) BranchExists(d, b string) bool                  { return a.r.BranchExists(d, b) }
func (a gitAdapter) AddWorktree(d, p, b, base string) error         { return a.r.AddWorktree(d, p, b, base) }
func (a gitAdapter) RemoveWorktree(d, p string, f bool) error       { return a.r.RemoveWorktree(d, p, f) }
func (a gitAdapter) DeleteBranch(d, b string, f bool) (bool, error) { return a.r.DeleteBranch(d, b, f) }
func (a gitAdapter) ListBranches(d, p string) ([]string, error)     { return a.r.ListBranches(d, p) }
func (a gitAdapter) Prune(d string) error                           { return a.r.Prune(d) }
func (a gitAdapter) ListWorktrees(d string) ([]worktree.GitWorktree, error) {
	ws, err := a.r.ListWorktrees(d)
	if err != nil {
		return nil, err
	}
	out := make([]worktree.GitWorktree, len(ws))
	for i, w := range ws {
		out[i] = worktree.GitWorktree{Path: w.Path, Branch: w.Branch, HEAD: w.HEAD, Bare: w.Bare, Detached: w.Detached}
	}
	return out, nil
}

// cfgAdapter adapts config.Config to worktree.ConfigProvider.
type cfgAdapter struct{ c config.Config }

func (a cfgAdapter) BaseRef() string      { return a.c.BaseRef }
func (a cfgAdapter) Container() string    { return a.c.Container }
func (a cfgAdapter) NameTemplate() string { return a.c.NameTemplate }
func (a cfgAdapter) BranchPrefix() string { return a.c.BranchPrefix }

// repoRootFor resolves the main repo root for cwd, after checking the git
// version. Shared by commands that need the repo root without a full Manager.
func repoRootFor(cwd string) (string, error) {
	r := git.New()
	if err := r.EnsureMinVersion(2, 30); err != nil {
		return "", err
	}
	root, err := r.MainRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotARepo, err)
	}
	return root, nil
}

// buildManager resolves the repo root and wires a Manager. cwd is where wt runs.
func buildManager(cwd string) (*worktree.Manager, error) {
	r := git.New()
	repoRoot, err := repoRootFor(cwd)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return nil, err
	}
	m := worktree.New(gitAdapter{r}, hooks.New(repoRoot), cfgAdapter{cfg})
	m.SetDigits(randomDigits)
	return m, nil
}

func randomDigits() int {
	var b [2]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return 1
	}
	return int(binary.BigEndian.Uint16(b[:]) % 10000)
}
