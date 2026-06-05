# External Integrations

**Analysis Date:** 2026-06-05

## APIs & External Services

**None.** This tool has no HTTP clients, no REST/gRPC calls, and no third-party API SDKs. All external interaction is local filesystem and process invocation.

## Data Storage

**Databases:**
- None. No database client or ORM.

**File Storage:**
- Local filesystem only.
  - Config file: `<repo-root>/.worktrees/config.yaml` (read/write via `internal/config/config.go`)
  - Worktree directories: created/removed under `<repo-root>.worktrees/` by default, or a configured `container` path
  - Hook scripts: `<repo-root>/.worktrees/<hook-name>` (executable files, discovered and run by `internal/hooks/hooks.go`)
  - TUI action logs: temp files in the OS temp dir, created per-action run in `internal/tui/model.go`

**Caching:**
- None.

## Authentication & Identity

**Auth Provider:**
- None. The tool inherits the user's git credentials via the ambient environment (git credential helpers, SSH agent, etc.). It explicitly sets `GIT_TERMINAL_PROMPT=0` in `internal/git/git.go` to prevent git from prompting interactively during any operation.

## Shell-out: Git Binary

This is the primary and only external system integration. The tool shells out to the `git` binary on PATH for all repository operations.

**Implementation:** `internal/git/git.go` — `Runner.Run(dir string, args ...string)`

**Environment set on every git call:**
- `LC_ALL=C` — locale-stable output for reliable parsing
- `GIT_TERMINAL_PROMPT=0` — disables credential prompts

**Minimum git version:** 2.30 (enforced by `Runner.EnsureMinVersion(2, 30)` in `cmd/wt/root.go`)

**Git subcommands invoked:**

| Operation | Git command | Source file |
|-----------|-------------|-------------|
| Resolve repo root | `git rev-parse --show-superproject-working-tree --show-toplevel` | `internal/git/resolve.go` |
| Check git version | `git --version` | `internal/git/git.go` |
| Verify a ref exists | `git rev-parse --verify <ref>` | `internal/git/resolve.go` |
| Check ref name format | `git check-ref-format --branch <name>` | `internal/git/resolve.go` |
| List worktrees | `git worktree list --porcelain -z` | `internal/git/worktree.go` |
| Add worktree (new branch) | `git worktree add -b <branch> <path> <base>` | `internal/git/worktree.go` |
| Add worktree (existing branch) | `git worktree add <path> <branch>` | `internal/git/worktree.go` |
| Remove worktree | `git worktree remove [--force] <path>` | `internal/git/worktree.go` |
| Prune stale worktree entries | `git worktree prune` | `internal/git/worktree.go` |
| List branches by prefix | `git for-each-ref --format=%(refname:short) refs/heads/<prefix>*` | `internal/git/branch.go` |
| Delete branch (safe) | `git branch -d <branch>` | `internal/git/branch.go` |
| Delete branch (force) | `git branch -D <branch>` | `internal/git/branch.go` |
| Check branch exists | `git rev-parse --verify refs/heads/<branch>` | `internal/git/branch.go` via `VerifyRef` |

## Lifecycle Hooks (User-Defined Shell Scripts)

The tool discovers and executes user-provided executable scripts from `<repo-root>/.worktrees/`. This is a local extension point, not an external service.

**Implementation:** `internal/hooks/hooks.go` — `Runner.Run(ctx HookContext)`

**Hook events:**

| Event name | When fired | Working directory |
|------------|------------|-------------------|
| `pre-create` | Before `git worktree add` | repo root |
| `post-create` | After `git worktree add` succeeds | new worktree path |
| `pre-remove` | Before `git worktree remove` | worktree being removed |
| `post-remove` | After worktree and branch are removed | repo root |

**Environment variables passed to every hook:**

| Variable | Value |
|----------|-------|
| `WT_SOURCE_ROOT` | Main repo root path |
| `WT_TARGET_ROOT` | Worktree path |
| `WT_NAME` | Short worktree name |
| `WT_BRANCH` | Full branch name |
| `WT_BASE_REF` | Base ref used for creation |
| `WT_CONTAINER` | Container directory path |
| `WT_REPO_NAME` | Basename of repo root |
| `WT_HOOK` | Event name (e.g. `pre-create`) |

Hook stdout/stderr stream directly to the process stdout/stderr. In TUI mode, hook output is tee'd to a temp log file via `internal/tui/model.go`.

## TUI: Self-Invocation via `tea.ExecProcess`

In TUI mode (`internal/tui/model.go`), interactive create/delete actions work by suspending the TUI and re-invoking the `wt` binary itself as a subprocess (via `tea.ExecProcess`/`exec.Command`). This is not an external integration but a self-call pattern.

- The binary path is resolved via `os.Executable()` or the `wt` command on PATH.
- Combined output is tee'd to a temp log file and shown on the normal screen while hooks run.

## Monitoring & Observability

**Error Tracking:**
- None.

**Logs:**
- No structured logging framework. Errors are printed to stderr via `fmt.Fprintln(os.Stderr, ...)`.
- TUI action logs: per-run temp files (path shown to user in TUI status bar), not persisted beyond session.

## CI/CD & Deployment

**Hosting:**
- Not applicable. This is a CLI binary distributed from source.

**CI Pipeline:**
- No CI config files detected in repo (no `.github/`, `.gitlab-ci.yml`, etc.).

## Webhooks & Callbacks

**Incoming:**
- None.

**Outgoing:**
- None.

## Environment Configuration Summary

**Variables consumed by the tool:**

| Variable | Purpose | Precedence |
|----------|---------|------------|
| `WT_BRANCH_PREFIX` | Override `branch_prefix` config | Highest (beats config file) |

**Variables set by the tool (on subprocesses only):**

| Variable | Value | Set on |
|----------|-------|--------|
| `LC_ALL` | `C` | Every `git` invocation |
| `GIT_TERMINAL_PROMPT` | `0` | Every `git` invocation |
| `WT_SOURCE_ROOT`, `WT_TARGET_ROOT`, `WT_NAME`, `WT_BRANCH`, `WT_BASE_REF`, `WT_CONTAINER`, `WT_REPO_NAME`, `WT_HOOK` | Contextual | Hook invocations |

---

*Integration audit: 2026-06-05*
