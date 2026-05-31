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
	name, branch, err := m.resolveNames(AddOptions{})
	if err != nil {
		t.Fatalf("resolveNames: %v", err)
	}
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
	name, branch, err := m.resolveNames(AddOptions{Name: "hotfix"})
	if err != nil {
		t.Fatalf("resolveNames: %v", err)
	}
	if name != "hotfix" || branch != "wt/hotfix" {
		t.Errorf("name=%q branch=%q", name, branch)
	}
}

func TestResolveNames_ExplicitBranchHonoredWithPrefix(t *testing.T) {
	m, _, _ := newTestManager("/home/me/myrepo")
	_, branch, err := m.resolveNames(AddOptions{Name: "x", Branch: "feature/foo"})
	if err != nil {
		t.Fatalf("resolveNames: %v", err)
	}
	if branch != "wt/feature/foo" {
		t.Errorf("branch = %q, want wt/feature/foo", branch)
	}
}

func TestResolveNames_HonorsNameTemplate(t *testing.T) {
	g := newFakeGit("/home/me/myrepo")
	m := New(g, newFakeHooks(), fakeConfig{baseRef: "HEAD", nameTemplate: "{{.Adjective}}_{{.Noun}}"})
	m.now = func() time.Time { return time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC) }
	m.digits = func() int { return 4821 }
	name, branch, err := m.resolveNames(AddOptions{})
	if err != nil {
		t.Fatalf("resolveNames: %v", err)
	}
	if name != "eager_canyon" {
		t.Errorf("templated name = %q, want eager_canyon", name)
	}
	if branch != "wt/eager_canyon" {
		t.Errorf("templated branch = %q, want wt/eager_canyon", branch)
	}
}

func TestResolveNames_InvalidTemplateErrors(t *testing.T) {
	g := newFakeGit("/home/me/myrepo")
	m := New(g, newFakeHooks(), fakeConfig{baseRef: "HEAD", nameTemplate: "{{.Nope}}"})
	m.now = func() time.Time { return time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC) }
	m.digits = func() int { return 1 }
	if _, _, err := m.resolveNames(AddOptions{}); err == nil {
		t.Error("invalid name_template should produce an error")
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

func TestList_MapsGitWorktrees(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	g.worktrees = []GitWorktree{
		{Path: "/home/me/myrepo", Branch: "refs/heads/main"},
		{Path: "/home/me/myrepo.worktrees/feat", Branch: "refs/heads/wt/feat"},
	}
	list, err := m.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	if !list[0].IsMain {
		t.Errorf("first entry (repo root) should be marked main")
	}
	if list[1].Branch != "refs/heads/wt/feat" {
		t.Errorf("branch passthrough wrong: %q", list[1].Branch)
	}
}

func TestList_ExcludesWorktreesOutsideContainer(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	g.worktrees = []GitWorktree{
		{Path: "/home/me/myrepo", Branch: "refs/heads/main"},
		{Path: "/home/me/myrepo.worktrees/feat", Branch: "refs/heads/wt/feat"},
		{Path: "/elsewhere/external", Branch: "refs/heads/other"},
	}
	list, err := m.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 (main + container), got %d: %+v", len(list), list)
	}
	for _, w := range list {
		if w.Path == "/elsewhere/external" {
			t.Errorf("worktree outside the container should be excluded")
		}
	}
}

func TestResolveWorktree_ByDirThenBranch(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	g.worktrees = []GitWorktree{
		{Path: "/home/me/myrepo", Branch: "refs/heads/main"},
		{Path: "/home/me/myrepo.worktrees/feat", Branch: "refs/heads/wt/feat"},
	}
	w, err := m.resolveWorktree(".", "feat")
	if err != nil {
		t.Fatalf("resolve by dir: %v", err)
	}
	if w.Path != "/home/me/myrepo.worktrees/feat" {
		t.Errorf("resolved wrong path: %q", w.Path)
	}
	if _, err := m.resolveWorktree(".", "wt/feat"); err != nil {
		t.Errorf("resolve by branch failed: %v", err)
	}
	if _, err := m.resolveWorktree(".", "missing"); err == nil {
		t.Error("expected not-found error")
	}
}

func TestResolveWorktree_RefusesMain(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	g.worktrees = []GitWorktree{{Path: "/home/me/myrepo", Branch: "refs/heads/main"}}
	if _, err := m.resolveWorktree(".", "myrepo"); err == nil {
		t.Error("resolving the main worktree for removal should be refused")
	}
}

func seedRemovable(g *fakeGit) {
	g.worktrees = []GitWorktree{
		{Path: "/home/me/myrepo", Branch: "refs/heads/main"},
		{Path: "/home/me/myrepo.worktrees/feat", Branch: "refs/heads/wt/feat"},
	}
}

func TestRemove_RunsHooksRemovesWorktreeAndBranch(t *testing.T) {
	m, g, h := newTestManager("/home/me/myrepo")
	seedRemovable(g)
	res, err := m.Remove(".", RemoveOptions{Name: "feat"})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(g.removedPaths) != 1 {
		t.Errorf("worktree not removed: %v", g.removedPaths)
	}
	if !res.BranchDeleted {
		t.Errorf("branch should be deleted; res=%+v", res)
	}
	wantOrder := []HookEvent{PreRemove, PostRemove}
	if len(h.calls) != 2 || h.calls[0] != wantOrder[0] || h.calls[1] != wantOrder[1] {
		t.Errorf("hook order = %v, want %v", h.calls, wantOrder)
	}
}

func TestRemove_KeepBranchSkipsDeletion(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	seedRemovable(g)
	res, err := m.Remove(".", RemoveOptions{Name: "feat", KeepBranch: true})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(g.deleted) != 0 {
		t.Errorf("branch should not be deleted: %v", g.deleted)
	}
	if res.BranchDeleted {
		t.Error("BranchDeleted should be false with KeepBranch")
	}
}

func TestRemove_UnmergedBranchKeptAndReported(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	seedRemovable(g)
	g.deleteOK = false
	res, err := m.Remove(".", RemoveOptions{Name: "feat"})
	if err != nil {
		t.Fatalf("Remove should succeed even if branch kept: %v", err)
	}
	if res.BranchDeleted {
		t.Error("branch should not be reported deleted")
	}
	if !res.BranchKept {
		t.Error("BranchKept should be true so the CLI can report it")
	}
	if len(g.removedPaths) != 1 {
		t.Error("worktree should still be removed")
	}
}
