package git

// BranchExists reports whether refs/heads/<branch> exists. It collapses any
// verification failure (including operational git errors) to false; callers
// that need to distinguish "absent" from "git failed" should not rely on it.
func (r *Runner) BranchExists(dir, branch string) bool {
	err := r.VerifyRef(dir, "refs/heads/"+branch)
	return err == nil
}

// DeleteBranch deletes a branch. With force=false it uses safe delete
// (`git branch -d`): any failure (typically an unmerged branch, but also e.g. a
// branch checked out elsewhere) is reported as (false, nil) so the caller can
// surface that the branch was kept rather than treating it as fatal. With
// force=true it uses `git branch -D` and returns (true, nil) on success or
// (false, err) on failure.
func (r *Runner) DeleteBranch(dir, branch string, force bool) (bool, error) {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.Run(dir, "branch", flag, branch)
	if err != nil {
		if !force {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
