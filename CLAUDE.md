<!-- GSD:project-start source:PROJECT.md -->
## Project

**wt — Git Worktree Manager**

`wt` is a Go CLI + TUI for managing git worktrees. It creates worktrees in a sibling `<repo>.worktrees/` container (one flat, sanitized directory per branch), lists/removes/prunes them, runs lifecycle hooks, and renders branch names from gg-style `<token>` templates. Pressing Enter in the TUI cd's the user's shell into the selected worktree via shell wrappers / an emitted shell function. Branch names are never generated and never prefixed: the user supplies a name, a named template renders one, or — run from inside a worktree — the branch derives from that worktree's branch (`<branch>-vNNN` or `<branch>-<suffix>`).

**Core Value:** frictionless worktree iteration — `wt new` from inside a worktree spawns a sibling iteration off that worktree's branch; Enter in the TUI transports the shell into any worktree; a single self-installing binary (`worktrees`) bootstraps the whole `wt` entry-point layout.

### Constraints

- **Tech stack**: Go + cobra CLI; extend `Manager.Add` / `AddOptions` and the `GitRunner` interface rather than bypass them.
- **Template/config parity with gg**: the `<token>` template syntax (`internal/naming`) and the TOML config layering (user config overlaid by committed `.wt.toml`) mirror gigagit (`/mnt/t/others/gigagit`); do not diverge without cause.
- **Testing**: new logic must be unit-testable via the injected `GitRunner`/`ConfigProvider`/`HookRunner` fakes; integration tests (`-tags integration`) cover real git.
- **Naming safety**: branch names pass `CheckRefFormat` and collision checks before creation; worktree dirs are `naming.SanitizeSegment(branch)` — flat, never nested.
- **Windows is first-class**: all path comparisons go through `pkg/worktree/paths.go`; no binary named `wt.exe` may ever ship (it would shadow the `wt.cmd` wrapper in cmd's lookup).
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.25.0 — all application code (`main.go`, `cmd/`, `internal/`, `pkg/`)
- TOML — configuration (`.wt.toml`, user config, seq-state file)
- Shell/batch — entry-point wrappers (`shell/wt`, `shell/wt.sh`, `shell/wt.cmd`; also embedded as Go consts in `cmd/wt/selfinstall.go`)

## Frameworks & Key Dependencies
- `github.com/spf13/cobra` — CLI framework (`cmd/wt/*.go`)
- `github.com/charmbracelet/bubbletea` + `lipgloss` — TUI (`internal/tui/`)
- `github.com/pelletier/go-toml/v2` — config + seq-state parsing (`internal/config/`)
- `golang.org/x/term` — TTY detection (TUI launch, interactive prompts)
- `os/exec` — shell-out to `git` (`internal/git/`) and hooks (`internal/hooks/`)
- Standard `testing` package only; integration tests use real git repos behind `//go:build integration`

## Configuration
- Layering (highest wins): `<repo>/.wt.toml` > `<UserConfigDir>/wt/config.toml` > defaults (`base_ref = "HEAD"`). Field-by-field overlay; a `[templates]` table in a higher layer replaces the lower one wholesale.
- Keys: `base_ref`, `container`, `[templates]` (named gg-style templates only).
- `<seq:NAME>` counter state: `<git-common-dir>/wt/state.toml` (machine-local, shared by all linked worktrees; stored value = last consumed, peek = stored+1; bumped only after a successful create).
- Hooks live in `<repo>/.wt/<event>` (pre-create, post-create, pre-remove, post-remove).
- No env-var config layer (WT_BRANCH_PREFIX is gone with the prefix feature).
- `LC_ALL=C` and `GIT_TERMINAL_PROMPT=0` are set by the git runner (not user-facing).

## Build & Install
- `./build.sh` / `build.cmd` → `bin/`: real binaries `wt.bin` / `wt.bin.exe` behind entry points `wt` (POSIX script) / `wt.cmd` (batch). The binary is deliberately NOT named `wt.exe`.
- `./build-worktrees.sh` / `build-worktrees.cmd` → `bin/worktrees[.exe]`: the full-name, self-installing binary.
- `go install .` installs `worktrees`; on any run under that full name it self-installs the wt entry points next to itself (`cmd/wt/selfinstall.go`).
- Shell integration: `wt shell-init <bash|zsh> [--install]` emits/installs a `wt()` function bound to the binary by absolute path (POSIX builds only; not registered on Windows).

## Platform Requirements
- Go 1.25+, `git` 2.30+ on PATH (enforced at runtime)
- Windows fully supported: gg's `SanitizeSegmentFor` handles reserved names; path comparisons are separator/case-insensitive there
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
- Snake-case multi-word files (`kill_em_all.go`, `shell_init.go`, `cd_file.go`); tests co-located `_test.go`; integration tests `_integration_test.go` behind `//go:build integration`
- The CLI package is `package cmd` in `cmd/wt/`; `main.go` at the module root delegates to `cmd.Execute()` (this keeps the go-install binary named `worktrees` — load-bearing for self-install detection)
- Interfaces named for the implementor's role: `GitRunner`, `HookRunner`, `ConfigProvider` (defined in `pkg/worktree/interfaces.go`, the consumer)
- Options/Result structs: `AddOptions`, `RemoveOptions`, `AddResult`, `RemoveAllPlan`, …
- Sentinel errors `Err…` in `cmd/wt/errors.go`, mapped to stable exit codes 2–6 by `exitCodeFor`; `classify()` string-matches domain errors into sentinels at the CLI boundary
- Cobra flag vars: lowercase with command prefix (`newTemplate`, `rmForce`, `killYes`, `editUserCfg`)

## Code Style & Comments
- Standard gofmt; every exported identifier has a doc comment starting with its name; comments explain *why* (transaction semantics, Windows constraints), not *what*
- Errors wrapped with context: `fmt.Errorf("git worktree add: %w", err)`; user-facing errors quote the affected value; parenthetical transaction notes ("(nothing created)")

## Function & Module Design
- Logic extracted from cobra `RunE` closures into testable helpers: `parseVars`, `promptMissing`, `lookupTemplate`, `runKillEmAll`, `runSet`, `editorCommand`, `appendShellInit`, `selfInstallAt`, `rootLongFor`
- Platform-dependent behavior is written as pure functions parameterized by GOOS/flags so both branches unit-test on any host: `SanitizeSegmentFor(s, goos)`, `normPathOS(p, windows)`, `rootLongFor(goos)`, `selfInstallAt(exe, goos)`
- Injectables for tests: TUI `model.runAction` and `model.escapeCwd`; `naming.Ctx{Now, Rand, Seqs}` makes template rendering deterministic
- `pkg/worktree` imports only `internal/naming`; git/hooks/config reach it through interfaces wired by adapters in `cmd/wt/root.go`

## Path Handling (critical)
- NEVER compare paths with `==` / `strings.HasPrefix`: use `pathsEqual` / `hasPathPrefix` / `relUnder` from `pkg/worktree/paths.go` (separator-unified, case-folded on Windows — git-for-Windows emits forward-slash paths with arbitrary drive-letter case)
- Worktree dir = `naming.SanitizeSegment(branch)`: one flat segment, `/` → `-`
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Component Responsibilities
| Component | Responsibility | File |
|-----------|----------------|------|
| `main.go` | Process entry; delegates to `cmd.Execute()` | `main.go` |
| `cmd.Execute()` | Self-install check, run cobra root, map errors to exit codes | `cmd/wt/root.go` |
| `maybeSelfInstall()` | When running as `worktrees[.exe]`, materialize wt entry points next to self | `cmd/wt/selfinstall.go` |
| `rootCmd.RunE` | TTY detection → TUI; afterwards emit Enter-selection (print + `--cd-file`) | `cmd/wt/root.go` |
| `worktree.Manager` | Domain orchestrator: add/list/remove/kill, derive mode, cwd escape | `pkg/worktree/manager.go` |
| `pkg/worktree/paths.go` | Normalized path comparisons (Windows-safe) | |
| `internal/naming` | gg template engine (`Resolve`, `UserLabels`, `SeqNames`) + `SanitizeSegment` | ported from gigagit `internal/template` |
| `internal/config` | TOML config layering + `Set` upsert + seq state (`PeekSeq`/`BumpSeq`) | `config.go`, `state.go` |
| `internal/git` | git subprocess wrapper; `CommonDir`/`MainRoot` via `--git-common-dir` | |
| `internal/hooks` | Runs `.wt/<event>` scripts with `WT_*` env | |
| `internal/tui` | Bubbletea browser; Enter-select, name input, template picker with per-label var prompts; destructive actions call `escapeCwd` then re-invoke `wt <subcmd>` via `tea.Exec` | |
| CLI subcommands | `new`, `rm`, `list`, `path`, `kill-em-all`, `set`, `edit`, `init`, `prune`, `templates`, `shell-init` (POSIX only), `completion` | `cmd/wt/*.go` |

## Key Flows
- **wt new**: cmd layer loads config, renders template (interactive `<user:>` prompts on TTY, `<seq:>` peek via git common dir) → `Manager.Add` with a final `Branch`, or a raw `Name`; derive mode triggers inside a worktree when no rendered Branch is given; seq counters bump only after success.
- **cd-on-Enter**: TUI records selection → `emitSelection` prints it and writes `--cd-file`; shell wrapper (`wt()` function from `shell-init`, `wt` script, or `wt.cmd`) cd's after exit. A child process can never cd its parent — the wrapper layer is mandatory.
- **Destructive ops**: `Manager.Remove`/`RemoveAll` call `EscapeCwd` first (a dir that is any process's cwd cannot be deleted on Windows); `rm`/`kill-em-all`/TUI write the repo root to `--cd-file` when the starting directory was removed (`escapeDeadCwd`).
- **kill-em-all** sweeps container worktrees and *their* branches only — no prefix exists to find orphaned branches.

## Architectural Constraints
- `pkg/worktree` → `internal/naming` only; `internal/git`/`internal/hooks` import `pkg/worktree` types (one-way)
- git 2.30+ gate at `buildManager()`; TUI mutations run as subprocesses, never in-process
- Cross-platform testability trumps `runtime.GOOS` branching: parameterize by GOOS and unit-test both branches
<!-- GSD:architecture-end -->

## Common Pitfalls

- CRLF repo on a Windows drive: `gofmt -l .` flags nearly every pre-existing file — that is line-ending noise, not misformatting; never "fix" it repo-wide. `Edit` preserves CRLF; new files may be LF (git normalizes to LF in the repo).
- sed/grep `$`-anchored patterns silently miss on CRLF files (the line ends `\r`); use `\r\?$` or match without the anchor.
- Windows path comparisons broke `wt list` entirely once: git-for-Windows emits `T:/x/y` while `filepath` builds `T:\x\y`. Always use `pkg/worktree/paths.go` helpers; the same applies to `filepath.Base` on backslash paths under Linux (split on both separators first).
- cmd.exe resolves `wt.exe` before `wt.cmd` within a directory — never produce a binary named `wt.exe`; the real binary is `wt.bin.exe`/`worktrees.exe`.
- On Windows a directory that is ANY process's cwd cannot be deleted — call `Manager.EscapeCwd` before removals (already wired in Remove/RemoveAll/TUI); the parent console's own cwd lock is outside wt's control.
- WSL in Claude sessions cannot execute Windows binaries (`exec format error`): cross-compile with `GOOS=windows` and have the user test; drive the Linux TUI end-to-end with `{ sleep 1; printf 'j'; sleep 0.5; printf '\r'; } | script -qec "bin/wt …" /dev/null` (plain piping hangs bubbletea).
- `go install` names the binary after the module path's last element (`worktrees`); self-install detection (`isFullNameInvocation`) depends on that name — don't move the main package.
- Stale binaries on PATH (`~/go/bin/wt`, old `wt.exe` copies) shadow the wrapper/function and resurface long-fixed bugs; check `which wt` / `where wt` first when "it doesn't work".

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
