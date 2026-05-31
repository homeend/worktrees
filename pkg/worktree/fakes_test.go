package worktree

import "fmt"

type fakeGit struct {
	mainRoot     string
	branches     map[string]bool
	worktrees    []GitWorktree
	addErr       error
	removeErr    error
	verifyRefErr error
	added        []string
	removedPaths []string
	deleted      []string
	deleteOK     bool
}

func newFakeGit(root string) *fakeGit {
	return &fakeGit{mainRoot: root, branches: map[string]bool{}, deleteOK: true}
}

func (f *fakeGit) MainRoot(string) (string, error) { return f.mainRoot, nil }
func (f *fakeGit) VerifyRef(_, _ string) error     { return f.verifyRefErr }
func (f *fakeGit) CheckRefFormat(string) error     { return nil }
func (f *fakeGit) BranchExists(_, b string) bool   { return f.branches[b] }
func (f *fakeGit) ListWorktrees(string) ([]GitWorktree, error) {
	return f.worktrees, nil
}
func (f *fakeGit) Prune(string) error { return nil }

func (f *fakeGit) AddWorktree(_, path, branch, _ string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, path)
	f.branches[branch] = true
	f.worktrees = append(f.worktrees, GitWorktree{Path: path, Branch: "refs/heads/" + branch})
	return nil
}

func (f *fakeGit) RemoveWorktree(_, path string, _ bool) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	f.removedPaths = append(f.removedPaths, path)
	return nil
}

func (f *fakeGit) DeleteBranch(_, branch string, force bool) (bool, error) {
	f.deleted = append(f.deleted, branch)
	if !force && !f.deleteOK {
		return false, nil
	}
	return true, nil
}

type fakeHooks struct {
	calls  []HookEvent
	failOn map[HookEvent]error
}

func newFakeHooks() *fakeHooks { return &fakeHooks{failOn: map[HookEvent]error{}} }

func (h *fakeHooks) Run(ctx HookContext) error {
	h.calls = append(h.calls, ctx.Event)
	if err, ok := h.failOn[ctx.Event]; ok {
		return fmt.Errorf("hook %s failed: %w", ctx.Event, err)
	}
	return nil
}

type fakeConfig struct {
	baseRef   string
	container string
}

func (c fakeConfig) BaseRef() string   { return c.baseRef }
func (c fakeConfig) Container() string { return c.container }
