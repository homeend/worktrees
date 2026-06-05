package worktree

// WorktreeInfo describes a worktree for listing/resolution.
type WorktreeInfo struct {
	Path     string
	Branch   string // refs/heads/...; empty if detached/bare
	HEAD     string
	IsMain   bool
	Detached bool
}

// Template is a named branch-name template exposed to the CLI/TUI.
type Template struct {
	Name     string
	Template string
}

// AddOptions controls Add.
type AddOptions struct {
	Name           string // optional; generated if empty
	Branch         string // optional; defaults to "wt/"+Name
	BaseRef        string // optional; defaults to config BaseRef
	NoHooks        bool
	FromBranch     string // when set: check out this existing branch instead of cutting a new one
	NoPrefix       bool   // skip the configured branch prefix
	PrefixOverride string // override the configured prefix for this run (normalized; ignored if NoPrefix)
	// FromTemplate is set by the CLI when Name was produced by --template. In
	// derive mode (Add run from inside a worktree) it suppresses reinterpreting
	// Name as a literal suffix token, so a template-rendered name falls through
	// to the normal naming path instead of being appended to the parent branch.
	FromTemplate bool
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

// RemoveAllPlan is the read-only preview of a kill-em-all operation.
type RemoveAllPlan struct {
	Worktrees []WorktreeInfo // non-main, in-container
	Branches  []string       // prefix-matching short names
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
