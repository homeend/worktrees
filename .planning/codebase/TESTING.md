# Testing Patterns

**Analysis Date:** 2026-06-05

## Test Framework

**Runner:**
- Go standard library `testing` package — no third-party test runner
- No `testify`, `gomock`, or other assertion libraries; all assertions use `t.Errorf` / `t.Fatalf`

**Assertion Library:**
- Standard `testing.T` methods only: `t.Errorf`, `t.Fatalf`, `t.Fatal`, `t.Error`, `t.Helper`

**Build tags:**
- Integration tests gated with `//go:build integration` at the top of the file
- Unit tests have no build tag (run by default)

**Run Commands:**
```bash
go test ./...                          # Run all unit tests
go test -tags integration ./...        # Run unit + integration tests
go test ./pkg/worktree/...             # Run a specific package
go test -run TestAdd_HappyPath ./...   # Run a specific test
go test -count=1 ./...                 # Disable test caching
```

## Test File Organization

**Location:**
- Co-located with source: every `foo.go` has a `foo_test.go` in the same directory
- Integration tests use a separate file: `foo_integration_test.go` with `//go:build integration`
- Fakes/helpers for a package live in a dedicated file in the same package: `pkg/worktree/fakes_test.go`

**Naming:**
- Test files: `<source>_test.go` or `<source>_integration_test.go`
- Test functions: `Test<Subject>_<Scenario>` — e.g., `TestAdd_HappyPathRunsHooksInOrder`, `TestResolveNames_ExplicitNameGetsWtPrefix`
- Helper builders: `newTestManager`, `newFakeGit`, `newTestModel`, `newRealRepo`, `newTestRepo`

**Package strategy:**
- White-box tests (access unexported symbols) use `package worktree` / `package git` / `package cmd` (same package)
- All test packages match their source package (no `_test` package suffix used)

**Structure:**
```
pkg/worktree/
  manager.go
  manager_test.go           # white-box unit tests
  manager_integration_test.go  # real-git integration tests (build tag)
  fakes_test.go             # shared fakes for the package

internal/git/
  git.go
  git_test.go               # pure unit tests (no git subprocess)
  git_integration_test.go   # real-git integration tests (build tag)

internal/config/config_test.go
internal/naming/naming_test.go
internal/hooks/hooks_test.go
internal/tui/model_test.go
cmd/wt/new_test.go
cmd/wt/root_test.go
cmd/wt/kill_em_all_test.go
cmd/wt/set_test.go
cmd/wt/templates_test.go
cmd/wt/cli_integration_test.go
```

## Test Structure

**Suite Organization:**
```go
// Each test is a standalone function — no shared test suite type
func TestAdd_HappyPathRunsHooksInOrder(t *testing.T) {
    m, g, h := newTestManager("/home/me/myrepo")
    res, err := m.Add(".", AddOptions{Name: "feat"})
    if err != nil {
        t.Fatalf("Add: %v", err)
    }
    if res.Branch != "wt/feat" {
        t.Errorf("branch = %q", res.Branch)
    }
    // ...
}
```

**Table-driven tests:**
Used where multiple input/output pairs exist for pure logic:
```go
// From internal/git/git_test.go
cases := []struct {
    v        Version
    maj, min int
    wantLess bool
}{
    {Version{2, 30, 0}, 2, 30, false},
    {Version{2, 29, 9}, 2, 30, true},
    // ...
}
for _, c := range cases {
    if got := versionLess(c.v, c.maj, c.min); got != c.wantLess {
        t.Errorf("versionLess(%+v, %d, %d) = %v, want %v", c.v, c.maj, c.min, got, c.wantLess)
    }
}
```

**Patterns:**
- `t.Fatalf` for setup failures (stops the test immediately)
- `t.Errorf` for assertion failures (continues test execution to collect all failures)
- `t.Helper()` marked on all helper functions that call `t.Fatal`/`t.Error`
- `t.TempDir()` used universally for filesystem tests (auto-cleaned)
- `t.Setenv` used for environment variable tests (auto-restored)
- `t.Cleanup` used in integration tests for worktree removal

## Mocking

**Framework:** Manual hand-written fakes — no mock generation library

**Patterns:**
```go
// In pkg/worktree/fakes_test.go — struct implementing the interface
type fakeGit struct {
    mainRoot     string
    branches     map[string]bool
    worktrees    []GitWorktree
    addErr       error
    added        []string
    removedPaths []string
    // per-call error injection:
    removeWtErr     map[string]error // keyed by path
    deleteBranchErr map[string]error // keyed by branch
    pruned          bool
}

func (f *fakeGit) MainRoot(string) (string, error) { return f.mainRoot, nil }
// ... all interface methods implemented
```

```go
// Hook fake with per-event failure injection:
type fakeHooks struct {
    calls  []HookEvent
    failOn map[HookEvent]error
}

func (h *fakeHooks) Run(ctx HookContext) error {
    h.calls = append(h.calls, ctx.Event)
    if err, ok := h.failOn[ctx.Event]; ok {
        return fmt.Errorf("hook %s failed: %w", ctx.Event, err)
    }
    return nil
}
```

```go
// TUI runAction injectable via model field:
m.runAction = func(args ...string) tea.Cmd {
    *rec = append(*rec, strings.Join(args, " "))
    return func() tea.Msg { return actionFinishedMsg{} }
}
```

**What to mock:**
- `GitRunner` interface — avoids real git subprocess calls in unit tests
- `HookRunner` interface — avoids real hook execution and shell dependency
- `ConfigProvider` interface — provides controlled config values
- `lister` interface in TUI — avoids needing a real `Manager` for model tests
- `killer` interface in `cmd/wt/kill_em_all.go` — avoids real git for kill-em-all tests
- `addResolver` interface in `cmd/wt/new.go` — avoids real `Manager` for option-parsing tests

**What NOT to mock:**
- Filesystem operations in config/hooks tests (use `t.TempDir()` with real files)
- Git subprocess in integration tests (use real git with `t.TempDir()` repos)
- Standard library functions (`os`, `filepath`, `strings`)

## Fixtures and Factories

**Test Data:**
```go
// Shared test manager factory in pkg/worktree/manager_test.go
func newTestManager(root string) (*Manager, *fakeGit, *fakeHooks) {
    g := newFakeGit(root)
    h := newFakeHooks()
    cfg := fakeConfig{baseRef: "HEAD", branchPrefix: "wt/"}
    m := New(g, h, cfg)
    m.now = func() time.Time { return time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC) }
    m.digits = func() int { return 4821 }
    return m, g, h
}

// Seed helper for remove tests:
func seedRemovable(g *fakeGit) {
    g.worktrees = []GitWorktree{
        {Path: "/home/me/myrepo", Branch: "refs/heads/main"},
        {Path: "/home/me/myrepo.worktrees/feat", Branch: "refs/heads/wt/feat"},
    }
}
```

```go
// Real repo factory for integration tests:
func newRealRepo(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()
    run := func(a ...string) { /* exec git, fatal on error */ }
    run("init", "-b", "main")
    os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644)
    run("add", ".")
    run("commit", "-m", "init")
    return dir
}
```

```go
// TUI test model factory in internal/tui/model_test.go:
func newTestModel(items []worktree.WorktreeInfo) (model, *[]string) {
    rec := &[]string{}
    m := newModel(&fakeLister{items: items}, "/repo", items, nil)
    m.runAction = func(args ...string) tea.Cmd { /* record, return no-op cmd */ }
    return m, rec
}

func sample() []worktree.WorktreeInfo {
    return []worktree.WorktreeInfo{
        {Path: "/repo", Branch: "refs/heads/main", IsMain: true},
        {Path: "/repo.worktrees/feat", Branch: "refs/heads/wt/feat"},
    }
}
```

**Location:**
- Fakes for `pkg/worktree`: `pkg/worktree/fakes_test.go`
- Fakes for `cmd/wt`: defined locally in each `*_test.go` file (`fakeResolver`, `fakeKiller`, `fakeLister`)
- Hook test helpers: `writeHook` helper in `internal/hooks/hooks_test.go`
- No separate `testdata/` directory exists

## Coverage

**Requirements:** No enforced coverage threshold (no CI config found)

**View Coverage:**
```bash
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

## Test Types

**Unit Tests:**
- Scope: single function or method in isolation, all dependencies faked
- Files: `manager_test.go`, `git_test.go`, `naming_test.go`, `config_test.go`, `hooks_test.go`, `model_test.go`, `new_test.go`, `root_test.go`, `kill_em_all_test.go`, `set_test.go`, `templates_test.go`
- Approach: construct the subject via factory, invoke the function under test, assert on return values and recorded calls on fakes

**Integration Tests:**
- Scope: real git subprocess invocations using `t.TempDir()` repos; real filesystem operations
- Files: `git_integration_test.go`, `manager_integration_test.go`, `cli_integration_test.go`
- Build tag: `//go:build integration` — excluded from `go test ./...`
- Approach: `newRealRepo`/`newTestRepo` helpers create a real committed git repo in a temp dir; tests exercise the full stack

**E2E Tests:**
- Not used (no binary invocation tests)

## Common Patterns

**Async/Cmd Testing (BubbleTea):**
```go
// Model.Update returns (tea.Model, tea.Cmd) — test via message dispatch:
m, rec := newTestModel(sample())
updated, cmd := m.Update(key("n"))   // key() builds a tea.KeyMsg
if cmd == nil {
    t.Fatal("n should return an action command immediately")
}
// Assert recorded invocations:
if (*rec)[0] != "new --repo /repo" {
    t.Errorf("runAction = %v", *rec)
}
// Chain messages:
down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
conf, _ := down.(model).Update(key("d"))
```

**Error Testing:**
```go
// Negative path: expect error
if _, err := m.Add(".", AddOptions{FromBranch: "feature/missing"}); err == nil {
    t.Fatal("expected error when from-branch does not exist")
}

// Positive path: no error
res, err := m.Add(".", AddOptions{Name: "feat"})
if err != nil {
    t.Fatalf("Add: %v", err)
}

// Error classification:
hook := classify(fmt.Errorf("post-create hook failed (worktree left in place): boom"))
if exitCodeFor(hook) != 4 {
    t.Errorf("hook failure should map to exit 4, got %d", exitCodeFor(hook))
}
```

**Injection-based Error Injection:**
```go
// Inject a failure on a specific hook event:
h.failOn[PreCreate] = errInjected
_, err := m.Add(".", AddOptions{Name: "feat"})
if err == nil {
    t.Fatal("expected error from pre-create failure")
}

// Inject a per-path error on git RemoveWorktree:
fg.removeWtErr = map[string]error{"/repo.worktrees/a": errInjected}
```

**Filesystem Assertions:**
```go
// Verify a path was created:
if _, err := os.Stat(res.Path); err != nil {
    t.Fatalf("worktree dir not created: %v", err)
}
// Verify a path was removed:
if _, err := os.Stat(res.Path); !os.IsNotExist(err) {
    t.Errorf("worktree dir should be gone")
}
```

**Output Capture:**
```go
// Commands that write to io.Writer are testable by passing bytes.Buffer:
var out bytes.Buffer
err := runKillEmAll(fakeKiller{}, "/repo", killOpts{yes: true, isTTY: false}, &out)
if !bytes.Contains(out.Bytes(), []byte("nothing to remove")) {
    t.Errorf("output = %q", out.String())
}
```

---

*Testing analysis: 2026-06-05*
