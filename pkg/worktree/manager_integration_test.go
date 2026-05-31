//go:build integration

package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/code-drill/wt/internal/git"
)

type gitAdapter struct{ r *git.Runner }

func (a gitAdapter) MainRoot(d string) (string, error)        { return a.r.MainRoot(d) }
func (a gitAdapter) VerifyRef(d, ref string) error            { return a.r.VerifyRef(d, ref) }
func (a gitAdapter) CheckRefFormat(b string) error            { return a.r.CheckRefFormat(b) }
func (a gitAdapter) BranchExists(d, b string) bool            { return a.r.BranchExists(d, b) }
func (a gitAdapter) AddWorktree(d, p, b, base string) error   { return a.r.AddWorktree(d, p, b, base) }
func (a gitAdapter) RemoveWorktree(d, p string, f bool) error { return a.r.RemoveWorktree(d, p, f) }
func (a gitAdapter) DeleteBranch(d, b string, f bool) (bool, error) {
	return a.r.DeleteBranch(d, b, f)
}
func (a gitAdapter) Prune(d string) error { return a.r.Prune(d) }
func (a gitAdapter) ListWorktrees(d string) ([]GitWorktree, error) {
	ws, err := a.r.ListWorktrees(d)
	if err != nil {
		return nil, err
	}
	out := make([]GitWorktree, len(ws))
	for i, w := range ws {
		out[i] = GitWorktree{Path: w.Path, Branch: w.Branch, HEAD: w.HEAD, Bare: w.Bare, Detached: w.Detached}
	}
	return out, nil
}

type noopHooks struct{}

func (noopHooks) Run(HookContext) error { return nil }

type staticCfg struct{}

func (staticCfg) BaseRef() string      { return "HEAD" }
func (staticCfg) Container() string    { return "" }
func (staticCfg) NameTemplate() string { return "" }

func newRealRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(a ...string) {
		c := exec.Command("git", a...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", a, err, out)
		}
	}
	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644)
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func TestManager_AddListRemove_RealGit(t *testing.T) {
	repo := newRealRepo(t)
	m := New(gitAdapter{git.New()}, noopHooks{}, staticCfg{})

	res, err := m.Add(repo, AddOptions{Name: "feat", NoHooks: true})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}

	list, err := m.List(repo)
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %v, err=%v", list, err)
	}

	rmRes, err := m.Remove(repo, RemoveOptions{Name: "feat", NoHooks: true})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rmRes.BranchDeleted {
		t.Errorf("merged branch should be deleted: %+v", rmRes)
	}
	if _, err := os.Stat(res.Path); !os.IsNotExist(err) {
		t.Errorf("worktree dir should be gone")
	}
}
