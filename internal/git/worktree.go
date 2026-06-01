package git

// AddWorktree creates a new worktree at path on a new branch cut from base.
func (r *Runner) AddWorktree(dir, path, branch, base string) error {
	_, err := r.Run(dir, "worktree", "add", "-b", branch, path, base)
	return err
}

// AddWorktreeExisting checks out an existing branch into a new worktree at path
// (git worktree add <path> <branch>, no -b).
func (r *Runner) AddWorktreeExisting(dir, path, branch string) error {
	_, err := r.Run(dir, "worktree", "add", path, branch)
	return err
}

// ListWorktrees returns parsed worktree entries for the repo containing dir.
func (r *Runner) ListWorktrees(dir string) ([]WorktreeInfo, error) {
	out, err := r.Run(dir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parsePorcelainZ(out)
}

// RemoveWorktree removes the worktree at path. force maps to git's -f.
func (r *Runner) RemoveWorktree(dir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := r.Run(dir, args...)
	return err
}

// Prune clears stale worktree administrative entries.
func (r *Runner) Prune(dir string) error {
	_, err := r.Run(dir, "worktree", "prune")
	return err
}
