package hooks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/homeend/worktrees/pkg/worktree"
)

// Runner runs convention-dir hooks from <repoRoot>/.wt/.
type Runner struct {
	repoRoot string
}

// New returns a hook Runner rooted at repoRoot.
func New(repoRoot string) *Runner { return &Runner{repoRoot: repoRoot} }

// Run executes the hook for ctx.Event if it exists and is executable. An absent
// or non-executable hook is a silent no-op. A non-zero exit is returned as an
// error. Hook stdout/stderr stream to the process's stdout/stderr.
func (r *Runner) Run(ctx worktree.HookContext) error {
	path := filepath.Join(r.repoRoot, ".wt", string(ctx.Event))
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return nil // not executable -> skip
	}

	cmd := exec.Command(path)
	cmd.Dir = ctx.Cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env(ctx)...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %s exited with error: %w", ctx.Event, err)
	}
	return nil
}

func env(ctx worktree.HookContext) []string {
	return []string{
		"WT_SOURCE_ROOT=" + ctx.SourceRoot,
		"WT_TARGET_ROOT=" + ctx.TargetRoot,
		"WT_NAME=" + ctx.Name,
		"WT_BRANCH=" + ctx.Branch,
		"WT_BASE_REF=" + ctx.BaseRef,
		"WT_CONTAINER=" + ctx.Container,
		"WT_REPO_NAME=" + ctx.RepoName,
		"WT_HOOK=" + string(ctx.Event),
	}
}
