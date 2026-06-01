package worktree

import "fmt"

type fakeGit struct {
	mainRoot        string
	branches        map[string]bool
	worktrees       []GitWorktree
	addErr          error
	removeErr       error
	verifyRefErr    error
	added           []string
	removedPaths    []string
	deleted         []string
	deleteOK        bool
	listBranches    []string
	removeWtErr     map[string]error // keyed by path
	deleteBranchErr map[string]error // keyed by branch
	pruned          bool
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
func (f *fakeGit) ListBranches(_, _ string) ([]string, error) { return f.listBranches, nil }
func (f *fakeGit) Prune(string) error                         { f.pruned = true; return nil }

func (f *fakeGit) AddWorktree(_, path, branch, _ string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, path)
	f.branches[branch] = true
	f.worktrees = append(f.worktrees, GitWorktree{Path: path, Branch: "refs/heads/" + branch})
	return nil
}

func (f *fakeGit) AddWorktreeExisting(_, path, branch string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, path)
	f.worktrees = append(f.worktrees, GitWorktree{Path: path, Branch: "refs/heads/" + branch})
	return nil
}

func (f *fakeGit) RemoveWorktree(_, path string, _ bool) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	if err := f.removeWtErr[path]; err != nil {
		return err
	}
	f.removedPaths = append(f.removedPaths, path)
	return nil
}

func (f *fakeGit) DeleteBranch(_, branch string, force bool) (bool, error) {
	if err := f.deleteBranchErr[branch]; err != nil {
		return false, err
	}
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
	baseRef      string
	container    string
	nameTemplate string
	branchPrefix string
	templates    []Template
}

func (c fakeConfig) BaseRef() string       { return c.baseRef }
func (c fakeConfig) Container() string     { return c.container }
func (c fakeConfig) NameTemplate() string  { return c.nameTemplate }
func (c fakeConfig) BranchPrefix() string  { return c.branchPrefix }
func (c fakeConfig) Templates() []Template { return c.templates }
