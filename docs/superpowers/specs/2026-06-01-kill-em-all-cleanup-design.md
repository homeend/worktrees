# kill-em-all — bulk cleanup of worktrees & branches — design

**Date:** 2026-06-01
**Status:** Approved (pending spec review)
**Depends on:** [configurable branch prefix](./2026-06-01-configurable-branch-prefix-design.md)
(uses `cfg.BranchPrefix()` to bound its scope)

## Problem

There is no fast way to tear down everything `wt` created. Removing many
worktrees one at a time (`wt rm` per item) is tedious, and it never touches
orphan branches (prefixed branches whose worktree is already gone). Users want a
single destructive "clean slate" action, available from both the TUI and the
command line, that force-removes all prefixed worktrees and branches regardless
of committed/uncommitted state.

## Goals

1. One command to remove **all** worktrees in the repo's container **and** all
   branches matching the configured prefix — including orphan branches.
2. Available from the **CLI** (`wt kill-em-all [--yes]`) and the **TUI**.
3. Force removal: committed or not, merged or not.

## Non-goals (YAGNI)

- No root-level `--kill-em-all` flag (rejected for consistency — the CLI is
  subcommand-only).
- No selective/filtered variant (e.g. "older than N days"). This is all-or-nothing.
- No `nuke`/alias names.

## Decisions (confirmed with user)

| Decision | Choice |
| --- | --- |
| CLI shape | Subcommand `wt kill-em-all --yes` (consistent with `new`/`rm`/`prune`). |
| Branch scope | **All** prefix-matching branches, including orphans with no worktree. |
| No `--yes` on CLI | Interactive `y/N` when stdout is a TTY; **refuse with an error** when not a TTY. |
| Hooks | **Skipped**, and the command **prints a notice** that hooks are skipped. |
| Partial failures | **Best-effort** — keep going, collect failures, report a summary, exit non-zero. |

## Scope definition (what gets removed)

- **Worktrees:** every entry from `Manager.List` that is **not** `IsMain` (i.e.
  everything living in the repo's container). The main worktree is never touched.
- **Branches:** every branch matching `refs/heads/<prefix>*` where `<prefix>` is
  the resolved `cfg.BranchPrefix()` (default `wt/`). Includes orphans (no
  worktree). Non-prefixed branches (e.g. `main`) are never deleted.
- **Force:** worktrees removed with `--force`; branches deleted with `-D`.

## Safety invariants (enforced + explicitly tested)

1. `IsMain` worktrees are never removed.
2. Branches not matching the configured prefix are never deleted.
3. The prefix is always sourced from `cfg.BranchPrefix()` — a custom prefix is
   honored; nothing outside it is in scope.
4. A prefixed branch that is checked out in the main worktree cannot be `-D`'d;
   this is recorded as a failure, not treated as fatal.

## Architecture & components

The design follows the established layering (cmd → pkg/worktree → interfaces →
internal/git) and the review findings.

### 1. `internal/git` — `branch.go`

Add branch enumeration:

```go
// ListBranches returns short names of branches matching <prefix>* (e.g. "wt/").
// Implemented via: git for-each-ref --format='%(refname:short)' refs/heads/<prefix>*
func (r *Runner) ListBranches(dir, prefix string) ([]string, error)
```

### 2. `pkg/worktree/interfaces.go`

Add to `GitRunner`:

```go
ListBranches(dir, prefix string) ([]string, error)
```

Wire it through `gitAdapter` (`cmd/wt/root.go`) and the `fakeGit` test fake.

### 3. `pkg/worktree/types.go`

```go
type RemoveAllPlan struct {
    Worktrees []WorktreeInfo // non-main, in-container
    Branches  []string       // prefix-matching, short names
}

type CleanupFailure struct {
    Kind string // "worktree" | "branch"
    Ref  string // path or branch name
    Err  string
}

type RemoveAllResult struct {
    WorktreesRemoved int
    BranchesDeleted  int
    Failures         []CleanupFailure
}
```

### 4. `pkg/worktree/manager.go`

**Read-only planner** (powers confirmation in both CLI and TUI):

```go
func (m *Manager) PlanRemoveAll(dir string) (RemoveAllPlan, error)
```

- Lists worktrees, keeps non-`IsMain`.
- Lists prefix branches via `m.git.ListBranches(repoRoot, m.cfg.BranchPrefix())`.

**Executor** (mutating, best-effort):

```go
func (m *Manager) RemoveAll(dir string) (RemoveAllResult, error)
```

- Computes the plan once.
- For each worktree: `m.git.RemoveWorktree(repoRoot, path, true)` — on error
  append a `CleanupFailure{Kind:"worktree"}` and **continue**.
- For each prefix branch: `m.git.DeleteBranch(repoRoot, branch, true)` — on
  error append `CleanupFailure{Kind:"branch"}` and continue.
- **Does not reuse `Remove`** (which re-lists per call → O(n²) and mutates the
  list mid-iteration). Issues raw force git calls after a single enumeration.
- **Skips hooks entirely** (no `HookRunner` calls in this path).
- Tail step: `m.git.Prune(repoRoot)` to clear stale admin entries.
- Returns `(result, nil)` for partial failures; a non-nil error is returned only
  for fatal setup failures (e.g. can't resolve repo root).

> Branch-removal order: worktrees first, then branches — removing a worktree
> does not delete its branch, so all in-scope branches remain deletable after.

### 5. `cmd/wt/kill_em_all.go` (new subcommand)

```go
Use:   "kill-em-all"
Short: "Remove ALL worktrees and prefixed branches (destructive)"
Args:  cobra.NoArgs
```

- Flag: `--yes` (`BoolVar`, kebab-case, matching `--force`/`--json`).
- `RunE`:
  1. Build manager + plan (`PlanRemoveAll`). If plan is empty → print
     "nothing to remove" and return nil.
  2. Print the hooks-skipped notice:
     `note: lifecycle hooks are skipped for kill-em-all`.
  3. Confirmation gate:
     - `--yes` → proceed.
     - else if `term.IsTerminal(stdout)` → print the plan counts + list and
       prompt `Remove everything? [y/N]`; proceed only on `y`/`yes`.
     - else (not a TTY) → return an error: `refusing to run without --yes (no TTY for confirmation)`.
  4. Execute `RemoveAll`; print summary
     `Removed N worktrees, deleted M branches (K failed)`; list failures.
  5. If `result.Failures` is non-empty, return `ErrPartialCleanup` so the
     process exits non-zero.

### 6. `cmd/wt/errors.go`

Add a sentinel + exit code (avoids teaching `classify` new substrings):

```go
ErrPartialCleanup = errors.New("cleanup completed with failures")
// exitCodeFor: case errors.Is(err, ErrPartialCleanup): return 6
```

### 7. `internal/tui` — `model.go` / `view.go`

- New `mode`: `modeConfirmKillAll`.
- Key **`K`** (capital — distinct from `k` = up) in `updateNormal` enters
  `modeConfirmKillAll`.
- Confirm view shows the count from the worktrees the TUI already holds
  (`m.items`, non-main) plus the hooks-skipped notice — e.g.
  `Remove ALL N worktrees and their branches? Hooks skipped. (y/n)`.
  *(The TUI `lister` interface is intentionally NOT widened; the exact orphan
  branch count and full summary come from the spawned subprocess output.)*
- On `y`: `m.runAction("kill-em-all", "--yes", "--repo", m.dir)` — same
  subprocess pattern as `new`/`rm`; `--yes` skips the subprocess's own prompt
  since the TUI already confirmed.
- `view.go` normal hint line gains `K kill-all`.

## Data flow

```
wt kill-em-all                         TUI: press K
  └─ PlanRemoveAll                       └─ confirm (count from m.items)
  └─ print "hooks skipped"               └─ runAction("kill-em-all","--yes",…)
  └─ --yes? else TTY y/N : else refuse        └─ (spawns the CLI path at left)
  └─ RemoveAll
        ├─ force-remove each worktree (continue on error)
        ├─ force-delete each prefix branch (continue on error)
        └─ git worktree prune
  └─ summary + ErrPartialCleanup if any failure
```

## Error handling

| Situation | Behavior |
| --- | --- |
| Nothing in scope | Print "nothing to remove", exit 0. |
| No `--yes`, TTY | Prompt `y/N`; `n`/anything-else aborts (exit 0, "aborted"). |
| No `--yes`, no TTY | Error, exit 1 — instruct to pass `--yes`. |
| Some removals fail | Best-effort; summary lists failures; exit 6 (`ErrPartialCleanup`). |
| Cannot resolve repo | Existing `ErrNotARepo` path (exit 2). |

## Testing

- **`internal/git`**: `ListBranches` returns only prefix matches (integration
  test against a temp repo with prefixed + non-prefixed branches).
- **`pkg/worktree`** (with fakes; extend `fakeGit` with `ListBranches` +
  failure injection): `PlanRemoveAll` excludes main and includes orphan
  branches; `RemoveAll` happy path counts; best-effort continues past an
  injected worktree failure and an injected branch failure and records them;
  main never removed; non-prefixed branch never listed/deleted; tail prune is
  invoked.
- **`cmd/wt`**: `--yes` executes; non-TTY without `--yes` refuses; empty plan
  prints "nothing to remove"; partial failure yields exit 6; hooks-skipped
  notice printed.
- **`internal/tui`** (`model_test`): `K` enters `modeConfirmKillAll`; `y`
  dispatches `runAction("kill-em-all","--yes","--repo",dir)`; `n`/`esc` cancels.

## Risks

- **Destructive by design.** Mitigations: prefix-bounded scope, never-touch-main
  invariant, explicit confirmation (TTY prompt or `--yes`), and a read-only
  `PlanRemoveAll` that the confirmation is derived from.
- **Best-effort means partial state** on failure — surfaced via the summary and
  a non-zero exit code rather than hidden.
