package worktree

// WorktreeInfo describes a worktree for listing/resolution.
type WorktreeInfo struct {
	Path     string
	Branch   string // refs/heads/...; empty if detached/bare
	HEAD     string
	IsMain   bool
	Detached bool
}

// Template is a named branch-name template exposed to the CLI/TUI (gg
// <token> syntax, rendered by internal/naming).
type Template struct {
	Name     string
	Template string
}

// AddOptions controls Add. Branch and Name are resolved by the CLI layer:
// a template renders into Branch; a plain positional argument arrives as
// Name. With neither set, Add derives from the current worktree's branch
// when run inside one, and errors at the repo root.
type AddOptions struct {
	// Name is the raw user-supplied name. In derive mode it becomes the
	// -<name> suffix on the parent branch; at the repo root it is the branch
	// name itself.
	Name string
	// Branch is a fully rendered branch name used verbatim (template output).
	// When set it wins everywhere, including inside a worktree.
	Branch string
}

// AddResult reports the outcome of Add. Name equals the created branch.
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

// RemoveAllPlan is the read-only preview of a kill-em-all operation.
type RemoveAllPlan struct {
	Worktrees []WorktreeInfo // non-main, in-container
	Branches  []string       // short branch names of those worktrees
}

// CleanupFailure records a single non-fatal failure during RemoveAll.
type CleanupFailure struct {
	Kind string // "worktree" | "branch" | "prune"
	Ref  string // path or branch name
	Err  string
}

// RemoveAllResult summarizes a kill-em-all execution (best-effort).
type RemoveAllResult struct {
	WorktreesRemoved int
	BranchesDeleted  int
	Failures         []CleanupFailure
}
