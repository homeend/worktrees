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
func (a gitAdapter) RemoveWorktree(d, p string, f bool) error {
	return a.r.RemoveWorktree(d, p, f)
}
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

func (staticCfg) BaseRef() string   { return "HEAD" }
func (staticCfg) Container() string { return "" }

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

// TestManager_DeriveFromWorktree_Integration_RealGit exercises derive mode
// end-to-end against real git: an initial worktree is created from the repo
// root, then Add is invoked from INSIDE that worktree.
func TestManager_DeriveFromWorktree_Integration_RealGit(t *testing.T) {
	repo := newRealRepo(t)
	m := New(gitAdapter{git.New()}, noopHooks{}, staticCfg{})
	container := repo + ".worktrees"

	// Seed: a managed worktree on branch feature-login, with a commit of its
	// own so the parent tip differs from main.
	parent, err := m.Add(repo, AddOptions{Name: "feature-login"})
	if err != nil {
		t.Fatalf("seed Add: %v", err)
	}
	if parent.Branch != "feature-login" {
		t.Fatalf("seed branch = %q, want feature-login (no prefix)", parent.Branch)
	}
	wtDir := parent.Path
	os.WriteFile(filepath.Join(wtDir, "g"), []byte("y"), 0o644)
	gitOut(t, wtDir, "add", ".")
	gitOut(t, wtDir, "commit", "-m", "work")
	parentTip := gitOut(t, wtDir, "rev-parse", "feature-login")

	// (1) bare Add from inside the worktree → <branch>-v001, cut from the
	//     parent branch's committed tip, placed flat under the main container.
	v1, err := m.Add(wtDir, AddOptions{})
	if err != nil {
		t.Fatalf("derive v001 Add: %v", err)
	}
	if v1.Branch != "feature-login-v001" {
		t.Errorf("derive branch = %q, want feature-login-v001", v1.Branch)
	}
	wantV1Path := filepath.Join(container, "feature-login-v001")
	if v1.Path != wantV1Path {
		t.Errorf("derive path = %q, want %q", v1.Path, wantV1Path)
	}
	if _, err := os.Stat(wantV1Path); err != nil {
		t.Errorf("derived worktree dir not created: %v", err)
	}
	v1Tip := gitOut(t, repo, "rev-parse", "feature-login-v001")
	if v1Tip != parentTip {
		t.Errorf("derived branch tip = %s, want parent tip %s", v1Tip, parentTip)
	}

	// (2) repeated Add picks the lowest free -vNNN, skipping existing and
	//     filling a manually-created gap.
	gitOut(t, repo, "branch", "feature-login-v003")
	v2, err := m.Add(wtDir, AddOptions{})
	if err != nil {
		t.Fatalf("derive gap Add: %v", err)
	}
	if v2.Branch != "feature-login-v002" {
		t.Errorf("gap-fill branch = %q, want feature-login-v002", v2.Branch)
	}
	v3, err := m.Add(wtDir, AddOptions{})
	if err != nil {
		t.Fatalf("derive after-gap Add: %v", err)
	}
	if v3.Branch != "feature-login-v004" {
		t.Errorf("after-gap branch = %q, want feature-login-v004 (skips v001..v003)", v3.Branch)
	}

	// (3) custom name → <branch>-patch01; a second Add with the same name is a
	//     hard error and creates no second worktree.
	c1, err := m.Add(wtDir, AddOptions{Name: "-patch01"})
	if err != nil {
		t.Fatalf("derive custom name Add: %v", err)
	}
	if c1.Branch != "feature-login-patch01" {
		t.Errorf("custom branch = %q, want feature-login-patch01", c1.Branch)
	}
	beforeList, _ := m.List(repo)
	if _, err := m.Add(wtDir, AddOptions{Name: "-patch01"}); err == nil {
		t.Error("second custom-name Add should fail with a collision error")
	}
	afterList, _ := m.List(repo)
	if len(afterList) != len(beforeList) {
		t.Errorf("collision should not create a worktree: before=%d after=%d", len(beforeList), len(afterList))
	}

	// (4) a rendered Branch wins over derive mode and branches off base_ref.
	mainTip := gitOut(t, repo, "rev-parse", "main")
	rb, err := m.Add(wtDir, AddOptions{Branch: "fix/GH-1"})
	if err != nil {
		t.Fatalf("rendered-branch Add: %v", err)
	}
	if rb.Branch != "fix/GH-1" {
		t.Errorf("rendered branch = %q, want fix/GH-1", rb.Branch)
	}
	if got := filepath.Base(rb.Path); got != "fix-GH-1" {
		t.Errorf("rendered branch dir = %q, want fix-GH-1 (flat sanitized)", got)
	}
	rbTip := gitOut(t, repo, "rev-parse", "fix/GH-1")
	if rbTip != mainTip {
		t.Errorf("rendered branch tip = %s, want HEAD %s (config base, not parent)", rbTip, mainTip)
	}

	// (5) Add from the main repo root: branch = name verbatim, no -vNNN.
	root, err := m.Add(repo, AddOptions{Name: "from-root"})
	if err != nil {
		t.Fatalf("root Add: %v", err)
	}
	if root.Branch != "from-root" {
		t.Errorf("root branch = %q, want from-root (no derive)", root.Branch)
	}
	rootTip := gitOut(t, repo, "rev-parse", "from-root")
	if rootTip != mainTip {
		t.Errorf("root branch tip = %s, want HEAD %s (not the worktree branch)", rootTip, mainTip)
	}
}

func TestManager_FlatLayout_RealGit(t *testing.T) {
	repo := newRealRepo(t)
	m := New(gitAdapter{git.New()}, noopHooks{}, staticCfg{})

	res, err := m.Add(repo, AddOptions{Name: "autofix/MTRH-2132"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	container := repo + ".worktrees"
	wantPath := filepath.Join(container, "autofix-MTRH-2132")
	if res.Path != wantPath {
		t.Fatalf("path = %q, want %q (flat sanitized segment)", res.Path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}

	if _, err := m.Remove(repo, RemoveOptions{Name: "autofix/MTRH-2132", NoHooks: true}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Errorf("worktree dir should be gone")
	}
}

func TestManager_AddListRemove_RealGit(t *testing.T) {
	repo := newRealRepo(t)
	m := New(gitAdapter{git.New()}, noopHooks{}, staticCfg{})

	res, err := m.Add(repo, AddOptions{Name: "feat"})
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
