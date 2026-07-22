package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var errInjected = fmt.Errorf("injected failure")

func newTestManager(root string) (*Manager, *fakeGit, *fakeHooks) {
	g := newFakeGit(root)
	h := newFakeHooks()
	m := New(g, h, fakeConfig{baseRef: "HEAD"})
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

func TestWorktreePath_FlatSanitizedSegment(t *testing.T) {
	m, _, _ := newTestManager("/home/me/myrepo")
	got := m.worktreePath("/home/me/myrepo", "feature/foo")
	want := filepath.Join("/home/me/myrepo.worktrees", "feature-foo")
	if got != want {
		t.Errorf("worktreePath = %q, want %q (flat, sanitized)", got, want)
	}
}

func TestAdd_NameIsBranchVerbatim(t *testing.T) {
	m, g, h := newTestManager("/home/me/myrepo")
	res, err := m.Add(".", AddOptions{Name: "feat"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if res.Branch != "feat" {
		t.Errorf("branch = %q, want feat (no prefix)", res.Branch)
	}
	if res.Name != "feat" {
		t.Errorf("name = %q, want feat", res.Name)
	}
	if res.BaseRef != "HEAD" {
		t.Errorf("baseRef = %q, want HEAD", res.BaseRef)
	}
	if len(g.added) != 1 {
		t.Errorf("expected one worktree added, got %v", g.added)
	}
	wantOrder := []HookEvent{PreCreate, PostCreate}
	if len(h.calls) != 2 || h.calls[0] != wantOrder[0] || h.calls[1] != wantOrder[1] {
		t.Errorf("hook order = %v, want %v", h.calls, wantOrder)
	}
}

func TestAdd_RenderedBranchUsedVerbatim(t *testing.T) {
	m, _, _ := newTestManager("/home/me/myrepo")
	res, err := m.Add(".", AddOptions{Branch: "fix/GH-42-review"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if res.Branch != "fix/GH-42-review" {
		t.Errorf("branch = %q", res.Branch)
	}
	wantPath := filepath.Join("/home/me/myrepo.worktrees", "fix-GH-42-review")
	if res.Path != wantPath {
		t.Errorf("path = %q, want %q (slash sanitized to flat dir)", res.Path, wantPath)
	}
}

func TestAdd_NoNameAtRootErrors(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	_, err := m.Add(".", AddOptions{})
	if err == nil {
		t.Fatal("bare Add at the repo root must error (names are never generated)")
	}
	if !strings.Contains(err.Error(), "branch name required") {
		t.Errorf("error should say a name is required, got %q", err)
	}
	if len(g.added) != 0 {
		t.Errorf("nothing should be added, got %v", g.added)
	}
}

// deriveManager builds a Manager whose fakeGit reports a single non-main
// worktree on branch feature-login living in the sibling container, plus the
// main worktree at the repo root.
func deriveManager(t *testing.T) (*Manager, *fakeGit, string) {
	t.Helper()
	repoRoot := "/home/me/myrepo"
	wtDir := "/home/me/myrepo.worktrees/feature-login"
	m, g, _ := newTestManager(repoRoot)
	g.worktrees = []GitWorktree{
		{Path: repoRoot, Branch: "refs/heads/main"},
		{Path: wtDir, Branch: "refs/heads/feature-login"},
	}
	return m, g, wtDir
}

func TestAdd_DeriveMode_AutoV001(t *testing.T) {
	m, _, wtDir := deriveManager(t)
	res, err := m.Add(wtDir, AddOptions{})
	if err != nil {
		t.Fatalf("Add derive: %v", err)
	}
	if res.Branch != "feature-login-v001" {
		t.Errorf("branch = %q, want feature-login-v001", res.Branch)
	}
	if res.BaseRef != "feature-login" {
		t.Errorf("baseRef = %q, want feature-login (parent branch tip)", res.BaseRef)
	}
	wantPath := filepath.Join("/home/me/myrepo.worktrees", "feature-login-v001")
	if res.Path != wantPath {
		t.Errorf("path = %q, want %q", res.Path, wantPath)
	}
}

func TestAdd_DeriveMode_SubdirOfWorktree(t *testing.T) {
	m, _, wtDir := deriveManager(t)
	deep := filepath.Join(wtDir, "subdir", "deep")
	res, err := m.Add(deep, AddOptions{})
	if err != nil {
		t.Fatalf("Add from subdir: %v", err)
	}
	if res.Branch != "feature-login-v001" {
		t.Errorf("branch = %q, want feature-login-v001 (parent resolved from subdir)", res.Branch)
	}
}

func TestAdd_DeriveMode_SharedPrefixNoFalseMatch(t *testing.T) {
	repoRoot := "/repo"
	featDir := "/repo.worktrees/feat"
	featExtraDir := "/repo.worktrees/feat-extra"
	mk := func() (*Manager, *fakeGit) {
		m, g, _ := newTestManager(repoRoot)
		g.worktrees = []GitWorktree{
			{Path: repoRoot, Branch: "refs/heads/main"},
			{Path: featDir, Branch: "refs/heads/feat"},
			{Path: featExtraDir, Branch: "refs/heads/feat-extra"},
		}
		return m, g
	}

	m, _ := mk()
	res, err := m.Add(featExtraDir, AddOptions{})
	if err != nil {
		t.Fatalf("Add feat-extra: %v", err)
	}
	if res.Branch != "feat-extra-v001" {
		t.Errorf("branch = %q, want feat-extra-v001 (must not false-match feat)", res.Branch)
	}

	m2, _ := mk()
	res2, err := m2.Add(featDir, AddOptions{})
	if err != nil {
		t.Fatalf("Add feat: %v", err)
	}
	if res2.Branch != "feat-v001" {
		t.Errorf("branch = %q, want feat-v001 (must match feat, not feat-extra)", res2.Branch)
	}
}

func TestAdd_DeriveMode_GapFill(t *testing.T) {
	// -v001 and -v002 exist → next is -v003.
	m, g, wtDir := deriveManager(t)
	g.branches["feature-login-v001"] = true
	g.branches["feature-login-v002"] = true
	res, err := m.Add(wtDir, AddOptions{})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if res.Branch != "feature-login-v003" {
		t.Errorf("branch = %q, want feature-login-v003", res.Branch)
	}

	// Only -v002 exists → lowest free is -v001 (fills the gap).
	m2, g2, wtDir2 := deriveManager(t)
	g2.branches["feature-login-v002"] = true
	res2, err := m2.Add(wtDir2, AddOptions{})
	if err != nil {
		t.Fatalf("Add gap: %v", err)
	}
	if res2.Branch != "feature-login-v001" {
		t.Errorf("branch = %q, want feature-login-v001 (lowest free)", res2.Branch)
	}
}

func TestAdd_DeriveMode_CustomNameSuffix(t *testing.T) {
	m, _, wtDir := deriveManager(t)
	res, err := m.Add(wtDir, AddOptions{Name: "-patch01"})
	if err != nil {
		t.Fatalf("Add custom suffix: %v", err)
	}
	if res.Branch != "feature-login-patch01" {
		t.Errorf("branch = %q, want feature-login-patch01 (literal suffix)", res.Branch)
	}
	if res.BaseRef != "feature-login" {
		t.Errorf("baseRef = %q, want feature-login", res.BaseRef)
	}
}

func TestAdd_DeriveMode_BareNameGetsSeparator(t *testing.T) {
	// A name without a leading dash must have the "-" separator inserted so it
	// does not glue onto the parent branch (no "feature-loginfix").
	m, _, wtDir := deriveManager(t)
	res, err := m.Add(wtDir, AddOptions{Name: "fix"})
	if err != nil {
		t.Fatalf("Add bare name: %v", err)
	}
	if res.Branch != "feature-login-fix" {
		t.Errorf("branch = %q, want feature-login-fix (separator inserted)", res.Branch)
	}
}

func TestAdd_DeriveMode_CustomNameCollisionErrors(t *testing.T) {
	m, g, wtDir := deriveManager(t)
	g.branches["feature-login-patch01"] = true
	_, err := m.Add(wtDir, AddOptions{Name: "-patch01"})
	if err == nil {
		t.Fatal("expected collision error for existing derived branch")
	}
	if !strings.Contains(err.Error(), "feature-login-patch01") {
		t.Errorf("error should name the derived branch, got %q", err)
	}
	if len(g.added) != 0 {
		t.Errorf("nothing should be created on collision, added=%v", g.added)
	}
}

func TestAdd_DeriveMode_NonCanonicalDir(t *testing.T) {
	// A relative dir (e.g. from a relative --repo) must still trigger derive
	// mode: currentWorktreeBranch normalizes dir to an absolute path before the
	// prefix comparison against git's absolute --show-toplevel paths.
	repoRoot := "/home/me/myrepo"
	relDir := filepath.Join("rel", "feature-login")
	absDir, err := filepath.Abs(relDir)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	m, g, _ := newTestManager(repoRoot)
	g.worktrees = []GitWorktree{
		{Path: repoRoot, Branch: "refs/heads/main"},
		{Path: absDir, Branch: "refs/heads/feature-login"},
	}
	res, err := m.Add(relDir, AddOptions{})
	if err != nil {
		t.Fatalf("Add relative dir: %v", err)
	}
	if res.Branch != "feature-login-v001" {
		t.Errorf("branch = %q, want feature-login-v001 (derive after normalization)", res.Branch)
	}
}

func TestAdd_DeriveMode_MainRootRegression(t *testing.T) {
	// dir == MainRoot: matched worktree entry is the main one → no derive.
	m, _, _ := newTestManager("/home/me/myrepo")
	res, err := m.Add("/home/me/myrepo", AddOptions{Name: "feat"})
	if err != nil {
		t.Fatalf("Add main root: %v", err)
	}
	if res.Branch != "feat" {
		t.Errorf("branch = %q, want feat (no derive, no -vNNN)", res.Branch)
	}
}

func TestAdd_DeriveMode_RenderedBranchWins(t *testing.T) {
	// A template-rendered Branch suppresses derive mode entirely.
	m, _, wtDir := deriveManager(t)
	res, err := m.Add(wtDir, AddOptions{Branch: "fix/GH-1"})
	if err != nil {
		t.Fatalf("Add rendered branch: %v", err)
	}
	if res.Branch != "fix/GH-1" {
		t.Errorf("branch = %q, want fix/GH-1 (rendered branch verbatim)", res.Branch)
	}
	if res.BaseRef != "HEAD" {
		t.Errorf("baseRef = %q, want HEAD (config base, not parent branch)", res.BaseRef)
	}
}

func TestParentBranch(t *testing.T) {
	m, _, wtDir := deriveManager(t)
	pb, ok := m.ParentBranch(wtDir)
	if !ok || pb != "feature-login" {
		t.Errorf("ParentBranch = %q, %v; want feature-login, true", pb, ok)
	}
	if _, ok := m.ParentBranch("/home/me/myrepo"); ok {
		t.Error("ParentBranch at the repo root should report ok=false")
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
	g.branches["feat"] = true
	_, err := m.Add(".", AddOptions{Name: "feat"})
	if err == nil {
		t.Fatal("expected error when branch already exists")
	}
}

func TestList_MapsGitWorktrees(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	g.worktrees = []GitWorktree{
		{Path: "/home/me/myrepo", Branch: "refs/heads/main"},
		{Path: "/home/me/myrepo.worktrees/feat", Branch: "refs/heads/feat"},
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
	if list[1].Branch != "refs/heads/feat" {
		t.Errorf("branch passthrough wrong: %q", list[1].Branch)
	}
}

func TestList_ExcludesWorktreesOutsideContainer(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	g.worktrees = []GitWorktree{
		{Path: "/home/me/myrepo", Branch: "refs/heads/main"},
		{Path: "/home/me/myrepo.worktrees/feat", Branch: "refs/heads/feat"},
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

func TestResolveWorktree_ByDirBranchAndSanitized(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	g.worktrees = []GitWorktree{
		{Path: "/home/me/myrepo", Branch: "refs/heads/main"},
		{Path: "/home/me/myrepo.worktrees/fix-GH-42", Branch: "refs/heads/fix/GH-42"},
	}
	if _, err := m.resolveWorktree(".", "fix-GH-42"); err != nil {
		t.Errorf("resolve by dir failed: %v", err)
	}
	if _, err := m.resolveWorktree(".", "fix/GH-42"); err != nil {
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
		{Path: "/home/me/myrepo.worktrees/feat", Branch: "refs/heads/feat"},
	}
}

// realWorktreeLayout creates an on-disk repo root + container/wt1 layout and
// a manager whose fake git reports them, so cwd-escape behavior can be tested
// with real directories.
func realWorktreeLayout(t *testing.T) (m *Manager, repoRoot, wtDir string) {
	t.Helper()
	base := t.TempDir()
	repoRoot = filepath.Join(base, "repo")
	wtDir = filepath.Join(base, "repo.worktrees", "wt1")
	for _, d := range []string{repoRoot, wtDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	g := newFakeGit(repoRoot)
	g.worktrees = []GitWorktree{
		{Path: repoRoot, Branch: "refs/heads/main"},
		{Path: wtDir, Branch: "refs/heads/wt1"},
	}
	return New(g, newFakeHooks(), fakeConfig{baseRef: "HEAD"}), repoRoot, wtDir
}

func TestEscapeCwd_MovesProcessOutOfWorktree(t *testing.T) {
	m, repoRoot, wtDir := realWorktreeLayout(t)
	t.Chdir(wtDir)
	m.EscapeCwd(wtDir)
	got, _ := os.Getwd()
	if !pathsEqual(got, repoRoot) {
		t.Errorf("cwd = %q, want repo root %q", got, repoRoot)
	}
}

func TestEscapeCwd_NoopOutsideWorktrees(t *testing.T) {
	m, repoRoot, _ := realWorktreeLayout(t)
	t.Chdir(repoRoot)
	m.EscapeCwd(repoRoot)
	got, _ := os.Getwd()
	if !pathsEqual(got, repoRoot) {
		t.Errorf("cwd should be unchanged at repo root, got %q", got)
	}
}

func TestRemove_EscapesCwdBeforeDeleting(t *testing.T) {
	m, repoRoot, wtDir := realWorktreeLayout(t)
	t.Chdir(wtDir)
	if _, err := m.Remove(wtDir, RemoveOptions{Name: "wt1", NoHooks: true}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got, _ := os.Getwd()
	if !pathsEqual(got, repoRoot) {
		t.Errorf("Remove should move the process out of the worktree first; cwd = %q", got)
	}
}

func TestRemoveAll_EscapesCwdBeforeDeleting(t *testing.T) {
	m, repoRoot, wtDir := realWorktreeLayout(t)
	t.Chdir(wtDir)
	if _, err := m.RemoveAll(wtDir); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	got, _ := os.Getwd()
	if !pathsEqual(got, repoRoot) {
		t.Errorf("RemoveAll should move the process out of the worktree first; cwd = %q", got)
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

func TestPlanRemoveAll_BranchesComeFromContainerWorktrees(t *testing.T) {
	fg := &fakeGit{
		mainRoot: "/repo",
		worktrees: []GitWorktree{
			{Path: "/repo", Branch: "refs/heads/main"},
			{Path: "/repo.worktrees/a", Branch: "refs/heads/a"},
			{Path: "/repo.worktrees/detached"},
		},
	}
	m := New(fg, &fakeHooks{}, fakeConfig{})
	plan, err := m.PlanRemoveAll("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Worktrees) != 2 {
		t.Errorf("plan worktrees = %+v, want 2 non-main", plan.Worktrees)
	}
	if len(plan.Branches) != 1 || plan.Branches[0] != "a" {
		t.Errorf("plan branches = %v, want [a] (detached has none)", plan.Branches)
	}
}

func TestRemoveAll_BestEffortContinuesPastFailures(t *testing.T) {
	fg := &fakeGit{
		mainRoot: "/repo",
		worktrees: []GitWorktree{
			{Path: "/repo", Branch: "refs/heads/main"},
			{Path: "/repo.worktrees/a", Branch: "refs/heads/a"},
			{Path: "/repo.worktrees/b", Branch: "refs/heads/b"},
		},
		removeWtErr:     map[string]error{"/repo.worktrees/a": errInjected},
		deleteBranchErr: map[string]error{"b": errInjected},
	}
	m := New(fg, &fakeHooks{}, fakeConfig{})
	res, err := m.RemoveAll("/repo")
	if err != nil {
		t.Fatalf("RemoveAll returned fatal error: %v", err)
	}
	if res.WorktreesRemoved != 1 { // b succeeded, a failed
		t.Errorf("WorktreesRemoved = %d, want 1", res.WorktreesRemoved)
	}
	if res.BranchesDeleted != 1 { // a succeeded; b failed
		t.Errorf("BranchesDeleted = %d, want 1", res.BranchesDeleted)
	}
	if len(res.Failures) != 2 {
		t.Errorf("Failures = %+v, want 2", res.Failures)
	}
	if !fg.pruned {
		t.Error("expected tail prune to run")
	}
}
