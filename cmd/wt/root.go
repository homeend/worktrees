package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/homeend/worktrees/internal/config"
	"github.com/homeend/worktrees/internal/git"
	"github.com/homeend/worktrees/internal/hooks"
	"github.com/homeend/worktrees/internal/tui"
	"github.com/homeend/worktrees/pkg/worktree"
)

var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "Fast git worktree management with lifecycle hooks",
	Long:  rootLongFor(runtime.GOOS),
}

// rootLongFor returns the root help text for the given GOOS. The shell
// integration guidance differs: POSIX shells install a function via
// shell-init (that command only exists on POSIX builds), while on Windows
// the wt.cmd wrapper next to wt.bin.exe covers cd-on-Enter.
func rootLongFor(goos string) string {
	base := `Fast git worktree management with lifecycle hooks.

Run bare wt in a terminal to open the interactive TUI. Pressing Enter on a
worktree quits and prints its path — and cds your shell into it once the
shell integration is installed:

`
	if goos == "windows" {
		return base + `  add the built bin\ directory to PATH: typing wt then runs the wt.cmd
  wrapper, which launches the wt.bin.exe next to it and performs the cd.`
	}
	return base + `  wt shell-init zsh --install   (or: bash; then restart the shell)

See "wt shell-init --help" for details.`
}

var repoFlag string

func init() {
	rootCmd.PersistentFlags().StringVarP(&repoFlag, "repo", "r", "", "source repo (default: current dir)")
	rootCmd.PersistentFlags().StringVar(&cdFileFlag, "cd-file", "",
		"write the worktree path selected in the TUI to this file (for shell cd wrappers)")

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		if !shouldLaunchTUI(isTTY) {
			return cmd.Help()
		}
		m, cfg, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}
		// Resolved up front: cwd may no longer exist after a kill-em-all/rm
		// performed from inside a worktree.
		repoRoot, rootErr := repoRootFor(cwd)
		sel, err := tui.Run(m, cwd, templateSlice(cfg))
		if err != nil {
			return err
		}
		if sel == "" && rootErr == nil {
			if _, statErr := os.Stat(cwd); statErr != nil {
				// The directory the TUI was started in was removed: transport
				// the shell to the repo root instead of a dead directory.
				sel = repoRoot
			}
		}
		return emitSelection(sel)
	}
}

// shouldLaunchTUI reports whether the bare command should open the TUI.
func shouldLaunchTUI(isTTY bool) bool { return isTTY }

// workdir returns the directory wt should operate from: the --repo flag if set,
// otherwise the current working directory. The result is absolute, so it stays
// valid after the process escapes a worktree cwd (EscapeCwd) mid-operation.
func workdir() (string, error) {
	dir := repoFlag
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = cwd
	}
	return filepath.Abs(dir)
}

// managerForWorkdir resolves the workdir and builds a Manager plus the
// resolved config for it.
func managerForWorkdir() (*worktree.Manager, config.Config, string, error) {
	cwd, err := workdir()
	if err != nil {
		return nil, config.Config{}, "", err
	}
	m, cfg, err := buildManager(cwd)
	if err != nil {
		return nil, config.Config{}, "", err
	}
	return m, cfg, cwd, nil
}

// templateSlice returns the configured named templates sorted by name (for
// stable CLI/TUI listings).
func templateSlice(cfg config.Config) []worktree.Template {
	names := make([]string, 0, len(cfg.Templates))
	for n := range cfg.Templates {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]worktree.Template, len(names))
	for i, n := range names {
		out[i] = worktree.Template{Name: n, Template: cfg.Templates[n]}
	}
	return out
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

func (a gitAdapter) MainRoot(d string) (string, error)      { return a.r.MainRoot(d) }
func (a gitAdapter) VerifyRef(d, ref string) error          { return a.r.VerifyRef(d, ref) }
func (a gitAdapter) CheckRefFormat(b string) error          { return a.r.CheckRefFormat(b) }
func (a gitAdapter) BranchExists(d, b string) bool          { return a.r.BranchExists(d, b) }
func (a gitAdapter) AddWorktree(d, p, b, base string) error { return a.r.AddWorktree(d, p, b, base) }
func (a gitAdapter) RemoveWorktree(d, p string, f bool) error {
	return a.r.RemoveWorktree(d, p, f)
}
func (a gitAdapter) DeleteBranch(d, b string, f bool) (bool, error) {
	return a.r.DeleteBranch(d, b, f)
}
func (a gitAdapter) Prune(d string) error { return a.r.Prune(d) }
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
func buildManager(cwd string) (*worktree.Manager, config.Config, error) {
	r := git.New()
	repoRoot, err := repoRootFor(cwd)
	if err != nil {
		return nil, config.Config{}, err
	}
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return nil, config.Config{}, err
	}
	m := worktree.New(gitAdapter{r}, hooks.New(repoRoot), cfgAdapter{cfg})
	return m, cfg, nil
}
