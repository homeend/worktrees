# Named Templates + Worktrees-from-Branch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add named branch templates (`wt new -t <ref> k:v …`), worktree creation from an existing branch (`wt new --from-branch` / TUI `b`), and template listing (`wt templates` / TUI `t`).

**Architecture:** Templates live in `internal/config` and render through a new `naming.RenderTemplate`; the rendered string becomes `AddOptions.Name` so the existing prefix flow applies. `Manager.Add` gains a `FromBranch` path (verbatim existing branch, `git worktree add` without `-b`, hooks still run) while reusing the single hook transaction. The CLI grows `-t`/`--from-branch` with three-way mutual exclusivity (`--template`/`--from-branch`/`--branch`) and a `parseVars` helper; the TUI gains its first text-input mode plus a read-only templates view.

**Tech Stack:** Go 1.25, cobra, bubbletea/lipgloss, gopkg.in/yaml.v3, text/template.

**Spec:** `docs/superpowers/specs/2026-06-01-template-names-and-branch-worktrees-design.md`

**Conventions:** `go test ./...` (add `-tags integration` for git/cmd integration files). Commands are cobra subcommands; flags kebab-case. Tests use `t.Run`/`t.Errorf`. Worktree fakes live in `pkg/worktree/fakes_test.go` (unit) and `pkg/worktree/manager_integration_test.go` (integration).

---

## Task 1: config — `Template` type, `Templates` field, `Resolve` clause

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/config_test.go`:

```go
func TestLoad_ReadsTemplates(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "templates:\n  - name: autofix\n    template: \"autofix/{{.ticketName}}\"\n  - name: feature\n    template: \"feat/{{.ticketName}}\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Templates) != 2 {
		t.Fatalf("Templates = %+v, want 2", cfg.Templates)
	}
	if cfg.Templates[0].Name != "autofix" || cfg.Templates[0].Template != "autofix/{{.ticketName}}" {
		t.Errorf("Templates[0] = %+v", cfg.Templates[0])
	}
}

func TestResolve_TemplatesOverrideOnlyWhenSet(t *testing.T) {
	lo := Config{Templates: []Template{{Name: "a", Template: "a/{{.x}}"}}}
	if got := Resolve(lo, Config{}).Templates; len(got) != 1 {
		t.Errorf("empty hi should not clear templates, got %+v", got)
	}
	hi := Config{Templates: []Template{{Name: "b", Template: "b/{{.x}}"}}}
	if got := Resolve(lo, hi).Templates; len(got) != 1 || got[0].Name != "b" {
		t.Errorf("non-nil hi should override, got %+v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run 'Templates' -v`
Expected: FAIL — `Template` type and `Config.Templates` undefined.

- [ ] **Step 3: Implement the type, field, and Resolve clause**

In `internal/config/config.go`, add the type above `Config`:

```go
// Template is a named branch-name template (Go text/template, user variables).
type Template struct {
	Name     string `yaml:"name"`
	Template string `yaml:"template"`
}
```

Add the field to `Config` (after `BranchPrefix`):

```go
	Templates []Template `yaml:"templates"`
```

Add the override clause inside `Resolve` (before `return out`):

```go
	if hi.Templates != nil {
		out.Templates = hi.Templates
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add named branch Template type and templates key"
```

---

## Task 2: naming — `RenderTemplate`

**Files:**
- Modify: `internal/naming/naming.go`
- Test: `internal/naming/naming_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/naming/naming_test.go`:

```go
func TestRenderTemplate_Renders(t *testing.T) {
	got, err := RenderTemplate("autofix/{{.ticketName}}", map[string]string{"ticketName": "ZX-12"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "autofix/ZX-12" {
		t.Errorf("RenderTemplate = %q, want autofix/ZX-12", got)
	}
}

func TestRenderTemplate_MissingVarErrors(t *testing.T) {
	if _, err := RenderTemplate("{{.nope}}", map[string]string{}); err == nil {
		t.Error("missing variable should error")
	}
}

func TestRenderTemplate_InvalidTemplateErrors(t *testing.T) {
	if _, err := RenderTemplate("{{.x", nil); err == nil {
		t.Error("malformed template should error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/naming/ -run RenderTemplate -v`
Expected: FAIL — `RenderTemplate` undefined.

- [ ] **Step 3: Implement RenderTemplate**

In `internal/naming/naming.go`, add (the `text/template`, `strings`, `fmt`
imports already exist in this file):

```go
// RenderTemplate renders a user template against string variables, erroring on
// any referenced-but-missing variable (missingkey=error), mirroring GenerateFrom.
func RenderTemplate(tmpl string, vars map[string]string) (string, error) {
	t, err := template.New("tmpl").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("invalid template: %w", err)
	}
	var b strings.Builder
	if err := t.Execute(&b, vars); err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	return b.String(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/naming/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/naming/naming.go internal/naming/naming_test.go
git commit -m "feat(naming): add RenderTemplate with missingkey=error"
```

---

## Task 3: worktree — `Template` type, `ConfigProvider.Templates`, `Manager.ResolveTemplate`/`Templates`

**Files:**
- Modify: `pkg/worktree/types.go` (add `Template`; add `AddOptions.FromBranch`)
- Modify: `pkg/worktree/interfaces.go` (`ConfigProvider.Templates`)
- Modify: `cmd/wt/root.go` (`cfgAdapter.Templates`)
- Modify: `pkg/worktree/fakes_test.go` (`fakeConfig.Templates`)
- Modify: `pkg/worktree/manager_integration_test.go` (`staticCfg.Templates`)
- Modify: `pkg/worktree/manager.go` (`Templates`, `ResolveTemplate`)
- Test: `pkg/worktree/manager_test.go`

- [ ] **Step 1: Write the failing tests**

Add the type/field/interface first so tests compile. In `pkg/worktree/types.go`,
add near the top (after the package's existing types):

```go
// Template is a named branch-name template exposed to the CLI/TUI.
type Template struct {
	Name     string
	Template string
}
```

In `pkg/worktree/types.go`, add a field to `AddOptions`:

```go
	FromBranch string // when set: check out this existing branch instead of cutting a new one
```

In `pkg/worktree/interfaces.go`, add to `ConfigProvider`:

```go
	Templates() []Template
```

In `cmd/wt/root.go`, add the adapter method (next to other `cfgAdapter` methods):

```go
func (a cfgAdapter) Templates() []worktree.Template {
	out := make([]worktree.Template, len(a.c.Templates))
	for i, t := range a.c.Templates {
		out[i] = worktree.Template{Name: t.Name, Template: t.Template}
	}
	return out
}
```

In `pkg/worktree/fakes_test.go`, add a field + method to `fakeConfig`:

```go
// add to fakeConfig struct:
	templates []Template
// add method:
func (c fakeConfig) Templates() []Template { return c.templates }
```

In `pkg/worktree/manager_integration_test.go`, add to `staticCfg`:

```go
func (staticCfg) Templates() []Template { return nil }
```

Now add the tests to `pkg/worktree/manager_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/worktree/ -run 'ResolveTemplate|Templates_' -v`
Expected: FAIL — `ResolveTemplate`/`Templates` undefined.

- [ ] **Step 3: Implement Templates and ResolveTemplate**

In `pkg/worktree/manager.go`, add `"strconv"` to the import block, then add:

```go
// Templates returns the configured templates (for `wt templates` / the TUI).
func (m *Manager) Templates() []Template { return m.cfg.Templates() }

// ResolveTemplate finds a template by name or 1-based number and renders it with
// vars. The rendered string is intended to be used as AddOptions.Name (the
// prefix is applied by the normal Add flow). Unknown ref or a missing variable
// is an error.
func (m *Manager) ResolveTemplate(ref string, vars map[string]string) (string, error) {
	tmpls := m.cfg.Templates()
	tmpl := ""
	found := false
	if n, err := strconv.Atoi(ref); err == nil {
		if n >= 1 && n <= len(tmpls) {
			tmpl = tmpls[n-1].Template
			found = true
		}
	} else {
		for _, t := range tmpls {
			if t.Name == ref {
				tmpl = t.Template
				found = true
				break
			}
		}
	}
	if !found {
		return "", fmt.Errorf("unknown template %q", ref)
	}
	return naming.RenderTemplate(tmpl, vars)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/worktree/ -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add pkg/worktree/ cmd/wt/root.go
git commit -m "feat(worktree): expose Templates and ResolveTemplate (name or 1-based number)"
```

---

## Task 4: git — `AddWorktreeExisting`

**Files:**
- Modify: `internal/git/worktree.go`
- Modify: `pkg/worktree/interfaces.go` (`GitRunner`)
- Modify: `cmd/wt/root.go` (`gitAdapter`)
- Modify: `pkg/worktree/fakes_test.go` (`fakeGit`)
- Modify: `pkg/worktree/manager_integration_test.go` (`gitAdapter`)
- Test: `internal/git/git_integration_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/git/git_integration_test.go`:

```go
func TestAddWorktreeExisting_ChecksOutExistingBranch(t *testing.T) {
	r := New()
	repo := newTestRepo(t)
	if _, err := r.Run(repo, "branch", "feature/login"); err != nil {
		t.Fatalf("setup branch: %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "login")
	if err := r.AddWorktreeExisting(repo, wtPath, "feature/login"); err != nil {
		t.Fatalf("AddWorktreeExisting: %v", err)
	}
	list, err := r.ListWorktrees(repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range list {
		if w.Path == wtPath && w.Branch == "refs/heads/feature/login" {
			found = true
		}
	}
	if !found {
		t.Errorf("existing-branch worktree not in list: %+v", list)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags integration ./internal/git/ -run TestAddWorktreeExisting -v`
Expected: FAIL — `AddWorktreeExisting` undefined.

- [ ] **Step 3: Implement and wire it through interface + adapters + fakes**

In `internal/git/worktree.go`, add (after `AddWorktree`):

```go
// AddWorktreeExisting checks out an existing branch into a new worktree at path
// (git worktree add <path> <branch>, no -b).
func (r *Runner) AddWorktreeExisting(dir, path, branch string) error {
	_, err := r.Run(dir, "worktree", "add", path, branch)
	return err
}
```

In `pkg/worktree/interfaces.go`, add to `GitRunner` (after `AddWorktree`):

```go
	AddWorktreeExisting(dir, path, branch string) error
```

In `cmd/wt/root.go`, add to `gitAdapter`:

```go
func (a gitAdapter) AddWorktreeExisting(d, p, b string) error { return a.r.AddWorktreeExisting(d, p, b) }
```

In `pkg/worktree/fakes_test.go`, add a method to `fakeGit` (after `AddWorktree`):

```go
func (f *fakeGit) AddWorktreeExisting(_, path, branch string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, path)
	f.worktrees = append(f.worktrees, GitWorktree{Path: path, Branch: "refs/heads/" + branch})
	return nil
}
```

In `pkg/worktree/manager_integration_test.go`, add to its `gitAdapter`:

```go
func (a gitAdapter) AddWorktreeExisting(d, p, b string) error { return a.r.AddWorktreeExisting(d, p, b) }
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test -tags integration ./internal/git/ -run TestAddWorktreeExisting -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/git/ pkg/worktree/interfaces.go cmd/wt/root.go pkg/worktree/fakes_test.go pkg/worktree/manager_integration_test.go
git commit -m "feat(git): add AddWorktreeExisting (git worktree add without -b)"
```

---

## Task 5: Manager.Add — from-branch path

**Files:**
- Modify: `pkg/worktree/manager.go` (`Add`)
- Test: `pkg/worktree/manager_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `pkg/worktree/manager_test.go`:

```go
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
	if res.Name != "feature-login" {
		t.Errorf("name = %q, want feature-login (sanitized dir)", res.Name)
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/worktree/ -run 'Add_FromBranch' -v`
Expected: FAIL — from-branch is treated as a new branch (no `FromBranch` handling
yet), so the assertions about verbatim branch / missing-branch error fail.

- [ ] **Step 3: Rewrite Add to support the from-branch path**

Replace the existing `Add` function in `pkg/worktree/manager.go` with:

```go
// Add creates a new worktree following the create transaction:
// resolve+validate -> pre-create hook -> git worktree add -> post-create hook.
// With opts.FromBranch set, it checks out that existing branch instead of cutting
// a new one (the branch must already exist locally); base ref is unused and the
// shared hook transaction is reused. A pre-create failure aborts before anything
// is created. A post-create failure returns an error but leaves the worktree in
// place (no rollback, by design).
func (m *Manager) Add(dir string, opts AddOptions) (AddResult, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return AddResult{}, fmt.Errorf("resolve repo root: %w", err)
	}

	fromExisting := opts.FromBranch != ""
	var name, branch, baseRef string
	if fromExisting {
		branch = opts.FromBranch
		name = naming.SanitizeDir(branch, m.cfg.BranchPrefix())
	} else {
		name, branch, err = m.resolveNames(opts)
		if err != nil {
			return AddResult{}, err
		}
	}

	if err := m.git.CheckRefFormat(branch); err != nil {
		return AddResult{}, fmt.Errorf("invalid branch name %q: %w", branch, err)
	}

	if fromExisting {
		if !m.git.BranchExists(repoRoot, branch) {
			return AddResult{}, fmt.Errorf("branch %q does not exist locally", branch)
		}
	} else {
		if m.git.BranchExists(repoRoot, branch) {
			return AddResult{}, fmt.Errorf("branch %q already exists; pass a different --branch", branch)
		}
		baseRef = opts.BaseRef
		if baseRef == "" {
			baseRef = m.cfg.BaseRef()
		}
		if err := m.git.VerifyRef(repoRoot, baseRef); err != nil {
			return AddResult{}, fmt.Errorf("base ref %q not found: %w", baseRef, err)
		}
	}

	container := m.containerPath(repoRoot)
	target := m.worktreePath(repoRoot, branch)

	hc := HookContext{
		SourceRoot: repoRoot,
		TargetRoot: target,
		Name:       name,
		Branch:     branch,
		BaseRef:    baseRef,
		Container:  container,
		RepoName:   filepath.Base(repoRoot),
	}

	if !opts.NoHooks {
		pc := hc
		pc.Event = PreCreate
		pc.Cwd = repoRoot
		if err := m.hooks.Run(pc); err != nil {
			return AddResult{}, fmt.Errorf("pre-create hook failed (nothing created): %w", err)
		}
	}

	if fromExisting {
		if err := m.git.AddWorktreeExisting(repoRoot, target, branch); err != nil {
			return AddResult{}, fmt.Errorf("git worktree add: %w", err)
		}
	} else {
		if err := m.git.AddWorktree(repoRoot, target, branch, baseRef); err != nil {
			return AddResult{}, fmt.Errorf("git worktree add: %w", err)
		}
	}

	if !opts.NoHooks {
		poc := hc
		poc.Event = PostCreate
		poc.Cwd = target
		if err := m.hooks.Run(poc); err != nil {
			return AddResult{Name: name, Branch: branch, Path: target, BaseRef: baseRef},
				fmt.Errorf("post-create hook failed (worktree left in place at %s): %w", target, err)
		}
	}

	return AddResult{Name: name, Branch: branch, Path: target, BaseRef: baseRef}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/worktree/ -v`
Expected: PASS (new from-branch tests plus all existing Add tests).

- [ ] **Step 5: Commit**

```bash
git add pkg/worktree/manager.go pkg/worktree/manager_test.go
git commit -m "feat(worktree): Add supports FromBranch (checkout existing branch, hooks run)"
```

---

## Task 6: cmd/wt/new.go — `-t`, `--from-branch`, `parseVars`, mutual exclusivity

**Files:**
- Modify: `cmd/wt/new.go`
- Test: `cmd/wt/new_test.go` (create)

- [ ] **Step 1: Write the failing tests**

Create `cmd/wt/new_test.go`:

```go
package cmd

import (
	"testing"
)

type fakeResolver struct {
	name string
	err  error
}

func (f fakeResolver) ResolveTemplate(string, map[string]string) (string, error) {
	return f.name, f.err
}

func TestParseVars(t *testing.T) {
	vars, err := parseVars([]string{"a:1", "b:2:3"})
	if err != nil {
		t.Fatal(err)
	}
	if vars["a"] != "1" || vars["b"] != "2:3" {
		t.Errorf("vars = %+v", vars)
	}
	if _, err := parseVars([]string{"nocolon"}); err == nil {
		t.Error("missing colon should error")
	}
	if _, err := parseVars([]string{":v"}); err == nil {
		t.Error("empty key should error")
	}
}

func TestBuildAddOptions_Template(t *testing.T) {
	opts, err := buildAddOptions(fakeResolver{name: "autofix/ZX-12"},
		[]string{"ticketName:ZX-12"}, "autofix", "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Name != "autofix/ZX-12" {
		t.Errorf("Name = %q, want autofix/ZX-12", opts.Name)
	}
}

func TestBuildAddOptions_FromBranch(t *testing.T) {
	opts, err := buildAddOptions(fakeResolver{}, nil, "", "feature/login", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if opts.FromBranch != "feature/login" {
		t.Errorf("FromBranch = %q", opts.FromBranch)
	}
	if _, err := buildAddOptions(fakeResolver{}, []string{"x"}, "", "feature/login", "", "", false); err == nil {
		t.Error("--from-branch with a positional arg should error")
	}
}

func TestBuildAddOptions_MutualExclusion(t *testing.T) {
	if _, err := buildAddOptions(fakeResolver{}, nil, "autofix", "", "feat", "", false); err == nil {
		t.Error("--template + --branch should be mutually exclusive")
	}
	if _, err := buildAddOptions(fakeResolver{}, nil, "autofix", "feature/x", "", "", false); err == nil {
		t.Error("--template + --from-branch should be mutually exclusive")
	}
}

func TestBuildAddOptions_PlainName(t *testing.T) {
	opts, err := buildAddOptions(fakeResolver{}, []string{"hotfix"}, "", "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Name != "hotfix" {
		t.Errorf("Name = %q, want hotfix", opts.Name)
	}
	if _, err := buildAddOptions(fakeResolver{}, []string{"a", "b"}, "", "", "", "", false); err == nil {
		t.Error("more than one positional name should error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/wt/ -run 'TestParseVars|TestBuildAddOptions' -v`
Expected: FAIL — `parseVars`/`buildAddOptions`/`addResolver` undefined.

- [ ] **Step 3: Implement the helpers and rewire the command**

Replace the entire contents of `cmd/wt/new.go` with:

```go
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/pkg/worktree"
)

var (
	newBranch     string
	newBase       string
	newNoHooks    bool
	newTemplate   string
	newFromBranch string
)

// worktreeAddOptions builds AddOptions from flag values (extracted for testing).
func worktreeAddOptions(name, branch, base string, noHooks bool) worktree.AddOptions {
	return worktree.AddOptions{Name: name, Branch: branch, BaseRef: base, NoHooks: noHooks}
}

// addResolver renders a template ref into a branch name. *worktree.Manager
// satisfies it; a fake is used in tests.
type addResolver interface {
	ResolveTemplate(ref string, vars map[string]string) (string, error)
}

// parseVars turns ["k:v", ...] into a map. The value may contain colons (split
// on the first only). A missing colon or empty key is an error.
func parseVars(args []string) (map[string]string, error) {
	vars := make(map[string]string, len(args))
	for _, a := range args {
		k, v, ok := strings.Cut(a, ":")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid variable %q (expected name:value)", a)
		}
		vars[k] = v
	}
	return vars, nil
}

// buildAddOptions resolves new's flags/args into AddOptions. --template,
// --from-branch, and --branch are mutually exclusive (each defines the branch).
func buildAddOptions(r addResolver, args []string, tmpl, fromBranch, branch, base string, noHooks bool) (worktree.AddOptions, error) {
	set := 0
	for _, s := range []string{tmpl, fromBranch, branch} {
		if s != "" {
			set++
		}
	}
	if set > 1 {
		return worktree.AddOptions{}, fmt.Errorf("--template, --from-branch, and --branch are mutually exclusive")
	}

	opts := worktreeAddOptions("", branch, base, noHooks)
	switch {
	case fromBranch != "":
		if len(args) > 0 {
			return worktree.AddOptions{}, fmt.Errorf("--from-branch takes no positional arguments")
		}
		opts.FromBranch = fromBranch
	case tmpl != "":
		vars, err := parseVars(args)
		if err != nil {
			return worktree.AddOptions{}, err
		}
		name, err := r.ResolveTemplate(tmpl, vars)
		if err != nil {
			return worktree.AddOptions{}, err
		}
		opts.Name = name
	default:
		if len(args) > 1 {
			return worktree.AddOptions{}, fmt.Errorf("expected at most one name argument, got %d", len(args))
		}
		if len(args) == 1 {
			opts.Name = args[0]
		}
	}
	return opts, nil
}

var newCmd = &cobra.Command{
	Use:   "new [name | var:value ...]",
	Short: "Create a new worktree",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}
		opts, err := buildAddOptions(m, args, newTemplate, newFromBranch, newBranch, newBase, newNoHooks)
		if err != nil {
			return err
		}
		res, err := m.Add(cwd, opts)
		if err != nil {
			return err
		}
		fmt.Printf("Created worktree %q\n  branch: %s\n  path:   %s\n", res.Name, res.Branch, res.Path)
		return nil
	},
}

func init() {
	newCmd.Flags().StringVarP(&newBranch, "branch", "b", "", "branch name (default: derived from name)")
	newCmd.Flags().StringVar(&newBase, "base", "", "base ref to branch from (default: config base_ref / HEAD)")
	newCmd.Flags().BoolVar(&newNoHooks, "no-hooks", false, "skip lifecycle hooks")
	newCmd.Flags().StringVarP(&newTemplate, "template", "t", "", "render the branch from a named/numbered template")
	newCmd.Flags().StringVar(&newFromBranch, "from-branch", "", "create a worktree from an existing local branch")
	rootCmd.AddCommand(newCmd)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/wt/ -v && go build ./...`
Expected: PASS (incl. the existing `cli_integration_test.go` callers of
`worktreeAddOptions`, whose signature is unchanged) and clean build.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/new.go cmd/wt/new_test.go
git commit -m "feat(cli): wt new gains -t/--template and --from-branch with mutual exclusivity"
```

---

## Task 7: cmd/wt/templates.go — `wt templates`

**Files:**
- Create: `cmd/wt/templates.go`
- Test: `cmd/wt/templates_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/wt/templates_test.go`:

```go
package cmd

import (
	"bytes"
	"testing"

	"github.com/homeend/worktrees/pkg/worktree"
)

func TestPrintTemplates_Lists(t *testing.T) {
	var out bytes.Buffer
	err := printTemplates(&out, []worktree.Template{
		{Name: "autofix", Template: "autofix/{{.ticketName}}"},
		{Name: "feature", Template: "feat/{{.ticketName}}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !bytes.Contains(out.Bytes(), []byte("autofix")) || !bytes.Contains(out.Bytes(), []byte("feat/{{.ticketName}}")) {
		t.Errorf("output missing templates:\n%s", s)
	}
	if !bytes.Contains(out.Bytes(), []byte("1")) || !bytes.Contains(out.Bytes(), []byte("2")) {
		t.Errorf("output missing 1-based indices:\n%s", s)
	}
}

func TestPrintTemplates_Empty(t *testing.T) {
	var out bytes.Buffer
	if err := printTemplates(&out, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("no templates defined")) {
		t.Errorf("empty output = %q", out.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/wt/ -run TestPrintTemplates -v`
Expected: FAIL — `printTemplates` undefined.

- [ ] **Step 3: Implement the command**

Create `cmd/wt/templates.go`:

```go
package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/pkg/worktree"
)

// printTemplates writes a tab-aligned table of templates (extracted for testing).
func printTemplates(out io.Writer, tmpls []worktree.Template) error {
	if len(tmpls) == 0 {
		fmt.Fprintln(out, "no templates defined")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "#\tNAME\tTEMPLATE")
	for i, t := range tmpls {
		fmt.Fprintf(w, "%d\t%s\t%s\n", i+1, t.Name, t.Template)
	}
	return w.Flush()
}

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "List configured branch templates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, _, err := managerForWorkdir()
		if err != nil {
			return err
		}
		return printTemplates(os.Stdout, m.Templates())
	},
}

func init() { rootCmd.AddCommand(templatesCmd) }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/wt/ -run TestPrintTemplates -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/templates.go cmd/wt/templates_test.go
git commit -m "feat(cli): add 'wt templates' to list configured templates"
```

---

## Task 8: TUI — `b` from-branch text input

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/view.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/model_test.go`:

```go
func TestFromBranch_KeyEntersInput(t *testing.T) {
	m, _ := newTestModel(sample())
	updated, _ := m.Update(key("b"))
	if updated.(model).mode != modeInputBranch {
		t.Errorf("mode = %v, want modeInputBranch", updated.(model).mode)
	}
}

func TestFromBranch_TypeAndEnterDispatches(t *testing.T) {
	m, rec := newTestModel(sample())
	cur := tea.Model(m)
	cur, _ = cur.Update(key("b"))
	for _, ch := range []string{"f", "e", "a", "t"} {
		cur, _ = cur.Update(key(ch))
	}
	done, cmd := cur.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should dispatch an action")
	}
	if done.(model).mode != modeNormal {
		t.Errorf("mode should return to normal after Enter")
	}
	if len(*rec) != 1 || (*rec)[0] != "new --from-branch feat --repo /repo" {
		t.Errorf("runAction = %v, want [new --from-branch feat --repo /repo]", *rec)
	}
}

func TestFromBranch_EscCancels(t *testing.T) {
	m, rec := newTestModel(sample())
	cur, _ := m.Update(key("b"))
	cur, _ = cur.(model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cur.(model).mode != modeNormal {
		t.Errorf("Esc should cancel to normal mode")
	}
	if len(*rec) != 0 {
		t.Errorf("cancel should run no action, got %v", *rec)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run TestFromBranch -v`
Expected: FAIL — `modeInputBranch` undefined.

- [ ] **Step 3: Implement the input mode**

In `internal/tui/model.go`, add `"strings"` to the imports if not present, then
add the input mode to the const block (`modeTemplates` is added in Task 9):

```go
const (
	modeNormal mode = iota
	modeConfirmDelete
	modeConfirmKillAll
	modeInputBranch
)
```

Add an `input` field to the `model` struct (next to `status`):

```go
	input  string
```

Route the input mode in `Update`'s `tea.KeyMsg` branch (extend the switch):

```go
	case tea.KeyMsg:
		switch m.mode {
		case modeConfirmDelete:
			return m.updateConfirm(msg)
		case modeConfirmKillAll:
			return m.updateConfirmKillAll(msg)
		case modeInputBranch:
			return m.updateInputBranch(msg)
		}
		return m.updateNormal(msg)
```

Add a `b` case to `updateNormal` (inside its `switch msg.String()`):

```go
	case "b":
		m.mode = modeInputBranch
		m.input = ""
		m.status = ""
```

Add the handler:

```go
func (m model) updateInputBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		branch := strings.TrimSpace(m.input)
		m.mode = modeNormal
		m.input = ""
		if branch == "" {
			return m, nil
		}
		m.status = "creating from " + branch + "…"
		return m, m.runAction("new", "--from-branch", branch, "--repo", m.dir)
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input = ""
		return m, nil
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyRunes:
		m.input += string(msg.Runes)
		return m, nil
	}
	return m, nil
}
```

In `internal/tui/view.go`, add an input-mode case to the mode `switch` (before
`default`):

```go
	case modeInputBranch:
		b.WriteString(promptStyle.Render("New worktree from branch (Enter create, Esc cancel):") + "\n")
		b.WriteString("  " + m.input + "_\n")
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run TestFromBranch -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/model.go internal/tui/view.go internal/tui/model_test.go
git commit -m "feat(tui): add 'b' to create a worktree from an existing branch"
```

---

## Task 9: TUI — `t` templates view (+ pass templates into the model)

**Files:**
- Modify: `internal/tui/model.go` (`model` field, `newModel`, `updateNormal`, `updateTemplates`)
- Modify: `internal/tui/tui.go` (pass templates)
- Modify: `internal/tui/view.go` (render templates + hint line)
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/model_test.go`:

```go
func TestTemplates_KeyShowsList(t *testing.T) {
	m, _ := newTestModel(sample())
	m.templates = []worktree.Template{{Name: "autofix", Template: "autofix/{{.t}}"}}
	updated, _ := m.Update(key("t"))
	mm := updated.(model)
	if mm.mode != modeTemplates {
		t.Fatalf("mode = %v, want modeTemplates", mm.mode)
	}
	if !strings.Contains(mm.View(), "autofix") {
		t.Errorf("templates view should list 'autofix':\n%s", mm.View())
	}
}

func TestTemplates_AnyKeyReturns(t *testing.T) {
	m, _ := newTestModel(sample())
	m.templates = []worktree.Template{{Name: "autofix", Template: "autofix/{{.t}}"}}
	shown, _ := m.Update(key("t"))
	back, _ := shown.(model).Update(key("x"))
	if back.(model).mode != modeNormal {
		t.Errorf("any key should return to normal mode")
	}
}
```

Update the two existing `newModel(...)` call sites in this file to pass a
templates arg (`nil`):

- in `newTestModel`: `m := newModel(&fakeLister{items: items}, "/repo", items, nil)`
- in `TestView_EmptyListStillRenders`: `m := newModel(&fakeLister{}, "/repo", nil, nil)`

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestTemplates_' -v`
Expected: FAIL — `model.templates`/`modeTemplates` handling and the new
`newModel` arity are missing (compile error until Step 3).

- [ ] **Step 3: Implement the templates view and model wiring**

In `internal/tui/model.go`, add the `modeTemplates` const to the const block
(after `modeInputBranch`):

```go
	modeTemplates
```

Add the `modeTemplates` routing case to `Update`'s mode switch (after the
`modeInputBranch` case):

```go
		case modeTemplates:
			return m.updateTemplates(msg)
```

Add to the `model` struct (next to `input`):

```go
	templates []worktree.Template
```

Change `newModel` to accept templates:

```go
func newModel(store lister, dir string, items []worktree.WorktreeInfo, templates []worktree.Template) model {
	return model{store: store, dir: dir, items: items, templates: templates, runAction: defaultRunAction}
}
```

Add a `t` case to `updateNormal` (inside its `switch msg.String()`):

```go
	case "t":
		m.mode = modeTemplates
		m.status = ""
```

Add the handler:

```go
func (m model) updateTemplates(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	m.mode = modeNormal
	return m, nil
}
```

In `internal/tui/tui.go`, pass templates into the model:

```go
	p := tea.NewProgram(newModel(m, dir, items, m.Templates()), tea.WithAltScreen())
```

In `internal/tui/view.go`, add a templates-mode case to the mode `switch` (before
`default`):

```go
	case modeTemplates:
		b.WriteString(titleStyle.Render("Templates") + "\n")
		if len(m.templates) == 0 {
			b.WriteString("  (none defined)\n")
		}
		for i, tpl := range m.templates {
			b.WriteString(fmt.Sprintf("  %d  %s  %s\n", i+1, tpl.Name, tpl.Template))
		}
		b.WriteString("\n" + statusStyle.Render("press any key to return") + "\n")
```

Update the `default` hint line to advertise the new keys:

```go
		b.WriteString("↑/↓ move • n new • b from-branch • t templates • d delete • K kill-all • q quit\n")
```

> Note: `tui.Run` already holds the `*worktree.Manager` as `m`, which satisfies
> the `lister` interface and also exposes `Templates()` — no interface widening.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/model.go internal/tui/tui.go internal/tui/view.go internal/tui/model_test.go
git commit -m "feat(tui): add 't' to view configured templates"
```

---

## Task 10: Docs + init template hint

**Files:**
- Modify: `cmd/wt/init.go` (config template)
- Modify: `README.md`
- Modify: `docs/FUNCTIONAL-REQUIREMENTS.md`

- [ ] **Step 1: Add the init template hint**

In `cmd/wt/init.go`, append to `configTemplate` (after the `branch_prefix` line):

```go
# templates:               # named branch templates for `wt new -t <name|number>`
#   - name: autofix        #   vars are Go text/template fields, e.g. {{.ticketName}}
#     template: "autofix/{{.ticketName}}"
```

- [ ] **Step 2: Update README**

In `README.md`:
- In the **Commands** block, add `wt templates` and document `wt new`'s new flags
  (`-t/--template <ref>`, `--from-branch <branch>`).
- Add a **Templates** subsection under Config: how to define `templates:`, how to
  select with `wt new -t <name|number> k:v …`, that variables are
  `{{.name}}` Go-template fields rendered into the prefix (example: template
  `autofix/{{.ticketName}}` + prefix `mrutkowski/` + `ticketName:ZX-12` →
  `mrutkowski/autofix/ZX-12`), and that `wt templates` lists them.
- Document **new from a branch**: `wt new --from-branch <branch>` (and TUI `b`)
  checks out an existing local branch and runs hooks; note `--template`,
  `--from-branch`, `--branch` are mutually exclusive.
- In the **TUI keys** table, add `b` (from-branch) and `t` (templates).

- [ ] **Step 3: Update FUNCTIONAL-REQUIREMENTS**

In `docs/FUNCTIONAL-REQUIREMENTS.md`:
- §4 Naming & branching: add **FR-4.7 (templates)** — `wt new -t <name|1-based
  number>` renders a configured template (Go `text/template`, user vars via
  `name:value`, missing var errors) and prepends the prefix; `--template`,
  `--from-branch`, `--branch` are mutually exclusive.
- §4: add **FR-4.8 (from existing branch)** — `wt new --from-branch <branch>`
  checks out an existing **local** branch into a new worktree (error if it
  doesn't exist locally) and runs lifecycle hooks; no new branch is created.
- §5 CLI commands: add **FR-5.11 `wt templates`** — list configured templates
  (index, name, template; "no templates defined" when empty).
- §9 Configuration: add `templates` to FR-9.2's key list (a list of
  `{name, template}`).
- §10 TUI: add **FR-10.10 (`b`)** from-branch text input and **FR-10.11 (`t`)**
  templates view (read-only).
- Update the "Last verified" date to `2026-06-01`.

- [ ] **Step 4: Verify the full suite**

Run: `go test ./... && go test -tags integration ./... && go build ./...`
Expected: all PASS, clean build.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/init.go README.md docs/FUNCTIONAL-REQUIREMENTS.md
git commit -m "docs: document templates and worktrees-from-branch"
```

---

# Final verification

- [ ] **Run everything:**

Run: `go test ./... && go test -tags integration ./... && go build ./...`
Expected: all packages PASS, clean build.

- [ ] **Manual smoke (throwaway repo):**

```bash
wt init
printf 'templates:\n  - name: autofix\n    template: "autofix/{{.ticketName}}"\n' >> .worktrees/config.yaml
wt set branch_prefix mrutkowski
wt templates                              # lists: 1 autofix autofix/{{.ticketName}}
wt new -t autofix ticketName:ZX-12        # branch mrutkowski/autofix/ZX-12
wt new -t 1 ticketName:ZX-13              # same template by number
git branch existing-thing
wt new --from-branch existing-thing       # worktree on existing branch, hooks run
wt new -t autofix --branch x ticketName:Y # error: mutually exclusive
```
