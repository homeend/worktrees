# Configurable Branch Prefix + kill-em-all Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the `wt new` branch prefix configurable (file + `WT_BRANCH_PREFIX` env + `wt set` command), then add a destructive `wt kill-em-all` cleanup (CLI + TUI) that force-removes all prefixed worktrees and branches.

**Architecture:** Prefix resolution lives entirely in `internal/config` (Defaults → file → env, normalized once) and is threaded through `pkg/worktree.Manager` via the `ConfigProvider` interface, replacing the three hardcoded `"wt/"` literals. The bulk cleanup adds a read-only `PlanRemoveAll` (powers confirmation) and a best-effort `RemoveAll` (raw force git calls, no hooks, tail prune) exposed through a new `wt kill-em-all` subcommand and a TUI `K` confirm screen that re-invokes it.

**Tech Stack:** Go 1.25, cobra (CLI), bubbletea/lipgloss (TUI), gopkg.in/yaml.v3, golang.org/x/term.

**Specs:**
- `docs/superpowers/specs/2026-06-01-configurable-branch-prefix-design.md`
- `docs/superpowers/specs/2026-06-01-kill-em-all-cleanup-design.md`

**Conventions to follow (observed in the codebase):**
- Run tests with `go test ./...` (or a package path). Build with `go build ./...`.
- Commands are cobra subcommands registered in `init()`; flags are kebab-case `BoolVar`/`StringVar`.
- Tests use plain `t.Run` / `t.Errorf` style; fakes live in `pkg/worktree/fakes_test.go`.
- Config keys are snake_case in YAML, PascalCase fields with `yaml:"..."` tags.

---

# Part A — Configurable branch prefix

## Task A1: Config field, default, Resolve clause, NormalizePrefix

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/config_test.go`:

```go
func TestDefaults_BranchPrefix(t *testing.T) {
	if got := Defaults().BranchPrefix; got != "wt/" {
		t.Errorf("Defaults().BranchPrefix = %q, want %q", got, "wt/")
	}
}

func TestResolve_BranchPrefixOverrides(t *testing.T) {
	lo := Config{BranchPrefix: "wt/"}
	hi := Config{BranchPrefix: "feature/"}
	if got := Resolve(lo, hi).BranchPrefix; got != "feature/" {
		t.Errorf("Resolve BranchPrefix = %q, want %q", got, "feature/")
	}
	// empty hi does not override
	if got := Resolve(lo, Config{}).BranchPrefix; got != "wt/" {
		t.Errorf("Resolve with empty hi = %q, want %q", got, "wt/")
	}
}

func TestNormalizePrefix(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"feature":  "feature/",
		"feature/": "feature/",
		"wt":       "wt/",
	}
	for in, want := range cases {
		if got := NormalizePrefix(in); got != want {
			t.Errorf("NormalizePrefix(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run 'BranchPrefix|NormalizePrefix' -v`
Expected: FAIL — `BranchPrefix` field and `NormalizePrefix` undefined (compile error).

- [ ] **Step 3: Implement field, default, Resolve clause, helper**

In `internal/config/config.go`, add `"strings"` to imports, then:

Add the field to `Config`:

```go
type Config struct {
	BaseRef      string `yaml:"base_ref"`
	Container    string `yaml:"container"`
	NameTemplate string `yaml:"name_template"`
	BranchPrefix string `yaml:"branch_prefix"`
}
```

Set the default:

```go
func Defaults() Config {
	return Config{BaseRef: "HEAD", BranchPrefix: "wt/"}
}
```

Add the Resolve clause (inside `Resolve`, before `return out`):

```go
	if hi.BranchPrefix != "" {
		out.BranchPrefix = hi.BranchPrefix
	}
```

Add the helper:

```go
// NormalizePrefix returns the branch prefix with a single trailing slash. An
// empty prefix is returned unchanged (callers treat "" as "use the default").
func NormalizePrefix(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasSuffix(s, "/") {
		return s
	}
	return s + "/"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -run 'BranchPrefix|NormalizePrefix' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add branch_prefix field, default, and NormalizePrefix"
```

---

## Task A2: LoadFile + env-chain Load (WT_BRANCH_PREFIX)

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/config_test.go`:

```go
func TestLoadFile_MissingReturnsZero(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadFile(dir)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}
	if cfg.BranchPrefix != "" {
		t.Errorf("missing file BranchPrefix = %q, want empty", cfg.BranchPrefix)
	}
}

func TestLoad_EnvOverridesFileAndNormalizes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".worktrees"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "branch_prefix: \"file/\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".worktrees", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// File wins when no env.
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "file/" {
		t.Errorf("file BranchPrefix = %q, want %q", cfg.BranchPrefix, "file/")
	}

	// Env overrides file and is normalized (no trailing slash supplied).
	t.Setenv("WT_BRANCH_PREFIX", "env")
	cfg, err = Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "env/" {
		t.Errorf("env BranchPrefix = %q, want %q", cfg.BranchPrefix, "env/")
	}
}

func TestLoad_DefaultWhenNothingSet(t *testing.T) {
	t.Setenv("WT_BRANCH_PREFIX", "")
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "wt/" {
		t.Errorf("default BranchPrefix = %q, want %q", cfg.BranchPrefix, "wt/")
	}
}
```

(Ensure `config_test.go` imports `os` and `path/filepath` — add them if missing.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run 'LoadFile|Load_' -v`
Expected: FAIL — `LoadFile` undefined.

- [ ] **Step 3: Refactor Load into the Resolve-chain + add LoadFile and envCfg**

Replace the existing `Load` function in `internal/config/config.go` with:

```go
// LoadFile reads only <repoRoot>/.worktrees/config.yaml and returns the raw
// parsed config. A missing file yields a zero Config and a nil error. No
// defaults or env layering are applied — callers that want the resolved config
// use Load; --safe inspects the persisted value via LoadFile.
func LoadFile(repoRoot string) (Config, error) {
	path := filepath.Join(repoRoot, ".worktrees", "config.yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return Config{}, err
	}
	return fileCfg, nil
}

// envCfg reads the environment-variable config layer. Today only
// WT_BRANCH_PREFIX is supported; other fields stay file-only.
func envCfg() Config {
	return Config{BranchPrefix: os.Getenv("WT_BRANCH_PREFIX")}
}

// Load resolves configuration in precedence order (highest wins):
// WT_BRANCH_PREFIX env > .worktrees/config.yaml > built-in defaults. The
// resolved branch prefix is normalized to carry a single trailing slash.
func Load(repoRoot string) (Config, error) {
	fileCfg, err := LoadFile(repoRoot)
	if err != nil {
		return Defaults(), err
	}
	cfg := Resolve(Resolve(Defaults(), fileCfg), envCfg())
	cfg.BranchPrefix = NormalizePrefix(cfg.BranchPrefix)
	return cfg, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: PASS (all config tests, including pre-existing ones).

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): LoadFile + WT_BRANCH_PREFIX env layer with Resolve-chain"
```

---

## Task A3: Thread prefix through ConfigProvider, Manager, and naming

**Files:**
- Modify: `pkg/worktree/interfaces.go`
- Modify: `cmd/wt/root.go` (`cfgAdapter`)
- Modify: `pkg/worktree/fakes_test.go` (`fakeConfig`)
- Modify: `internal/naming/naming.go` (`SanitizeDir`)
- Modify: `internal/naming/naming_test.go`
- Modify: `pkg/worktree/manager.go` (`resolveNames`, `resolveWorktree`, `worktreePath`)
- Test: `pkg/worktree/manager_test.go`

- [ ] **Step 1: Write the failing tests**

Update the `SanitizeDir` test in `internal/naming/naming_test.go` to the new signature (find `TestSanitizeDir_StripsPrefixAndSlashes` and replace its body):

```go
func TestSanitizeDir_StripsPrefixAndSlashes(t *testing.T) {
	if got := SanitizeDir("wt/foo/bar", "wt/"); got != "foo-bar" {
		t.Errorf("SanitizeDir = %q, want %q", got, "foo-bar")
	}
	if got := SanitizeDir("feature/x", "feature/"); got != "x" {
		t.Errorf("SanitizeDir custom prefix = %q, want %q", got, "x")
	}
}
```

Add to `pkg/worktree/manager_test.go`:

```go
func TestResolveNames_CustomPrefix(t *testing.T) {
	m := New(&fakeGit{}, &fakeHooks{}, fakeConfig{branchPrefix: "feature/"})
	_, branch, err := m.resolveNames(AddOptions{Name: "thing"})
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/thing" {
		t.Errorf("branch = %q, want %q", branch, "feature/thing")
	}
}

func TestResolveNames_DefaultPrefixUnchanged(t *testing.T) {
	m := New(&fakeGit{}, &fakeHooks{}, fakeConfig{branchPrefix: "wt/"})
	_, branch, err := m.resolveNames(AddOptions{Name: "thing"})
	if err != nil {
		t.Fatal(err)
	}
	if branch != "wt/thing" {
		t.Errorf("branch = %q, want %q", branch, "wt/thing")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/naming/ ./pkg/worktree/ -run 'SanitizeDir|ResolveNames_' -v`
Expected: FAIL — `SanitizeDir` arg count mismatch and `fakeConfig.branchPrefix` undefined (compile errors).

- [ ] **Step 3: Implement the threading**

In `pkg/worktree/interfaces.go`, add to the `ConfigProvider` interface:

```go
	BranchPrefix() string // "" => caller falls back to default
```

In `cmd/wt/root.go`, add the adapter method (next to the other `cfgAdapter` methods):

```go
func (a cfgAdapter) BranchPrefix() string { return a.c.BranchPrefix }
```

In `pkg/worktree/fakes_test.go`, add the field and method:

```go
type fakeConfig struct {
	baseRef      string
	container    string
	nameTemplate string
	branchPrefix string
}

func (c fakeConfig) BranchPrefix() string { return c.branchPrefix }
```

In `internal/naming/naming.go`, change `SanitizeDir`:

```go
// SanitizeDir converts a branch-style name into a filesystem-safe directory
// name: strips the given branch prefix and replaces remaining slashes with "-".
func SanitizeDir(name, prefix string) string {
	name = strings.TrimPrefix(name, prefix)
	return strings.ReplaceAll(name, "/", "-")
}
```

In `pkg/worktree/manager.go`:

`resolveNames` — replace the `branch = "wt/" + ...` line:

```go
	prefix := m.cfg.BranchPrefix()
	branch = prefix + strings.TrimPrefix(base, prefix)
```

`worktreePath` — pass the prefix:

```go
func (m *Manager) worktreePath(repoRoot, branch string) string {
	return filepath.Join(m.containerPath(repoRoot), naming.SanitizeDir(branch, m.cfg.BranchPrefix()))
}
```

`resolveWorktree` — replace the `wantBranch := "refs/heads/wt/" + ...` line, and update the `SanitizeDir` call:

```go
	prefix := m.cfg.BranchPrefix()
	wantBranch := "refs/heads/" + prefix + strings.TrimPrefix(name, prefix)
	for _, w := range list {
		byDir := filepath.Base(w.Path) == naming.SanitizeDir(name, prefix)
```

- [ ] **Step 4: Run the full suite to verify it passes**

Run: `go test ./...`
Expected: PASS. (Pre-existing manager/naming tests that constructed `fakeConfig` without `branchPrefix` still compile — the new field defaults to `""`. If any pre-existing test asserts a `wt/` branch via a `fakeConfig` with an empty prefix, set its `branchPrefix: "wt/"`.)

- [ ] **Step 5: Commit**

```bash
git add pkg/worktree/ cmd/wt/root.go internal/naming/
git commit -m "feat(worktree): thread configured branch prefix through Manager and naming"
```

---

## Task A4: config.Set writer (line-based upsert)

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/config_test.go`:

```go
func TestSet_WritesAndReadsBack(t *testing.T) {
	dir := t.TempDir()
	if err := Set(dir, "branch_prefix", "feature"); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	cfg, err := LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "feature/" {
		t.Errorf("BranchPrefix = %q, want %q (normalized)", cfg.BranchPrefix, "feature/")
	}
}

func TestSet_UpdatesExistingAndPreservesComments(t *testing.T) {
	dir := t.TempDir()
	wt := filepath.Join(dir, ".worktrees")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# a helpful comment\nbranch_prefix: \"old/\"\n"
	if err := os.WriteFile(filepath.Join(wt, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Set(dir, "branch_prefix", "new"); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(wt, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# a helpful comment") {
		t.Errorf("comment not preserved:\n%s", s)
	}
	if strings.Contains(s, "old/") {
		t.Errorf("old value not replaced:\n%s", s)
	}
	if !strings.Contains(s, "branch_prefix: \"new/\"") {
		t.Errorf("new value missing:\n%s", s)
	}
}

func TestSet_RejectsUnknownKeyAndEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := Set(dir, "nope", "x"); err == nil {
		t.Error("expected error for unknown key")
	}
	if err := Set(dir, "branch_prefix", ""); err == nil {
		t.Error("expected error for empty value")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run 'TestSet_' -v`
Expected: FAIL — `Set` undefined.

- [ ] **Step 3: Implement Set with a line-based upsert**

Add to `internal/config/config.go` (add `"fmt"` and `"bufio"`... actually use `strings.Split`; add `"fmt"` to imports):

```go
// Set persists a single config key to <repoRoot>/.worktrees/config.yaml,
// creating the directory/file if absent. Only branch_prefix is supported today;
// an unknown key or empty value is rejected. The write is a line-based upsert:
// an existing top-level "<key>:" line is replaced, otherwise the line is
// appended — preserving any surrounding comments. This is sound only because
// every config key is a flat top-level scalar.
func Set(repoRoot, key, value string) error {
	if key != "branch_prefix" {
		return fmt.Errorf("unknown config key %q (supported: branch_prefix)", key)
	}
	value = NormalizePrefix(value)
	if value == "" {
		return fmt.Errorf("branch_prefix cannot be empty")
	}

	dir := filepath.Join(repoRoot, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "config.yaml")

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	line := fmt.Sprintf("%s: %q", key, value)
	out := upsertLine(string(existing), key, line)
	return os.WriteFile(path, []byte(out), 0o644)
}

// upsertLine replaces the first uncommented "<key>:" line in content, or appends
// the line if none exists. It preserves all other lines (including comments).
func upsertLine(content, key, line string) string {
	lines := strings.Split(content, "\n")
	prefix := key + ":"
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, prefix) {
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}
	// Append. Drop a single trailing empty element so we don't add blank lines.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	lines = append(lines, line, "")
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add Set() writer with comment-preserving line upsert"
```

---

## Task A5: `wt set` command + init template hint

**Files:**
- Create: `cmd/wt/set.go`
- Modify: `cmd/wt/init.go` (`configTemplate`)
- Test: `cmd/wt/set_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/wt/set_test.go`:

```go
package cmd

import (
	"path/filepath"
	"testing"

	"github.com/code-drill/wt/internal/config"
)

func TestRunSet_Writes(t *testing.T) {
	dir := t.TempDir()
	if err := runSet(dir, "branch_prefix", "feature", false); err != nil {
		t.Fatalf("runSet error: %v", err)
	}
	cfg, err := config.LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "feature/" {
		t.Errorf("BranchPrefix = %q, want %q", cfg.BranchPrefix, "feature/")
	}
}

func TestRunSet_SafeErrorsOnDifferent(t *testing.T) {
	dir := t.TempDir()
	if err := runSet(dir, "branch_prefix", "old", false); err != nil {
		t.Fatal(err)
	}
	if err := runSet(dir, "branch_prefix", "new", true); err == nil {
		t.Error("expected --safe to error on a different existing value")
	}
	// equal value is a no-op success under --safe
	if err := runSet(dir, "branch_prefix", "old", true); err != nil {
		t.Errorf("--safe with equal value should succeed, got %v", err)
	}
	_ = filepath.Separator
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/wt/ -run 'TestRunSet_' -v`
Expected: FAIL — `runSet` undefined.

- [ ] **Step 3: Implement the command and the testable helper**

Create `cmd/wt/set.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/code-drill/wt/internal/config"
)

var setSafe bool

// runSet applies a config write with optional --safe protection. Extracted from
// the cobra command for testing. With safe=true it refuses to overwrite an
// existing, different value (an equal value is a no-op success).
func runSet(repoRoot, key, value string, safe bool) error {
	if safe && key == "branch_prefix" {
		fileCfg, err := config.LoadFile(repoRoot)
		if err != nil {
			return err
		}
		existing := config.NormalizePrefix(fileCfg.BranchPrefix)
		want := config.NormalizePrefix(value)
		if existing != "" && existing != want {
			return fmt.Errorf("branch_prefix already set to %q; refusing to overwrite with %q (--safe)", existing, want)
		}
	}
	if err := config.Set(repoRoot, key, value); err != nil {
		return err
	}
	return nil
}

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value (e.g. branch_prefix)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := workdir()
		if err != nil {
			return err
		}
		repoRoot, err := repoRootFor(cwd)
		if err != nil {
			return err
		}
		key, value := args[0], args[1]
		if err := runSet(repoRoot, key, value, setSafe); err != nil {
			return err
		}
		fmt.Printf("Set %s = %q\n", key, config.NormalizePrefix(value))
		return nil
	},
}

func init() {
	setCmd.Flags().BoolVar(&setSafe, "safe", false, "error if the key already has a different value")
	rootCmd.AddCommand(setCmd)
}
```

Add the shared `repoRootFor` helper to `cmd/wt/root.go` (refactor `buildManager` to use it too, keeping behavior identical):

```go
// repoRootFor resolves the main repo root for cwd, after checking the git
// version. Shared by commands that need the repo root without a full Manager.
func repoRootFor(cwd string) (string, error) {
	r := git.New()
	if err := r.EnsureMinVersion(2, 30); err != nil {
		return "", err
	}
	root, err := r.MainRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotARepo, err)
	}
	return root, nil
}
```

Then in `buildManager`, replace the version-check + `MainRoot` block with:

```go
	r := git.New()
	repoRoot, err := repoRootFor(cwd)
	if err != nil {
		return nil, err
	}
```

(Keep the rest of `buildManager` — `config.Load`, `worktree.New`, `SetDigits` — unchanged. Note `r` is still used for `gitAdapter{r}`.)

In `cmd/wt/init.go`, add a hint line to `configTemplate` (after the `name_template` lines):

```go
# branch_prefix: "wt/"    # prefix for branches created by `wt new`
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/wt/ -run 'TestRunSet_' -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/set.go cmd/wt/set_test.go cmd/wt/init.go cmd/wt/root.go
git commit -m "feat(cli): add 'wt set <key> <value>' with --safe; init template hint"
```

---

## Task A6: Document the configurable prefix

**Files:**
- Modify: `README.md`
- Modify: `docs/FUNCTIONAL-REQUIREMENTS.md`

- [ ] **Step 1: Update the docs**

In `README.md`, in the configuration section, document: the `branch_prefix`
config key (default `wt/`), the `WT_BRANCH_PREFIX` environment variable, the
precedence (env > file > default), and the `wt set branch_prefix <value> [--safe]`
command. Note that the prefix gets a trailing `/` appended automatically.

In `docs/FUNCTIONAL-REQUIREMENTS.md`, add a requirement entry describing the
configurable branch prefix and the `wt set` command in the same style as the
surrounding entries.

- [ ] **Step 2: Verify the build/tests still pass**

Run: `go test ./...`
Expected: PASS (docs-only change).

- [ ] **Step 3: Commit**

```bash
git add README.md docs/FUNCTIONAL-REQUIREMENTS.md
git commit -m "docs: document configurable branch prefix and wt set"
```

---

# Part B — kill-em-all bulk cleanup

## Task B1: git `ListBranches(dir, prefix)`

**Files:**
- Modify: `internal/git/branch.go`
- Test: `internal/git/git_integration_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/git/git_integration_test.go` (follow the existing temp-repo
setup helpers in that file; the snippet below assumes a helper that inits a repo
and returns its dir — match whatever the file already uses, e.g. `newTestRepo(t)`):

```go
func TestListBranches_FiltersByPrefix(t *testing.T) {
	dir := newTestRepo(t) // existing helper: inits repo with an initial commit
	r := New()
	// Create branches: two prefixed, one not.
	if _, err := r.Run(dir, "branch", "wt/alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(dir, "branch", "wt/beta"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(dir, "branch", "feature/keep"); err != nil {
		t.Fatal(err)
	}
	got, err := r.ListBranches(dir, "wt/")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"wt/alpha": true, "wt/beta": true}
	if len(got) != 2 {
		t.Fatalf("ListBranches = %v, want 2 wt/ branches", got)
	}
	for _, b := range got {
		if !want[b] {
			t.Errorf("unexpected branch %q in result", b)
		}
	}
}
```

> If `git_integration_test.go` has no `newTestRepo` helper, use the same
> repo-bootstrap pattern the other tests in that file already use to create a
> temp repo with one commit, then create the branches as above.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/git/ -run TestListBranches_FiltersByPrefix -v`
Expected: FAIL — `ListBranches` undefined.

- [ ] **Step 3: Implement ListBranches**

Add to `internal/git/branch.go`:

```go
import "strings"

// ListBranches returns the short names of local branches whose name starts with
// prefix (e.g. "wt/"). An empty prefix matches all local branches.
func (r *Runner) ListBranches(dir, prefix string) ([]string, error) {
	out, err := r.Run(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+prefix+"*")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}
```

> Note: `branch.go` currently has no imports. Add the `import "strings"` line (or
> a grouped import block) at the top of the file below the `package git` line.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/git/ -run TestListBranches_FiltersByPrefix -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/git/branch.go internal/git/git_integration_test.go
git commit -m "feat(git): add ListBranches(dir, prefix) via for-each-ref"
```

---

## Task B2: Manager `PlanRemoveAll` + `RemoveAll` + types

**Files:**
- Modify: `pkg/worktree/types.go`
- Modify: `pkg/worktree/interfaces.go` (`GitRunner`)
- Modify: `cmd/wt/root.go` (`gitAdapter`)
- Modify: `pkg/worktree/fakes_test.go` (`fakeGit`)
- Modify: `pkg/worktree/manager.go`
- Test: `pkg/worktree/manager_test.go`

- [ ] **Step 1: Write the failing tests**

First, the new types and interface method are needed to compile the test. Add to
`pkg/worktree/types.go`:

```go
// RemoveAllPlan is the read-only preview of a kill-em-all operation.
type RemoveAllPlan struct {
	Worktrees []WorktreeInfo // non-main, in-container
	Branches  []string       // prefix-matching short names
}

// CleanupFailure records a single non-fatal failure during RemoveAll.
type CleanupFailure struct {
	Kind string // "worktree" | "branch"
	Ref  string // path or branch name
	Err  string
}

// RemoveAllResult summarizes a kill-em-all execution (best-effort).
type RemoveAllResult struct {
	WorktreesRemoved int
	BranchesDeleted  int
	Failures         []CleanupFailure
}
```

Add to the `GitRunner` interface in `pkg/worktree/interfaces.go`:

```go
	ListBranches(dir, prefix string) ([]string, error)
```

Wire it in `cmd/wt/root.go` (`gitAdapter`):

```go
func (a gitAdapter) ListBranches(d, p string) ([]string, error) { return a.r.ListBranches(d, p) }
```

Extend `fakeGit` in `pkg/worktree/fakes_test.go` with branch listing + failure
injection (add fields to the struct and methods):

```go
// add to fakeGit struct:
//   listBranches    []string
//   removeWtErr     map[string]error // keyed by path
//   deleteBranchErr map[string]error // keyed by branch
//   pruned          bool

func (f *fakeGit) ListBranches(_, _ string) ([]string, error) { return f.listBranches, nil }
```

Update the existing `fakeGit.RemoveWorktree`, `fakeGit.DeleteBranch`, and
`fakeGit.Prune` to honor injected errors / record prune (merge with current
bodies):

```go
func (f *fakeGit) RemoveWorktree(_, path string, _ bool) error {
	if f.removeWtErr != nil {
		if err := f.removeWtErr[path]; err != nil {
			return err
		}
	}
	// ... keep existing removal-tracking behavior ...
	return nil
}

func (f *fakeGit) DeleteBranch(_, branch string, force bool) (bool, error) {
	if f.deleteBranchErr != nil {
		if err := f.deleteBranchErr[branch]; err != nil {
			return false, err
		}
	}
	// ... keep existing behavior (return true,nil on force success) ...
	return true, nil
}

func (f *fakeGit) Prune(string) error { f.pruned = true; return nil }
```

> **Before writing the tests:** open `pkg/worktree/fakes_test.go` and confirm
> the exact field name `fakeGit` uses for its worktree list (its `ListWorktrees`
> returns that field) and for `mainRoot`. The tests below assume fields named
> `worktrees` and `mainRoot` — rename in the tests to match the actual fields.

Now add the tests to `pkg/worktree/manager_test.go`:

```go
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
		removeWtErr:     map[string]error{"/repo.worktrees/a": errFake},
		deleteBranchErr: map[string]error{"wt/b": errFake},
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
```

Add a shared fake error near the top of `manager_test.go` (if not already present):

```go
var errFake = errors.New("fake failure")
```

(Ensure `errors` is imported in `manager_test.go`.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/worktree/ -run 'RemoveAll' -v`
Expected: FAIL — `PlanRemoveAll`/`RemoveAll` undefined.

- [ ] **Step 3: Implement PlanRemoveAll and RemoveAll**

Add to `pkg/worktree/manager.go`:

```go
// PlanRemoveAll returns the read-only preview of a kill-em-all run: every
// non-main worktree in the container and every branch matching the configured
// prefix (including orphans with no worktree). It performs no mutation.
func (m *Manager) PlanRemoveAll(dir string) (RemoveAllPlan, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return RemoveAllPlan{}, err
	}
	list, err := m.List(dir)
	if err != nil {
		return RemoveAllPlan{}, err
	}
	var plan RemoveAllPlan
	for _, w := range list {
		if !w.IsMain {
			plan.Worktrees = append(plan.Worktrees, w)
		}
	}
	branches, err := m.git.ListBranches(repoRoot, m.cfg.BranchPrefix())
	if err != nil {
		return RemoveAllPlan{}, err
	}
	plan.Branches = branches
	return plan, nil
}

// RemoveAll force-removes every non-main container worktree and force-deletes
// every prefix-matching branch (orphans included), skipping lifecycle hooks. It
// is best-effort: a failure on one item is recorded and execution continues. A
// non-nil error is returned only for a fatal setup failure (e.g. planning). A
// final `git worktree prune` clears stale admin entries.
func (m *Manager) RemoveAll(dir string) (RemoveAllResult, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return RemoveAllResult{}, err
	}
	plan, err := m.PlanRemoveAll(dir)
	if err != nil {
		return RemoveAllResult{}, err
	}

	var res RemoveAllResult
	for _, w := range plan.Worktrees {
		if err := m.git.RemoveWorktree(repoRoot, w.Path, true); err != nil {
			res.Failures = append(res.Failures, CleanupFailure{Kind: "worktree", Ref: w.Path, Err: err.Error()})
			continue
		}
		res.WorktreesRemoved++
	}
	for _, b := range plan.Branches {
		if _, err := m.git.DeleteBranch(repoRoot, b, true); err != nil {
			res.Failures = append(res.Failures, CleanupFailure{Kind: "branch", Ref: b, Err: err.Error()})
			continue
		}
		res.BranchesDeleted++
	}
	if err := m.git.Prune(repoRoot); err != nil {
		res.Failures = append(res.Failures, CleanupFailure{Kind: "prune", Ref: repoRoot, Err: err.Error()})
	}
	return res, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/worktree/ -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add pkg/worktree/ cmd/wt/root.go
git commit -m "feat(worktree): add PlanRemoveAll + best-effort RemoveAll with tail prune"
```

---

## Task B3: `ErrPartialCleanup` sentinel + exit code

**Files:**
- Modify: `cmd/wt/errors.go`
- Test: `cmd/wt/errors_test.go` (create if absent; otherwise add to existing test file for this package)

- [ ] **Step 1: Write the failing test**

Add to `cmd/wt/errors_test.go` (create the file if it does not exist, package `cmd`):

```go
package cmd

import (
	"errors"
	"testing"
)

func TestExitCodeFor_PartialCleanup(t *testing.T) {
	err := errors.Join(ErrPartialCleanup, errors.New("2 failures"))
	if code := exitCodeFor(err); code != 6 {
		t.Errorf("exitCodeFor(ErrPartialCleanup) = %d, want 6", code)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./cmd/wt/ -run TestExitCodeFor_PartialCleanup -v`
Expected: FAIL — `ErrPartialCleanup` undefined.

- [ ] **Step 3: Add the sentinel and exit-code case**

In `cmd/wt/errors.go`, add to the sentinel block:

```go
	ErrPartialCleanup = errors.New("cleanup completed with failures")
```

Add a case to `exitCodeFor` (before `default`):

```go
	case errors.Is(err, ErrPartialCleanup):
		return 6
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./cmd/wt/ -run TestExitCodeFor_PartialCleanup -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/errors.go cmd/wt/errors_test.go
git commit -m "feat(cli): add ErrPartialCleanup sentinel mapped to exit code 6"
```

---

## Task B4: `wt kill-em-all` command

**Files:**
- Create: `cmd/wt/kill_em_all.go`
- Test: `cmd/wt/kill_em_all_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/wt/kill_em_all_test.go`:

```go
package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/code-drill/wt/pkg/worktree"
)

// fakeKiller implements the killer interface used by runKillEmAll.
type fakeKiller struct {
	plan worktree.RemoveAllPlan
	res  worktree.RemoveAllResult
}

func (f fakeKiller) PlanRemoveAll(string) (worktree.RemoveAllPlan, error) { return f.plan, nil }
func (f fakeKiller) RemoveAll(string) (worktree.RemoveAllResult, error)   { return f.res, nil }

func TestRunKillEmAll_EmptyPlan(t *testing.T) {
	var out bytes.Buffer
	k := fakeKiller{}
	err := runKillEmAll(k, "/repo", killOpts{yes: true, isTTY: false}, &out)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("nothing to remove")) {
		t.Errorf("output = %q, want 'nothing to remove'", out.String())
	}
}

func TestRunKillEmAll_RefusesWithoutYesNoTTY(t *testing.T) {
	var out bytes.Buffer
	k := fakeKiller{plan: worktree.RemoveAllPlan{Branches: []string{"wt/a"}}}
	err := runKillEmAll(k, "/repo", killOpts{yes: false, isTTY: false}, &out)
	if err == nil {
		t.Error("expected refusal error without --yes and no TTY")
	}
}

func TestRunKillEmAll_PartialFailureReturnsSentinel(t *testing.T) {
	var out bytes.Buffer
	k := fakeKiller{
		plan: worktree.RemoveAllPlan{Branches: []string{"wt/a"}},
		res:  worktree.RemoveAllResult{Failures: []worktree.CleanupFailure{{Kind: "branch", Ref: "wt/a", Err: "boom"}}},
	}
	err := runKillEmAll(k, "/repo", killOpts{yes: true, isTTY: false}, &out)
	if !errors.Is(err, ErrPartialCleanup) {
		t.Errorf("err = %v, want ErrPartialCleanup", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/wt/ -run TestRunKillEmAll_ -v`
Expected: FAIL — `runKillEmAll`, `killOpts`, `killer` undefined.

- [ ] **Step 3: Implement the command**

Create `cmd/wt/kill_em_all.go`:

```go
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/code-drill/wt/pkg/worktree"
)

var killYes bool

// killer is the subset of *worktree.Manager kill-em-all needs.
type killer interface {
	PlanRemoveAll(dir string) (worktree.RemoveAllPlan, error)
	RemoveAll(dir string) (worktree.RemoveAllResult, error)
}

type killOpts struct {
	yes   bool
	isTTY bool
	in    io.Reader // confirmation input (defaults to os.Stdin in the command)
}

// runKillEmAll drives the destructive cleanup with a confirmation gate.
// Extracted from the cobra command for testing. It writes user-facing output to
// out and returns ErrPartialCleanup when any item failed.
func runKillEmAll(k killer, repoRoot string, opts killOpts, out io.Writer) error {
	plan, err := k.PlanRemoveAll(repoRoot)
	if err != nil {
		return err
	}
	if len(plan.Worktrees) == 0 && len(plan.Branches) == 0 {
		fmt.Fprintln(out, "nothing to remove")
		return nil
	}

	fmt.Fprintln(out, "note: lifecycle hooks are skipped for kill-em-all")
	fmt.Fprintf(out, "This will force-remove %d worktree(s) and delete %d branch(es):\n",
		len(plan.Worktrees), len(plan.Branches))
	for _, w := range plan.Worktrees {
		fmt.Fprintf(out, "  worktree %s\n", w.Path)
	}
	for _, b := range plan.Branches {
		fmt.Fprintf(out, "  branch   %s\n", b)
	}

	if !opts.yes {
		if !opts.isTTY {
			return fmt.Errorf("refusing to run without --yes (no TTY for confirmation)")
		}
		fmt.Fprint(out, "Remove everything? [y/N]: ")
		reader := bufio.NewReader(opts.in)
		line, _ := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			// proceed
		default:
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	res, err := k.RemoveAll(repoRoot)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Removed %d worktree(s), deleted %d branch(es) (%d failed)\n",
		res.WorktreesRemoved, res.BranchesDeleted, len(res.Failures))
	for _, f := range res.Failures {
		fmt.Fprintf(out, "  FAILED %s %s: %s\n", f.Kind, f.Ref, f.Err)
	}
	if len(res.Failures) > 0 {
		return fmt.Errorf("%w: %d item(s)", ErrPartialCleanup, len(res.Failures))
	}
	return nil
}

var killEmAllCmd = &cobra.Command{
	Use:   "kill-em-all",
	Short: "Remove ALL worktrees and prefixed branches (destructive)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}
		opts := killOpts{
			yes:   killYes,
			isTTY: term.IsTerminal(int(os.Stdout.Fd())),
			in:    os.Stdin,
		}
		return runKillEmAll(m, cwd, opts, os.Stdout)
	},
}

func init() {
	killEmAllCmd.Flags().BoolVar(&killYes, "yes", false, "skip the confirmation prompt")
	rootCmd.AddCommand(killEmAllCmd)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/wt/ -run TestRunKillEmAll_ -v && go build ./...`
Expected: PASS and clean build. (`*worktree.Manager` satisfies `killer` via the
methods added in Task B2.)

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/kill_em_all.go cmd/wt/kill_em_all_test.go
git commit -m "feat(cli): add 'wt kill-em-all' with confirmation gate and summary"
```

---

## Task B5: TUI `K` confirm + dispatch

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/view.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/model_test.go` (match the existing test style in that file —
it already constructs a `model` and feeds `tea.KeyMsg`; reuse its helpers/fakes):

```go
func TestKillAll_KeyEntersConfirm(t *testing.T) {
	m := newModel(fakeLister{}, "/repo", []worktree.WorktreeInfo{
		{Path: "/repo", IsMain: true},
		{Path: "/repo.worktrees/a", Branch: "refs/heads/wt/a"},
	})
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})
	if updated.(model).mode != modeConfirmKillAll {
		t.Errorf("mode = %v, want modeConfirmKillAll", updated.(model).mode)
	}
}

func TestKillAll_ConfirmYesDispatches(t *testing.T) {
	var gotArgs []string
	m := newModel(fakeLister{}, "/repo", nil)
	m.mode = modeConfirmKillAll
	m.runAction = func(args ...string) tea.Cmd {
		gotArgs = args
		return nil
	}
	m.updateConfirmKillAll(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	want := []string{"kill-em-all", "--yes", "--repo", "/repo"}
	if len(gotArgs) != len(want) {
		t.Fatalf("args = %v, want %v", gotArgs, want)
	}
	for i := range want {
		if gotArgs[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, gotArgs[i], want[i])
		}
	}
}
```

> Match the lister fake name the existing `model_test.go` uses (e.g. it may have
> a `fakeLister` or build a `model` directly). Adjust the constructor call to the
> file's established helper if different.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run TestKillAll_ -v`
Expected: FAIL — `modeConfirmKillAll` / `updateConfirmKillAll` undefined.

- [ ] **Step 3: Implement the mode, key handling, and view**

In `internal/tui/model.go`, add the mode constant:

```go
const (
	modeNormal mode = iota
	modeConfirmDelete
	modeConfirmKillAll
)
```

Add a `K` case to `updateNormal` (inside the `switch msg.String()`):

```go
	case "K":
		m.mode = modeConfirmKillAll
		m.status = ""
```

Route the new mode in `Update` (in the `tea.KeyMsg` branch, extend the mode
check):

```go
	case tea.KeyMsg:
		switch m.mode {
		case modeConfirmDelete:
			return m.updateConfirm(msg)
		case modeConfirmKillAll:
			return m.updateConfirmKillAll(msg)
		}
		return m.updateNormal(msg)
```

Add the handler:

```go
func (m model) updateConfirmKillAll(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.mode = modeNormal
		m.status = "removing all worktrees…"
		return m, m.runAction("kill-em-all", "--yes", "--repo", m.dir)
	case "n", "N", "esc":
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}
```

In `internal/tui/view.go`, add a case to the mode `switch` for the prompt:

```go
	case modeConfirmKillAll:
		n := 0
		for _, it := range m.items {
			if !it.IsMain {
				n++
			}
		}
		b.WriteString(promptStyle.Render(
			fmt.Sprintf("Remove ALL %d worktrees and their branches? Hooks skipped. (y/n)", n)) + "\n")
```

And extend the default hint line to advertise the key:

```go
		b.WriteString("↑/↓ move • n new • d delete • K kill-all • q quit\n")
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/model.go internal/tui/view.go internal/tui/model_test.go
git commit -m "feat(tui): add K kill-all confirm screen that dispatches kill-em-all"
```

---

## Task B6: Document kill-em-all

**Files:**
- Modify: `README.md`
- Modify: `docs/FUNCTIONAL-REQUIREMENTS.md`

- [ ] **Step 1: Update the docs**

In `README.md`, document `wt kill-em-all [--yes]`: what it removes (all container
worktrees + all prefixed branches, including orphans), that it force-removes
regardless of committed state, that hooks are skipped, the TTY confirmation vs
`--yes`, the `K` TUI key, and the exit code (6) on partial failure.

In `docs/FUNCTIONAL-REQUIREMENTS.md`, add a requirement entry for the bulk
cleanup in the surrounding style.

- [ ] **Step 2: Verify the full suite passes**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add README.md docs/FUNCTIONAL-REQUIREMENTS.md
git commit -m "docs: document wt kill-em-all"
```

---

# Final verification

- [ ] **Run the whole suite and build:**

Run: `go test ./... && go build ./...`
Expected: all packages PASS, clean build.

- [ ] **Manual smoke (optional, in a throwaway repo):**

```bash
wt init
wt set branch_prefix feature
wt new                      # branch should be feature/<generated>
wt set branch_prefix other --safe   # should error (different existing value)
WT_BRANCH_PREFIX=env wt new # branch should be env/<generated>
wt kill-em-all              # prompts y/N, then removes everything
```
