# `wt` — Git Worktree Utility — Design

**Date:** 2026-05-31
**Status:** Approved design, pre-implementation
**Author:** code.drill.eu@gmail.com (with Claude)

## 1. Purpose

`wt` is a Go utility for quickly creating, listing, and removing **git worktrees** from
an existing repository, with lifecycle hooks for setup/teardown. It exposes a reusable
**library** (`pkg/worktree`) consumed by both a **CLI** and an interactive **TUI**.

The core need: spin up an isolated worktree with one short command (`wt new`), have
project-defined hooks copy gitignored files (e.g. `.env`) or mint tokens into it, work
freely, then tear it down cleanly when done.

## 2. Scope

### In scope (v1)
- Create / list / remove worktrees against a git repo.
- Sibling **container** layout: `<repo>.worktrees/<name>/`.
- Generated default names; user-overridable name/branch/base.
- Convention-dir hooks: `pre-create`, `post-create`, `pre-remove`, `post-remove`.
- Config file `.worktrees/config.yaml` (CLI flags override).
- CLI (cobra) **and** TUI (Bubble Tea) over one shared library.
- `wt init`, `wt prune`, `wt path`, shell completion, `--json` list output.

### Out of scope (v1) — see §12 Future directions
- Worktree status/lifecycle state ("ready for merge", `wt done`).
- Merge / PR integration.
- Windows-native hook execution (hooks are POSIX/shebang for v1).

## 3. Key decisions

| Decision | Choice |
|---|---|
| Git interaction | Shell out to the `git` CLI (go-git lacks proper worktree support). |
| CLI framework | cobra |
| TUI framework | Bubble Tea + Lipgloss |
| Binary name | `wt` |
| Module path | `github.com/code-drill/wt` |
| Min Go | 1.23 |
| Min git | 2.30+ (needs `worktree list --porcelain -z`, `--path-format`); probed at startup |
| Layout | Container sibling of **main** repo root: `<repo>.worktrees/<name>/` |
| Repo anchor | `git rev-parse --git-common-dir` (works from inside a worktree) |
| Branching | New branch from HEAD by default, or `--base <ref>`; `--branch` to name it |
| Branch prefix | **All** branches prefixed `wt/` for easy tracking; the worktree directory name omits the prefix |
| Default name | Date-first: `YYYY-MM-DD_HH-mm-<adjective>-<noun>-NNNN` (random NNNN) so branches sort/stale-track by date; validated as a legal git ref |
| `rm` behavior | Removes worktree **and** branch; **safe delete** (`git branch -d`) refuses unmerged and **reports** the kept branch; `--force-branch`/`-D` to force |
| Hook config | Convention dir `.worktrees/`; presence + exec bit = enabled |
| Hook → script | Path data passed via **environment variables** |
| Hook failure | Strict, **no rollback** (post-create failure leaves the worktree in place) |
| Config location | `.worktrees/config.yaml` in source repo; CLI flags override |

## 4. Architecture

```
pkg/worktree     ← reusable library: Manager + GitRunner/HookRunner/ConfigProvider interfaces
internal/git     ← default GitRunner: thin, disciplined wrapper over the git CLI
internal/config  ← default ConfigProvider: loads .worktrees/config.yaml + defaults
internal/hooks   ← default HookRunner: discovers & runs convention scripts with env vars
internal/naming  ← random adjective-noun-timestamp-digits generator (embedded wordlists)
internal/tui     ← Bubble Tea UI over Manager
cmd/wt           ← cobra root + subcommands; wires concrete impls into Manager
main.go          ← thin entrypoint
```

### Dependency direction & reusability
`pkg/worktree.Manager` depends only on **interfaces** it declares
(`GitRunner`, `HookRunner`, `ConfigProvider`). The `internal/*` packages provide the
default concrete implementations, wired together in `cmd/wt`. This makes the library
genuinely reusable (consumers can substitute implementations) and unit-testable with
fakes — not solely via temp-repo integration tests.

## 5. The git wrapper (`internal/git`) — disciplines

This layer carries essentially all the correctness risk; it is built and tested **first**.

1. **List via `git worktree list --porcelain -z`.** Parse NUL-terminated records into:
   `Path, HEAD, Branch (may be absent), Bare, Detached, Locked(+reason), Prunable(+reason)`.
   `-z` is the only safe option for paths containing spaces/newlines.
2. **Always run git with `LC_ALL=C`** so messages/errors are locale-stable and parseable.
3. **Prefer plumbing**: `rev-parse --git-common-dir`, `rev-parse --show-toplevel`,
   `rev-parse --verify <ref>^{commit}`, `check-ref-format`. Reserve porcelain
   (`worktree add/remove/list/prune`, `branch -d/-D`) for ops with no plumbing equivalent.
4. **Capture stdout/stderr separately**; decide success/failure by **exit code**, never by
   parsing human stdout.
5. **`exec.Command` arg slices only** — never a shell string. Eliminates quoting/injection.
6. **Probe `git --version` once at startup**; fail fast with a clear message if below minimum.

### Repo resolution
- The **main repo root** is derived from `git rev-parse --git-common-dir` (then its parent),
  so invoking `wt` from inside a linked worktree still anchors to the real repo.
- `<repo>` for naming the container = basename of the main repo root, sanitized.
- The tool only operates inside a git repo; otherwise it errors cleanly.

## 6. Commands (CLI)

| Command | Behavior |
|---|---|
| `wt new [name]` | Create a worktree. Flags: `-r/--repo` (default cwd), `-b/--branch` (default = name), `--base <ref>` (default HEAD), `--no-hooks`. Omitted name → generated. |
| `wt list` / `wt ls` | List worktrees in the container. `--json` for machine output. |
| `wt rm <name>` | Remove worktree **and** branch (safe). Flags: `--force` (dirty worktree), `--force-branch`/`-D` (unmerged branch), `--keep-branch`. If safe-delete refuses an unmerged branch, the worktree is still removed and `wt` **reports** the kept branch name + how to force-delete it. |
| `wt prune` | Wrap `git worktree prune` to clear stale admin state. |
| `wt path <name>` | Print the absolute path of a worktree (for a shell `cd` alias; a child process cannot change the parent shell's dir). |
| `wt init` | Scaffold `.worktrees/` with a `config.yaml` and hook stubs. Idempotent. |
| `wt completion [shell]` | cobra-generated bash/zsh/fish completion. |
| `wt` (no args) | Launch the TUI **only if** stdout is a TTY; otherwise print help. |

### Naming
- **Generated default** (no name given): `YYYY-MM-DD_HH-mm-<adjective>-<noun>-NNNN`
  (date first for chronological sorting / stale detection; `NNNN` random).
- **Branch** = `wt/` + the name (the `wt/` prefix is applied to *all* branches, generated or
  user-supplied), e.g. `wt/2026-05-31_14-30-snowy-beach-4821`.
- **Worktree directory** = the name **without** the `wt/` prefix, sanitized so it contains no
  `/` (a user-supplied `--branch feature/x` still yields a clean directory name).
- The final branch is validated with `check-ref-format` before use.

### Name → worktree resolution
`rm`/`path`/TUI resolve `<name>` against actual `git worktree list` output: match by
container-child directory basename first, then branch name (with or without the `wt/`
prefix); **error on ambiguity** or not-found.

## 7. Create transaction (`Manager.Add`)

Ordered steps with explicit cleanup contract:

1. **Resolve & validate**: main repo root, container path, `--base` exists
   (`rev-parse --verify`), name/branch validity, branch availability, target path free.
2. **mkdir container** (idempotent; leaving an empty container on later failure is harmless).
3. **pre-create hook** — cwd = source repo root. Failure → **abort, nothing created.**
4. **`git worktree add`** — git's `add` is effectively atomic; on failure, nothing to clean.
5. **post-create hook** — cwd = new worktree root. Failure → **error out, worktree left in
   place** (explicit no-rollback choice) so the user can inspect/fix.

### Branch handling edge cases
- Branch already exists → **error** telling the user to pick `--branch` (no silent `-B` reset).
- Requested branch already checked out in another worktree → clean error (not raw git stderr).
- `rm` targeting the **main** worktree → refused.
- Dirty worktree on `rm` → refused unless `--force`; clear message.

## 8. Hooks (`internal/hooks`)

Convention dir `.worktrees/` in the **source repo root** (committed with the project):

| Hook | cwd | When | Failure |
|---|---|---|---|
| `pre-create` | source repo root | before `git worktree add` | abort, nothing created |
| `post-create` | new worktree root | after creation | error, worktree left in place |
| `pre-remove` | worktree being removed | before removal | abort removal |
| `post-remove` | source repo root | after removal | error (already removed) |

- Run only if the file **exists and is executable**; interpreter via shebang (POSIX, v1).
- stdout/stderr are **streamed** (long setup hooks like `npm install` show progress).
- Easily bypassed with `--no-hooks`.
- **Trust note:** hooks are arbitrary executables from the repo; this is documented as a
  supply-chain consideration. `--no-hooks` is the escape hatch; auto-run is intentional but
  loud in docs.

### Environment variables (exported to all hooks, docroot == repo root)

| Var | Meaning |
|---|---|
| `WT_SOURCE_ROOT` | main repo root |
| `WT_TARGET_ROOT` | worktree root |
| `WT_NAME` | worktree name (no `wt/` prefix) |
| `WT_BRANCH` | branch name (incl. `wt/` prefix) |
| `WT_BASE_REF` | base ref the branch was cut from |
| `WT_CONTAINER` | container dir |
| `WT_REPO_NAME` | repo basename |
| `WT_HOOK` | which hook is running (`pre-create`, …) |

Example: `cp "$WT_SOURCE_ROOT/.env" "$WT_TARGET_ROOT/.env"`

## 9. Config (`.worktrees/config.yaml`)

Precedence: **CLI flags > config file > built-in defaults.**

```yaml
base_ref: HEAD          # default ref new branches are cut from
container: ""           # optional override of the container path; used VERBATIM
                        # (absolute, or relative to repo root). No reponame prefix applied.
name_template: ""       # optional override of the default name pattern
```

Resist adding more knobs (YAGNI).

## 10. `wt init` scaffolding

Creates `.worktrees/` containing:
- `config.yaml` — commented template of the schema above.
- `pre-create`, `post-create`, `pre-remove`, `post-remove` — executable stubs, each:
  ```bash
  #!/usr/bin/env bash
  # wt hook. Available: $WT_SOURCE_ROOT $WT_TARGET_ROOT $WT_NAME $WT_BRANCH ...
  exit 0
  ```
  (Empty files with no shebang would error when executed — stubs are valid no-ops.)
- **Idempotent**: never clobbers an existing file.
- `.worktrees/` is intended to be **committed** to the repo (project policy/hooks).

## 11. Testing strategy

TDD throughout. Riskiest-first. **Every package ships both unit tests and integration
tests** — unit tests with fakes for fast logic coverage, integration tests against
throwaway real git repos in temp dirs for end-to-end correctness.

- `internal/git`: integration tests against throwaway real git repos in temp dirs —
  porcelain `-z` parsing (incl. spaces in paths, bare/detached/locked/prunable), common-dir
  anchoring from inside a worktree, add/remove/prune, ref verification, version probe.
- `pkg/worktree.Manager`: unit tests with fake GitRunner/HookRunner/ConfigProvider for the
  create transaction, name resolution, and failure paths; plus temp-repo integration tests.
- `internal/naming`: pure unit tests; generated names are valid git refs.
- `internal/hooks`: unit tests for discovery, env, cwd, streaming, and a **failing-hook** case
  per hook type.
- `internal/config`: fixture-based load + precedence tests.
- Consistent **exit-code taxonomy** (not-a-repo, name-collision, hook-failed, git-failed,
  dirty-worktree) asserted in CLI tests.

## 12. Build order

1. `internal/git` wrapper (highest risk — bulletproof first).
2. `internal/config` loader + precedence + container-override semantics.
3. `internal/naming` (pure, embedded wordlists, ref validity).
4. `pkg/worktree.Manager` with injected interfaces; create transaction + resolution.
5. `internal/hooks` (discovery, env, cwd, streaming, failure policy); wire into Manager.
6. `cmd/wt` cobra: `new`, `list`, `rm`, `prune`, `path`, `init`, `completion`; bare→TUI TTY
   guard; exit codes.
7. `internal/tui` (last; over the now-stable Manager).
8. `main.go` thin entrypoint.

### Per-phase review gates (mandatory)
Each phase above is **not complete** until all of the following pass — no phase advances
with a gate outstanding:
1. **Tests written** — unit **and** integration tests for the phase's code, all green.
2. **Code review** — correctness, error handling, idioms (subagent-driven).
3. **Test review** — coverage and quality of the tests themselves (are the right failure
   modes and edge cases exercised, not just happy paths).
4. **Architectural review** — boundaries, dependency direction, and interface fit vs. this
   spec (subagent-driven).

## 13. Future directions (NOT v1)

The "work freely → mark ready for merge → cleanup" lifecycle vision:
- **Cleanup half is already covered** by `pre-remove`/`post-remove` hooks.
- **Status half** (e.g. `wt ready`, `wt done`, a status column in the TUI) needs **per-worktree
  metadata**. Designed-for, not built: metadata can live as a sidecar in the container
  (e.g. `<container>/.wt/<name>.yaml`) without disturbing v1.
- **Merge/PR integration** is a separate, larger milestone.

These are recorded so v1 stays focused while leaving the door open.
