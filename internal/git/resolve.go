package git

import (
	"path/filepath"
	"strings"
)

// TopLevel returns the working-tree root for dir (the current worktree).
func (r *Runner) TopLevel(dir string) (string, error) {
	out, err := r.Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// CommonDir returns the absolute git common directory (shared by all linked
// worktrees of a repo). Machine-local state like <seq> counters lives there.
func (r *Runner) CommonDir(dir string) (string, error) {
	out, err := r.Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(string(out))), nil
}

// MainRoot returns the MAIN working tree root, even when dir is inside a linked
// worktree. It derives the path from --git-common-dir and returns that
// directory's parent.
func (r *Runner) MainRoot(dir string) (string, error) {
	commonDir, err := r.CommonDir(dir)
	if err != nil {
		return "", err
	}
	return filepath.Dir(commonDir), nil
}

// VerifyRef returns nil if ref resolves to a commit.
func (r *Runner) VerifyRef(dir, ref string) error {
	_, err := r.Run(dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err
}

// CheckRefFormat validates a branch name as a legal git ref.
func (r *Runner) CheckRefFormat(branch string) error {
	_, err := r.Run("", "check-ref-format", "--branch", branch)
	return err
}
