<!-- refreshed: 2026-06-05 -->
# Architecture

**Analysis Date:** 2026-06-05

## System Overview

```text
┌──────────────────────────────────────────────────────────────────────┐
│                      Entry Point                                      │
│  `main.go`  →  cmd.Execute()  →  cobra rootCmd                       │
├──────────────┬──────────────┬────────────────────────────────────────┤
│  TUI mode    │  CLI mode    │    Config/Setup mode                   │
│  `tui.Run`   │  subcommands │   `init`, `set`, `prune`               │
│ (bare `wt`)  │ new/rm/ls/…  │   (no Manager required)                │
└──────┬───────┴──────┬───────┴───────────────────────────────────────┘
       │              │
       ▼              ▼
┌──────────────────────────────────────────────────────────────────────┐
│                   pkg/worktree.Manager                               │
│                   `pkg/worktree/manager.go`                          │
│   Add · Remove · RemoveAll · List · Find · PlanRemoveAll            │
│   Templates · ResolveTemplate                                        │
└──────┬───────────────────────────────────────────┬──────────────────┘
       │                                           │
       ▼                                           ▼
┌──────────────────────────────┐   ┌──────────────────────────────────┐
│  internal/git.Runner         │   │  internal/hooks.Runner           │
│  `internal/git/`             │   │  `internal/hooks/hooks.go`       │
│  Executes git subprocess     │   │  Runs .worktrees/<event> scripts │
└──────────────────────────────┘   └──────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────────┐
│  Git binary (subprocess)  +  .worktrees/config.yaml                  │
│  internal/config.Config  loaded via `internal/config/config.go`      │
└──────────────────────────────────────────────────────────────────────┘
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

**Overall:** Hexagonal / Ports-and-Adapters (dependency inversion at the domain boundary)

**Key Characteristics:**
- `pkg/worktree` is the domain core — it imports nothing from `internal/` or `cmd/`
- All external collaborators are injected as interfaces (`GitRunner`, `HookRunner`, `ConfigProvider`)
- Adapters (`gitAdapter`, `cfgAdapter` in `cmd/wt/root.go`) translate concrete types to interfaces
- The TUI delegates mutations back to the CLI binary via `tea.Exec` (re-invokes `wt <subcmd>`)
- Error classification and exit-code mapping are a dedicated CLI-boundary concern (`errors.go`)

## Layers

**CLI Layer (`cmd/wt/`):**
- Purpose: Parse flags/args, build Manager, call domain, format output
- Location: `cmd/wt/`
- Contains: One `.go` file per subcommand plus `root.go`, `errors.go`, adapter structs
- Depends on: `pkg/worktree`, `internal/config`, `internal/git`, `internal/hooks`, `internal/tui`
- Used by: `main.go`

**TUI Layer (`internal/tui/`):**
- Purpose: Interactive full-screen Bubbletea browser; delegates mutations to CLI subcommands
- Location: `internal/tui/`
- Contains: `tui.go` (entry), `model.go` (Elm-architecture model + update), `view.go` (rendering)
- Depends on: `pkg/worktree` (for `List` and type definitions only), re-invokes `wt` binary for mutations
- Used by: `cmd/wt/root.go`

**Domain Layer (`pkg/worktree/`):**
- Purpose: Business logic — create/remove/list/resolve worktrees, lifecycle hooks, name generation
- Location: `pkg/worktree/`
- Contains: `manager.go` (operations), `types.go` (value types), `interfaces.go` (port definitions)
- Depends on: `internal/naming` only; all git/hook/config access is via injected interfaces
- Used by: `cmd/wt/`, `internal/tui/`

**Git Adapter (`internal/git/`):**
- Purpose: Thin subprocess wrapper; parses git porcelain output; no business logic
- Location: `internal/git/`
- Contains: `git.go` (Runner + version), `worktree.go` (worktree commands), `branch.go` (branch commands), `resolve.go` (rev-parse helpers), `parse.go` (porcelain-z parser)
- Depends on: `pkg/worktree` (for `GitWorktree` type in ListWorktrees)
- Used by: `cmd/wt/root.go` (via `gitAdapter`), `cmd/wt/prune.go` (directly)

**Config Layer (`internal/config/`):**
- Purpose: Load and layer configuration from file + env; write config values back
- Location: `internal/config/config.go`
- Contains: `Config` struct, `Load()`, `LoadFile()`, `Set()`, `Resolve()`, `NormalizePrefix()`
- Depends on: stdlib only, `gopkg.in/yaml.v3`
- Used by: `cmd/wt/root.go`, `cmd/wt/set.go`, `cmd/wt/new.go`

**Hooks Layer (`internal/hooks/`):**
- Purpose: Discover and exec `.worktrees/<event>` scripts with `WT_*` env vars
- Location: `internal/hooks/hooks.go`
- Contains: `Runner` struct, `Run(HookContext)` method
- Depends on: `pkg/worktree` (for `HookContext` and `HookEvent`)
- Used by: `cmd/wt/root.go` (via `hooks.New(repoRoot)`)

**Naming Utility (`internal/naming/`):**
- Purpose: Generate worktree names from Go `text/template` or default pattern
- Location: `internal/naming/naming.go`, `internal/naming/words.go`
- Contains: `Generate()`, `GenerateFrom()`, `RenderTemplate()`, word lists for adjectives/nouns
- Depends on: stdlib only
- Used by: `pkg/worktree/manager.go`

## Data Flow

### New Worktree (CLI path)

1. User runs `wt new [name]` (`cmd/wt/new.go`)
2. `buildManager(cwd)` in `root.go`: git version check → `config.Load(repoRoot)` → `worktree.New(gitAdapter, hooksRunner, cfgAdapter)`
3. `buildAddOptions()` parses flags into `worktree.AddOptions`
4. `Manager.Add(cwd, opts)` in `pkg/worktree/manager.go`:
   a. `resolveNames(opts)` → `naming.GenerateFrom(nameTemplate, now, digits)` for auto-name
   b. `effectivePrefix(opts)` resolves branch prefix from flags/config
   c. `git.CheckRefFormat(branch)`, `git.BranchExists(...)`, `git.VerifyRef(baseRef)`
   d. `hooks.Run(PreCreate HookContext)` — aborts if non-zero exit
   e. `git.AddWorktree(repoRoot, target, branch, baseRef)` — dir mirrors full branch ref
   f. `hooks.Run(PostCreate HookContext)` — non-zero leaves worktree but returns error
5. CLI prints `Created worktree` summary to stdout

### New Worktree (TUI path)

1. User presses `n` in TUI (`internal/tui/model.go:updateNormal`)
2. `m.runAction("new", "--repo", m.dir)` → `defaultRunAction()` in `model.go`
3. `tea.Exec(loggedExec{...}, callback)` — suspends TUI, restores terminal
4. TUI binary re-invoked as `wt new --repo <dir>` (same binary, CLI subcommand path above)
5. On completion: `actionFinishedMsg` → `m.reloadCmd()` refreshes worktree list

### Remove Worktree

1. `Manager.Remove(dir, opts)` in `pkg/worktree/manager.go`:
   a. `resolveWorktree(dir, name)` — matches by branch, container-relative path, or leaf name
   b. `hooks.Run(PreRemove)` — aborts if non-zero exit
   c. `git.RemoveWorktree(repoRoot, path, force)`
   d. `pruneEmptyParents(container, path)` — removes empty intermediate dirs up to container
   e. `git.DeleteBranch(repoRoot, branch, forceBranch)` — safe delete returns (false, nil) for unmerged
   f. `hooks.Run(PostRemove)` — runs from repoRoot after removal

### Config Resolution

1. `config.Load(repoRoot)` called in `buildManager()`
2. `Defaults()` → `Config{BaseRef:"HEAD", BranchPrefix:"wt/"}`
3. `Resolve(defaults, fileCfg)` — file values win over defaults
4. `Resolve(merged, envCfg())` — `WT_BRANCH_PREFIX` env wins over file
5. `NormalizePrefix()` ensures single trailing slash on `BranchPrefix`

**State Management:**
- No in-process persistent state; every command re-reads config and git state
- TUI model holds a snapshot of `[]WorktreeInfo` refreshed after each action via `reloadCmd()`

## Key Abstractions

**`worktree.GitRunner` interface (`pkg/worktree/interfaces.go`):**
- Purpose: Isolates domain from git subprocess; enables fake in tests
- Examples: implemented by `*git.Runner` (production), faked in `pkg/worktree/fakes_test.go`
- Pattern: Minimal surface — only the operations Manager needs

**`worktree.HookRunner` interface (`pkg/worktree/interfaces.go`):**
- Purpose: Isolates domain from filesystem hook discovery; enables fake in tests
- Examples: implemented by `*hooks.Runner`, faked in tests

**`worktree.ConfigProvider` interface (`pkg/worktree/interfaces.go`):**
- Purpose: Decouples Manager from config struct details
- Examples: implemented by `cfgAdapter` in `cmd/wt/root.go`

**`worktree.Manager` (`pkg/worktree/manager.go`):**
- Purpose: Single orchestrator for all worktree operations; holds no mutable state
- Pattern: Constructor injection; `SetDigits()` for test overrides

**`tui.lister` interface (`internal/tui/model.go`):**
- Purpose: Minimal interface needed by TUI model to refresh its list; avoids full Manager dependency
- Pattern: Narrow interface defined at point of use

**`cmd.gitAdapter` / `cmd.cfgAdapter` (`cmd/wt/root.go`):**
- Purpose: Adapter structs bridging concrete internal types to `pkg/worktree` interfaces
- Pattern: Structural conversion only — no logic

## Entry Points

**Binary Entry:**
- Location: `main.go`
- Triggers: Process start
- Responsibilities: Calls `cmd.Execute()` and passes exit code to `os.Exit`

**CLI Dispatch:**
- Location: `cmd/wt/root.go:Execute()`
- Triggers: Called from `main.go`
- Responsibilities: Cobra `rootCmd.Execute()`, error classification (`classify()`), exit code mapping

**TUI Entry:**
- Location: `internal/tui/tui.go:Run()`
- Triggers: `rootCmd.RunE` when bare `wt` on a TTY
- Responsibilities: Initial `List()`, Bubbletea program setup with alt-screen

**Manager Construction:**
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

**What happens:** `cmd/wt/prune.go` constructs a `git.Runner` directly and calls `r.Prune()`, bypassing the Manager entirely.
**Why it's wrong:** It skips the version check wiring path that `buildManager` provides and doesn't benefit from the Manager abstraction (e.g., would not run hooks if prune ever needed them).
**Do this instead:** Route through `buildManager()` and expose `Prune` on `Manager` similarly to `RemoveAll`, or add `Prune` to the `GitRunner` interface and call it via Manager.

### Flag variables as package-level globals

**What happens:** All subcommand flags are declared as package-level vars (`var rmForce bool`, etc.) in `cmd/wt/`.
**Why it's wrong:** Makes unit testing cobra commands harder (state bleeds between test cases) and prevents concurrency.
**Do this instead:** Each subcommand's RunE closure captures flag vars from a local struct or uses `cmd.Flags().GetBool()`. The extraction pattern (e.g., `buildAddOptions`, `worktreeRemoveOptions`) already partially addresses this for testing.

## Error Handling

**Strategy:** Errors propagate from domain → CLI boundary → classify → exit code mapping

**Patterns:**
- Domain (`pkg/worktree`): returns wrapped errors with `fmt.Errorf("%w: detail")` — descriptive, not sentinel
- CLI boundary (`cmd/wt/errors.go`): `classify()` inspects error message strings to wrap with sentinels (e.g., `ErrHookFailed`, `ErrNameCollision`)
- Exit codes: `exitCodeFor()` maps sentinels to stable codes (2–6); generic errors → 1
- Mid-transaction partial results: `RemoveResult` is populated with completed steps even when a later step fails; the TUI surfaces the log path via `actionFinishedMsg.logPath`

## Cross-Cutting Concerns

**Logging:** None — no structured logger. Git subprocess stdout/stderr streams directly to process stdout/stderr. Hook output streams live via `cmd.Stdout = os.Stdout`. TUI actions tee output to a temp log file (`wt-action-*.log`) via `loggedExec` in `internal/tui/model.go`.

**Validation:** Branch name validation via `git check-ref-format` (delegated to git). Config key validation via explicit allowlist in `config.Set()`. Template variable validation via Go `text/template` with `missingkey=error`.

**Authentication:** Not applicable — operates on local git repositories only.

---

*Architecture analysis: 2026-06-05*
