package worktree

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

var errInjected = fmt.Errorf("injected failure")

func newTestManager(root string) (*Manager, *fakeGit, *fakeHooks) {
	g := newFakeGit(root)
	h := newFakeHooks()
	cfg := fakeConfig{baseRef: "HEAD"}
	m := New(g, h, cfg)
	m.now = func() time.Time { return time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC) }
	m.digits = func() int { return 4821 }
	return m, g, h
}

func TestContainerPath_DefaultSibling(t *testing.T) {
	m, _, _ := newTestManager("/home/me/myrepo")
	got := m.containerPath("/home/me/myrepo")
	want := "/home/me/myrepo.worktrees"
	if got != want {
		t.Errorf("containerPath = %q, want %q", got, want)
	}
}

func TestContainerPath_ConfigOverrideUsedVerbatim(t *testing.T) {
	g := newFakeGit("/home/me/myrepo")
	m := New(g, newFakeHooks(), fakeConfig{baseRef: "HEAD", container: "/custom/wts"})
	if got := m.containerPath("/home/me/myrepo"); got != "/custom/wts" {
		t.Errorf("override container = %q, want /custom/wts", got)
	}
}

func TestResolveNames_GeneratedWhenEmpty(t *testing.T) {
	m, _, _ := newTestManager("/home/me/myrepo")
	name, branch := m.resolveNames(AddOptions{})
	// Expectation verified against internal/naming.Generate(2026-05-31 14:30, 4821):
	// adjectives[(4821/16)%16]=adjectives[13]="eager", nouns[4821%16]=nouns[5]="canyon".
	if name != "2026-05-31_14-30-eager-canyon-4821" {
		t.Errorf("generated name = %q", name)
	}
	if branch != "wt/2026-05-31_14-30-eager-canyon-4821" {
		t.Errorf("generated branch = %q", branch)
	}
}

func TestResolveNames_ExplicitNameGetsWtPrefix(t *testing.T) {
	m, _, _ := newTestManager("/home/me/myrepo")
	name, branch := m.resolveNames(AddOptions{Name: "hotfix"})
	if name != "hotfix" || branch != "wt/hotfix" {
		t.Errorf("name=%q branch=%q", name, branch)
	}
}

func TestResolveNames_ExplicitBranchHonoredWithPrefix(t *testing.T) {
	m, _, _ := newTestManager("/home/me/myrepo")
	_, branch := m.resolveNames(AddOptions{Name: "x", Branch: "feature/foo"})
	if branch != "wt/feature/foo" {
		t.Errorf("branch = %q, want wt/feature/foo", branch)
	}
}

func TestWorktreePath_UsesSanitizedDir(t *testing.T) {
	m, _, _ := newTestManager("/home/me/myrepo")
	got := m.worktreePath("/home/me/myrepo", "wt/feature/foo")
	want := filepath.Join("/home/me/myrepo.worktrees", "feature-foo")
	if got != want {
		t.Errorf("worktreePath = %q, want %q", got, want)
	}
}

func TestAdd_HappyPathRunsHooksInOrder(t *testing.T) {
	m, g, h := newTestManager("/home/me/myrepo")
	res, err := m.Add(".", AddOptions{Name: "feat"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if res.Branch != "wt/feat" {
		t.Errorf("branch = %q", res.Branch)
	}
	if len(g.added) != 1 {
		t.Errorf("expected one worktree added, got %v", g.added)
	}
	wantOrder := []HookEvent{PreCreate, PostCreate}
	if len(h.calls) != 2 || h.calls[0] != wantOrder[0] || h.calls[1] != wantOrder[1] {
		t.Errorf("hook order = %v, want %v", h.calls, wantOrder)
	}
}

func TestAdd_PreCreateFailureAbortsBeforeAdd(t *testing.T) {
	m, g, h := newTestManager("/home/me/myrepo")
	h.failOn[PreCreate] = errInjected
	_, err := m.Add(".", AddOptions{Name: "feat"})
	if err == nil {
		t.Fatal("expected error from pre-create failure")
	}
	if len(g.added) != 0 {
		t.Errorf("nothing should be added when pre-create fails, got %v", g.added)
	}
}

func TestAdd_PostCreateFailureLeavesWorktree(t *testing.T) {
	m, g, h := newTestManager("/home/me/myrepo")
	h.failOn[PostCreate] = errInjected
	_, err := m.Add(".", AddOptions{Name: "feat"})
	if err == nil {
		t.Fatal("expected error from post-create failure")
	}
	if len(g.added) != 1 {
		t.Errorf("worktree should remain after post-create failure (no rollback); added=%v", g.added)
	}
	if len(g.removedPaths) != 0 {
		t.Errorf("no rollback expected; removed=%v", g.removedPaths)
	}
}

func TestAdd_RejectsExistingBranch(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	g.branches["wt/feat"] = true
	_, err := m.Add(".", AddOptions{Name: "feat"})
	if err == nil {
		t.Fatal("expected error when branch already exists")
	}
}

func TestAdd_NoHooksSkipsHooks(t *testing.T) {
	m, _, h := newTestManager("/home/me/myrepo")
	if _, err := m.Add(".", AddOptions{Name: "feat", NoHooks: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(h.calls) != 0 {
		t.Errorf("hooks should be skipped, got %v", h.calls)
	}
}
