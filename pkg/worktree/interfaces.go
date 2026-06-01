package worktree

// GitRunner is the git operations the Manager needs. Implemented by internal/git.
type GitRunner interface {
	MainRoot(dir string) (string, error)
	VerifyRef(dir, ref string) error
	CheckRefFormat(branch string) error
	BranchExists(dir, branch string) bool
	AddWorktree(dir, path, branch, base string) error
	AddWorktreeExisting(dir, path, branch string) error
	ListWorktrees(dir string) ([]GitWorktree, error)
	RemoveWorktree(dir, path string, force bool) error
	DeleteBranch(dir, branch string, force bool) (bool, error)
	ListBranches(dir, prefix string) ([]string, error)
	Prune(dir string) error
}

// GitWorktree is the subset of git worktree data the Manager consumes.
type GitWorktree struct {
	Path     string
	Branch   string
	HEAD     string
	Bare     bool
	Detached bool
}

// HookEvent identifies a lifecycle hook.
type HookEvent string

const (
	PreCreate  HookEvent = "pre-create"
	PostCreate HookEvent = "post-create"
	PreRemove  HookEvent = "pre-remove"
	PostRemove HookEvent = "post-remove"
)

// HookContext is passed to the HookRunner; it becomes WT_* env vars.
type HookContext struct {
	Event      HookEvent
	SourceRoot string
	TargetRoot string
	Name       string
	Branch     string
	BaseRef    string
	Container  string
	RepoName   string
	Cwd        string // working directory the hook runs in
}

// HookRunner discovers and runs a hook. Returns nil if the hook is absent.
// A non-nil error means the hook ran and failed (non-zero exit).
type HookRunner interface {
	Run(ctx HookContext) error
}

// ConfigProvider supplies resolved configuration for a repo.
type ConfigProvider interface {
	BaseRef() string
	Container() string    // "" => default sibling container
	NameTemplate() string // "" => default generated name pattern
	BranchPrefix() string // "" => caller falls back to default
	Templates() []Template
}
