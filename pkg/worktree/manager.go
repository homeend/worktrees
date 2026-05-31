package worktree

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/code-drill/wt/internal/naming"
)

// Manager orchestrates worktree operations over injected collaborators.
type Manager struct {
	git    GitRunner
	hooks  HookRunner
	cfg    ConfigProvider
	now    func() time.Time
	digits func() int
}

// New constructs a Manager with default time/random sources.
func New(g GitRunner, h HookRunner, c ConfigProvider) *Manager {
	return &Manager{
		git:    g,
		hooks:  h,
		cfg:    c,
		now:    time.Now,
		digits: defaultDigits,
	}
}

// SetDigits overrides the digit source used for generated names (e.g. random in
// production). Intended for wiring and tests.
func (m *Manager) SetDigits(fn func() int) { m.digits = fn }

// containerPath returns the worktree container for a repo root. A configured
// container overrides the default sibling and is used verbatim.
func (m *Manager) containerPath(repoRoot string) string {
	if c := m.cfg.Container(); c != "" {
		return c
	}
	return repoRoot + ".worktrees"
}

// resolveNames computes (name, branch). name omits the wt/ prefix; branch always
// carries it. An explicit Branch overrides the derived one (still prefixed).
func (m *Manager) resolveNames(opts AddOptions) (name, branch string) {
	name = opts.Name
	if name == "" {
		name = naming.Generate(m.now(), m.digits())
	}
	base := opts.Branch
	if base == "" {
		base = name
	}
	branch = "wt/" + strings.TrimPrefix(base, "wt/")
	return name, branch
}

// worktreePath returns the on-disk path for a branch within the container.
func (m *Manager) worktreePath(repoRoot, branch string) string {
	return filepath.Join(m.containerPath(repoRoot), naming.SanitizeDir(branch))
}

func defaultDigits() int {
	return 1
}

// Add creates a new worktree following the create transaction:
// resolve+validate -> pre-create hook -> git worktree add -> post-create hook.
// A pre-create failure aborts before anything is created. A post-create failure
// returns an error but leaves the worktree in place (no rollback, by design).
func (m *Manager) Add(dir string, opts AddOptions) (AddResult, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return AddResult{}, fmt.Errorf("resolve repo root: %w", err)
	}

	name, branch := m.resolveNames(opts)
	if err := m.git.CheckRefFormat(branch); err != nil {
		return AddResult{}, fmt.Errorf("invalid branch name %q: %w", branch, err)
	}
	if m.git.BranchExists(repoRoot, branch) {
		return AddResult{}, fmt.Errorf("branch %q already exists; pass a different --branch", branch)
	}

	baseRef := opts.BaseRef
	if baseRef == "" {
		baseRef = m.cfg.BaseRef()
	}
	if err := m.git.VerifyRef(repoRoot, baseRef); err != nil {
		return AddResult{}, fmt.Errorf("base ref %q not found: %w", baseRef, err)
	}

	container := m.containerPath(repoRoot)
	target := m.worktreePath(repoRoot, branch)

	hc := HookContext{
		SourceRoot: repoRoot,
		TargetRoot: target,
		Name:       name,
		Branch:     branch,
		BaseRef:    baseRef,
		Container:  container,
		RepoName:   filepath.Base(repoRoot),
	}

	if !opts.NoHooks {
		pc := hc
		pc.Event = PreCreate
		pc.Cwd = repoRoot
		if err := m.hooks.Run(pc); err != nil {
			return AddResult{}, fmt.Errorf("pre-create hook failed (nothing created): %w", err)
		}
	}

	if err := m.git.AddWorktree(repoRoot, target, branch, baseRef); err != nil {
		return AddResult{}, fmt.Errorf("git worktree add: %w", err)
	}

	if !opts.NoHooks {
		poc := hc
		poc.Event = PostCreate
		poc.Cwd = target
		if err := m.hooks.Run(poc); err != nil {
			return AddResult{Name: name, Branch: branch, Path: target, BaseRef: baseRef},
				fmt.Errorf("post-create hook failed (worktree left in place at %s): %w", target, err)
		}
	}

	return AddResult{Name: name, Branch: branch, Path: target, BaseRef: baseRef}, nil
}
