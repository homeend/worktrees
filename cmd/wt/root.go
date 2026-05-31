package cmd

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/code-drill/wt/internal/config"
	"github.com/code-drill/wt/internal/git"
	"github.com/code-drill/wt/internal/hooks"
	"github.com/code-drill/wt/pkg/worktree"
)

var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "Fast git worktree management with lifecycle hooks",
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
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
func (a gitAdapter) Prune(d string) error                          { return a.r.Prune(d) }
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

func (a cfgAdapter) BaseRef() string   { return a.c.BaseRef }
func (a cfgAdapter) Container() string { return a.c.Container }

// buildManager resolves the repo root and wires a Manager. cwd is where wt runs.
func buildManager(cwd string) (*worktree.Manager, error) {
	r := git.New()
	if err := r.EnsureMinVersion(2, 30); err != nil {
		return nil, err
	}
	repoRoot, err := r.MainRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotARepo, err)
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
