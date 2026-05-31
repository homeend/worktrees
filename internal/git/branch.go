package git

// BranchExists reports whether refs/heads/<branch> exists.
func (r *Runner) BranchExists(dir, branch string) bool {
	err := r.VerifyRef(dir, "refs/heads/"+branch)
	return err == nil
}

// DeleteBranch deletes a branch. With force=false it uses safe delete
// (`git branch -d`), which refuses unmerged branches — in that case it returns
// (false, nil) so callers can report the branch was kept. With force=true it
// uses `git branch -D` and returns (true, nil) on success.
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
