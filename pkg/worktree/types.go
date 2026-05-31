package worktree

// WorktreeInfo describes a worktree for listing/resolution.
type WorktreeInfo struct {
	Path     string
	Branch   string // refs/heads/...; empty if detached/bare
	HEAD     string
	IsMain   bool
	Detached bool
}

// AddOptions controls Add.
type AddOptions struct {
	Name    string // optional; generated if empty
	Branch  string // optional; defaults to "wt/"+Name
	BaseRef string // optional; defaults to config BaseRef
	NoHooks bool
}

// AddResult reports the outcome of Add.
type AddResult struct {
	Name    string
	Branch  string
	Path    string
	BaseRef string
}

// RemoveOptions controls Remove.
type RemoveOptions struct {
	Name        string
	Force       bool // force-remove dirty worktree
	ForceBranch bool // force-delete unmerged branch
	KeepBranch  bool // do not delete the branch
	NoHooks     bool
}

// RemoveResult reports the outcome of Remove.
type RemoveResult struct {
	Name          string
	Branch        string
	Path          string
	BranchDeleted bool
	BranchKept    bool // true if safe-delete refused (unmerged)
}
