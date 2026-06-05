# Codebase Structure

**Analysis Date:** 2026-06-05

## Directory Layout

```
worktrees/                        # module: github.com/code-drill/wt
├── main.go                       # Binary entry point (3 lines)
├── go.mod                        # Module definition + dependency pinning
├── go.sum                        # Dependency checksums
│
├── cmd/
│   └── wt/                       # All CLI subcommands (package cmd)
│       ├── root.go               # rootCmd, Execute(), buildManager(), adapters
│       ├── errors.go             # Sentinel errors + classify() + exitCodeFor()
│       ├── new.go                # `wt new` — create worktree
│       ├── rm.go                 # `wt rm` — remove worktree
│       ├── list.go               # `wt list` / `wt ls`
│       ├── path.go               # `wt path` — print worktree path
│       ├── kill_em_all.go        # `wt kill-em-all` — remove all worktrees
│       ├── set.go                # `wt set` — write config key
│       ├── init.go               # `wt init` — scaffold .worktrees/
│       ├── prune.go              # `wt prune` — git worktree prune
│       ├── templates.go          # `wt templates` — list branch templates
│       ├── completion.go         # `wt completion` — shell completion
│       ├── root_test.go          # Tests for root helpers
│       ├── new_test.go           # Tests for buildAddOptions
│       ├── set_test.go           # Tests for runSet
│       ├── templates_test.go     # Tests for printTemplates
│       ├── kill_em_all_test.go   # Tests for runKillEmAll
│       └── cli_integration_test.go # Integration tests (real git repos)
│
├── pkg/
│   └── worktree/                 # Domain package (importable by external consumers)
│       ├── interfaces.go         # GitRunner, HookRunner, ConfigProvider interfaces
│       ├── types.go              # Value types: AddOptions, RemoveOptions, WorktreeInfo, etc.
│       ├── manager.go            # Manager struct + all domain operations
│       ├── manager_test.go       # Unit tests with fakes
│       ├── manager_integration_test.go # Integration tests (real git)
│       └── fakes_test.go         # Fake implementations of interfaces (test-only)
│
├── internal/
│   ├── git/                      # Git subprocess adapter
│   │   ├── git.go                # Runner struct, Run(), Version(), EnsureMinVersion()
│   │   ├── worktree.go           # AddWorktree, ListWorktrees, RemoveWorktree, Prune
│   │   ├── branch.go             # ListBranches, BranchExists, DeleteBranch
│   │   ├── resolve.go            # TopLevel, MainRoot, VerifyRef, CheckRefFormat
│   │   ├── parse.go              # parsePorcelainZ (--porcelain -z output parser)
│   │   ├── git_test.go           # Unit tests
│   │   └── git_integration_test.go # Integration tests (real git)
│   │
│   ├── config/                   # Configuration loading and writing
│   │   ├── config.go             # Config struct, Load(), LoadFile(), Set(), Resolve()
│   │   └── config_test.go
│   │
│   ├── hooks/                    # Lifecycle hook runner
│   │   ├── hooks.go              # Runner struct, Run(HookContext)
│   │   └── hooks_test.go
│   │
│   ├── naming/                   # Worktree name generation
│   │   ├── naming.go             # Generate(), GenerateFrom(), RenderTemplate()
│   │   ├── words.go              # Adjective and noun word lists
│   │   └── naming_test.go
│   │
│   └── tui/                      # Bubbletea interactive TUI
│       ├── tui.go                # Run() entry point
│       ├── model.go              # Model struct, Init/Update logic, loggedExec
│       ├── view.go               # View() rendering, lipgloss styles
│       └── model_test.go
│
├── docs/
│   └── superpowers/
│       ├── specs/                # Design spec documents
│       └── plans/                # Implementation plan documents
│
└── .planning/
    └── codebase/                 # GSD codebase analysis documents
        ├── STACK.md
        ├── ARCHITECTURE.md
        └── STRUCTURE.md
```

## Directory Purposes

**`cmd/wt/`:**
- Purpose: All Cobra subcommand definitions and CLI wiring
- Contains: One file per subcommand; shared wiring in `root.go` and `errors.go`
- Key files: `root.go` (manager construction, adapters), `errors.go` (exit codes)
- Note: Every subcommand registers itself via `func init() { rootCmd.AddCommand(...) }`

**`pkg/worktree/`:**
- Purpose: Domain core — the public API for worktree management
- Contains: Interfaces, types, and the Manager orchestrator
- Key files: `interfaces.go` (port definitions), `manager.go` (all operations)
- Note: Uses `pkg/` (not `internal/`) intentionally — could be imported by external tools

**`internal/git/`:**
- Purpose: Subprocess adapter for git; parses porcelain output
- Contains: Method-per-git-operation; no business logic
- Key files: `git.go` (Runner base), `resolve.go` (repo navigation), `parse.go` (output parsing)

**`internal/config/`:**
- Purpose: YAML config loading with layered precedence
- Contains: Single `config.go` handling load, resolve, and write
- Config file location at runtime: `<repoRoot>/.worktrees/config.yaml`

**`internal/hooks/`:**
- Purpose: Discover and execute `.worktrees/<event>` hook scripts
- Contains: Single `hooks.go`; hook discovery is filesystem-based (no registry)

**`internal/naming/`:**
- Purpose: Generate human-readable worktree names
- Contains: Template rendering (`naming.go`) and word lists (`words.go`)
- Default pattern: `YYYY-MM-DD_HH-mm-<adjective>-<noun>-NNNN`

**`internal/tui/`:**
- Purpose: Full-screen interactive list in Bubbletea (Elm architecture)
- Contains: Entry (`tui.go`), model+update (`model.go`), rendering (`view.go`)

## Key File Locations

**Entry Points:**
- `main.go`: Binary entry; delegates immediately to `cmd.Execute()`
- `cmd/wt/root.go`: All wiring — `buildManager()`, adapters, `Execute()`, TTY detection

**Domain Core:**
- `pkg/worktree/manager.go`: All worktree lifecycle logic
- `pkg/worktree/interfaces.go`: Port definitions (what Manager needs from outside)
- `pkg/worktree/types.go`: All value types passed between layers

**Configuration:**
- `internal/config/config.go`: Load/write/resolve config
- Runtime config file: `<repoRoot>/.worktrees/config.yaml` (not in source tree)

**Git Operations:**
- `internal/git/git.go`: Base `Runner`; all git commands go through `Runner.Run()`
- `internal/git/resolve.go`: Repo root navigation (`MainRoot`, `VerifyRef`)
- `internal/git/parse.go`: Porcelain-z output parser for `git worktree list`

**Error Handling:**
- `cmd/wt/errors.go`: Sentinel errors, `classify()`, `exitCodeFor()`

**TUI:**
- `internal/tui/model.go`: Bubbletea model, all key handlers, `defaultRunAction()`
- `internal/tui/view.go`: All rendering and lipgloss styles

**Tests:**
- `pkg/worktree/fakes_test.go`: Fake implementations of `GitRunner`, `HookRunner`, `ConfigProvider`
- `pkg/worktree/manager_integration_test.go`: Integration tests against real git repos
- `cmd/wt/cli_integration_test.go`: End-to-end CLI integration tests

## Naming Conventions

**Files:**
- Snake_case for multi-word files: `kill_em_all.go`, `kill_em_all_test.go`
- Single concept per file, named after the command or concern it implements
- Test files: `<file>_test.go` co-located with source
- Integration tests: `<package>_integration_test.go`

**Directories:**
- `cmd/<binary-name>/` for command packages (Go convention)
- `pkg/` for domain packages intended to be importable
- `internal/` for implementation packages restricted to this module

**Go identifiers:**
- Exported types: PascalCase (`Manager`, `AddOptions`, `WorktreeInfo`)
- Interfaces: Named by role, not "I" prefix (`GitRunner`, `HookRunner`, `ConfigProvider`)
- Unexported mode constants: `modeNormal`, `modeConfirmDelete` (TUI modes)
- Error sentinels: `Err` prefix (`ErrNotARepo`, `ErrHookFailed`)
- Constructor functions: `New(...)` pattern (`git.New()`, `hooks.New(repoRoot)`, `worktree.New(...)`)

## Where to Add New Code

**New CLI subcommand:**
- Create `cmd/wt/<command>.go` with a `var <cmd>Cmd = &cobra.Command{...}` and `func init() { rootCmd.AddCommand(<cmd>Cmd) }`
- Add test: `cmd/wt/<command>_test.go`
- If it needs Manager: call `managerForWorkdir()` like all other subcommands

**New Manager operation:**
- Add method to `Manager` in `pkg/worktree/manager.go`
- Add any new input/output types to `pkg/worktree/types.go`
- If it needs a new git capability: add method to `GitRunner` interface in `pkg/worktree/interfaces.go`, implement on `*git.Runner` in the appropriate `internal/git/*.go` file, and add the adapter delegation in `cmd/wt/root.go:gitAdapter`
- Add unit tests with fakes in `pkg/worktree/manager_test.go`

**New git command wrapper:**
- Add to the appropriate `internal/git/*.go` file (group by concern: `worktree.go`, `branch.go`, `resolve.go`)
- If needed by Manager: extend `GitRunner` interface in `pkg/worktree/interfaces.go`

**New config key:**
- Add field to `config.Config` struct in `internal/config/config.go`
- Add to `Resolve()` merge logic
- Add to `ConfigProvider` interface in `pkg/worktree/interfaces.go`
- Add to `cfgAdapter` in `cmd/wt/root.go`
- If user-settable: extend `config.Set()` allowlist

**New TUI interaction mode:**
- Add a new `mode` constant in `internal/tui/model.go`
- Add a `case modeXxx:` in `Update()` routing to a new `updateXxx()` method
- Add rendering in `View()` in `internal/tui/view.go`

**New worktree name component:**
- Add fields to `naming.NameContext` in `internal/naming/naming.go`
- Populate in `newContext()`
- Document in the `configTemplate` comment in `cmd/wt/init.go`

**Utilities/helpers:**
- Shared CLI helpers: `cmd/wt/root.go` (if needed by multiple subcommands)
- Shared domain helpers: private methods on `Manager` in `pkg/worktree/manager.go`

## Special Directories

**`.worktrees/` (runtime, per-repo):**
- Purpose: Per-repo configuration and lifecycle hooks; created by `wt init`
- Generated: By `wt init` scaffold; not in the `wt` source tree itself
- Committed: User's choice — hooks and config are typically committed to the managed repo
- Contains: `config.yaml`, `pre-create`, `post-create`, `pre-remove`, `post-remove` scripts

**`<repoRoot>.worktrees/` (runtime, default container):**
- Purpose: Default directory holding linked worktrees (sibling to repo root)
- Location: `<repoRoot> + ".worktrees"` unless `container:` is overridden in config
- Generated: On first `wt new`
- Committed: No — it's outside the repo directory

**`.planning/` (development):**
- Purpose: GSD planning documents — codebase maps, specs, phase plans
- Generated: By GSD tooling
- Committed: Yes — part of the development workflow

---

*Structure analysis: 2026-06-05*
