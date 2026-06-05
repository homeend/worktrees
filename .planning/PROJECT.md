# wt — Git Worktree Manager

## What This Is

`wt` is a Go CLI + TUI for managing git worktrees: it creates branch-mirrored worktrees in a sibling container, lists/removes/prunes them, runs lifecycle hooks, and supports name templates and a configurable branch prefix. This milestone adds the ability to spawn a *sibling iteration* of the worktree you're currently standing in — `wt new` run from inside a worktree branches off that worktree's own branch instead of the repo's base ref.

## Core Value

Running `wt new` from inside a worktree creates a new branch + worktree based on the current worktree's branch, auto-named with a free `-vNNN` suffix (or a caller-supplied suffix) — without the user having to return to the repo root or name the branch by hand.

## Requirements

### Validated

<!-- Existing, working capabilities inferred from the codebase map. -->

- ✓ Create a worktree with `wt new` (derived/explicit name, `--branch`, `--base`, name templates, `--from-branch`) — existing
- ✓ Branch-mirrored worktree layout in a sibling container; nested dirs for slashed branch refs — existing
- ✓ Configurable branch prefix with per-run `--no-prefix` / `--branch-prefix` overrides — existing
- ✓ List worktrees (`wt list`), interactive TUI, `wt path` resolution — existing
- ✓ Remove a worktree (`wt rm`) with safe branch delete + empty-parent pruning; `kill-em-all` bulk cleanup — existing
- ✓ Lifecycle hooks (pre/post create + remove) with `WT_*` env context — existing
- ✓ Config (`wt init` / `wt set`), shell completion — existing
- ✓ `wt new` from inside a worktree derives the new branch from that worktree's current branch (auto-detected by cwd; main-root behavior unchanged) — Validated in Phase 1
- ✓ Default naming appends a zero-padded `-vNNN` suffix, choosing the lowest free number and skipping any whose branch already exists — Validated in Phase 1
- ✓ A positional token (`wt new -- -patch01`) is appended literally as a suffix to the current branch name, replacing the auto-number — Validated in Phase 1
- ✓ The new branch is created from the committed tip of the current worktree's branch; the new worktree lands in the main repo's container — Validated in Phase 1
- ✓ A custom-token name that already exists fails with a clear error instead of silently picking another name — Validated in Phase 1

### Active

<!-- This milestone. Building toward these. -->

(None — milestone v1.0 delivered in Phase 1. Code-review follow-ups tracked below under Key Decisions / see `01-REVIEW.md`.)

### Out of Scope

- Copying uncommitted/working-tree changes into the new worktree — new branch starts from the committed tip; git's normal behavior applies
- Prepending the token as a true prefix (`patch01-feature-login`) — decided to append as suffix instead
- Applying `--no-prefix` / `--branch-prefix` logic in worktree-derive mode — the parent branch already carries its prefix, which is inherited verbatim
- Recursively nesting the new worktree *under* the current worktree — it goes in the shared main-repo container, same as today

## Context

- Single-binary Go tool (`github.com/code-drill/wt`); commands under `cmd/wt/`, core logic in `pkg/worktree/manager.go`, git ops in `internal/git/`, config in `internal/config/`.
- Today `Manager.Add` always resolves `MainRoot(dir)` and branches off `cfg.BaseRef()`/HEAD — cwd does not influence the base. The new mode must detect when `dir` is inside a managed (non-main) worktree and use that worktree's branch as the base.
- A worktree's branch is discoverable via `GitRunner.ListWorktrees` (each entry carries `Branch`); branch existence checks already exist (`BranchExists`, `ListBranches(dir, prefix)`), which the free-`-vNNN` search can reuse.
- Codebase map available in `.planning/codebase/` (STACK, ARCHITECTURE, STRUCTURE, CONVENTIONS, TESTING, INTEGRATIONS, CONCERNS).
- Strong existing test coverage (`pkg/worktree/manager_test.go`, `cmd/wt/new_test.go`, integration tests) — new behavior should land with matching tests using the existing fakes.

## Constraints

- **Tech stack**: Go + cobra CLI; must extend the existing `Manager.Add` / `AddOptions` flow and `GitRunner` interface rather than bypass it.
- **Compatibility**: `wt new` from the main repo root must behave exactly as today (no regression); the new behavior triggers only from inside a worktree.
- **Testing**: New logic must be unit-testable via the injected `GitRunner`/`ConfigProvider` fakes; avoid hard dependencies on a real git repo in unit tests.
- **Naming safety**: Generated branch names still pass `CheckRefFormat` and collision checks before creation.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Trigger the new mode by auto-detecting cwd is inside a worktree | Zero-friction; no flag to remember | ✓ Good — shipped in Phase 1 |
| Custom token appended as a literal suffix (not a true prefix) | Matches the leading-dash form the user wrote (`-patch01` → `feature-login-patch01`) | ✓ Good — shipped; pass via `wt new -- -patch01` (cobra parses leading dash as a flag otherwise) |
| Zero-padded suffix width = 3 (`-v001`) | Matches the `vXXX` the user wrote | ✓ Good — shipped in Phase 1 |
| Default `-vNNN` picks the lowest free N, skipping existing branches | Predictable, gap-filling numbering | ✓ Good — shipped in Phase 1 |
| Custom token replaces numbering; collision is an error | User explicitly chose error over auto-bumping | ✓ Good — shipped in Phase 1 |
| Derived branch inherits the parent branch's prefix; `--no-prefix`/`--branch-prefix` don't apply in this mode | Prefix is already part of the parent branch name | ✓ Good — shipped in Phase 1 |
| New branch from committed tip; uncommitted changes not copied | Standard `git worktree add` semantics | ✓ Good — shipped in Phase 1 |
| Code-review warnings WR-01/WR-02/WR-03 | From `01-REVIEW.md`, applied via `/gsd:code-review 1 --fix` | ✓ Good — WR-01 (separator inserted for no-dash token) + WR-02 (`dir` normalized via Abs+EvalSymlinks) fixed with tests; WR-03 documented (BranchExists error semantics left as-is per reviewer). Info findings IN-01/IN-02 also addressed via `--fix --all` (doc-only: IN-01 restructure is algebraically identical, IN-02 loop-bound note). All 5 findings closed. |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-06-05 after Phase 1 completion (milestone v1.0 delivered — worktree-derived `wt new`)*
