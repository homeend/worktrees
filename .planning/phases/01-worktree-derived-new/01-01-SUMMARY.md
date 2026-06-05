---
phase: 01-worktree-derived-new
plan: 01
subsystem: worktree-create
tags: [worktree, derive-mode, cli, naming]
requires:
  - "pkg/worktree.Manager.Add"
  - "GitRunner.ListWorktrees / BranchExists / VerifyRef / CheckRefFormat / AddWorktree"
provides:
  - "Derive-mode branch creation from the enclosing worktree's branch"
  - "Lowest-free -vNNN auto-naming (gap-filling) and literal-suffix tokens"
  - "AddOptions.FromTemplate signal"
affects:
  - "pkg/worktree/manager.go"
  - "cmd/wt/new.go"
tech-stack:
  added: []
  patterns:
    - "Cheap-condition gate before the extra ListWorktrees git call to keep non-derive paths byte-for-byte equivalent"
    - "Path-separator-bounded longest-prefix matching for worktree detection"
key-files:
  created:
    - ".planning/phases/01-worktree-derived-new/01-01-SUMMARY.md"
  modified:
    - "pkg/worktree/types.go"
    - "pkg/worktree/manager.go"
    - "pkg/worktree/manager_test.go"
    - "cmd/wt/new.go"
    - "cmd/wt/new_test.go"
    - "pkg/worktree/manager_integration_test.go"
decisions:
  - "Derive mode is gated on ok==true AND opts.Branch=='' AND opts.FromBranch=='' AND !opts.FromTemplate; any failure falls through to today's behavior."
  - "Custom-token collision returns a distinct error naming the derived branch (keeps the 'already exists' substring for classify() but never the generic 'pass a different --branch')."
  - "Integration test name carries Integration so the plan's `-run Integration` filter actually runs it (avoids a false-green silent skip)."
metrics:
  duration: "~25m"
  completed: "2026-06-05"
  tasks: 3
  files: 6
---

# Phase 01 Plan 01: Worktree-Derived `wt new` Summary

`wt new` now derives the new branch from the current worktree's branch (auto `<branch>-vNNN` or a literal suffix token) when run inside a managed non-main worktree, cut from that branch's committed tip into the main container — while main-repo-root behavior is unchanged.

## What Was Built

- **`AddOptions.FromTemplate bool`** (`pkg/worktree/types.go`) — set by the CLI when `Name` came from `--template`, so a rendered name falls through to normal naming instead of being reinterpreted as a derive-mode suffix token.
- **`Manager.currentWorktreeBranch(dir, repoRoot)`** — detects the enclosing non-main worktree by selecting the listed worktree whose `Path` is the longest *path-separator-bounded* prefix of `dir` (so a cwd that is a subdir resolves, and `feat` vs `feat-extra` siblings never false-match). Returns `ok=false` for the main worktree or a detached/empty branch.
- **`Manager.nextFreeVersion(repoRoot, parentBranch)`** — lowest-free zero-padded `-vNNN` search (gap-filling) skipping existing branches.
- **Derive fork in `Manager.Add`** — gated behind the cheap conditions *before* the extra `ListWorktrees` call so the explicit/`--from-branch`/`--template` paths make no new git call and stay byte-for-byte equivalent. In derive mode: auto `-vNNN` or `parentBranch + opts.Name` (verbatim), inherited prefix kept as-is (no `effectivePrefix`/`NoPrefix`/`PrefixOverride`), `baseRef = parentBranch` (`opts.BaseRef`/config base ignored), shared `CheckRefFormat` gate reused, custom-token collision is a hard error naming the derived branch.
- **CLI (`cmd/wt/new.go`)** — `buildAddOptions` sets `opts.FromTemplate=true` in the `--template` branch; the positional case stays dumb (`opts.Name = args[0]` verbatim, leading dash preserved). `newCmd.Long` documents derive mode and the `wt new -- -patch01` form.
- **Tests** — 11 derive-mode unit tests (`pkg/worktree/manager_test.go`) on injected fakes; 3 CLI tests (`cmd/wt/new_test.go`); one real-git integration test (`//go:build integration`) covering all five success criteria including a `rev-parse` check that the derived branch starts at the parent tip.

## Tasks

| Task | Name | Commits |
| ---- | ---- | ------- |
| 1 | AddOptions.FromTemplate + derive detection/resolution in Manager.Add | 2aa2717 (test), 223537a (feat) |
| 2 | CLI token passthrough + FromTemplate signal + help text | f3d40dc (test), 6b55ee0 (feat) |
| 3 | End-to-end real-git integration test (all five criteria) | 874cce9 (test) |

## TDD Gate Compliance

Tasks 1 and 2 followed RED → GREEN: a `test(...)` commit with failing tests precedes each `feat(...)` commit. Task 3 is a `type=auto` test-only task (no implementation), committed as `test(...)`.

## Verification

- `go build ./...` — succeeds.
- `go test ./... -count=1` — all packages pass.
- `go test -tags integration ./... -count=1` — all packages pass; `go test -tags integration ./pkg/worktree/ -run Integration -v` shows `TestManager_DeriveFromWorktree_Integration_RealGit` RUN + PASS (not a silent skip).
- `go vet ./...` — clean.
- `gofmt -l pkg/worktree cmd/wt` — prints nothing.

## Success Criteria (ROADMAP Phase 1)

1. `wt new` (no token) inside a worktree → `<branch>-v001`, cut from the committed tip, in the main container — covered (unit + integration, `rev-parse` tip check).
2. Repeated `wt new` → lowest-free `-vNNN`, skipping existing, filling gaps — covered.
3. `wt new -- "-patch01"` → `<branch>-patch01`; existing → clear collision error, no rename/auto-bump, no second worktree — covered.
4. Derived branch keeps the parent prefix verbatim; `--no-prefix`/`--branch-prefix`/`--base` have no effect in derive mode — covered.
5. `wt new` from the main repo root unchanged (branches off base_ref/HEAD, same naming/placement) — covered.

## Threat Model

The single load-bearing control (T-01-01/T-01-02) is the existing `m.git.CheckRefFormat(branch)` gate, reused unchanged on the derived branch — an illegal suffix token is rejected before any branch/worktree creation, so no path traversal is reachable. No new dependencies (T-01-SC). No new threat surface introduced beyond the register.

Note: the `CheckRefFormat` rejection control is structurally present (applied to the derived branch before collision/Add) but **not exercised** by tests — the unit fake's `CheckRefFormat` always returns nil and the integration test feeds only valid tokens. No acceptance criterion required exercising the rejection path; on record here as "control present, not exercised."

## Deviations from Plan

**1. [Rule 3 - Blocking] Integration test name adjusted to satisfy the plan's `-run Integration` verify filter**
- **Found during:** Task 3
- **Issue:** The plan's verify command is `go test -tags integration ./pkg/worktree/ -run Integration`, but the codebase convention names integration tests `TestManager_..._RealGit` (no "Integration" substring). With a `_RealGit`-only name, `-run Integration` matched nothing and printed `no tests to run` — the exact false-green silent skip the acceptance criteria warns against.
- **Fix:** Named the test `TestManager_DeriveFromWorktree_Integration_RealGit` so it matches `-run Integration` (still carries the `//go:build integration` tag and the `_RealGit` convention suffix). Verified the filter now executes the test (RUN + PASS).
- **Files modified:** pkg/worktree/manager_integration_test.go
- **Commit:** 874cce9

## Known Stubs

None.

## Self-Check: PASSED
