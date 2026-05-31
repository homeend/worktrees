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
// On a post-create failure the returned AddResult is populated (the worktree
// exists); on earlier failures it is the zero value.
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

// List returns worktrees for the repo containing dir. The main working tree is
// flagged IsMain.
func (m *Manager) List(dir string) ([]WorktreeInfo, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return nil, err
	}
	raw, err := m.git.ListWorktrees(dir)
	if err != nil {
		return nil, err
	}
	out := make([]WorktreeInfo, 0, len(raw))
	for _, w := range raw {
		out = append(out, WorktreeInfo{
			Path:     w.Path,
			Branch:   w.Branch,
			HEAD:     w.HEAD,
			Detached: w.Detached,
			IsMain:   w.Path == repoRoot,
		})
	}
	return out, nil
}

// resolveWorktree maps a user-supplied name to a worktree, matching by
// directory basename first, then branch (with/without wt/ prefix). It refuses
// the main worktree and errors on not-found.
func (m *Manager) resolveWorktree(dir, name string) (WorktreeInfo, error) {
	list, err := m.List(dir)
	if err != nil {
		return WorktreeInfo{}, err
	}
	wantBranch := "refs/heads/wt/" + strings.TrimPrefix(name, "wt/")
	for _, w := range list {
		byDir := filepath.Base(w.Path) == naming.SanitizeDir(name)
		byBranch := w.Branch == wantBranch || w.Branch == "refs/heads/"+name
		if byDir || byBranch {
			if w.IsMain {
				return WorktreeInfo{}, fmt.Errorf("%q is the main worktree and cannot be removed", name)
			}
			return w, nil
		}
	}
	return WorktreeInfo{}, fmt.Errorf("no worktree matching %q", name)
}

// Remove tears down a worktree: pre-remove hook -> git worktree remove ->
// branch delete (safe unless ForceBranch) -> post-remove hook. A safe-delete
// refusal (unmerged branch) is not fatal: the worktree is still removed and the
// result reports BranchKept so the CLI can tell the user. When an error is
// returned mid-transaction the RemoveResult is partially populated, reflecting
// the steps that completed.
func (m *Manager) Remove(dir string, opts RemoveOptions) (RemoveResult, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return RemoveResult{}, err
	}
	w, err := m.resolveWorktree(dir, opts.Name)
	if err != nil {
		return RemoveResult{}, err
	}
	branch := strings.TrimPrefix(w.Branch, "refs/heads/")
	res := RemoveResult{Name: opts.Name, Branch: branch, Path: w.Path}

	hc := HookContext{
		SourceRoot: repoRoot,
		TargetRoot: w.Path,
		Name:       opts.Name,
		Branch:     branch,
		Container:  m.containerPath(repoRoot),
		RepoName:   filepath.Base(repoRoot),
	}

	if !opts.NoHooks {
		pr := hc
		pr.Event = PreRemove
		pr.Cwd = w.Path
		if err := m.hooks.Run(pr); err != nil {
			return res, fmt.Errorf("pre-remove hook failed (nothing removed): %w", err)
		}
	}

	if err := m.git.RemoveWorktree(repoRoot, w.Path, opts.Force); err != nil {
		return res, fmt.Errorf("git worktree remove: %w", err)
	}

	if !opts.KeepBranch && branch != "" {
		deleted, err := m.git.DeleteBranch(repoRoot, branch, opts.ForceBranch)
		if err != nil {
			return res, fmt.Errorf("delete branch %q: %w", branch, err)
		}
		res.BranchDeleted = deleted
		res.BranchKept = !deleted
	}

	if !opts.NoHooks {
		por := hc
		por.Event = PostRemove
		por.Cwd = repoRoot
		if err := m.hooks.Run(por); err != nil {
			return res, fmt.Errorf("post-remove hook failed (worktree already removed): %w", err)
		}
	}

	return res, nil
}

// Find resolves a user-supplied name to a worktree (by dir basename or branch),
// refusing the main worktree. Exposed for callers like `wt path`.
func (m *Manager) Find(dir, name string) (WorktreeInfo, error) {
	return m.resolveWorktree(dir, name)
}
