<!-- GSD:project-start source:PROJECT.md -->
## Project

**wt — Git Worktree Manager**

`wt` is a Go CLI + TUI for managing git worktrees: it creates branch-mirrored worktrees in a sibling container, lists/removes/prunes them, runs lifecycle hooks, and supports name templates and a configurable branch prefix. This milestone adds the ability to spawn a *sibling iteration* of the worktree you're currently standing in — `wt new` run from inside a worktree branches off that worktree's own branch instead of the repo's base ref.

**Core Value:** Running `wt new` from inside a worktree creates a new branch + worktree based on the current worktree's branch, auto-named with a free `-vNNN` suffix (or a caller-supplied suffix) — without the user having to return to the repo root or name the branch by hand.

### Constraints

- **Tech stack**: Go + cobra CLI; must extend the existing `Manager.Add` / `AddOptions` flow and `GitRunner` interface rather than bypass it.
- **Compatibility**: `wt new` from the main repo root must behave exactly as today (no regression); the new behavior triggers only from inside a worktree.
- **Testing**: New logic must be unit-testable via the injected `GitRunner`/`ConfigProvider` fakes; avoid hard dependencies on a real git repo in unit tests.
- **Naming safety**: Generated branch names still pass `CheckRefFormat` and collision checks before creation.
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.25.0 - All application code (`main.go`, `cmd/`, `internal/`, `pkg/`)
- YAML - Configuration file format (`.worktrees/config.yaml`)
- Go `text/template` - Branch name templating (`internal/naming/naming.go`)
## Runtime
- Go runtime (compiled binary, no interpreter)
- Minimum: Go 1.25.0 (declared in `go.mod`)
- Go modules (`go mod`)
- Lockfile: `go.sum` present and committed
## Frameworks
- `github.com/spf13/cobra v1.10.2` - CLI command framework (`cmd/wt/root.go`, all `cmd/wt/*.go`)
- `github.com/charmbracelet/bubbletea v1.3.10` - TUI framework (`internal/tui/tui.go`, `internal/tui/model.go`)
- `github.com/charmbracelet/lipgloss v1.1.0` - TUI styling (`internal/tui/view.go`)
- `github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc` - Terminal color detection (indirect)
- Go standard library `testing` package - No external test framework
- Integration tests use real git repos: `internal/git/git_integration_test.go`, `pkg/worktree/manager_integration_test.go`
- Go toolchain only — no Makefile, task runner, or build scripts detected
## Key Dependencies
- `github.com/spf13/cobra v1.10.2` - CLI entrypoint and command routing
- `gopkg.in/yaml.v3 v3.0.1` - Parses `.worktrees/config.yaml` (`internal/config/config.go`)
- `github.com/charmbracelet/bubbletea v1.3.10` - Interactive TUI mode (launched when stdout is a TTY with no subcommand)
- `github.com/charmbracelet/x/ansi v0.10.1` - ANSI escape code handling
- `github.com/charmbracelet/x/term v0.2.1` - Terminal state management
- `github.com/muesli/cancelreader v0.2.2` - Cancellable stdin reader
- `github.com/muesli/termenv v0.16.0` - Terminal environment detection
- `github.com/mattn/go-isatty v0.0.20` - TTY detection
- `golang.org/x/term v0.43.0` - TTY check in `cmd/wt/root.go` (`term.IsTerminal`)
- `github.com/rivo/uniseg v0.4.7` - Unicode segmentation
- `github.com/lucasb-eyer/go-colorful v1.2.0` - Color conversion
- `os/exec` - Shell-out to `git` binary (`internal/git/git.go`, `internal/hooks/hooks.go`)
- `crypto/rand` - Secure random digits for name generation (`cmd/wt/root.go`)
- `text/template` - Branch name and user templates (`internal/naming/naming.go`)
- `path/filepath`, `os`, `io/fs` - File and directory operations throughout
## Configuration
- `WT_BRANCH_PREFIX` - Overrides `branch_prefix` from config file (highest precedence)
- `LC_ALL=C` - Set by the git runner to ensure locale-stable git output (not user-configurable)
- `GIT_TERMINAL_PROMPT=0` - Set by the git runner to disable git credential prompts
- Location: `<repo-root>/.worktrees/config.yaml`
- Read by: `internal/config/config.go`
- Supported keys: `base_ref`, `container`, `name_template`, `branch_prefix`, `templates`
- Precedence: `WT_BRANCH_PREFIX` env > `config.yaml` > built-in defaults
- Built-in defaults: `base_ref: HEAD`, `branch_prefix: wt/`
- No external build config. Build with `go build ./...`
- No `Dockerfile`, `Makefile`, or CI config detected in repo root
## Platform Requirements
- Go 1.25.0+
- `git` binary on PATH (minimum git 2.30, enforced at runtime in `cmd/wt/root.go`)
- POSIX-compatible terminal for TUI mode
- Single compiled binary (`wt`)
- `git` 2.30+ on PATH at runtime
- Any OS supported by Go (Linux, macOS, Windows — though some git invocation paths are Unix-idiomatic)
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
- Snake-case for multi-word files: `kill_em_all.go`, `kill_em_all_test.go`
- Test files co-located with source using the `_test.go` suffix
- Integration tests use the same naming with `_integration_test.go`
- Single-concept files use a plain noun: `manager.go`, `interfaces.go`, `types.go`, `view.go`
- Short, lowercase, singular: `worktree`, `git`, `config`, `naming`, `hooks`, `tui`, `cmd`
- The CLI package uses `package cmd` (not `package wt`) in `cmd/wt/`
- Exported: PascalCase — `AddWorktree`, `ListWorktrees`, `MainRoot`, `BranchExists`, `CheckRefFormat`
- Unexported: camelCase — `resolveNames`, `containerPath`, `worktreePath`, `effectivePrefix`, `pruneEmptyParents`
- Constructor pattern: `New(...)` for primary constructors; `Defaults()` for config
- Factory helpers: `newFakeGit`, `newFakeHooks`, `newTestManager` in test files
- Boolean-returning unexported predicates: `versionLess`, `shouldLaunchTUI`
- PascalCase structs: `Manager`, `Runner`, `WorktreeInfo`, `AddOptions`, `RemoveResult`, `HookContext`
- Interface names describe the implementor's role (not what it does): `GitRunner`, `HookRunner`, `ConfigProvider`
- Options structs for complex operations: `AddOptions`, `RemoveOptions` (passed by value)
- Result structs for return data: `AddResult`, `RemoveResult`, `RemoveAllResult`, `RemoveAllPlan`
- PascalCase exported constants: `PreCreate`, `PostCreate`, `PreRemove`, `PostRemove`
- Typed string constants for domain enums: `type HookEvent string`
- Package-level cobra command vars: lowercase with prefix matching the command — `newBranch`, `rmForce`, `killYes`
- Sentinel errors: `Err`-prefixed PascalCase — `ErrNotARepo`, `ErrNameCollision`, `ErrHookFailed`, `ErrDirtyWorktree`, `ErrPartialCleanup`
- Style vars (lipgloss): `titleStyle`, `selectedStyle`, `promptStyle`, `statusStyle`
## Code Style
- Standard `gofmt` formatting (implied; no alternate formatter config found)
- No `.editorconfig`, `.golangci.yml`, or Makefile present — formatting enforced by `go fmt` convention
- No golangci-lint config file present
- Code follows standard Go idioms (no non-idiomatic patterns observed)
- No enforced limit; long lines are rare and limited to error messages and multi-clause conditions
## Import Organization
- Used only to disambiguate: `cryptorand "crypto/rand"` in `cmd/wt/root.go`
- Third-party TUI library imported as `tea`: `tea "github.com/charmbracelet/bubbletea"`
- None — no `go.work` or alias configuration; full module paths used throughout
## Error Handling
- Errors always wrapped with context using `fmt.Errorf("context: %w", err)` — never returned bare
- Sentinel errors defined in `cmd/wt/errors.go` for stable CLI exit codes
- Descriptor wrapping at the CLI boundary: `classify(err)` in `cmd/wt/errors.go` inspects error message strings to wrap with the correct sentinel
- Internal packages return descriptive wrapped errors (not sentinels); CLI layer translates via `classify`
- `errors.Is` used for sentinel comparison (never string matching in `exitCodeFor`)
- Soft failures modeled via return values (`(bool, error)` for `DeleteBranch`) rather than separate error types
- Best-effort operations collect failures in a slice (`[]CleanupFailure`) and return a sentinel only at the end
- Lowercase, no trailing period, context-prefixed: `"resolve repo root: %w"`, `"git worktree add: %w"`, `"pre-create hook failed (nothing created): %w"`
- Include affected value in quotes for user-facing errors: `"branch %q already exists; pass a different --branch"`
- Parenthetical notes for transactions: `"(nothing created)"`, `"(worktree left in place at %s)"`
- `os.MkdirAll`, `os.WriteFile`, `yaml.Unmarshal` errors are always checked
- The only silently ignored errors are intentional (e.g., `pruneEmptyParents` stops on first non-empty parent)
## Function Design
- Functions are short and focused; the longest is `Manager.Add` at ~80 lines including blank lines
- Complex logic extracted into named helpers: `resolveNames`, `effectivePrefix`, `containerPath`, `worktreePath`
- Options structs used when a function takes more than 3–4 parameters: `AddOptions`, `RemoveOptions`, `killOpts`
- Interfaces injected into structs at construction time for testability
- Functions extracted from cobra `RunE` closures for unit-testability: `buildAddOptions`, `runKillEmAll`, `renderListJSON`, `runSet`, `resolveWorktreePath`
- Single `error` for operations with no output
- `(ResultType, error)` for operations that produce data
- `(bool, error)` for operations with a soft-failure outcome: `DeleteBranch` returns `(false, nil)` for safe-delete refusal
## Module Design
- Packages export a minimal surface: only types and functions that callers need
- Unexported helpers are the norm; exported only when consumed by another package
- No barrel/re-export files
- Interfaces defined in the consumer package, not the implementor: `GitRunner`, `HookRunner`, `ConfigProvider` are in `pkg/worktree/interfaces.go`, implemented by `internal/git`, `internal/hooks`, `internal/config`
- Thin adapters bridge concrete types to interfaces at the wiring point: `gitAdapter` and `cfgAdapter` in `cmd/wt/root.go` adapt `*git.Runner` and `config.Config` to `worktree.GitRunner` and `worktree.ConfigProvider`
- `Manager.now` and `Manager.digits` are injectable `func()` fields, set to deterministic values in tests via `m.now = ...` and `m.digits = ...`
- `model.runAction` in TUI is an injectable `func(args ...string) tea.Cmd` replaced in tests with a recorder
- `SetDigits(fn func() int)` is a public setter for external wiring
## Comments
- Every exported type, function, and interface has a doc comment
- Unexported functions have doc comments when non-obvious (most do)
- Comments explain *why*, not *what*, for non-trivial logic
- Transaction semantics documented inline: what happens on pre-hook failure vs post-hook failure
- Doc comments start with the identifier name: `// Manager orchestrates...`, `// New constructs...`, `// Run executes...`
- Multi-sentence comments are used freely for complex behaviors
- Used sparingly for non-obvious expressions
- Test files include comments explaining expected values: `// Expectation verified against internal/naming.Generate(...)`
## Logging
- User-facing output via `fmt.Printf` / `fmt.Fprintln` directly to `os.Stdout`
- Hook stdout/stderr stream directly to the process's `os.Stdout` / `os.Stderr`
- TUI actions log to a temp file via `os.CreateTemp("", "wt-action-*.log")` and tee output with `io.MultiWriter`
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## System Overview
```text
```
## Component Responsibilities
| Component | Responsibility | File |
|-----------|----------------|------|
| `main.go` | Process entry; delegates to `cmd.Execute()` | `main.go` |
| `cmd.Execute()` | Run cobra root; translate errors to exit codes | `cmd/wt/root.go` |
| `rootCmd.RunE` | TTY detection: launch TUI or print help | `cmd/wt/root.go` |
| `buildManager()` | Wire git runner + config + hooks into Manager | `cmd/wt/root.go` |
| `gitAdapter` | Adapt `*git.Runner` to `worktree.GitRunner` interface | `cmd/wt/root.go` |
| `cfgAdapter` | Adapt `config.Config` to `worktree.ConfigProvider` interface | `cmd/wt/root.go` |
| `worktree.Manager` | Domain orchestrator: all worktree lifecycle operations | `pkg/worktree/manager.go` |
| `git.Runner` | Subprocess wrapper: all git commands with LC_ALL=C | `internal/git/git.go` |
| `hooks.Runner` | Discovers and runs `.worktrees/<event>` hook scripts | `internal/hooks/hooks.go` |
| `config.Load()` | Layered config resolution: defaults < file < env | `internal/config/config.go` |
| `naming` | Generates worktree names from templates / defaults | `internal/naming/naming.go` |
| `tui.Run()` | Bubbletea TUI: interactive worktree browser | `internal/tui/tui.go` |
| CLI subcommands | `new`, `rm`, `list`, `path`, `kill-em-all`, `set`, `init`, `prune`, `templates`, `completion` | `cmd/wt/*.go` |
## Pattern Overview
- `pkg/worktree` is the domain core — it imports nothing from `internal/` or `cmd/`
- All external collaborators are injected as interfaces (`GitRunner`, `HookRunner`, `ConfigProvider`)
- Adapters (`gitAdapter`, `cfgAdapter` in `cmd/wt/root.go`) translate concrete types to interfaces
- The TUI delegates mutations back to the CLI binary via `tea.Exec` (re-invokes `wt <subcmd>`)
- Error classification and exit-code mapping are a dedicated CLI-boundary concern (`errors.go`)
## Layers
- Purpose: Parse flags/args, build Manager, call domain, format output
- Location: `cmd/wt/`
- Contains: One `.go` file per subcommand plus `root.go`, `errors.go`, adapter structs
- Depends on: `pkg/worktree`, `internal/config`, `internal/git`, `internal/hooks`, `internal/tui`
- Used by: `main.go`
- Purpose: Interactive full-screen Bubbletea browser; delegates mutations to CLI subcommands
- Location: `internal/tui/`
- Contains: `tui.go` (entry), `model.go` (Elm-architecture model + update), `view.go` (rendering)
- Depends on: `pkg/worktree` (for `List` and type definitions only), re-invokes `wt` binary for mutations
- Used by: `cmd/wt/root.go`
- Purpose: Business logic — create/remove/list/resolve worktrees, lifecycle hooks, name generation
- Location: `pkg/worktree/`
- Contains: `manager.go` (operations), `types.go` (value types), `interfaces.go` (port definitions)
- Depends on: `internal/naming` only; all git/hook/config access is via injected interfaces
- Used by: `cmd/wt/`, `internal/tui/`
- Purpose: Thin subprocess wrapper; parses git porcelain output; no business logic
- Location: `internal/git/`
- Contains: `git.go` (Runner + version), `worktree.go` (worktree commands), `branch.go` (branch commands), `resolve.go` (rev-parse helpers), `parse.go` (porcelain-z parser)
- Depends on: `pkg/worktree` (for `GitWorktree` type in ListWorktrees)
- Used by: `cmd/wt/root.go` (via `gitAdapter`), `cmd/wt/prune.go` (directly)
- Purpose: Load and layer configuration from file + env; write config values back
- Location: `internal/config/config.go`
- Contains: `Config` struct, `Load()`, `LoadFile()`, `Set()`, `Resolve()`, `NormalizePrefix()`
- Depends on: stdlib only, `gopkg.in/yaml.v3`
- Used by: `cmd/wt/root.go`, `cmd/wt/set.go`, `cmd/wt/new.go`
- Purpose: Discover and exec `.worktrees/<event>` scripts with `WT_*` env vars
- Location: `internal/hooks/hooks.go`
- Contains: `Runner` struct, `Run(HookContext)` method
- Depends on: `pkg/worktree` (for `HookContext` and `HookEvent`)
- Used by: `cmd/wt/root.go` (via `hooks.New(repoRoot)`)
- Purpose: Generate worktree names from Go `text/template` or default pattern
- Location: `internal/naming/naming.go`, `internal/naming/words.go`
- Contains: `Generate()`, `GenerateFrom()`, `RenderTemplate()`, word lists for adjectives/nouns
- Depends on: stdlib only
- Used by: `pkg/worktree/manager.go`
## Data Flow
### New Worktree (CLI path)
### New Worktree (TUI path)
### Remove Worktree
### Config Resolution
- No in-process persistent state; every command re-reads config and git state
- TUI model holds a snapshot of `[]WorktreeInfo` refreshed after each action via `reloadCmd()`
## Key Abstractions
- Purpose: Isolates domain from git subprocess; enables fake in tests
- Examples: implemented by `*git.Runner` (production), faked in `pkg/worktree/fakes_test.go`
- Pattern: Minimal surface — only the operations Manager needs
- Purpose: Isolates domain from filesystem hook discovery; enables fake in tests
- Examples: implemented by `*hooks.Runner`, faked in tests
- Purpose: Decouples Manager from config struct details
- Examples: implemented by `cfgAdapter` in `cmd/wt/root.go`
- Purpose: Single orchestrator for all worktree operations; holds no mutable state
- Pattern: Constructor injection; `SetDigits()` for test overrides
- Purpose: Minimal interface needed by TUI model to refresh its list; avoids full Manager dependency
- Pattern: Narrow interface defined at point of use
- Purpose: Adapter structs bridging concrete internal types to `pkg/worktree` interfaces
- Pattern: Structural conversion only — no logic
## Entry Points
- Location: `main.go`
- Triggers: Process start
- Responsibilities: Calls `cmd.Execute()` and passes exit code to `os.Exit`
- Location: `cmd/wt/root.go:Execute()`
- Triggers: Called from `main.go`
- Responsibilities: Cobra `rootCmd.Execute()`, error classification (`classify()`), exit code mapping
- Location: `internal/tui/tui.go:Run()`
- Triggers: `rootCmd.RunE` when bare `wt` on a TTY
- Responsibilities: Initial `List()`, Bubbletea program setup with alt-screen
- Location: `cmd/wt/root.go:buildManager()`
- Triggers: Every subcommand that needs a Manager
- Responsibilities: Git version gate, repo root resolution, config load, adapter wiring
## Architectural Constraints
- **Threading:** Single-threaded; Bubbletea manages its own goroutines internally. TUI mutations run as a subprocess (`tea.Exec`), not concurrently in-process.
- **Global state:** `rootCmd`, `repoFlag`, and per-subcommand flag variables are package-level globals in `cmd/wt/`. These are Cobra-idiomatic but not safe for concurrent reuse.
- **Interface coupling direction:** `pkg/worktree` imports `internal/naming`; it does NOT import `internal/git`, `internal/hooks`, or `internal/config`. The import goes upward only through adapters in `cmd/wt/root.go`.
- **Circular imports:** None. `pkg/worktree` → `internal/naming`; `internal/git` → `pkg/worktree` (for `GitWorktree` type); `internal/hooks` → `pkg/worktree` (for `HookContext`). These are one-way.
- **Git version requirement:** `git 2.30+` enforced at `buildManager()` and `pruneCmd` via `EnsureMinVersion(2, 30)`.
## Anti-Patterns
### Direct git.Runner use in `prune` subcommand
### Flag variables as package-level globals
## Error Handling
- Domain (`pkg/worktree`): returns wrapped errors with `fmt.Errorf("%w: detail")` — descriptive, not sentinel
- CLI boundary (`cmd/wt/errors.go`): `classify()` inspects error message strings to wrap with sentinels (e.g., `ErrHookFailed`, `ErrNameCollision`)
- Exit codes: `exitCodeFor()` maps sentinels to stable codes (2–6); generic errors → 1
- Mid-transaction partial results: `RemoveResult` is populated with completed steps even when a later step fails; the TUI surfaces the log path via `actionFinishedMsg.logPath`
## Cross-Cutting Concerns
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
