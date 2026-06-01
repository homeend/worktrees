package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
// When no name is given, it is generated — honoring a configured name_template
// if set (an invalid template is reported as an error).
func (m *Manager) resolveNames(opts AddOptions) (name, branch string, err error) {
	name = opts.Name
	if name == "" {
		name, err = naming.GenerateFrom(m.cfg.NameTemplate(), m.now(), m.digits())
		if err != nil {
			return "", "", err
		}
	}
	base := opts.Branch
	if base == "" {
		base = name
	}
	prefix := m.effectivePrefix(opts)
	branch = prefix + strings.TrimPrefix(base, prefix)
	return name, branch, nil
}

// effectivePrefix resolves the branch prefix for this Add: --no-prefix wins and
// yields none; otherwise a per-run override replaces the configured prefix.
func (m *Manager) effectivePrefix(opts AddOptions) string {
	if opts.NoPrefix {
		return ""
	}
	if opts.PrefixOverride != "" {
		return opts.PrefixOverride
	}
	return m.cfg.BranchPrefix()
}

// worktreePath returns the on-disk path for a branch within the container. The
// directory mirrors the full branch ref (slashes become nested subdirectories,
// prefix included).
func (m *Manager) worktreePath(repoRoot, branch string) string {
	return filepath.Join(m.containerPath(repoRoot), filepath.FromSlash(branch))
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

	fromExisting := opts.FromBranch != ""
	var name, branch, baseRef string
	if fromExisting {
		branch = opts.FromBranch
		name = strings.TrimPrefix(branch, m.cfg.BranchPrefix())
	} else {
		name, branch, err = m.resolveNames(opts)
		if err != nil {
			return AddResult{}, err
		}
	}

	if err := m.git.CheckRefFormat(branch); err != nil {
		return AddResult{}, fmt.Errorf("invalid branch name %q: %w", branch, err)
	}

	if fromExisting {
		if !m.git.BranchExists(repoRoot, branch) {
			return AddResult{}, fmt.Errorf("branch %q does not exist locally", branch)
		}
	} else {
		if m.git.BranchExists(repoRoot, branch) {
			return AddResult{}, fmt.Errorf("branch %q already exists; pass a different --branch", branch)
		}
		baseRef = opts.BaseRef
		if baseRef == "" {
			baseRef = m.cfg.BaseRef()
		}
		if err := m.git.VerifyRef(repoRoot, baseRef); err != nil {
			return AddResult{}, fmt.Errorf("base ref %q not found: %w", baseRef, err)
		}
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

	if fromExisting {
		if err := m.git.AddWorktreeExisting(repoRoot, target, branch); err != nil {
			return AddResult{}, fmt.Errorf("git worktree add: %w", err)
		}
	} else {
		if err := m.git.AddWorktree(repoRoot, target, branch, baseRef); err != nil {
			return AddResult{}, fmt.Errorf("git worktree add: %w", err)
		}
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

// List returns the worktrees wt manages: the main working tree (flagged IsMain)
// plus any worktree living inside this repo's container. Linked worktrees git
// knows about but that live elsewhere are omitted, since wt only manages the
// container.
func (m *Manager) List(dir string) ([]WorktreeInfo, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return nil, err
	}
	raw, err := m.git.ListWorktrees(dir)
	if err != nil {
		return nil, err
	}
	container := m.containerPath(repoRoot)
	out := make([]WorktreeInfo, 0, len(raw))
	for _, w := range raw {
		isMain := w.Path == repoRoot
		inContainer := w.Path == container ||
			strings.HasPrefix(w.Path, container+string(filepath.Separator))
		if !isMain && !inContainer {
			continue
		}
		out = append(out, WorktreeInfo{
			Path:     w.Path,
			Branch:   w.Branch,
			HEAD:     w.HEAD,
			Detached: w.Detached,
			IsMain:   isMain,
		})
	}
	return out, nil
}

// resolveWorktree maps a user-supplied name to a worktree. Since the worktree
// directory now mirrors the full branch (nested, prefixed), a name matches when
// it equals the branch (with or without prefix), the container-relative path,
// or the leaf directory name. It refuses the main worktree and errors on
// not-found.
func (m *Manager) resolveWorktree(dir, name string) (WorktreeInfo, error) {
	list, err := m.List(dir)
	if err != nil {
		return WorktreeInfo{}, err
	}
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return WorktreeInfo{}, err
	}
	container := m.containerPath(repoRoot)
	prefix := m.cfg.BranchPrefix()
	nameNoPrefix := strings.TrimPrefix(name, prefix)
	wantBranch := "refs/heads/" + prefix + nameNoPrefix
	leaf := filepath.Base(filepath.FromSlash(name))
	for _, w := range list {
		rel := filepath.ToSlash(strings.TrimPrefix(w.Path, container+string(filepath.Separator)))
		byBranch := w.Branch == wantBranch || w.Branch == "refs/heads/"+name
		byPath := rel == name || rel == prefix+nameNoPrefix || rel == nameNoPrefix
		byLeaf := filepath.Base(w.Path) == leaf
		if byBranch || byPath || byLeaf {
			if w.IsMain {
				return WorktreeInfo{}, fmt.Errorf("%q is the main worktree and cannot be removed", name)
			}
			return w, nil
		}
	}
	return WorktreeInfo{}, fmt.Errorf("no worktree matching %q", name)
}

// pruneEmptyParents removes now-empty parent directories of a removed worktree,
// walking up to (but not including) the container. The first non-empty parent
// (os.Remove fails) stops the walk.
func (m *Manager) pruneEmptyParents(container, worktreePath string) {
	for parent := filepath.Dir(worktreePath); parent != container &&
		strings.HasPrefix(parent, container+string(filepath.Separator)); parent = filepath.Dir(parent) {
		if err := os.Remove(parent); err != nil {
			break
		}
	}
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
	m.pruneEmptyParents(m.containerPath(repoRoot), w.Path)

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

// Templates returns the configured templates (for `wt templates` / the TUI).
func (m *Manager) Templates() []Template { return m.cfg.Templates() }

// ResolveTemplate finds a template by name or 1-based number and renders it with
// vars. The rendered string is intended to be used as AddOptions.Name (the
// prefix is applied by the normal Add flow). Unknown ref or a missing variable
// is an error.
func (m *Manager) ResolveTemplate(ref string, vars map[string]string) (string, error) {
	tmpls := m.cfg.Templates()
	tmpl := ""
	found := false
	if n, err := strconv.Atoi(ref); err == nil {
		if n >= 1 && n <= len(tmpls) {
			tmpl = tmpls[n-1].Template
			found = true
		}
	} else {
		for _, t := range tmpls {
			if t.Name == ref {
				tmpl = t.Template
				found = true
				break
			}
		}
	}
	if !found {
		return "", fmt.Errorf("unknown template %q", ref)
	}
	return naming.RenderTemplate(tmpl, vars)
}

// PlanRemoveAll returns the read-only preview of a kill-em-all run: every
// non-main worktree in the container and every branch matching the configured
// prefix (including orphans with no worktree). It performs no mutation.
func (m *Manager) PlanRemoveAll(dir string) (RemoveAllPlan, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return RemoveAllPlan{}, err
	}
	list, err := m.List(dir)
	if err != nil {
		return RemoveAllPlan{}, err
	}
	var plan RemoveAllPlan
	for _, w := range list {
		if !w.IsMain {
			plan.Worktrees = append(plan.Worktrees, w)
		}
	}
	branches, err := m.git.ListBranches(repoRoot, m.cfg.BranchPrefix())
	if err != nil {
		return RemoveAllPlan{}, err
	}
	plan.Branches = branches
	return plan, nil
}

// RemoveAll force-removes every non-main container worktree and force-deletes
// every prefix-matching branch (orphans included), skipping lifecycle hooks. It
// is best-effort: a failure on one item is recorded and execution continues. A
// non-nil error is returned only for a fatal setup failure (e.g. planning). A
// final `git worktree prune` clears stale admin entries.
func (m *Manager) RemoveAll(dir string) (RemoveAllResult, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return RemoveAllResult{}, err
	}
	plan, err := m.PlanRemoveAll(dir)
	if err != nil {
		return RemoveAllResult{}, err
	}

	var res RemoveAllResult
	for _, w := range plan.Worktrees {
		if err := m.git.RemoveWorktree(repoRoot, w.Path, true); err != nil {
			res.Failures = append(res.Failures, CleanupFailure{Kind: "worktree", Ref: w.Path, Err: err.Error()})
			continue
		}
		res.WorktreesRemoved++
	}
	for _, b := range plan.Branches {
		if _, err := m.git.DeleteBranch(repoRoot, b, true); err != nil {
			res.Failures = append(res.Failures, CleanupFailure{Kind: "branch", Ref: b, Err: err.Error()})
			continue
		}
		res.BranchesDeleted++
	}
	if err := m.git.Prune(repoRoot); err != nil {
		res.Failures = append(res.Failures, CleanupFailure{Kind: "prune", Ref: repoRoot, Err: err.Error()})
	}
	return res, nil
}
