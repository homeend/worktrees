# Roadmap: wt — Worktree-Derived `new`

## Overview

This milestone adds one user-facing capability to the existing `wt` CLI: running `wt new` from inside a worktree creates a sibling iteration branched off that worktree's own branch, auto-named with a free `-vNNN` suffix (or a caller-supplied suffix), without returning to the repo root or naming the branch by hand. Mode detection, branch derivation, and naming are facets of a single code path through `Manager.Add` — they ship together as one coherent vertical slice.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Worktree-Derived `new`** - `wt new` inside a worktree branches off the current branch with auto/custom suffix naming

## Phase Details

### Phase 1: Worktree-Derived `new`
**Goal**: Running `wt new` from inside a worktree creates a new branch and worktree derived from the current worktree's branch — auto-named `<branch>-vNNN` or with a caller-supplied suffix — while main-root behavior stays exactly as today.
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: DETECT-01, DETECT-02, DERIVE-01, DERIVE-02, DERIVE-03, NAME-01, NAME-02, NAME-03, NAME-04
**Success Criteria** (what must be TRUE):
  1. Running `wt new` with no token from inside a worktree creates a branch and worktree named `<current-branch>-v001`, branched off the committed tip of the current branch and placed in the main repo's worktree container (DETECT-01, DERIVE-01, DERIVE-02, NAME-01).
  2. Running `wt new` repeatedly picks the lowest free `-vNNN` number ≥ 1, skipping numbers whose branch already exists and filling gaps (NAME-02).
  3. Running `wt new "-patch01"` inside a worktree creates `<current-branch>-patch01`, and if that branch already exists the command fails with a clear collision error instead of renaming or auto-bumping (NAME-03, NAME-04).
  4. A derived branch keeps the parent branch's prefix verbatim; passing `--no-prefix` / `--branch-prefix` in worktree-derive mode does not alter the inherited prefix (DERIVE-03).
  5. Running `wt new` from the main repo root branches off `base_ref`/HEAD exactly as today, with no change in naming or placement (DETECT-02).
**Plans**: 1 plan

Plans:
- [ ] 01-01-PLAN.md — Derive-mode detection, `-vNNN`/custom-suffix naming, and CLI token passthrough through Manager.Add, with unit + integration tests

## Progress

**Execution Order:**
Phases execute in numeric order: 1

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Worktree-Derived `new` | 0/1 | Not started | - |
