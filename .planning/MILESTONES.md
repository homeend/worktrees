# Milestones

## v1.0 MVP (Shipped: 2026-06-06)

**Phases completed:** 1 phase, 1 plan, 3 tasks
**Timeline:** 2026-05-31 → 2026-06-06
**Feature source:** ~155 LOC across `pkg/worktree/manager.go`, `pkg/worktree/types.go`, `cmd/wt/new.go` (plus unit + integration tests)

**Delivered:** `wt new` run from inside a worktree now creates a sibling iteration — a new branch + worktree derived from the current worktree's own branch — without returning to the repo root or naming the branch by hand.

**Key accomplishments:**

- Worktree-derive mode for `wt new`, auto-detected by cwd (inside a worktree → derive; main repo root → unchanged base-ref behavior, no regression).
- Default auto-naming `<branch>-vNNN` (zero-padded width 3, lowest-free number, gap-filling), branched off the committed tip and placed in the main repo's worktree container.
- Caller-supplied suffix tokens (`wt new -- -patch01` → `<branch>-patch01`), with hard collision error and an auto-inserted `-` separator for bare tokens.
- Parent-branch prefix inherited verbatim; `--no-prefix` / `--branch-prefix` correctly inert in derive mode.
- All 9 requirements (DETECT/DERIVE/NAME) verified 5/5 against ROADMAP success criteria; full unit + real-git integration test coverage.
- Code review completed at standard depth — all 5 findings (3 warning, 2 info) remediated.

**Quality:** Phase goal verification passed (5/5 must-haves). `go build`, `go vet`, `gofmt`, unit tests, and `-tags integration` tests all green.

**Known deferred items:** none open. One follow-up noted for next milestone: give `BranchExists` `(bool, error)` semantics so derive-mode numbering/collision fail loudly (code-review WR-03 deeper fix).

---
