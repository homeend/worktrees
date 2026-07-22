package cmd

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"runtime"

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
		m, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}
		sel, err := tui.Run(m, cwd)
		if err != nil {
			return err
		}
		return emitSelection(sel)
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

func (a gitAdapter) MainRoot(d string) (string, error)      { return a.r.MainRoot(d) }
func (a gitAdapter) VerifyRef(d, ref string) error          { return a.r.VerifyRef(d, ref) }
func (a gitAdapter) CheckRefFormat(b string) error          { return a.r.CheckRefFormat(b) }
func (a gitAdapter) BranchExists(d, b string) bool          { return a.r.BranchExists(d, b) }
func (a gitAdapter) AddWorktree(d, p, b, base string) error { return a.r.AddWorktree(d, p, b, base) }
func (a gitAdapter) AddWorktreeExisting(d, p, b string) error {
	return a.r.AddWorktreeExisting(d, p, b)
}
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
func (a cfgAdapter) Templates() []worktree.Template {
	out := make([]worktree.Template, len(a.c.Templates))
	for i, t := range a.c.Templates {
		out[i] = worktree.Template{Name: t.Name, Template: t.Template}
	}
	return out
}

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
