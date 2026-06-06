//go:build integration

package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/homeend/worktrees/internal/git"
)

type gitAdapter struct{ r *git.Runner }

func (a gitAdapter) MainRoot(d string) (string, error)      { return a.r.MainRoot(d) }
func (a gitAdapter) VerifyRef(d, ref string) error          { return a.r.VerifyRef(d, ref) }
func (a gitAdapter) CheckRefFormat(b string) error          { return a.r.CheckRefFormat(b) }
func (a gitAdapter) BranchExists(d, b string) bool          { return a.r.BranchExists(d, b) }
func (a gitAdapter) AddWorktree(d, p, b, base string) error { return a.r.AddWorktree(d, p, b, base) }
func (a gitAdapter) AddWorktreeExisting(d, p, b string) error {
	return a.r.AddWorktreeExisting(d, p, b)
}
func (a gitAdapter) RemoveWorktree(d, p string, f bool) error { return a.r.RemoveWorktree(d, p, f) }
func (a gitAdapter) DeleteBranch(d, b string, f bool) (bool, error) {
	return a.r.DeleteBranch(d, b, f)
}
func (a gitAdapter) Prune(d string) error                       { return a.r.Prune(d) }
func (a gitAdapter) ListBranches(d, p string) ([]string, error) { return a.r.ListBranches(d, p) }
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

func (staticCfg) BaseRef() string       { return "HEAD" }
func (staticCfg) Container() string     { return "" }
func (staticCfg) NameTemplate() string  { return "" }
func (staticCfg) BranchPrefix() string  { return "wt/" }
func (staticCfg) Templates() []Template { return nil }

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

// gitOut runs git in dir and returns trimmed stdout, failing the test on error.
func gitOut(t *testing.T, dir string, a ...string) string {
	t.Helper()
	c := exec.Command("git", a...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", a, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestManager_DeriveFromWorktree_RealGit exercises all five ROADMAP Phase 1
// success criteria end-to-end against real git: an initial managed worktree is
// created from the repo root, then Add is invoked from INSIDE that worktree.
func TestManager_DeriveFromWorktree_Integration_RealGit(t *testing.T) {
	repo := newRealRepo(t)
	m := New(gitAdapter{git.New()}, noopHooks{}, staticCfg{})
	container := repo + ".worktrees"

	// Seed: a managed worktree on branch wt/feature-login, with a commit of its
	// own so the parent tip differs from main.
	parent, err := m.Add(repo, AddOptions{Name: "feature-login", NoHooks: true})
	if err != nil {
		t.Fatalf("seed Add: %v", err)
	}
	if parent.Branch != "wt/feature-login" {
		t.Fatalf("seed branch = %q, want wt/feature-login", parent.Branch)
	}
	wtDir := parent.Path
	os.WriteFile(filepath.Join(wtDir, "g"), []byte("y"), 0o644)
	gitOut(t, wtDir, "add", ".")
	gitOut(t, wtDir, "commit", "-m", "work")
	parentTip := gitOut(t, wtDir, "rev-parse", "wt/feature-login")

	// (1) empty token from inside the worktree → <branch>-v001, cut from the
	//     parent branch's committed tip, placed under the main container.
	v1, err := m.Add(wtDir, AddOptions{NoHooks: true})
	if err != nil {
		t.Fatalf("derive v001 Add: %v", err)
	}
	if v1.Branch != "wt/feature-login-v001" {
		t.Errorf("derive branch = %q, want wt/feature-login-v001", v1.Branch)
	}
	wantV1Path := filepath.Join(container, "wt", "feature-login-v001")
	if v1.Path != wantV1Path {
		t.Errorf("derive path = %q, want %q", v1.Path, wantV1Path)
	}
	if _, err := os.Stat(wantV1Path); err != nil {
		t.Errorf("derived worktree dir not created: %v", err)
	}
	// DERIVE-01: the new branch's start commit equals the parent branch tip.
	v1Tip := gitOut(t, repo, "rev-parse", "wt/feature-login-v001")
	if v1Tip != parentTip {
		t.Errorf("derived branch tip = %s, want parent tip %s", v1Tip, parentTip)
	}

	// (2) repeated Add picks the lowest free -vNNN, skipping existing and filling
	//     a manually-created gap. v001 exists; create v003 by hand → next is v002,
	//     then the following Add is v004.
	gitOut(t, repo, "branch", "wt/feature-login-v003")
	v2, err := m.Add(wtDir, AddOptions{NoHooks: true})
	if err != nil {
		t.Fatalf("derive gap Add: %v", err)
	}
	if v2.Branch != "wt/feature-login-v002" {
		t.Errorf("gap-fill branch = %q, want wt/feature-login-v002", v2.Branch)
	}
	v3, err := m.Add(wtDir, AddOptions{NoHooks: true})
	if err != nil {
		t.Fatalf("derive after-gap Add: %v", err)
	}
	if v3.Branch != "wt/feature-login-v004" {
		t.Errorf("after-gap branch = %q, want wt/feature-login-v004 (skips v001..v003)", v3.Branch)
	}

	// (3) custom token → <branch>-patch01; a second Add with the same token is a
	//     hard error and creates no second worktree.
	c1, err := m.Add(wtDir, AddOptions{Name: "-patch01", NoHooks: true})
	if err != nil {
		t.Fatalf("derive custom token Add: %v", err)
	}
	if c1.Branch != "wt/feature-login-patch01" {
		t.Errorf("custom branch = %q, want wt/feature-login-patch01", c1.Branch)
	}
	beforeList, _ := m.List(repo)
	if _, err := m.Add(wtDir, AddOptions{Name: "-patch01", NoHooks: true}); err == nil {
		t.Error("second custom-token Add should fail with a collision error")
	}
	afterList, _ := m.List(repo)
	if len(afterList) != len(beforeList) {
		t.Errorf("collision should not create a worktree: before=%d after=%d", len(beforeList), len(afterList))
	}

	// (4) --no-prefix / PrefixOverride in derive mode leave the inherited prefix
	//     intact (branch still carries wt/feature-login).
	p4, err := m.Add(wtDir, AddOptions{NoPrefix: true, PrefixOverride: "x/", BaseRef: "main", NoHooks: true})
	if err != nil {
		t.Fatalf("derive prefix-override Add: %v", err)
	}
	if !strings.HasPrefix(p4.Branch, "wt/feature-login-v") {
		t.Errorf("derive branch = %q, want inherited wt/feature-login prefix verbatim", p4.Branch)
	}
	p4Tip := gitOut(t, repo, "rev-parse", p4.Branch)
	if p4Tip != parentTip {
		t.Errorf("derive base ignored override: tip = %s, want parent tip %s", p4Tip, parentTip)
	}

	// (5) Add from the main repo root is unchanged: branches off base_ref/HEAD,
	//     no -vNNN suffix, placed under the container by name.
	mainTip := gitOut(t, repo, "rev-parse", "HEAD")
	root, err := m.Add(repo, AddOptions{Name: "from-root", NoHooks: true})
	if err != nil {
		t.Fatalf("root Add: %v", err)
	}
	if root.Branch != "wt/from-root" {
		t.Errorf("root branch = %q, want wt/from-root (no derive)", root.Branch)
	}
	if strings.Contains(root.Branch, "-v0") {
		t.Errorf("root branch should carry no -vNNN suffix: %q", root.Branch)
	}
	rootTip := gitOut(t, repo, "rev-parse", "wt/from-root")
	if rootTip != mainTip {
		t.Errorf("root branch tip = %s, want HEAD %s (not the worktree branch)", rootTip, mainTip)
	}
}

func TestManager_NestedLayoutAndPrune_RealGit(t *testing.T) {
	repo := newRealRepo(t)
	m := New(gitAdapter{git.New()}, noopHooks{}, staticCfg{})

	res, err := m.Add(repo, AddOptions{Name: "autofix/MTRH-2132", NoHooks: true})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	container := repo + ".worktrees"
	wantPath := filepath.Join(container, "wt", "autofix", "MTRH-2132")
	if res.Path != wantPath {
		t.Fatalf("path = %q, want %q (mirror full branch)", res.Path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("nested worktree dir not created: %v", err)
	}

	if _, err := m.Remove(repo, RemoveOptions{Name: "autofix/MTRH-2132", NoHooks: true}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(container, "wt")); !os.IsNotExist(err) {
		t.Errorf("empty parent dirs should be pruned up to the container")
	}
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
