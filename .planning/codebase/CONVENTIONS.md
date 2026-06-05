# Coding Conventions

**Analysis Date:** 2026-06-05

## Naming Patterns

**Files:**
- Snake-case for multi-word files: `kill_em_all.go`, `kill_em_all_test.go`
- Test files co-located with source using the `_test.go` suffix
- Integration tests use the same naming with `_integration_test.go`
- Single-concept files use a plain noun: `manager.go`, `interfaces.go`, `types.go`, `view.go`

**Packages:**
- Short, lowercase, singular: `worktree`, `git`, `config`, `naming`, `hooks`, `tui`, `cmd`
- The CLI package uses `package cmd` (not `package wt`) in `cmd/wt/`

**Functions and Methods:**
- Exported: PascalCase — `AddWorktree`, `ListWorktrees`, `MainRoot`, `BranchExists`, `CheckRefFormat`
- Unexported: camelCase — `resolveNames`, `containerPath`, `worktreePath`, `effectivePrefix`, `pruneEmptyParents`
- Constructor pattern: `New(...)` for primary constructors; `Defaults()` for config
- Factory helpers: `newFakeGit`, `newFakeHooks`, `newTestManager` in test files
- Boolean-returning unexported predicates: `versionLess`, `shouldLaunchTUI`

**Types:**
- PascalCase structs: `Manager`, `Runner`, `WorktreeInfo`, `AddOptions`, `RemoveResult`, `HookContext`
- Interface names describe the implementor's role (not what it does): `GitRunner`, `HookRunner`, `ConfigProvider`
- Options structs for complex operations: `AddOptions`, `RemoveOptions` (passed by value)
- Result structs for return data: `AddResult`, `RemoveResult`, `RemoveAllResult`, `RemoveAllPlan`

**Constants:**
- PascalCase exported constants: `PreCreate`, `PostCreate`, `PreRemove`, `PostRemove`
- Typed string constants for domain enums: `type HookEvent string`

**Variables:**
- Package-level cobra command vars: lowercase with prefix matching the command — `newBranch`, `rmForce`, `killYes`
- Sentinel errors: `Err`-prefixed PascalCase — `ErrNotARepo`, `ErrNameCollision`, `ErrHookFailed`, `ErrDirtyWorktree`, `ErrPartialCleanup`
- Style vars (lipgloss): `titleStyle`, `selectedStyle`, `promptStyle`, `statusStyle`

## Code Style

**Formatting:**
- Standard `gofmt` formatting (implied; no alternate formatter config found)
- No `.editorconfig`, `.golangci.yml`, or Makefile present — formatting enforced by `go fmt` convention

**Linting:**
- No golangci-lint config file present
- Code follows standard Go idioms (no non-idiomatic patterns observed)

**Line length:**
- No enforced limit; long lines are rare and limited to error messages and multi-clause conditions

## Import Organization

**Order (standard Go):**
1. Standard library: `fmt`, `os`, `path/filepath`, `strings`, `errors`, `time`, `text/template`
2. Third-party: `github.com/spf13/cobra`, `github.com/charmbracelet/bubbletea`, `gopkg.in/yaml.v3`
3. Internal project: `github.com/code-drill/wt/internal/...`, `github.com/code-drill/wt/pkg/worktree`

**Import aliases:**
- Used only to disambiguate: `cryptorand "crypto/rand"` in `cmd/wt/root.go`
- Third-party TUI library imported as `tea`: `tea "github.com/charmbracelet/bubbletea"`

**Path aliases:**
- None — no `go.work` or alias configuration; full module paths used throughout

## Error Handling

**Patterns:**
- Errors always wrapped with context using `fmt.Errorf("context: %w", err)` — never returned bare
- Sentinel errors defined in `cmd/wt/errors.go` for stable CLI exit codes
- Descriptor wrapping at the CLI boundary: `classify(err)` in `cmd/wt/errors.go` inspects error message strings to wrap with the correct sentinel
- Internal packages return descriptive wrapped errors (not sentinels); CLI layer translates via `classify`
- `errors.Is` used for sentinel comparison (never string matching in `exitCodeFor`)
- Soft failures modeled via return values (`(bool, error)` for `DeleteBranch`) rather than separate error types
- Best-effort operations collect failures in a slice (`[]CleanupFailure`) and return a sentinel only at the end

**Error message style:**
- Lowercase, no trailing period, context-prefixed: `"resolve repo root: %w"`, `"git worktree add: %w"`, `"pre-create hook failed (nothing created): %w"`
- Include affected value in quotes for user-facing errors: `"branch %q already exists; pass a different --branch"`
- Parenthetical notes for transactions: `"(nothing created)"`, `"(worktree left in place at %s)"`

**Never ignored:**
- `os.MkdirAll`, `os.WriteFile`, `yaml.Unmarshal` errors are always checked
- The only silently ignored errors are intentional (e.g., `pruneEmptyParents` stops on first non-empty parent)

## Function Design

**Size:**
- Functions are short and focused; the longest is `Manager.Add` at ~80 lines including blank lines
- Complex logic extracted into named helpers: `resolveNames`, `effectivePrefix`, `containerPath`, `worktreePath`

**Parameters:**
- Options structs used when a function takes more than 3–4 parameters: `AddOptions`, `RemoveOptions`, `killOpts`
- Interfaces injected into structs at construction time for testability
- Functions extracted from cobra `RunE` closures for unit-testability: `buildAddOptions`, `runKillEmAll`, `renderListJSON`, `runSet`, `resolveWorktreePath`

**Return values:**
- Single `error` for operations with no output
- `(ResultType, error)` for operations that produce data
- `(bool, error)` for operations with a soft-failure outcome: `DeleteBranch` returns `(false, nil)` for safe-delete refusal

## Module Design

**Exports:**
- Packages export a minimal surface: only types and functions that callers need
- Unexported helpers are the norm; exported only when consumed by another package
- No barrel/re-export files

**Interface placement:**
- Interfaces defined in the consumer package, not the implementor: `GitRunner`, `HookRunner`, `ConfigProvider` are in `pkg/worktree/interfaces.go`, implemented by `internal/git`, `internal/hooks`, `internal/config`

**Adapter pattern:**
- Thin adapters bridge concrete types to interfaces at the wiring point: `gitAdapter` and `cfgAdapter` in `cmd/wt/root.go` adapt `*git.Runner` and `config.Config` to `worktree.GitRunner` and `worktree.ConfigProvider`

**Testability injection:**
- `Manager.now` and `Manager.digits` are injectable `func()` fields, set to deterministic values in tests via `m.now = ...` and `m.digits = ...`
- `model.runAction` in TUI is an injectable `func(args ...string) tea.Cmd` replaced in tests with a recorder
- `SetDigits(fn func() int)` is a public setter for external wiring

## Comments

**When to comment:**
- Every exported type, function, and interface has a doc comment
- Unexported functions have doc comments when non-obvious (most do)
- Comments explain *why*, not *what*, for non-trivial logic
- Transaction semantics documented inline: what happens on pre-hook failure vs post-hook failure

**Comment style:**
- Doc comments start with the identifier name: `// Manager orchestrates...`, `// New constructs...`, `// Run executes...`
- Multi-sentence comments are used freely for complex behaviors

**Inline comments:**
- Used sparingly for non-obvious expressions
- Test files include comments explaining expected values: `// Expectation verified against internal/naming.Generate(...)`

## Logging

**Framework:** None — no structured logging library used

**Patterns:**
- User-facing output via `fmt.Printf` / `fmt.Fprintln` directly to `os.Stdout`
- Hook stdout/stderr stream directly to the process's `os.Stdout` / `os.Stderr`
- TUI actions log to a temp file via `os.CreateTemp("", "wt-action-*.log")` and tee output with `io.MultiWriter`

---

*Convention analysis: 2026-06-05*
