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
	cfg := fakeConfig{baseRef: "HEAD", branchPrefix: "wt/"}
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

func TestResolveTemplate_ByNameAndNumber(t *testing.T) {
	cfg := fakeConfig{templates: []Template{
		{Name: "autofix", Template: "autofix/{{.ticketName}}"},
		{Name: "feature", Template: "feat/{{.ticketName}}"},
	}}
	m := New(newFakeGit("/repo"), newFakeHooks(), cfg)

	byName, err := m.ResolveTemplate("autofix", map[string]string{"ticketName": "ZX-12"})
	if err != nil || byName != "autofix/ZX-12" {
		t.Fatalf("by name = %q, err=%v", byName, err)
	}
	byNum, err := m.ResolveTemplate("2", map[string]string{"ticketName": "ZX-12"})
	if err != nil || byNum != "feat/ZX-12" {
		t.Fatalf("by number = %q, err=%v", byNum, err)
	}
}

func TestResolveTemplate_UnknownRefErrors(t *testing.T) {
	m := New(newFakeGit("/repo"), newFakeHooks(), fakeConfig{})
	if _, err := m.ResolveTemplate("nope", nil); err == nil {
		t.Error("unknown template should error")
	}
	if _, err := m.ResolveTemplate("5", nil); err == nil {
		t.Error("out-of-range index should error")
	}
}

func TestTemplates_PassThrough(t *testing.T) {
	cfg := fakeConfig{templates: []Template{{Name: "a", Template: "a/{{.x}}"}}}
	m := New(newFakeGit("/repo"), newFakeHooks(), cfg)
	if got := m.Templates(); len(got) != 1 || got[0].Name != "a" {
		t.Errorf("Templates() = %+v", got)
	}
}

func TestAdd_FromBranchChecksOutExisting(t *testing.T) {
	m, g, h := newTestManager("/home/me/myrepo")
	g.branches["feature/login"] = true // branch exists
	res, err := m.Add(".", AddOptions{FromBranch: "feature/login"})
	if err != nil {
		t.Fatalf("Add from-branch: %v", err)
	}
	if res.Branch != "feature/login" {
		t.Errorf("branch = %q, want feature/login (verbatim)", res.Branch)
	}
	if res.Name != "feature/login" {
		t.Errorf("name = %q, want feature/login (branch w/o prefix)", res.Name)
	}
	if len(g.added) != 1 {
		t.Errorf("expected one worktree added, got %v", g.added)
	}
	wantOrder := []HookEvent{PreCreate, PostCreate}
	if len(h.calls) != 2 || h.calls[0] != wantOrder[0] || h.calls[1] != wantOrder[1] {
		t.Errorf("hooks should still run: %v", h.calls)
	}
}

func TestAdd_FromBranchMissingErrors(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	_, err := m.Add(".", AddOptions{FromBranch: "feature/missing"})
	if err == nil {
		t.Fatal("expected error when from-branch does not exist")
	}
	if len(g.added) != 0 {
		t.Errorf("nothing should be added when branch missing, got %v", g.added)
	}
}

func TestResolveNames_CustomPrefix(t *testing.T) {
	m := New(newFakeGit("/repo"), newFakeHooks(), fakeConfig{branchPrefix: "feature/"})
	_, branch, err := m.resolveNames(AddOptions{Name: "thing"})
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/thing" {
		t.Errorf("branch = %q, want %q", branch, "feature/thing")
	}
}

func TestResolveNames_NoPrefixAndOverride(t *testing.T) {
	m := New(newFakeGit("/repo"), newFakeHooks(), fakeConfig{branchPrefix: "wt/"})

	_, branch, err := m.resolveNames(AddOptions{Name: "autofix/X", NoPrefix: true})
	if err != nil {
		t.Fatal(err)
	}
	if branch != "autofix/X" {
		t.Errorf("--no-prefix branch = %q, want autofix/X", branch)
	}

	_, branch, err = m.resolveNames(AddOptions{Name: "autofix/X", PrefixOverride: "team/"})
	if err != nil {
		t.Fatal(err)
	}
	if branch != "team/autofix/X" {
		t.Errorf("override branch = %q, want team/autofix/X", branch)
	}

	// NoPrefix wins over an override.
	_, branch, err = m.resolveNames(AddOptions{Name: "autofix/X", NoPrefix: true, PrefixOverride: "team/"})
	if err != nil {
		t.Fatal(err)
	}
	if branch != "autofix/X" {
		t.Errorf("NoPrefix should win, branch = %q, want autofix/X", branch)
	}
}

func TestResolveNames_HonorsNameTemplate(t *testing.T) {
	g := newFakeGit("/home/me/myrepo")
	m := New(g, newFakeHooks(), fakeConfig{baseRef: "HEAD", branchPrefix: "wt/", nameTemplate: "{{.Adjective}}_{{.Noun}}"})
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

func TestWorktreePath_MirrorsFullBranch(t *testing.T) {
	m, _, _ := newTestManager("/home/me/myrepo")
	got := m.worktreePath("/home/me/myrepo", "wt/feature/foo")
	want := filepath.Join("/home/me/myrepo.worktrees", "wt", "feature", "foo")
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

func TestPlanRemoveAll_ExcludesMainIncludesOrphans(t *testing.T) {
	fg := &fakeGit{
		mainRoot: "/repo",
		worktrees: []GitWorktree{
			{Path: "/repo", Branch: "refs/heads/main"},
			{Path: "/repo.worktrees/a", Branch: "refs/heads/wt/a"},
		},
		listBranches: []string{"wt/a", "wt/orphan"},
	}
	m := New(fg, &fakeHooks{}, fakeConfig{branchPrefix: "wt/"})
	plan, err := m.PlanRemoveAll("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Worktrees) != 1 || plan.Worktrees[0].IsMain {
		t.Errorf("plan worktrees = %+v, want 1 non-main", plan.Worktrees)
	}
	if len(plan.Branches) != 2 {
		t.Errorf("plan branches = %v, want wt/a + wt/orphan", plan.Branches)
	}
}

func TestRemoveAll_BestEffortContinuesPastFailures(t *testing.T) {
	fg := &fakeGit{
		mainRoot: "/repo",
		worktrees: []GitWorktree{
			{Path: "/repo", Branch: "refs/heads/main"},
			{Path: "/repo.worktrees/a", Branch: "refs/heads/wt/a"},
			{Path: "/repo.worktrees/b", Branch: "refs/heads/wt/b"},
		},
		listBranches:    []string{"wt/a", "wt/b", "wt/orphan"},
		removeWtErr:     map[string]error{"/repo.worktrees/a": errInjected},
		deleteBranchErr: map[string]error{"wt/b": errInjected},
	}
	m := New(fg, &fakeHooks{}, fakeConfig{branchPrefix: "wt/"})
	res, err := m.RemoveAll("/repo")
	if err != nil {
		t.Fatalf("RemoveAll returned fatal error: %v", err)
	}
	if res.WorktreesRemoved != 1 { // b succeeded, a failed
		t.Errorf("WorktreesRemoved = %d, want 1", res.WorktreesRemoved)
	}
	if res.BranchesDeleted != 2 { // a + orphan; b failed
		t.Errorf("BranchesDeleted = %d, want 2", res.BranchesDeleted)
	}
	if len(res.Failures) != 2 {
		t.Errorf("Failures = %+v, want 2", res.Failures)
	}
	if !fg.pruned {
		t.Error("expected tail prune to run")
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
