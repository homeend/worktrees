# `wt` Git Worktree Utility — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `wt`, a Go CLI + TUI utility that creates, lists, and removes git worktrees in a sibling container with lifecycle hooks, backed by a reusable `pkg/worktree` library.

**Architecture:** A disciplined git-CLI wrapper (`internal/git`) underpins a `Manager` (`pkg/worktree`) that depends only on injected interfaces (`GitRunner`, `HookRunner`, `ConfigProvider`). Default implementations live in `internal/*`; a cobra CLI and a Bubble Tea TUI in `cmd/wt` + `internal/tui` wire them together.

**Tech Stack:** Go 1.23 (module directive; toolchain 1.26 present), cobra (CLI), Bubble Tea + Lipgloss (TUI), `gopkg.in/yaml.v3` (config), git 2.30+ CLI (shelled out), standard `testing` package.

**Reference spec:** `docs/superpowers/specs/2026-05-31-wt-git-worktree-utility-design.md`

**Process (per spec §11–12):** Every package ships unit AND integration tests. Each phase ends with a mandatory review gate — see "Phase Review Gate" task at the end of every phase. No phase advances until tests are green and all three reviews (code, test, architectural) pass.

---

## File Structure

```
go.mod                              module github.com/homeend/worktrees
go.sum
main.go                             thin entrypoint -> cmd/wt.Execute()

pkg/worktree/
  types.go                          WorktreeInfo, AddOptions, RemoveOptions, result types
  interfaces.go                     GitRunner, HookRunner, ConfigProvider interfaces
  manager.go                        Manager: List, Add, Remove (create/remove transactions)
  manager_test.go                   unit tests with fakes
  manager_integration_test.go       temp-repo integration tests (build tag: integration)
  fakes_test.go                     fake GitRunner/HookRunner/ConfigProvider for unit tests

internal/git/
  git.go                            Runner: exec.Command wrapper, LC_ALL=C, version probe
  resolve.go                        CommonDir, MainRoot, TopLevel, VerifyRef, CheckRefFormat
  worktree.go                       Add, Remove, List, Prune
  branch.go                         DeleteBranch (safe + force), BranchExists
  parse.go                          parsePorcelainZ for `worktree list --porcelain -z`
  git_test.go                       unit tests for parse.go (pure)
  git_integration_test.go           temp-repo integration tests (build tag: integration)

internal/config/
  config.go                         Config struct, Load, Resolve (flags > file > defaults)
  config_test.go                    fixture-based unit tests

internal/naming/
  naming.go                         Generate() date-first adj-noun-NNNN; SanitizeDir
  words.go                          embedded adjective/noun wordlists
  naming_test.go                    unit tests incl. ref-format validity

internal/hooks/
  hooks.go                          Runner: discover, build env, run with cwd, stream output
  hooks_test.go                     unit + integration tests incl. failing-hook cases

internal/tui/
  model.go                          Bubble Tea model over Manager
  view.go                           Lipgloss rendering
  tui.go                            Run(manager) entrypoint
  model_test.go                     unit tests for update/state transitions

cmd/wt/
  root.go                           cobra root; bare->TUI TTY guard; wiring; exit codes
  errors.go                         error taxonomy + exit code mapping
  new.go                            `wt new`
  list.go                           `wt list`/`ls` (+ --json)
  rm.go                             `wt rm`
  prune.go                          `wt prune`
  path.go                           `wt path`
  init.go                           `wt init` scaffolding (embedded stub templates)
  completion.go                     `wt completion`
  root_test.go                      CLI integration tests (exit codes, wiring)
```

---

## Phase 0: Project Scaffold

### Task 0.1: Initialize Go module and entrypoint

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/wt/root.go` (minimal placeholder)

- [ ] **Step 1: Create the Go module**

Run:
```bash
cd /home/homeend/agentos
go mod init github.com/homeend/worktrees
go mod edit -go=1.23
```
Expected: `go.mod` created with `module github.com/homeend/worktrees` and `go 1.23`.

- [ ] **Step 2: Create a minimal cobra root so the module compiles**

Create `cmd/wt/root.go`:
```go
package cmd

import (
	"github.com/spf13/cobra"
)

// rootCmd is the base command. Subcommands are registered in init() funcs
// across the package. Bare invocation will later launch the TUI (Phase 7).
var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "Fast git worktree management with lifecycle hooks",
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		return 1
	}
	return 0
}
```

Create `main.go`:
```go
package main

import (
	"os"

	cmd "github.com/homeend/worktrees/cmd/wt"
)

func main() {
	os.Exit(cmd.Execute())
}
```

- [ ] **Step 3: Add cobra dependency and build**

Run:
```bash
go get github.com/spf13/cobra@latest
go build ./...
```
Expected: builds with no errors; `go.mod`/`go.sum` updated with cobra.

- [ ] **Step 4: Verify the binary runs**

Run:
```bash
go run . --help
```
Expected: cobra prints usage for `wt` with the short description.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum main.go cmd/wt/root.go
git commit -m "chore: scaffold Go module, cobra root, and entrypoint"
```

---

## Phase 1: `internal/git` — the git CLI wrapper (highest risk)

This phase is built first and made bulletproof. The porcelain parser is pure and unit-tested; everything else is integration-tested against real temp repos.

### Task 1.1: Porcelain `-z` parser (pure, unit-tested first)

**Files:**
- Create: `internal/git/parse.go`
- Test: `internal/git/git_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/git/git_test.go`:
```go
package git

import "testing"

func TestParsePorcelainZ_BasicAndDetachedAndBare(t *testing.T) {
	// Records separated by NUL-NUL; attributes separated by single NUL.
	// Mirrors `git worktree list --porcelain -z` output.
	input := "worktree /repo\x00HEAD abc123\x00branch refs/heads/main\x00\x00" +
		"worktree /repo.worktrees/feat\x00HEAD def456\x00detached\x00\x00" +
		"worktree /bare\x00bare\x00\x00"

	got, err := parsePorcelainZ([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 worktrees, got %d", len(got))
	}

	if got[0].Path != "/repo" || got[0].HEAD != "abc123" || got[0].Branch != "refs/heads/main" {
		t.Errorf("record 0 wrong: %+v", got[0])
	}
	if got[1].Branch != "" || !got[1].Detached {
		t.Errorf("record 1 should be detached with no branch: %+v", got[1])
	}
	if !got[2].Bare {
		t.Errorf("record 2 should be bare: %+v", got[2])
	}
}

func TestParsePorcelainZ_LockedAndPrunableWithReasons(t *testing.T) {
	input := "worktree /a\x00HEAD a1\x00branch refs/heads/x\x00locked needs disk\x00\x00" +
		"worktree /b\x00HEAD b1\x00branch refs/heads/y\x00prunable gitdir gone\x00\x00"

	got, err := parsePorcelainZ([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got[0].Locked || got[0].LockedReason != "needs disk" {
		t.Errorf("record 0 lock wrong: %+v", got[0])
	}
	if !got[1].Prunable || got[1].PrunableReason != "gitdir gone" {
		t.Errorf("record 1 prunable wrong: %+v", got[1])
	}
}

func TestParsePorcelainZ_PathWithSpaces(t *testing.T) {
	input := "worktree /home/me/my repo.worktrees/cool feature\x00HEAD a1\x00branch refs/heads/z\x00\x00"
	got, err := parsePorcelainZ([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Path != "/home/me/my repo.worktrees/cool feature" {
		t.Errorf("path with spaces mangled: %q", got[0].Path)
	}
}

func TestParsePorcelainZ_Empty(t *testing.T) {
	got, err := parsePorcelainZ([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 worktrees, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/git/ -run TestParsePorcelainZ -v`
Expected: FAIL — `parsePorcelainZ` and the `WorktreeInfo` type are undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/git/parse.go`:
```go
package git

import "strings"

// WorktreeInfo is one entry from `git worktree list --porcelain -z`.
type WorktreeInfo struct {
	Path           string
	HEAD           string
	Branch         string // refs/heads/...; empty for bare or detached
	Bare           bool
	Detached       bool
	Locked         bool
	LockedReason   string
	Prunable       bool
	PrunableReason string
}

// parsePorcelainZ parses NUL-delimited porcelain output. Records are separated
// by a blank line, which in -z form is an empty attribute (i.e. "\x00\x00").
func parsePorcelainZ(data []byte) ([]WorktreeInfo, error) {
	s := string(data)
	var out []WorktreeInfo
	var cur *WorktreeInfo

	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}

	for _, attr := range strings.Split(s, "\x00") {
		if attr == "" { // record boundary (or trailing terminator)
			flush()
			continue
		}
		key, val, _ := strings.Cut(attr, " ")
		switch key {
		case "worktree":
			flush()
			cur = &WorktreeInfo{Path: val}
		case "HEAD":
			if cur != nil {
				cur.HEAD = val
			}
		case "branch":
			if cur != nil {
				cur.Branch = val
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		case "locked":
			if cur != nil {
				cur.Locked = true
				cur.LockedReason = val
			}
		case "prunable":
			if cur != nil {
				cur.Prunable = true
				cur.PrunableReason = val
			}
		}
	}
	flush()
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/git/ -run TestParsePorcelainZ -v`
Expected: PASS (all four tests).

- [ ] **Step 5: Commit**

```bash
git add internal/git/parse.go internal/git/git_test.go
git commit -m "feat(git): porcelain -z worktree list parser with unit tests"
```

### Task 1.2: The command Runner with `LC_ALL=C` and version probe

**Files:**
- Create: `internal/git/git.go`
- Test: `internal/git/git_integration_test.go`

- [ ] **Step 1: Write the failing integration test**

Create `internal/git/git_integration_test.go`:
```go
//go:build integration

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newTestRepo creates a throwaway git repo with one commit and returns its path.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestRunner_VersionAndRun(t *testing.T) {
	r := New()
	repo := newTestRepo(t)

	out, err := r.Run(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if got := string(out); got != "main\n" {
		t.Errorf("want branch main, got %q", got)
	}

	v, err := r.Version()
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}
	if v.Major < 2 {
		t.Errorf("unexpected git major version: %+v", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./internal/git/ -run TestRunner -v`
Expected: FAIL — `New`, `Run`, `Version` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/git/git.go`:
```go
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Runner executes git commands with a stable, parseable environment.
type Runner struct {
	bin string
}

// New returns a Runner that invokes the `git` binary on PATH.
func New() *Runner { return &Runner{bin: "git"} }

// Run executes git in dir and returns stdout. On non-zero exit it returns an
// error whose message includes stderr. LC_ALL=C makes messages locale-stable.
func (r *Runner) Run(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(r.bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// Version is a parsed git version.
type Version struct {
	Major, Minor, Patch int
}

// Version probes `git --version`.
func (r *Runner) Version() (Version, error) {
	out, err := r.Run("", "--version")
	if err != nil {
		return Version{}, err
	}
	// "git version 2.43.0"
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return Version{}, fmt.Errorf("cannot parse git version: %q", out)
	}
	parts := strings.SplitN(fields[2], ".", 3)
	v := Version{}
	if len(parts) > 0 {
		v.Major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		v.Minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		// Patch may carry suffixes (e.g. "0.windows.1"); take leading digits.
		v.Patch, _ = strconv.Atoi(leadingInt(parts[2]))
	}
	return v, nil
}

// EnsureMinVersion returns an error if git is older than major.minor.
func (r *Runner) EnsureMinVersion(major, minor int) error {
	v, err := r.Version()
	if err != nil {
		return err
	}
	if v.Major < major || (v.Major == major && v.Minor < minor) {
		return fmt.Errorf("git %d.%d+ required, found %d.%d", major, minor, v.Major, v.Minor)
	}
	return nil
}

func leadingInt(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return "0"
	}
	return s[:i]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags integration ./internal/git/ -run TestRunner -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/git/git.go internal/git/git_integration_test.go
git commit -m "feat(git): command runner with LC_ALL=C and version probe"
```

### Task 1.3: Repo resolution (common-dir anchoring) and ref helpers

**Files:**
- Create: `internal/git/resolve.go`
- Modify: `internal/git/git_integration_test.go` (add tests)

- [ ] **Step 1: Write the failing integration test**

Append to `internal/git/git_integration_test.go`:
```go
func TestResolve_MainRootFromWorktree(t *testing.T) {
	r := New()
	repo := newTestRepo(t)

	// Create a linked worktree using raw git so we can verify anchoring.
	wtPath := repo + ".wt-test"
	if _, err := r.Run(repo, "worktree", "add", wtPath); err != nil {
		t.Fatalf("setup worktree add: %v", err)
	}
	t.Cleanup(func() { _, _ = r.Run(repo, "worktree", "remove", "--force", wtPath) })

	// From inside the linked worktree, MainRoot must point back to the main repo.
	got, err := r.MainRoot(wtPath)
	if err != nil {
		t.Fatalf("MainRoot: %v", err)
	}
	want, _ := r.TopLevel(repo)
	if got != want {
		t.Errorf("MainRoot from worktree = %q, want main repo root %q", got, want)
	}
}

func TestResolve_VerifyRefAndCheckRefFormat(t *testing.T) {
	r := New()
	repo := newTestRepo(t)

	if err := r.VerifyRef(repo, "HEAD"); err != nil {
		t.Errorf("HEAD should verify: %v", err)
	}
	if err := r.VerifyRef(repo, "no-such-ref"); err == nil {
		t.Error("bogus ref should not verify")
	}
	if err := r.CheckRefFormat("wt/2026-05-31_10-00-snowy-beach-4821"); err != nil {
		t.Errorf("valid ref rejected: %v", err)
	}
	if err := r.CheckRefFormat("bad..ref"); err == nil {
		t.Error("invalid ref accepted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./internal/git/ -run TestResolve -v`
Expected: FAIL — `MainRoot`, `TopLevel`, `VerifyRef`, `CheckRefFormat` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/git/resolve.go`:
```go
package git

import (
	"path/filepath"
	"strings"
)

// TopLevel returns the working-tree root for dir (the current worktree).
func (r *Runner) TopLevel(dir string) (string, error) {
	out, err := r.Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// MainRoot returns the MAIN working tree root, even when dir is inside a linked
// worktree. It derives the path from --git-common-dir (which always points at
// the main repo's .git) and returns that directory's parent.
func (r *Runner) MainRoot(dir string) (string, error) {
	out, err := r.Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	commonDir := strings.TrimSpace(string(out)) // .../<main>/.git
	return filepath.Dir(commonDir), nil
}

// VerifyRef returns nil if ref resolves to a commit.
func (r *Runner) VerifyRef(dir, ref string) error {
	_, err := r.Run(dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err
}

// CheckRefFormat validates a branch name as a legal git ref.
func (r *Runner) CheckRefFormat(branch string) error {
	_, err := r.Run("", "check-ref-format", "--branch", branch)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags integration ./internal/git/ -run TestResolve -v`
Expected: PASS.

> Note: `check-ref-format --branch` resolves against the current repo for some
> inputs; if a CI environment lacks a repo in cwd, run it with `dir=""` from a
> temp dir. The test above runs from the module dir which is a git repo.

- [ ] **Step 5: Commit**

```bash
git add internal/git/resolve.go internal/git/git_integration_test.go
git commit -m "feat(git): common-dir anchoring and ref verification helpers"
```

### Task 1.4: Worktree operations (Add, List, Remove, Prune)

**Files:**
- Create: `internal/git/worktree.go`
- Modify: `internal/git/git_integration_test.go` (add tests)

- [ ] **Step 1: Write the failing integration test**

Append to `internal/git/git_integration_test.go`:
```go
func TestWorktree_AddListRemovePrune(t *testing.T) {
	r := New()
	repo := newTestRepo(t)
	wtPath := filepath.Join(t.TempDir(), "feature")

	if err := r.AddWorktree(repo, wtPath, "wt/feature", "HEAD"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	list, err := r.ListWorktrees(repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	found := false
	for _, w := range list {
		if w.Path == wtPath && w.Branch == "refs/heads/wt/feature" {
			found = true
		}
	}
	if !found {
		t.Errorf("new worktree not in list: %+v", list)
	}

	if err := r.RemoveWorktree(repo, wtPath, false); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if err := r.Prune(repo); err != nil {
		t.Fatalf("Prune: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./internal/git/ -run TestWorktree -v`
Expected: FAIL — `AddWorktree`, `ListWorktrees`, `RemoveWorktree`, `Prune` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/git/worktree.go`:
```go
package git

// AddWorktree creates a new worktree at path on a new branch cut from base.
func (r *Runner) AddWorktree(dir, path, branch, base string) error {
	_, err := r.Run(dir, "worktree", "add", "-b", branch, path, base)
	return err
}

// ListWorktrees returns parsed worktree entries for the repo containing dir.
func (r *Runner) ListWorktrees(dir string) ([]WorktreeInfo, error) {
	out, err := r.Run(dir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parsePorcelainZ(out)
}

// RemoveWorktree removes the worktree at path. force allows removal of a dirty
// worktree (maps to git's -f).
func (r *Runner) RemoveWorktree(dir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := r.Run(dir, args...)
	return err
}

// Prune clears stale worktree administrative entries.
func (r *Runner) Prune(dir string) error {
	_, err := r.Run(dir, "worktree", "prune")
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags integration ./internal/git/ -run TestWorktree -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/git/worktree.go internal/git/git_integration_test.go
git commit -m "feat(git): worktree add/list/remove/prune operations"
```

### Task 1.5: Branch deletion (safe + force) and existence check

**Files:**
- Create: `internal/git/branch.go`
- Modify: `internal/git/git_integration_test.go` (add tests)

- [ ] **Step 1: Write the failing integration test**

Append to `internal/git/git_integration_test.go`:
```go
func TestBranch_ExistsAndDelete(t *testing.T) {
	r := New()
	repo := newTestRepo(t)

	// Create a merged branch (points at HEAD) then delete it safely.
	if _, err := r.Run(repo, "branch", "wt/merged"); err != nil {
		t.Fatalf("setup branch: %v", err)
	}
	if !r.BranchExists(repo, "wt/merged") {
		t.Fatal("BranchExists should be true for created branch")
	}
	if r.BranchExists(repo, "wt/missing") {
		t.Fatal("BranchExists should be false for missing branch")
	}
	deleted, err := r.DeleteBranch(repo, "wt/merged", false)
	if err != nil {
		t.Fatalf("safe delete of merged branch failed: %v", err)
	}
	if !deleted {
		t.Fatal("merged branch should have been deleted")
	}

	// Create an UNMERGED branch: commit on it, switch away, safe-delete refused.
	if _, err := r.Run(repo, "checkout", "-b", "wt/unmerged"); err != nil {
		t.Fatalf("checkout -b: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Run(repo, "add", ".")
	r.Run(repo, "commit", "-m", "wip")
	r.Run(repo, "checkout", "main")

	deleted, err = r.DeleteBranch(repo, "wt/unmerged", false)
	if err == nil && deleted {
		t.Fatal("safe delete should refuse unmerged branch")
	}
	// Force delete must succeed.
	if _, err := r.DeleteBranch(repo, "wt/unmerged", true); err != nil {
		t.Fatalf("force delete failed: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./internal/git/ -run TestBranch -v`
Expected: FAIL — `BranchExists`, `DeleteBranch` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/git/branch.go`:
```go
package git

// BranchExists reports whether refs/heads/<branch> exists.
func (r *Runner) BranchExists(dir, branch string) bool {
	err := r.VerifyRef(dir, "refs/heads/"+branch)
	return err == nil
}

// DeleteBranch deletes a branch. With force=false it uses safe delete
// (`git branch -d`), which refuses unmerged branches — in that case it returns
// (false, nil) so callers can report the branch was kept. With force=true it
// uses `git branch -D` and returns (true, nil) on success.
func (r *Runner) DeleteBranch(dir, branch string, force bool) (bool, error) {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.Run(dir, "branch", flag, branch)
	if err != nil {
		if !force {
			// Could not safely delete (likely unmerged). Signal "kept", not fatal.
			return false, nil
		}
		return false, err
	}
	return true, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags integration ./internal/git/ -run TestBranch -v`
Expected: PASS.

- [ ] **Step 5: Run the full git package suite (unit + integration)**

Run:
```bash
go test ./internal/git/
go test -tags integration ./internal/git/
```
Expected: both PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/git/branch.go internal/git/git_integration_test.go
git commit -m "feat(git): safe and force branch deletion with existence check"
```

### Task 1.6: Phase 1 Review Gate

- [ ] **Step 1: Run all tests green**

Run:
```bash
go vet ./internal/git/
go test ./internal/git/ && go test -tags integration ./internal/git/
```
Expected: vet clean; both suites PASS.

- [ ] **Step 2: Code review (subagent)**

Dispatch a code-review subagent over `internal/git/*.go`. Focus: exit-code handling, stderr capture, edge cases in `parsePorcelainZ` (trailing terminators, missing attributes), `MainRoot` correctness, no shell-string interpolation. Address findings before proceeding.

- [ ] **Step 3: Test review (subagent)**

Dispatch a subagent to assess test coverage/quality of `internal/git`. Confirm: detached/bare/locked/prunable parsing covered, path-with-spaces covered, unmerged-branch refusal covered, anchoring-from-worktree covered. Add missing cases.

- [ ] **Step 4: Architectural review (subagent)**

Dispatch the Plan/architect subagent to verify `internal/git` exposes a cohesive Runner API suitable to back the `GitRunner` interface (Phase 4), with no leakage of git-CLI specifics beyond this package. Address findings.

- [ ] **Step 5: Commit any review fixes**

```bash
git add -A internal/git/
git commit -m "refactor(git): address phase 1 review findings"
```

---

## Phase 2: `internal/config` — configuration

### Task 2.1: Config load + precedence resolution

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if cfg.BaseRef != "HEAD" {
		t.Errorf("default BaseRef = %q, want HEAD", cfg.BaseRef)
	}
	if cfg.Container != "" || cfg.NameTemplate != "" {
		t.Errorf("unset fields should be empty: %+v", cfg)
	}
}

func TestLoad_ReadsYAML(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "base_ref: develop\ncontainer: /tmp/wts\nname_template: custom\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseRef != "develop" || cfg.Container != "/tmp/wts" || cfg.NameTemplate != "custom" {
		t.Errorf("loaded config wrong: %+v", cfg)
	}
}

func TestResolve_FlagsOverrideFileOverrideDefaults(t *testing.T) {
	file := Config{BaseRef: "develop", Container: "/from/file"}
	flags := Config{BaseRef: "feature"} // Container unset -> file wins
	got := Resolve(file, flags)
	if got.BaseRef != "feature" {
		t.Errorf("flag should override: BaseRef=%q", got.BaseRef)
	}
	if got.Container != "/from/file" {
		t.Errorf("file should win when flag unset: Container=%q", got.Container)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Add yaml dep and write implementation**

Run: `go get gopkg.in/yaml.v3@latest`

Create `internal/config/config.go`:
```go
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds resolved worktree settings.
type Config struct {
	BaseRef      string `yaml:"base_ref"`
	Container    string `yaml:"container"`
	NameTemplate string `yaml:"name_template"`
}

// Defaults returns the built-in defaults.
func Defaults() Config {
	return Config{BaseRef: "HEAD"}
}

// Load reads <repoRoot>/.worktrees/config.yaml. A missing file is not an error;
// defaults are returned. The file's set fields override defaults.
func Load(repoRoot string) (Config, error) {
	cfg := Defaults()
	path := filepath.Join(repoRoot, ".worktrees", "config.yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return cfg, err
	}
	return Resolve(cfg, fileCfg), nil
}

// Resolve layers higher-priority over lower: any non-empty field in hi wins.
func Resolve(lo, hi Config) Config {
	out := lo
	if hi.BaseRef != "" {
		out.BaseRef = hi.BaseRef
	}
	if hi.Container != "" {
		out.Container = hi.Container
	}
	if hi.NameTemplate != "" {
		out.NameTemplate = hi.NameTemplate
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat(config): load .worktrees/config.yaml with precedence resolution"
```

### Task 2.2: Phase 2 Review Gate

- [ ] **Step 1: Tests green**

Run: `go vet ./internal/config/ && go test ./internal/config/`
Expected: clean + PASS.

- [ ] **Step 2: Code review (subagent)** over `internal/config/`. Focus: precedence correctness, YAML error surfacing, missing-file handling. Fix findings.

- [ ] **Step 3: Test review (subagent).** Confirm defaults, file-read, and three-way precedence are all exercised. Add gaps.

- [ ] **Step 4: Architectural review (subagent).** Confirm `Config` is a clean value type ready to satisfy `ConfigProvider` (Phase 4). Fix findings.

- [ ] **Step 5: Commit fixes**

```bash
git add -A internal/config/ && git commit -m "refactor(config): address phase 2 review findings"
```

---

## Phase 3: `internal/naming` — name generation

### Task 3.1: Wordlists and name generator

**Files:**
- Create: `internal/naming/words.go`
- Create: `internal/naming/naming.go`
- Test: `internal/naming/naming_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/naming/naming_test.go`:
```go
package naming

import (
	"regexp"
	"testing"
	"time"
)

func TestGenerate_DateFirstFormat(t *testing.T) {
	ts := time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC)
	name := Generate(ts, 4821)
	// YYYY-MM-DD_HH-mm-<adj>-<noun>-NNNN
	re := regexp.MustCompile(`^2026-05-31_14-30-[a-z]+-[a-z]+-4821$`)
	if !re.MatchString(name) {
		t.Errorf("name %q does not match expected pattern", name)
	}
}

func TestGenerate_IsDeterministicForSeed(t *testing.T) {
	ts := time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC)
	// Same timestamp + same digits + same word index source must be stable
	// enough for the format; we assert the digits and date are echoed exactly.
	a := Generate(ts, 1)
	if a[:16] != "2026-05-31_14-30" {
		t.Errorf("date prefix wrong: %q", a)
	}
	if a[len(a)-4:] != "0001" {
		t.Errorf("digit suffix should be zero-padded: %q", a)
	}
}

func TestSanitizeDir_StripsPrefixAndSlashes(t *testing.T) {
	if got := SanitizeDir("wt/feature/foo"); got != "feature-foo" {
		t.Errorf("SanitizeDir = %q, want feature-foo", got)
	}
	if got := SanitizeDir("plain"); got != "plain" {
		t.Errorf("SanitizeDir = %q, want plain", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/naming/ -v`
Expected: FAIL — symbols undefined.

- [ ] **Step 3: Write implementation**

Create `internal/naming/words.go`:
```go
package naming

// Curated, ref-safe wordlists (lowercase ASCII only).
var adjectives = []string{
	"snowy", "calm", "brave", "amber", "quiet", "swift", "lucky", "bold",
	"clever", "gentle", "merry", "sunny", "witty", "eager", "fancy", "jolly",
}

var nouns = []string{
	"beach", "river", "forest", "meadow", "harbor", "canyon", "summit", "valley",
	"island", "garden", "lagoon", "prairie", "delta", "ridge", "grove", "cove",
}
```

Create `internal/naming/naming.go`:
```go
package naming

import (
	"fmt"
	"strings"
	"time"
)

// Generate builds a default name of the form
// YYYY-MM-DD_HH-mm-<adjective>-<noun>-NNNN. The digits are caller-supplied
// (random in production) and zero-padded to 4. Word selection is derived from
// the digits so the function is pure and testable.
func Generate(ts time.Time, digits int) string {
	adj := adjectives[(digits/len(nouns))%len(adjectives)]
	noun := nouns[digits%len(nouns)]
	return fmt.Sprintf("%s-%s-%s-%04d",
		ts.Format("2006-01-02_15-04"), adj, noun, digits%10000)
}

// SanitizeDir converts a branch-style name into a filesystem-safe directory
// name: strips a leading "wt/" prefix and replaces remaining slashes with "-".
func SanitizeDir(name string) string {
	name = strings.TrimPrefix(name, "wt/")
	return strings.ReplaceAll(name, "/", "-")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/naming/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/naming/
git commit -m "feat(naming): date-first name generator and dir sanitizer"
```

### Task 3.2: Phase 3 Review Gate

- [ ] **Step 1: Tests green** — `go vet ./internal/naming/ && go test ./internal/naming/`
- [ ] **Step 2: Code review (subagent)** — focus: ref-format validity of generated names, sanitizer edge cases (leading dash, dots).
- [ ] **Step 3: Test review (subagent)** — confirm format, padding, and sanitizer cases covered.
- [ ] **Step 4: Architectural review (subagent)** — confirm naming is a pure leaf package with no external deps.
- [ ] **Step 5: Commit fixes** — `git add -A internal/naming/ && git commit -m "refactor(naming): address phase 3 review findings"`

---

## Phase 4: `pkg/worktree` — the Manager and interfaces

### Task 4.1: Types and interfaces

**Files:**
- Create: `pkg/worktree/types.go`
- Create: `pkg/worktree/interfaces.go`

- [ ] **Step 1: Write the types**

Create `pkg/worktree/types.go`:
```go
package worktree

// WorktreeInfo describes a worktree for listing/resolution.
type WorktreeInfo struct {
	Path     string
	Branch   string // refs/heads/...; empty if detached/bare
	HEAD     string
	IsMain   bool
	Detached bool
}

// AddOptions controls Add.
type AddOptions struct {
	Name     string // optional; generated if empty
	Branch   string // optional; defaults to "wt/"+Name
	BaseRef  string // optional; defaults to config BaseRef
	NoHooks  bool
}

// AddResult reports the outcome of Add.
type AddResult struct {
	Name     string
	Branch   string
	Path     string
	BaseRef  string
}

// RemoveOptions controls Remove.
type RemoveOptions struct {
	Name        string
	Force       bool // force-remove dirty worktree
	ForceBranch bool // force-delete unmerged branch
	KeepBranch  bool // do not delete the branch
	NoHooks     bool
}

// RemoveResult reports the outcome of Remove.
type RemoveResult struct {
	Name          string
	Branch        string
	Path          string
	BranchDeleted bool
	BranchKept    bool   // true if safe-delete refused (unmerged)
}
```

Create `pkg/worktree/interfaces.go`:
```go
package worktree

// GitRunner is the git operations the Manager needs. Implemented by internal/git.
type GitRunner interface {
	MainRoot(dir string) (string, error)
	VerifyRef(dir, ref string) error
	CheckRefFormat(branch string) error
	BranchExists(dir, branch string) bool
	AddWorktree(dir, path, branch, base string) error
	ListWorktrees(dir string) ([]GitWorktree, error)
	RemoveWorktree(dir, path string, force bool) error
	DeleteBranch(dir, branch string, force bool) (bool, error)
	Prune(dir string) error
}

// GitWorktree is the subset of git worktree data the Manager consumes.
type GitWorktree struct {
	Path     string
	Branch   string
	HEAD     string
	Bare     bool
	Detached bool
}

// HookEvent identifies a lifecycle hook.
type HookEvent string

const (
	PreCreate  HookEvent = "pre-create"
	PostCreate HookEvent = "post-create"
	PreRemove  HookEvent = "pre-remove"
	PostRemove HookEvent = "post-remove"
)

// HookContext is passed to the HookRunner; it becomes WT_* env vars.
type HookContext struct {
	Event      HookEvent
	SourceRoot string
	TargetRoot string
	Name       string
	Branch     string
	BaseRef    string
	Container  string
	RepoName   string
	Cwd        string // working directory the hook runs in
}

// HookRunner discovers and runs a hook. Returns nil if the hook is absent.
// A non-nil error means the hook ran and failed (non-zero exit).
type HookRunner interface {
	Run(ctx HookContext) error
}

// ConfigProvider supplies resolved configuration for a repo.
type ConfigProvider interface {
	BaseRef() string
	Container() string // "" => default sibling container
}
```

- [ ] **Step 2: Build to verify it compiles**

Run: `go build ./pkg/worktree/`
Expected: builds (no consumers yet).

- [ ] **Step 3: Commit**

```bash
git add pkg/worktree/types.go pkg/worktree/interfaces.go
git commit -m "feat(worktree): Manager types and injected interfaces"
```

### Task 4.2: Fakes for unit testing

**Files:**
- Create: `pkg/worktree/fakes_test.go`

- [ ] **Step 1: Write the fakes**

Create `pkg/worktree/fakes_test.go`:
```go
package worktree

import "fmt"

type fakeGit struct {
	mainRoot     string
	branches     map[string]bool // branch -> exists
	worktrees    []GitWorktree
	addErr       error
	removeErr    error
	verifyRefErr error
	added        []string // paths added
	removedPaths []string
	deleted      []string
	deleteOK     bool // safe-delete result
}

func newFakeGit(root string) *fakeGit {
	return &fakeGit{mainRoot: root, branches: map[string]bool{}, deleteOK: true}
}

func (f *fakeGit) MainRoot(string) (string, error)    { return f.mainRoot, nil }
func (f *fakeGit) VerifyRef(_, _ string) error        { return f.verifyRefErr }
func (f *fakeGit) CheckRefFormat(string) error        { return nil }
func (f *fakeGit) BranchExists(_, b string) bool      { return f.branches[b] }
func (f *fakeGit) ListWorktrees(string) ([]GitWorktree, error) {
	return f.worktrees, nil
}
func (f *fakeGit) Prune(string) error { return nil }

func (f *fakeGit) AddWorktree(_, path, branch, _ string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, path)
	f.branches[branch] = true
	f.worktrees = append(f.worktrees, GitWorktree{Path: path, Branch: "refs/heads/" + branch})
	return nil
}

func (f *fakeGit) RemoveWorktree(_, path string, _ bool) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	f.removedPaths = append(f.removedPaths, path)
	return nil
}

func (f *fakeGit) DeleteBranch(_, branch string, force bool) (bool, error) {
	f.deleted = append(f.deleted, branch)
	if !force && !f.deleteOK {
		return false, nil // simulate unmerged refusal
	}
	return true, nil
}

type fakeHooks struct {
	calls []HookEvent
	failOn map[HookEvent]error
}

func newFakeHooks() *fakeHooks { return &fakeHooks{failOn: map[HookEvent]error{}} }

func (h *fakeHooks) Run(ctx HookContext) error {
	h.calls = append(h.calls, ctx.Event)
	if err, ok := h.failOn[ctx.Event]; ok {
		return fmt.Errorf("hook %s failed: %w", ctx.Event, err)
	}
	return nil
}

type fakeConfig struct {
	baseRef   string
	container string
}

func (c fakeConfig) BaseRef() string   { return c.baseRef }
func (c fakeConfig) Container() string { return c.container }
```

- [ ] **Step 2: Build the test package**

Run: `go vet ./pkg/worktree/`
Expected: compiles (fakes satisfy interfaces). If a method signature mismatches, fix the fake or interface now.

- [ ] **Step 3: Commit**

```bash
git add pkg/worktree/fakes_test.go
git commit -m "test(worktree): fakes for GitRunner/HookRunner/ConfigProvider"
```

### Task 4.3: Container path + name/branch resolution

**Files:**
- Create: `pkg/worktree/manager.go`
- Test: `pkg/worktree/manager_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/worktree/manager_test.go`:
```go
package worktree

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestManager(root string) (*Manager, *fakeGit, *fakeHooks) {
	g := newFakeGit(root)
	h := newFakeHooks()
	cfg := fakeConfig{baseRef: "HEAD"}
	m := New(g, h, cfg)
	// Deterministic name source for tests.
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
	if name != "2026-05-31_14-30-snowy-beach-4821" {
		t.Errorf("generated name = %q", name)
	}
	if branch != "wt/2026-05-31_14-30-snowy-beach-4821" {
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/worktree/ -run 'TestContainerPath|TestResolveNames|TestWorktreePath' -v`
Expected: FAIL — `New`, `Manager`, helpers undefined.

- [ ] **Step 3: Write implementation**

Create `pkg/worktree/manager.go`:
```go
package worktree

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/homeend/worktrees/internal/naming"
)

// Manager orchestrates worktree operations over injected collaborators.
type Manager struct {
	git    GitRunner
	hooks  HookRunner
	cfg    ConfigProvider
	now    func() time.Time
	digits func() int
}

// New constructs a Manager with default time/random sources.
func New(g GitRunner, h HookRunner, c ConfigProvider) *Manager {
	return &Manager{
		git:    g,
		hooks:  h,
		cfg:    c,
		now:    time.Now,
		digits: defaultDigits,
	}
}

// containerPath returns the worktree container for a repo root. A configured
// container overrides the default sibling and is used verbatim.
func (m *Manager) containerPath(repoRoot string) string {
	if c := m.cfg.Container(); c != "" {
		return c
	}
	return repoRoot + ".worktrees"
}

// resolveNames computes (name, branch). name omits the wt/ prefix; branch always
// carries it. An explicit Branch overrides the derived one (still prefixed).
func (m *Manager) resolveNames(opts AddOptions) (name, branch string) {
	name = opts.Name
	if name == "" {
		name = naming.Generate(m.now(), m.digits())
	}
	base := opts.Branch
	if base == "" {
		base = name
	}
	branch = "wt/" + strings.TrimPrefix(base, "wt/")
	return name, branch
}

// worktreePath returns the on-disk path for a branch within the container.
func (m *Manager) worktreePath(repoRoot, branch string) string {
	return filepath.Join(m.containerPath(repoRoot), naming.SanitizeDir(branch))
}

func defaultDigits() int {
	// Production randomness lives in cmd wiring; this fallback is non-zero.
	return 1
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/worktree/ -run 'TestContainerPath|TestResolveNames|TestWorktreePath' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/worktree/manager.go pkg/worktree/manager_test.go
git commit -m "feat(worktree): container path and name/branch resolution"
```

### Task 4.4: Add (create transaction)

**Files:**
- Modify: `pkg/worktree/manager.go`
- Modify: `pkg/worktree/manager_test.go`

- [ ] **Step 1: Write the failing test**

Append to `pkg/worktree/manager_test.go`:
```go
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
```

Add a shared sentinel at the top of the test file (after imports):
```go
var errInjected = fmt.Errorf("injected failure")
```
And add `"fmt"` to the test file's imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/worktree/ -run TestAdd -v`
Expected: FAIL — `Add` undefined.

- [ ] **Step 3: Write implementation**

Append to `pkg/worktree/manager.go`:
```go
import "fmt" // add to the existing import block

// Add creates a new worktree following the create transaction:
// resolve+validate -> pre-create hook -> git worktree add -> post-create hook.
// A pre-create failure aborts before anything is created. A post-create failure
// returns an error but leaves the worktree in place (no rollback, by design).
func (m *Manager) Add(dir string, opts AddOptions) (AddResult, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return AddResult{}, fmt.Errorf("resolve repo root: %w", err)
	}

	name, branch := m.resolveNames(opts)
	if err := m.git.CheckRefFormat(branch); err != nil {
		return AddResult{}, fmt.Errorf("invalid branch name %q: %w", branch, err)
	}
	if m.git.BranchExists(repoRoot, branch) {
		return AddResult{}, fmt.Errorf("branch %q already exists; pass a different --branch", branch)
	}

	baseRef := opts.BaseRef
	if baseRef == "" {
		baseRef = m.cfg.BaseRef()
	}
	if err := m.git.VerifyRef(repoRoot, baseRef); err != nil {
		return AddResult{}, fmt.Errorf("base ref %q not found: %w", baseRef, err)
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

	if err := m.git.AddWorktree(repoRoot, target, branch, baseRef); err != nil {
		return AddResult{}, fmt.Errorf("git worktree add: %w", err)
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

> Note: merge the `import "fmt"` into the existing import block rather than adding a second block.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/worktree/ -run TestAdd -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/worktree/manager.go pkg/worktree/manager_test.go
git commit -m "feat(worktree): Add create transaction with hook ordering"
```

### Task 4.5: List and name resolution against real worktrees

**Files:**
- Modify: `pkg/worktree/manager.go`
- Modify: `pkg/worktree/manager_test.go`

- [ ] **Step 1: Write the failing test**

Append to `pkg/worktree/manager_test.go`:
```go
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

func TestResolveWorktree_ByDirThenBranch(t *testing.T) {
	m, g, _ := newTestManager("/home/me/myrepo")
	g.worktrees = []GitWorktree{
		{Path: "/home/me/myrepo", Branch: "refs/heads/main"},
		{Path: "/home/me/myrepo.worktrees/feat", Branch: "refs/heads/wt/feat"},
	}
	// match by directory basename
	w, err := m.resolveWorktree(".", "feat")
	if err != nil {
		t.Fatalf("resolve by dir: %v", err)
	}
	if w.Path != "/home/me/myrepo.worktrees/feat" {
		t.Errorf("resolved wrong path: %q", w.Path)
	}
	// match by branch (with or without wt/ prefix)
	if _, err := m.resolveWorktree(".", "wt/feat"); err != nil {
		t.Errorf("resolve by branch failed: %v", err)
	}
	// not found
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/worktree/ -run 'TestList|TestResolveWorktree' -v`
Expected: FAIL — `List`, `resolveWorktree` undefined.

- [ ] **Step 3: Write implementation**

Append to `pkg/worktree/manager.go`:
```go
// List returns worktrees for the repo containing dir. The main working tree is
// flagged IsMain.
func (m *Manager) List(dir string) ([]WorktreeInfo, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return nil, err
	}
	raw, err := m.git.ListWorktrees(dir)
	if err != nil {
		return nil, err
	}
	out := make([]WorktreeInfo, 0, len(raw))
	for _, w := range raw {
		out = append(out, WorktreeInfo{
			Path:     w.Path,
			Branch:   w.Branch,
			HEAD:     w.HEAD,
			Detached: w.Detached,
			IsMain:   w.Path == repoRoot,
		})
	}
	return out, nil
}

// resolveWorktree maps a user-supplied name to a worktree, matching by
// directory basename first, then branch (with/without wt/ prefix). It refuses
// the main worktree and errors on not-found.
func (m *Manager) resolveWorktree(dir, name string) (WorktreeInfo, error) {
	list, err := m.List(dir)
	if err != nil {
		return WorktreeInfo{}, err
	}
	wantBranch := "refs/heads/wt/" + strings.TrimPrefix(name, "wt/")
	for _, w := range list {
		byDir := filepath.Base(w.Path) == naming.SanitizeDir(name)
		byBranch := w.Branch == wantBranch || w.Branch == "refs/heads/"+name
		if byDir || byBranch {
			if w.IsMain {
				return WorktreeInfo{}, fmt.Errorf("%q is the main worktree and cannot be removed", name)
			}
			return w, nil
		}
	}
	return WorktreeInfo{}, fmt.Errorf("no worktree matching %q", name)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/worktree/ -run 'TestList|TestResolveWorktree' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/worktree/manager.go pkg/worktree/manager_test.go
git commit -m "feat(worktree): List and name->worktree resolution"
```

### Task 4.6: Remove (remove transaction with safe branch delete)

**Files:**
- Modify: `pkg/worktree/manager.go`
- Modify: `pkg/worktree/manager_test.go`

- [ ] **Step 1: Write the failing test**

Append to `pkg/worktree/manager_test.go`:
```go
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
	g.deleteOK = false // simulate safe-delete refusal (unmerged)
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/worktree/ -run TestRemove -v`
Expected: FAIL — `Remove` undefined.

- [ ] **Step 3: Write implementation**

Append to `pkg/worktree/manager.go`:
```go
// Remove tears down a worktree: pre-remove hook -> git worktree remove ->
// branch delete (safe unless ForceBranch) -> post-remove hook. A safe-delete
// refusal (unmerged branch) is not fatal: the worktree is still removed and the
// result reports BranchKept so the CLI can tell the user.
func (m *Manager) Remove(dir string, opts RemoveOptions) (RemoveResult, error) {
	repoRoot, err := m.git.MainRoot(dir)
	if err != nil {
		return RemoveResult{}, err
	}
	w, err := m.resolveWorktree(dir, opts.Name)
	if err != nil {
		return RemoveResult{}, err
	}
	branch := strings.TrimPrefix(w.Branch, "refs/heads/")
	res := RemoveResult{Name: opts.Name, Branch: branch, Path: w.Path}

	hc := HookContext{
		SourceRoot: repoRoot,
		TargetRoot: w.Path,
		Name:       opts.Name,
		Branch:     branch,
		Container:  m.containerPath(repoRoot),
		RepoName:   filepath.Base(repoRoot),
	}

	if !opts.NoHooks {
		pr := hc
		pr.Event = PreRemove
		pr.Cwd = w.Path
		if err := m.hooks.Run(pr); err != nil {
			return res, fmt.Errorf("pre-remove hook failed (nothing removed): %w", err)
		}
	}

	if err := m.git.RemoveWorktree(repoRoot, w.Path, opts.Force); err != nil {
		return res, fmt.Errorf("git worktree remove: %w", err)
	}

	if !opts.KeepBranch && branch != "" {
		deleted, err := m.git.DeleteBranch(repoRoot, branch, opts.ForceBranch)
		if err != nil {
			return res, fmt.Errorf("delete branch %q: %w", branch, err)
		}
		res.BranchDeleted = deleted
		res.BranchKept = !deleted
	}

	if !opts.NoHooks {
		por := hc
		por.Event = PostRemove
		por.Cwd = repoRoot
		if err := m.hooks.Run(por); err != nil {
			return res, fmt.Errorf("post-remove hook failed (worktree already removed): %w", err)
		}
	}

	return res, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/worktree/ -run TestRemove -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/worktree/manager.go pkg/worktree/manager_test.go
git commit -m "feat(worktree): Remove transaction with safe branch delete reporting"
```

### Task 4.7: Manager integration test against real git + real internal/git

**Files:**
- Create: `pkg/worktree/manager_integration_test.go`

- [ ] **Step 1: Write the integration test**

Create `pkg/worktree/manager_integration_test.go`:
```go
//go:build integration

package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/homeend/worktrees/internal/git"
)

// gitAdapter bridges *git.Runner to the worktree.GitRunner interface.
type gitAdapter struct{ r *git.Runner }

func (a gitAdapter) MainRoot(d string) (string, error)          { return a.r.MainRoot(d) }
func (a gitAdapter) VerifyRef(d, ref string) error              { return a.r.VerifyRef(d, ref) }
func (a gitAdapter) CheckRefFormat(b string) error              { return a.r.CheckRefFormat(b) }
func (a gitAdapter) BranchExists(d, b string) bool              { return a.r.BranchExists(d, b) }
func (a gitAdapter) AddWorktree(d, p, b, base string) error     { return a.r.AddWorktree(d, p, b, base) }
func (a gitAdapter) RemoveWorktree(d, p string, f bool) error   { return a.r.RemoveWorktree(d, p, f) }
func (a gitAdapter) DeleteBranch(d, b string, f bool) (bool, error) { return a.r.DeleteBranch(d, b, f) }
func (a gitAdapter) Prune(d string) error                       { return a.r.Prune(d) }
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
```

- [ ] **Step 2: Run the integration test**

Run: `go test -tags integration ./pkg/worktree/ -run TestManager_AddListRemove_RealGit -v`
Expected: PASS.

- [ ] **Step 3: Run the whole worktree suite**

Run:
```bash
go test ./pkg/worktree/
go test -tags integration ./pkg/worktree/
```
Expected: both PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/worktree/manager_integration_test.go
git commit -m "test(worktree): end-to-end Add/List/Remove against real git"
```

### Task 4.8: Phase 4 Review Gate

- [ ] **Step 1: Tests green** — `go vet ./pkg/worktree/ && go test ./pkg/worktree/ && go test -tags integration ./pkg/worktree/`
- [ ] **Step 2: Code review (subagent)** over `pkg/worktree/*.go`. Focus: transaction correctness, error wrapping, no-rollback semantics, branch-kept reporting, refusal of main worktree.
- [ ] **Step 3: Test review (subagent).** Confirm both hook-failure paths, existing-branch rejection, unmerged-kept, keep-branch, and the real-git integration are covered.
- [ ] **Step 4: Architectural review (subagent).** Verify the Manager depends ONLY on its interfaces (no `internal/*` import except `naming`), and the interfaces are minimal/cohesive. Confirm `pkg/worktree` is independently consumable.
- [ ] **Step 5: Commit fixes** — `git add -A pkg/worktree/ && git commit -m "refactor(worktree): address phase 4 review findings"`

---

## Phase 5: `internal/hooks` — default HookRunner

### Task 5.1: Hook discovery, env construction, execution, streaming

**Files:**
- Create: `internal/hooks/hooks.go`
- Test: `internal/hooks/hooks_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/hooks/hooks_test.go`:
```go
package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/homeend/worktrees/pkg/worktree"
)

func writeHook(t *testing.T, dir, name, body string, exec bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mode := os.FileMode(0o644)
	if exec {
		mode = 0o755
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestRun_AbsentHookIsNoop(t *testing.T) {
	repo := t.TempDir()
	r := New(repo)
	err := r.Run(worktree.HookContext{Event: worktree.PreCreate, Cwd: repo})
	if err != nil {
		t.Errorf("absent hook should be a no-op, got %v", err)
	}
}

func TestRun_ExportsEnvAndRuns(t *testing.T) {
	repo := t.TempDir()
	target := t.TempDir()
	// Hook writes WT_TARGET_ROOT into a marker file we can assert on.
	writeHook(t, filepath.Join(repo, ".worktrees"), "post-create",
		"#!/usr/bin/env bash\necho \"$WT_TARGET_ROOT\" > \"$WT_TARGET_ROOT/marker\"\n", true)

	r := New(repo)
	err := r.Run(worktree.HookContext{
		Event:      worktree.PostCreate,
		SourceRoot: repo,
		TargetRoot: target,
		Name:       "feat",
		Branch:     "wt/feat",
		Cwd:        target,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "marker"))
	if err != nil {
		t.Fatalf("marker not written: %v", err)
	}
	if string(got) != target+"\n" {
		t.Errorf("WT_TARGET_ROOT env wrong: %q", got)
	}
}

func TestRun_NonExecutableIsSkipped(t *testing.T) {
	repo := t.TempDir()
	writeHook(t, filepath.Join(repo, ".worktrees"), "pre-create",
		"#!/usr/bin/env bash\nexit 7\n", false) // not executable
	r := New(repo)
	if err := r.Run(worktree.HookContext{Event: worktree.PreCreate, Cwd: repo}); err != nil {
		t.Errorf("non-executable hook should be skipped, got %v", err)
	}
}

func TestRun_FailingHookReturnsError(t *testing.T) {
	repo := t.TempDir()
	writeHook(t, filepath.Join(repo, ".worktrees"), "pre-create",
		"#!/usr/bin/env bash\nexit 3\n", true)
	r := New(repo)
	err := r.Run(worktree.HookContext{Event: worktree.PreCreate, Cwd: repo})
	if err == nil {
		t.Fatal("failing hook must return an error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hooks/ -v`
Expected: FAIL — `New`, `Run` undefined.

- [ ] **Step 3: Write implementation**

Create `internal/hooks/hooks.go`:
```go
package hooks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/homeend/worktrees/pkg/worktree"
)

// Runner runs convention-dir hooks from <repoRoot>/.worktrees/.
type Runner struct {
	repoRoot string
}

// New returns a hook Runner rooted at repoRoot.
func New(repoRoot string) *Runner { return &Runner{repoRoot: repoRoot} }

// Run executes the hook for ctx.Event if it exists and is executable. An absent
// or non-executable hook is a silent no-op. A non-zero exit is returned as an
// error. Hook stdout/stderr stream to the process's stdout/stderr.
func (r *Runner) Run(ctx worktree.HookContext) error {
	path := filepath.Join(r.repoRoot, ".worktrees", string(ctx.Event))
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return nil // not executable -> skip
	}

	cmd := exec.Command(path)
	cmd.Dir = ctx.Cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env(ctx)...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %s exited with error: %w", ctx.Event, err)
	}
	return nil
}

func env(ctx worktree.HookContext) []string {
	return []string{
		"WT_SOURCE_ROOT=" + ctx.SourceRoot,
		"WT_TARGET_ROOT=" + ctx.TargetRoot,
		"WT_NAME=" + ctx.Name,
		"WT_BRANCH=" + ctx.Branch,
		"WT_BASE_REF=" + ctx.BaseRef,
		"WT_CONTAINER=" + ctx.Container,
		"WT_REPO_NAME=" + ctx.RepoName,
		"WT_HOOK=" + string(ctx.Event),
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/hooks/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/
git commit -m "feat(hooks): convention-dir hook runner with WT_* env and streaming"
```

### Task 5.2: Phase 5 Review Gate

- [ ] **Step 1: Tests green** — `go vet ./internal/hooks/ && go test ./internal/hooks/`
- [ ] **Step 2: Code review (subagent)** — focus: exec-bit check, absent vs failing distinction, env completeness, no shell interpolation, cwd correctness.
- [ ] **Step 3: Test review (subagent)** — confirm absent/non-exec/failing/env-export all covered for at least one create and verify all four events route by filename.
- [ ] **Step 4: Architectural review (subagent)** — confirm `internal/hooks` cleanly satisfies `worktree.HookRunner` and depends only on `pkg/worktree` for types.
- [ ] **Step 5: Commit fixes** — `git add -A internal/hooks/ && git commit -m "refactor(hooks): address phase 5 review findings"`

---

## Phase 6: `cmd/wt` — CLI commands and wiring

### Task 6.1: Error taxonomy and exit codes

**Files:**
- Create: `cmd/wt/errors.go`
- Test: `cmd/wt/root_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/wt/root_test.go`:
```go
package cmd

import "testing"

func TestExitCodeFor(t *testing.T) {
	cases := map[error]int{
		nil:                 0,
		ErrNotARepo:         2,
		ErrNameCollision:    3,
		ErrHookFailed:       4,
		ErrDirtyWorktree:    5,
	}
	for err, want := range cases {
		if got := exitCodeFor(err); got != want {
			t.Errorf("exitCodeFor(%v) = %d, want %d", err, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/wt/ -run TestExitCodeFor -v`
Expected: FAIL — symbols undefined.

- [ ] **Step 3: Write implementation**

Create `cmd/wt/errors.go`:
```go
package cmd

import "errors"

// Sentinel errors mapped to stable process exit codes.
var (
	ErrNotARepo      = errors.New("not a git repository")
	ErrNameCollision = errors.New("name collision")
	ErrHookFailed    = errors.New("hook failed")
	ErrDirtyWorktree = errors.New("worktree has uncommitted changes")
)

// exitCodeFor maps an error to a process exit code. Unknown non-nil errors -> 1.
func exitCodeFor(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrNotARepo):
		return 2
	case errors.Is(err, ErrNameCollision):
		return 3
	case errors.Is(err, ErrHookFailed):
		return 4
	case errors.Is(err, ErrDirtyWorktree):
		return 5
	default:
		return 1
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/wt/ -run TestExitCodeFor -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/errors.go cmd/wt/root_test.go
git commit -m "feat(cmd): error taxonomy and exit code mapping"
```

### Task 6.2: Manager wiring helper

**Files:**
- Modify: `cmd/wt/root.go`

- [ ] **Step 1: Write the wiring helper**

Replace `cmd/wt/root.go` with:
```go
package cmd

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/internal/config"
	"github.com/homeend/worktrees/internal/git"
	"github.com/homeend/worktrees/internal/hooks"
	"github.com/homeend/worktrees/pkg/worktree"
)

var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "Fast git worktree management with lifecycle hooks",
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return exitCodeFor(err)
	}
	return 0
}

// gitAdapter bridges *git.Runner to worktree.GitRunner.
type gitAdapter struct{ r *git.Runner }

func (a gitAdapter) MainRoot(d string) (string, error)            { return a.r.MainRoot(d) }
func (a gitAdapter) VerifyRef(d, ref string) error                { return a.r.VerifyRef(d, ref) }
func (a gitAdapter) CheckRefFormat(b string) error                { return a.r.CheckRefFormat(b) }
func (a gitAdapter) BranchExists(d, b string) bool                { return a.r.BranchExists(d, b) }
func (a gitAdapter) AddWorktree(d, p, b, base string) error       { return a.r.AddWorktree(d, p, b, base) }
func (a gitAdapter) RemoveWorktree(d, p string, f bool) error     { return a.r.RemoveWorktree(d, p, f) }
func (a gitAdapter) DeleteBranch(d, b string, f bool) (bool, error) { return a.r.DeleteBranch(d, b, f) }
func (a gitAdapter) Prune(d string) error                         { return a.r.Prune(d) }
func (a gitAdapter) ListWorktrees(d string) ([]worktree.GitWorktree, error) {
	ws, err := a.r.ListWorktrees(d)
	if err != nil {
		return nil, err
	}
	out := make([]worktree.GitWorktree, len(ws))
	for i, w := range ws {
		out[i] = worktree.GitWorktree{Path: w.Path, Branch: w.Branch, HEAD: w.HEAD, Bare: w.Bare, Detached: w.Detached}
	}
	return out, nil
}

// cfgAdapter adapts config.Config to worktree.ConfigProvider.
type cfgAdapter struct{ c config.Config }

func (a cfgAdapter) BaseRef() string   { return a.c.BaseRef }
func (a cfgAdapter) Container() string { return a.c.Container }

// buildManager resolves the repo root and wires a Manager. cwd is where wt runs.
func buildManager(cwd string) (*worktree.Manager, error) {
	r := git.New()
	if err := r.EnsureMinVersion(2, 30); err != nil {
		return nil, err
	}
	repoRoot, err := r.MainRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotARepo, err)
	}
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return nil, err
	}
	m := worktree.New(gitAdapter{r}, hooks.New(repoRoot), cfgAdapter{cfg})
	m.SetDigits(randomDigits)
	return m, nil
}

func randomDigits() int {
	var b [2]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return 1
	}
	return int(binary.BigEndian.Uint16(b[:]) % 10000)
}
```

- [ ] **Step 2: Add the `SetDigits` setter to the Manager**

In `pkg/worktree/manager.go`, add:
```go
// SetDigits overrides the digit source used for generated names (e.g. random in
// production). Intended for wiring and tests.
func (m *Manager) SetDigits(fn func() int) { m.digits = fn }
```

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: builds.

- [ ] **Step 4: Commit**

```bash
git add cmd/wt/root.go pkg/worktree/manager.go
git commit -m "feat(cmd): wire Manager with git/config/hooks adapters"
```

### Task 6.3: `wt new`

**Files:**
- Create: `cmd/wt/new.go`
- Modify: `cmd/wt/root_test.go`

- [ ] **Step 1: Write the failing integration test**

Append to `cmd/wt/root_test.go`:
```go
import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func newRepoForCLI(t *testing.T) string {
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

func TestNewCommand_CreatesWorktree(t *testing.T) {
	repo := newRepoForCLI(t)
	m, err := buildManager(repo)
	if err != nil {
		t.Fatalf("buildManager: %v", err)
	}
	res, err := m.Add(repo, worktreeAddOptions("feat", "", "", true))
	if err != nil {
		t.Fatalf("Add via manager: %v", err)
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Errorf("worktree not created: %v", err)
	}
}
```

> This test exercises the wiring (`buildManager`) plus a helper that builds
> `AddOptions` from flag values; the cobra command is a thin shell over it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./cmd/wt/ -run TestNewCommand -v`
Expected: FAIL — `worktreeAddOptions` undefined.

- [ ] **Step 3: Write implementation**

Create `cmd/wt/new.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/pkg/worktree"
)

var (
	newRepo    string
	newBranch  string
	newBase    string
	newNoHooks bool
)

// worktreeAddOptions builds AddOptions from flag values (extracted for testing).
func worktreeAddOptions(name, branch, base string, noHooks bool) worktree.AddOptions {
	return worktree.AddOptions{Name: name, Branch: branch, BaseRef: base, NoHooks: noHooks}
}

var newCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new worktree",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := newRepo
		if cwd == "" {
			var err error
			if cwd, err = os.Getwd(); err != nil {
				return err
			}
		}
		m, err := buildManager(cwd)
		if err != nil {
			return err
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		res, err := m.Add(cwd, worktreeAddOptions(name, newBranch, newBase, newNoHooks))
		if err != nil {
			return err
		}
		fmt.Printf("Created worktree %q\n  branch: %s\n  path:   %s\n", res.Name, res.Branch, res.Path)
		return nil
	},
}

func init() {
	newCmd.Flags().StringVarP(&newRepo, "repo", "r", "", "source repo (default: current dir)")
	newCmd.Flags().StringVarP(&newBranch, "branch", "b", "", "branch name (default: derived from name)")
	newCmd.Flags().StringVar(&newBase, "base", "", "base ref to branch from (default: config base_ref / HEAD)")
	newCmd.Flags().BoolVar(&newNoHooks, "no-hooks", false, "skip lifecycle hooks")
	rootCmd.AddCommand(newCmd)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags integration ./cmd/wt/ -run TestNewCommand -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/new.go cmd/wt/root_test.go
git commit -m "feat(cmd): wt new command"
```

### Task 6.4: `wt list` / `ls` with `--json`

**Files:**
- Create: `cmd/wt/list.go`
- Modify: `cmd/wt/root_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/wt/root_test.go`:
```go
func TestListJSON_Marshals(t *testing.T) {
	items := []worktree.WorktreeInfo{
		{Path: "/r", Branch: "refs/heads/main", IsMain: true},
		{Path: "/r.worktrees/feat", Branch: "refs/heads/wt/feat"},
	}
	out, err := renderListJSON(items)
	if err != nil {
		t.Fatalf("renderListJSON: %v", err)
	}
	if !strings.Contains(out, "\"branch\": \"refs/heads/wt/feat\"") {
		t.Errorf("json missing branch: %s", out)
	}
}
```
Add `"strings"` and the `worktree` import to the test file if not present.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/wt/ -run TestListJSON -v`
Expected: FAIL — `renderListJSON` undefined.

- [ ] **Step 3: Write implementation**

Create `cmd/wt/list.go`:
```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/pkg/worktree"
)

var listJSON bool

// renderListJSON serializes worktrees as indented JSON (extracted for testing).
func renderListJSON(items []worktree.WorktreeInfo) (string, error) {
	type row struct {
		Path   string `json:"path"`
		Branch string `json:"branch"`
		HEAD   string `json:"head"`
		IsMain bool   `json:"is_main"`
	}
	rows := make([]row, len(items))
	for i, w := range items {
		rows[i] = row{Path: w.Path, Branch: w.Branch, HEAD: w.HEAD, IsMain: w.IsMain}
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List worktrees",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		m, err := buildManager(cwd)
		if err != nil {
			return err
		}
		items, err := m.List(cwd)
		if err != nil {
			return err
		}
		if listJSON {
			out, err := renderListJSON(items)
			if err != nil {
				return err
			}
			fmt.Println(out)
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "BRANCH\tPATH")
		for _, it := range items {
			marker := ""
			if it.IsMain {
				marker = " (main)"
			}
			fmt.Fprintf(w, "%s%s\t%s\n", it.Branch, marker, it.Path)
		}
		return w.Flush()
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(listCmd)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/wt/ -run TestListJSON -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/list.go cmd/wt/root_test.go
git commit -m "feat(cmd): wt list/ls with --json output"
```

### Task 6.5: `wt rm`

**Files:**
- Create: `cmd/wt/rm.go`
- Modify: `cmd/wt/root_test.go`

- [ ] **Step 1: Write the failing integration test**

Append to `cmd/wt/root_test.go`:
```go
func TestRmCommand_RemovesWorktreeAndReportsBranch(t *testing.T) {
	repo := newRepoForCLI(t)
	m, err := buildManager(repo)
	if err != nil {
		t.Fatalf("buildManager: %v", err)
	}
	if _, err := m.Add(repo, worktreeAddOptions("feat", "", "", true)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	res, err := m.Remove(repo, worktreeRemoveOptions("feat", false, false, false, true))
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !res.BranchDeleted {
		t.Errorf("merged branch should be deleted: %+v", res)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./cmd/wt/ -run TestRmCommand -v`
Expected: FAIL — `worktreeRemoveOptions` undefined.

- [ ] **Step 3: Write implementation**

Create `cmd/wt/rm.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/pkg/worktree"
)

var (
	rmForce       bool
	rmForceBranch bool
	rmKeepBranch  bool
	rmNoHooks     bool
)

func worktreeRemoveOptions(name string, force, forceBranch, keepBranch, noHooks bool) worktree.RemoveOptions {
	return worktree.RemoveOptions{
		Name: name, Force: force, ForceBranch: forceBranch,
		KeepBranch: keepBranch, NoHooks: noHooks,
	}
}

var rmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a worktree and its branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		m, err := buildManager(cwd)
		if err != nil {
			return err
		}
		res, err := m.Remove(cwd, worktreeRemoveOptions(args[0], rmForce, rmForceBranch, rmKeepBranch, rmNoHooks))
		if err != nil {
			return err
		}
		fmt.Printf("Removed worktree %q (%s)\n", res.Name, res.Path)
		switch {
		case res.BranchDeleted:
			fmt.Printf("Deleted branch %s\n", res.Branch)
		case res.BranchKept:
			fmt.Printf("Kept branch %s (unmerged). Delete with: wt rm %s --force-branch, or git branch -D %s\n",
				res.Branch, res.Name, res.Branch)
		}
		return nil
	},
}

func init() {
	rmCmd.Flags().BoolVar(&rmForce, "force", false, "force-remove a dirty worktree")
	rmCmd.Flags().BoolVarP(&rmForceBranch, "force-branch", "D", false, "force-delete an unmerged branch")
	rmCmd.Flags().BoolVar(&rmKeepBranch, "keep-branch", false, "do not delete the branch")
	rmCmd.Flags().BoolVar(&rmNoHooks, "no-hooks", false, "skip lifecycle hooks")
	rootCmd.AddCommand(rmCmd)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags integration ./cmd/wt/ -run TestRmCommand -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/rm.go cmd/wt/root_test.go
git commit -m "feat(cmd): wt rm with branch-kept reporting"
```

### Task 6.6: `wt prune` and `wt path`

**Files:**
- Create: `cmd/wt/prune.go`
- Create: `cmd/wt/path.go`
- Modify: `cmd/wt/root_test.go`

- [ ] **Step 1: Write the failing integration test**

Append to `cmd/wt/root_test.go`:
```go
func TestPathCommand_PrintsWorktreePath(t *testing.T) {
	repo := newRepoForCLI(t)
	m, err := buildManager(repo)
	if err != nil {
		t.Fatalf("buildManager: %v", err)
	}
	added, err := m.Add(repo, worktreeAddOptions("feat", "", "", true))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := resolveWorktreePath(m, repo, "feat")
	if err != nil {
		t.Fatalf("resolveWorktreePath: %v", err)
	}
	if got != added.Path {
		t.Errorf("path = %q, want %q", got, added.Path)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./cmd/wt/ -run TestPathCommand -v`
Expected: FAIL — `resolveWorktreePath` undefined.

- [ ] **Step 3: Add an exported resolver on Manager and write commands**

In `pkg/worktree/manager.go`, add a public resolver wrapping the internal one:
```go
// Find resolves a user-supplied name to a worktree (by dir basename or branch),
// refusing the main worktree. Exposed for callers like `wt path`.
func (m *Manager) Find(dir, name string) (WorktreeInfo, error) {
	return m.resolveWorktree(dir, name)
}
```

Create `cmd/wt/path.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/pkg/worktree"
)

// resolveWorktreePath returns the on-disk path for a named worktree.
func resolveWorktreePath(m *worktree.Manager, dir, name string) (string, error) {
	w, err := m.Find(dir, name)
	if err != nil {
		return "", err
	}
	return w.Path, nil
}

var pathCmd = &cobra.Command{
	Use:   "path <name>",
	Short: "Print the path of a worktree (for shell cd)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		m, err := buildManager(cwd)
		if err != nil {
			return err
		}
		p, err := resolveWorktreePath(m, cwd, args[0])
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	},
}

func init() { rootCmd.AddCommand(pathCmd) }
```

Create `cmd/wt/prune.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/internal/git"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune stale worktree administrative state",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		r := git.New()
		if err := r.Prune(cwd); err != nil {
			return err
		}
		fmt.Println("Pruned stale worktree entries.")
		return nil
	},
}

func init() { rootCmd.AddCommand(pruneCmd) }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags integration ./cmd/wt/ -run TestPathCommand -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/prune.go cmd/wt/path.go pkg/worktree/manager.go cmd/wt/root_test.go
git commit -m "feat(cmd): wt prune and wt path"
```

### Task 6.7: `wt init` scaffolding

**Files:**
- Create: `cmd/wt/init.go`
- Modify: `cmd/wt/root_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/wt/root_test.go`:
```go
func TestInitScaffold_CreatesFilesIdempotently(t *testing.T) {
	repo := t.TempDir()
	if err := scaffold(repo); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	wt := filepath.Join(repo, ".worktrees")
	for _, f := range []string{"config.yaml", "pre-create", "post-create", "pre-remove", "post-remove"} {
		if _, err := os.Stat(filepath.Join(wt, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	// Hook stubs must be executable.
	info, _ := os.Stat(filepath.Join(wt, "pre-create"))
	if info.Mode()&0o111 == 0 {
		t.Error("hook stub should be executable")
	}
	// Idempotent: writing custom content then re-running must not clobber.
	custom := filepath.Join(wt, "config.yaml")
	os.WriteFile(custom, []byte("base_ref: develop\n"), 0o644)
	if err := scaffold(repo); err != nil {
		t.Fatalf("second scaffold: %v", err)
	}
	got, _ := os.ReadFile(custom)
	if string(got) != "base_ref: develop\n" {
		t.Errorf("scaffold clobbered existing config: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/wt/ -run TestInitScaffold -v`
Expected: FAIL — `scaffold` undefined.

- [ ] **Step 3: Write implementation**

Create `cmd/wt/init.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const configTemplate = `# wt configuration. CLI flags override these values.
# base_ref: HEAD          # default ref new branches are cut from
# container: ""           # override container path; used verbatim
# name_template: ""       # override the default generated name pattern
`

const hookTemplate = `#!/usr/bin/env bash
# wt %s hook.
# Available environment variables:
#   WT_SOURCE_ROOT WT_TARGET_ROOT WT_NAME WT_BRANCH
#   WT_BASE_REF WT_CONTAINER WT_REPO_NAME WT_HOOK
# Example: cp "$WT_SOURCE_ROOT/.env" "$WT_TARGET_ROOT/.env"
exit 0
`

// scaffold creates .worktrees/ with a config and executable hook stubs. It never
// clobbers an existing file (idempotent).
func scaffold(repoRoot string) error {
	dir := filepath.Join(repoRoot, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeIfAbsent(filepath.Join(dir, "config.yaml"), configTemplate, 0o644); err != nil {
		return err
	}
	for _, ev := range []string{"pre-create", "post-create", "pre-remove", "post-remove"} {
		body := fmt.Sprintf(hookTemplate, ev)
		if err := writeIfAbsent(filepath.Join(dir, ev), body, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func writeIfAbsent(path, content string, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil // exists -> leave it
	}
	return os.WriteFile(path, []byte(content), mode)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold .worktrees/ with config and hook stubs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := scaffold(cwd); err != nil {
			return err
		}
		fmt.Println("Initialized .worktrees/ (config.yaml + hook stubs).")
		return nil
	},
}

func init() { rootCmd.AddCommand(initCmd) }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/wt/ -run TestInitScaffold -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/init.go cmd/wt/root_test.go
git commit -m "feat(cmd): wt init scaffolding (idempotent, executable stubs)"
```

### Task 6.8: `wt completion`

**Files:**
- Create: `cmd/wt/completion.go`

- [ ] **Step 1: Write implementation**

Create `cmd/wt/completion.go`:
```go
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:                   "completion [bash|zsh|fish|powershell]",
	Short:                 "Generate shell completion script",
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(completionCmd) }
```

- [ ] **Step 2: Build and smoke-test**

Run:
```bash
go build ./...
go run . completion bash | head -5
```
Expected: builds; prints the start of a bash completion script.

- [ ] **Step 3: Commit**

```bash
git add cmd/wt/completion.go
git commit -m "feat(cmd): wt completion for bash/zsh/fish/powershell"
```

### Task 6.9: Phase 6 Review Gate

- [ ] **Step 1: Tests green**

Run:
```bash
go vet ./cmd/wt/
go test ./cmd/wt/ && go test -tags integration ./cmd/wt/
```
Expected: clean + PASS.

- [ ] **Step 2: Code review (subagent)** over `cmd/wt/*.go`. Focus: `--repo` honored everywhere (currently only `new` uses it — decide if `list`/`rm`/`path` should too), exit-code propagation, error messages mapped to sentinels, no duplicated wiring.
- [ ] **Step 3: Test review (subagent).** Confirm each command has a test exercising its happy path; confirm JSON output, init idempotency, and branch-kept reporting are covered.
- [ ] **Step 4: Architectural review (subagent).** Confirm `cmd/wt` is the only place adapters/wiring live, and commands are thin shells over `pkg/worktree`. Address any leakage.
- [ ] **Step 5: Commit fixes** — `git add -A cmd/wt/ pkg/worktree/ && git commit -m "refactor(cmd): address phase 6 review findings"`

---

## Phase 7: `internal/tui` + bare-`wt` launch

### Task 7.1: TUI model over the Manager

**Files:**
- Create: `internal/tui/model.go`
- Create: `internal/tui/view.go`
- Create: `internal/tui/tui.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/model_test.go`:
```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/homeend/worktrees/pkg/worktree"
)

func TestModel_QuitOnQ(t *testing.T) {
	m := newModel([]worktree.WorktreeInfo{{Path: "/r", Branch: "refs/heads/main", IsMain: true}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("pressing q should return a command (tea.Quit)")
	}
}

func TestModel_CursorMovesDown(t *testing.T) {
	m := newModel([]worktree.WorktreeInfo{
		{Path: "/r", Branch: "refs/heads/main", IsMain: true},
		{Path: "/r.worktrees/feat", Branch: "refs/heads/wt/feat"},
	})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm := updated.(model)
	if mm.cursor != 1 {
		t.Errorf("cursor = %d, want 1", mm.cursor)
	}
}

func TestView_RendersBranches(t *testing.T) {
	m := newModel([]worktree.WorktreeInfo{{Path: "/r.worktrees/feat", Branch: "refs/heads/wt/feat"}})
	out := m.View()
	if out == "" {
		t.Fatal("view should render non-empty output")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -v`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Add deps and write implementation**

Run:
```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
```

Create `internal/tui/model.go`:
```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/homeend/worktrees/pkg/worktree"
)

type model struct {
	items  []worktree.WorktreeInfo
	cursor int
}

func newModel(items []worktree.WorktreeInfo) model {
	return model{items: items}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}
```

Create `internal/tui/view.go`:
```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Underline(true)
)

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Worktrees") + "\n\n")
	for i, it := range m.items {
		line := fmt.Sprintf("%s  %s", it.Branch, it.Path)
		if it.IsMain {
			line += "  (main)"
		}
		if i == m.cursor {
			line = "> " + selectedStyle.Render(line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n↑/↓ move • q quit\n")
	return b.String()
}
```

Create `internal/tui/tui.go`:
```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/homeend/worktrees/pkg/worktree"
)

// Run launches the interactive TUI listing worktrees for the given dir.
func Run(m *worktree.Manager, dir string) error {
	items, err := m.List(dir)
	if err != nil {
		return err
	}
	p := tea.NewProgram(newModel(items))
	_, err = p.Run()
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/ go.mod go.sum
git commit -m "feat(tui): worktree list model, view, and runner"
```

### Task 7.2: Bare `wt` launches TUI on a TTY

**Files:**
- Modify: `cmd/wt/root.go`
- Modify: `cmd/wt/root_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/wt/root_test.go`:
```go
func TestShouldLaunchTUI_RespectsTTY(t *testing.T) {
	if shouldLaunchTUI(false) {
		t.Error("must not launch TUI when stdout is not a TTY")
	}
	if !shouldLaunchTUI(true) {
		t.Error("should launch TUI when stdout is a TTY")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/wt/ -run TestShouldLaunchTUI -v`
Expected: FAIL — `shouldLaunchTUI` undefined.

- [ ] **Step 3: Write implementation**

In `cmd/wt/root.go`, add the import for the tui package and `golang.org/x/term`, and wire the root `RunE`. Add:
```go
// shouldLaunchTUI reports whether the bare command should open the TUI.
func shouldLaunchTUI(isTTY bool) bool { return isTTY }
```

Add a `RunE` to `rootCmd` (set it where `rootCmd` is defined):
```go
rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if !shouldLaunchTUI(isTTY) {
		return cmd.Help()
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	m, err := buildManager(cwd)
	if err != nil {
		return err
	}
	return tui.Run(m, cwd)
}
```

Run: `go get golang.org/x/term@latest`

Add imports to `root.go`:
```go
"golang.org/x/term"
"github.com/homeend/worktrees/internal/tui"
```

> Place the `rootCmd.RunE = ...` assignment inside an `init()` in root.go (cobra
> commands can take RunE after construction).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/wt/ -run TestShouldLaunchTUI -v`
Expected: PASS.

- [ ] **Step 5: Build and verify non-TTY prints help**

Run:
```bash
go build ./...
go run . | head -3   # stdout is piped -> not a TTY -> help
```
Expected: prints cobra help (not a TUI crash).

- [ ] **Step 6: Commit**

```bash
git add cmd/wt/root.go cmd/wt/root_test.go go.mod go.sum
git commit -m "feat(cmd): bare wt launches TUI on a TTY, help otherwise"
```

### Task 7.3: Phase 7 Review Gate

- [ ] **Step 1: Tests green** — `go vet ./internal/tui/ ./cmd/wt/ && go test ./... && go test -tags integration ./...`
- [ ] **Step 2: Code review (subagent)** over `internal/tui/*` and the root `RunE`. Focus: TTY guard correctness, no panic when repo absent, clean quit.
- [ ] **Step 3: Test review (subagent).** Confirm update/quit/cursor/view covered and TTY gating tested.
- [ ] **Step 4: Architectural review (subagent).** Confirm TUI depends only on `pkg/worktree` (not `internal/git` etc.) and rendering is separated from state.
- [ ] **Step 5: Commit fixes** — `git add -A internal/tui/ cmd/wt/ && git commit -m "refactor(tui): address phase 7 review findings"`

---

## Phase 8: Finalization

### Task 8.1: README and full-suite gate

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write the README**

Create `README.md`:
```markdown
# wt — fast git worktrees

`wt` creates, lists, and removes git worktrees in a sibling container, with
lifecycle hooks for copying gitignored files (like `.env`) into new worktrees.

## Install

	go install github.com/homeend/worktrees@latest

## Usage

	wt new [name]        # create a worktree (generated name if omitted)
	wt list | wt ls      # list worktrees (--json for machine output)
	wt rm <name>         # remove a worktree and its branch
	wt path <name>       # print a worktree's path
	wt prune             # clear stale worktree state
	wt init              # scaffold .worktrees/ (config + hook stubs)
	wt                   # interactive TUI (in a terminal)

Worktrees are placed at `<repo>.worktrees/<name>/`. Branches are prefixed `wt/`.

### Shell `cd` helper

`wt` cannot change your shell's directory; add this function:

	wtcd() { cd "$(wt path "$1")"; }

## Hooks

`wt init` creates `.worktrees/` with executable stubs: `pre-create`,
`post-create`, `pre-remove`, `post-remove`. Each receives `WT_*` env vars
(`WT_SOURCE_ROOT`, `WT_TARGET_ROOT`, `WT_NAME`, `WT_BRANCH`, `WT_BASE_REF`,
`WT_CONTAINER`, `WT_REPO_NAME`, `WT_HOOK`). Example post-create:

	cp "$WT_SOURCE_ROOT/.env" "$WT_TARGET_ROOT/.env"

Hooks are arbitrary executables from the repo — review them before use, or pass
`--no-hooks`. A failing hook aborts (no rollback of an already-created worktree).

## Config

`.worktrees/config.yaml` (CLI flags override):

	base_ref: HEAD
	container: ""
	name_template: ""
```

- [ ] **Step 2: Run the entire suite**

Run:
```bash
go vet ./...
go test ./...
go test -tags integration ./...
go build ./...
```
Expected: all clean + PASS; binary builds.

- [ ] **Step 3: Manual smoke test in a scratch repo**

Run:
```bash
tmp=$(mktemp -d) && git -C "$tmp" init -q -b main && \
  git -C "$tmp" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init && \
  (cd "$tmp" && go run github.com/homeend/worktrees new smoke --no-hooks && \
   go run github.com/homeend/worktrees ls && \
   go run github.com/homeend/worktrees rm smoke)
```
Expected: creates `smoke`, lists it, removes it without error.

> If `go run github.com/homeend/worktrees` fails to resolve in the scratch dir, build
> the binary first (`go build -o /tmp/wt .`) and call `/tmp/wt` instead.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add README with usage, hooks, and config"
```

### Task 8.2: Final Review Gate (whole project)

- [ ] **Step 1: Architectural review (subagent)** of the whole tree. Verify: dependency direction (`pkg/worktree` ← `internal/*` defaults ← `cmd/wt` wiring), no `internal/*` imported by `pkg/worktree` except `naming`, every spec §3 decision implemented.
- [ ] **Step 2: Spec coverage check.** Walk spec §6 commands and §8 hooks; confirm each maps to a task/file. Note any gaps as follow-up.
- [ ] **Step 3: Commit any final fixes** — `git add -A && git commit -m "chore: final review fixes"`

---

## Notes for the implementer

- **Build tags:** integration tests use `//go:build integration`. Run them with `-tags integration`. Plain `go test ./...` runs only fast unit tests.
- **git identity in tests:** temp-repo helpers set `GIT_AUTHOR_*`/`GIT_COMMITTER_*` via env so commits work without global git config.
- **No shell strings:** every git/hook invocation uses `exec.Command` arg slices — never build a shell command string.
- **Module go directive** is 1.23 for compatibility even though the local toolchain is newer.
