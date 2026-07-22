package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/homeend/worktrees/internal/naming"
)

// Manager orchestrates worktree operations over injected collaborators.
type Manager struct {
	git   GitRunner
	hooks HookRunner
	cfg   ConfigProvider
}

// New constructs a Manager.
func New(g GitRunner, h HookRunner, c ConfigProvider) *Manager {
	return &Manager{git: g, hooks: h, cfg: c}
}

// containerPath returns the worktree container for a repo root. A configured
// container overrides the default sibling and is used verbatim.
func (m *Manager) containerPath(repoRoot string) string {
	if c := m.cfg.Container(); c != "" {
		return c
	}
	return repoRoot + ".worktrees"
}

// worktreePath returns the on-disk path for a branch within the container.
// The directory is the branch sanitized into a single flat segment (gg
// convention: '/' becomes '-'), never nested.
func (m *Manager) worktreePath(repoRoot, branch string) string {
	return filepath.Join(m.containerPath(repoRoot), naming.SanitizeSegment(branch))
}

// ParentBranch reports the branch of the managed (non-main) worktree that
// dir lives in, for template rendering (<parent-branch>). ok is false when
// dir is not inside a managed worktree.
func (m *Manager) ParentBranch(dir string) (string, bool) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return "", false
	}
	return m.currentWorktreeBranch(dir, repoRoot)
}

// currentWorktreeBranch detects whether dir lives inside a managed (non-main)
// worktree and returns that worktree's branch (refs/heads/ stripped). It
// selects the listed worktree whose Path is the longest path-separator-bounded
// prefix of dir, so a cwd that is a subdirectory of the worktree still
// resolves, and sibling worktrees sharing a leading string prefix (e.g.
// "feat" vs "feat-extra") never false-match. It returns ok=false when the
// matched entry is the main worktree (Path == repoRoot) or has no branch
// (detached/bare).
func (m *Manager) currentWorktreeBranch(dir, repoRoot string) (parentBranch string, ok bool) {
	// git emits worktree paths via --show-toplevel, which are always absolute
	// and symlink-resolved. dir may arrive non-canonical (a relative --repo, or
	// a symlinked os.Getwd() such as macOS /var vs /private/var), which would
	// make the prefix comparison below silently fail and drop out of derive
	// mode. Normalize to an absolute, symlink-resolved path first; each step is
	// guarded and falls back to the prior value on error.
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	worktrees, err := m.git.ListWorktrees(dir)
	if err != nil {
		return "", false
	}
	best := ""
	bestBranch := ""
	for _, w := range worktrees {
		if !hasPathPrefix(dir, w.Path) {
			continue
		}
		if len(w.Path) > len(best) {
			best = w.Path
			bestBranch = w.Branch
		}
	}
	if best == "" || pathsEqual(best, repoRoot) || bestBranch == "" {
		return "", false
	}
	return strings.TrimPrefix(bestBranch, "refs/heads/"), true
}

// nextFreeVersion returns the lowest free "<parentBranch>-vNNN" suffix (NNN
// zero-padded to width 3, starting at 1), skipping any candidate whose branch
// already exists. It fills gaps: with only -v002 present it returns -v001.
//
// The loop is intentionally unbounded in form but always terminates: it returns
// at the first free slot, and the number of iterations is bounded by the count
// of existing -vNNN siblings (with N existing, the worst case scans N+1 before
// hitting a free slot). It is not an infinite-loop risk.
//
// This relies on BranchExists treating operational git errors (lock
// contention, a corrupt ref) as "absent" rather than surfacing them. A
// transient git failure could therefore be misread as a free slot and yield a
// candidate that actually exists; the subsequent git worktree add would then
// fail with a less clear downstream error. Distinguishing "absent" from "git
// errored" in the GitRunner interface is out of scope for this change.
func (m *Manager) nextFreeVersion(repoRoot, parentBranch string) string {
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("%s-v%03d", parentBranch, n)
		if !m.git.BranchExists(repoRoot, candidate) {
			return candidate
		}
	}
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

	// Derive mode: when Add runs from inside a managed (non-main) worktree and
	// no rendered Branch was supplied, the new branch is derived from the
	// current worktree's branch. A rendered Branch (template output) wins
	// everywhere and never derives.
	deriveMode := false
	var parentBranch string
	if opts.Branch == "" {
		if pb, ok := m.currentWorktreeBranch(dir, repoRoot); ok {
			deriveMode = true
			parentBranch = pb
		}
	}

	var branch, baseRef string
	switch {
	case deriveMode:
		// Auto -vNNN when no name; otherwise append the literal name. The
		// leading dash is the separator: if the caller omits it (e.g. "fix"
		// rather than "-fix"), it is inserted so the name never glues onto the
		// parent branch (feature-login + "fix" must not yield
		// "feature-loginfix").
		if opts.Name == "" {
			branch = m.nextFreeVersion(repoRoot, parentBranch)
		} else {
			sep := opts.Name
			if !strings.HasPrefix(sep, "-") {
				sep = "-" + sep
			}
			branch = parentBranch + sep
		}
	case opts.Branch != "":
		branch = opts.Branch
	default:
		if opts.Name == "" {
			return AddResult{}, fmt.Errorf("branch name required: pass a name or --template (names are never generated)")
		}
		branch = opts.Name
	}

	if err := m.git.CheckRefFormat(branch); err != nil {
		return AddResult{}, fmt.Errorf("invalid branch name %q: %w", branch, err)
	}

	switch {
	case deriveMode:
		// The auto -vNNN path is collision-free by construction; only the
		// custom-name path can collide, and that is a hard error (no rename, no
		// auto-bump) with a message that names the derived branch.
		if opts.Name != "" && m.git.BranchExists(repoRoot, branch) {
			return AddResult{}, fmt.Errorf("derived branch %q already exists", branch)
		}
		// In derive mode the base is the parent branch's committed tip; the
		// configured base_ref is ignored by design.
		baseRef = parentBranch
	default:
		if m.git.BranchExists(repoRoot, branch) {
			return AddResult{}, fmt.Errorf("branch %q already exists", branch)
		}
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
		Name:       branch,
		Branch:     branch,
		BaseRef:    baseRef,
		Container:  container,
		RepoName:   filepath.Base(repoRoot),
	}

	pc := hc
	pc.Event = PreCreate
	pc.Cwd = repoRoot
	if err := m.hooks.Run(pc); err != nil {
		return AddResult{}, fmt.Errorf("pre-create hook failed (nothing created): %w", err)
	}

	if err := m.git.AddWorktree(repoRoot, target, branch, baseRef); err != nil {
		return AddResult{}, fmt.Errorf("git worktree add: %w", err)
	}

	poc := hc
	poc.Event = PostCreate
	poc.Cwd = target
	if err := m.hooks.Run(poc); err != nil {
		return AddResult{Name: branch, Branch: branch, Path: target, BaseRef: baseRef},
			fmt.Errorf("post-create hook failed (worktree left in place at %s): %w", target, err)
	}

	return AddResult{Name: branch, Branch: branch, Path: target, BaseRef: baseRef}, nil
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
		// pathsEqual/hasPathPrefix tolerate the separator and drive-letter-case
		// differences between git-emitted paths and filepath-built ones on
		// Windows; a byte-for-byte comparison would filter everything out there.
		isMain := pathsEqual(w.Path, repoRoot)
		inContainer := hasPathPrefix(w.Path, container)
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

// resolveWorktree maps a user-supplied name to a worktree. A name matches
// when it equals the branch, the container-relative directory (the sanitized
// branch), or the leaf directory name. It refuses the main worktree and
// errors on not-found.
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
	leaf := filepath.Base(filepath.FromSlash(name))
	for _, w := range list {
		rel := relUnder(w.Path, container)
		byBranch := w.Branch == "refs/heads/"+name
		byDir := rel == name || rel == naming.SanitizeSegment(name)
		byLeaf := filepath.Base(w.Path) == leaf
		if byBranch || byDir || byLeaf {
			if w.IsMain {
				return WorktreeInfo{}, fmt.Errorf("%q is the main worktree and cannot be removed", name)
			}
			return w, nil
		}
	}
	return WorktreeInfo{}, fmt.Errorf("no worktree matching %q", name)
}

// pruneEmptyParents removes now-empty parent directories of a removed worktree,
// walking up to (but not including) the container. New worktree dirs are flat,
// but worktrees created by older layouts may still be nested. The first
// non-empty parent (os.Remove fails) stops the walk.
func (m *Manager) pruneEmptyParents(container, worktreePath string) {
	for parent := filepath.Dir(worktreePath); !pathsEqual(parent, container) &&
		hasPathPrefix(parent, container); parent = filepath.Dir(parent) {
		if err := os.Remove(parent); err != nil {
			break
		}
	}
}

// EscapeCwd moves the process's working directory to the main repo root when
// it currently sits inside a managed (non-main) worktree. On Windows a
// directory that is any process's cwd cannot be deleted, so a wt process
// standing inside a worktree would block its own rm/kill-em-all; on POSIX it
// merely avoids finishing in a deleted directory. Best-effort: any failure
// leaves the cwd unchanged.
func (m *Manager) EscapeCwd(dir string) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	worktrees, err := m.git.ListWorktrees(dir)
	if err != nil {
		return
	}
	for _, w := range worktrees {
		if pathsEqual(w.Path, repoRoot) {
			continue
		}
		if hasPathPrefix(cwd, w.Path) {
			_ = os.Chdir(repoRoot)
			return
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

	// Leave the worktree before deleting it: the wt process's own cwd would
	// otherwise block the removal on Windows.
	m.EscapeCwd(dir)
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

// PlanRemoveAll returns the read-only preview of a kill-em-all run: every
// non-main worktree in the container and each one's branch. It performs no
// mutation. Branches with no container worktree are not swept (there is no
// branch prefix to identify them by anymore).
func (m *Manager) PlanRemoveAll(dir string) (RemoveAllPlan, error) {
	list, err := m.List(dir)
	if err != nil {
		return RemoveAllPlan{}, err
	}
	var plan RemoveAllPlan
	for _, w := range list {
		if w.IsMain {
			continue
		}
		plan.Worktrees = append(plan.Worktrees, w)
		if b := strings.TrimPrefix(w.Branch, "refs/heads/"); b != "" {
			plan.Branches = append(plan.Branches, b)
		}
	}
	return plan, nil
}

// RemoveAll force-removes every non-main container worktree and force-deletes
// their branches, skipping lifecycle hooks. It is best-effort: a failure on
// one item is recorded and execution continues. A non-nil error is returned
// only for a fatal setup failure (e.g. planning). A final `git worktree
// prune` clears stale admin entries.
func (m *Manager) RemoveAll(dir string) (RemoveAllResult, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return RemoveAllResult{}, err
	}
	plan, err := m.PlanRemoveAll(dir)
	if err != nil {
		return RemoveAllResult{}, err
	}

	// Leave whichever worktree the process stands in before deleting: the wt
	// process's own cwd would otherwise block the removal on Windows.
	m.EscapeCwd(dir)

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
