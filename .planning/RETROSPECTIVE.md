# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — MVP

**Shipped:** 2026-06-06
**Phases:** 1 | **Plans:** 1 | **Tasks:** 3

### What Was Built
- Worktree-derive mode for `wt new`: run from inside a worktree, it branches off that worktree's own branch (auto-detected by cwd) instead of the repo base ref.
- Auto-naming `<branch>-vNNN` (zero-padded, lowest-free, gap-filling) plus caller-supplied suffix tokens (`wt new -- -patch01`) with hard collision errors.
- Parent-prefix inheritance, committed-tip base, main-container placement — all delivered as one coherent vertical slice through `Manager.Add`, with unit + real-git integration tests.

### What Worked
- Front-loading the design decisions during `/gsd:new-project` questioning meant planning needed no separate discuss-phase — the locked decisions flowed straight into REQUIREMENTS.md, PROJECT.md Key Decisions, and the planner prompt.
- Keeping the feature as a single coarse phase / single plan matched the reality that detection + derivation + naming are one code path; splitting would have fragmented a PR-sized change.
- The plan-checker caught two real issues pre-execution (cobra parsing of leading-dash tokens → `wt new -- -patch01`; the `--template`/positional-suffix `opts.Name` collision → `FromTemplate` signal), and again the integration-test `-tags integration` false-green before any code was written.
- Verification was goal-backward and genuine (real `rev-parse` tip comparison for DERIVE-01), not just task-completion checking.

### What Was Inefficient
- The executor dogfooded the feature against the live repo and left a stray `wt/-patch01` worktree+branch that needed manual cleanup.
- The code-fixer created a `fix/...` branch on the first `--fix` run despite the project's "none" branching strategy, requiring a fast-forward back onto `main`. The second `--fix --all` run with explicit branch-discipline instructions behaved correctly.
- The milestone-complete CLI auto-extracted a weak "accomplishment" (a deviation note) for MILESTONES.md, needing a manual rewrite.

### Patterns Established
- For sub-agent runs that commit, state the branching strategy explicitly in the prompt ("commit on the current branch; do not create branches") — relying on the default led to a deviation.
- Integration test files/functions must carry `//go:build integration` AND a name the plan's `-run` filter matches, or the verify step silently passes without running them.
- Brownfield phase 01 should explicitly disable Walking Skeleton (MVP vertical-slice planning still applies, but scaffolding does not).

### Key Lessons
1. Heavy up-front questioning is the cheapest leverage point — it removed an entire discuss-phase and made the planner's output concrete on the first pass.
2. Independent review agents (plan-checker, verifier, code-reviewer) each caught distinct, real issues; the layered gates earned their cost on even a small feature.
3. Sub-agent git behavior needs explicit guardrails (branch, worktree cleanup) — the harness defaults don't always match the project's branching strategy.

### Cost Observations
- Model mix: planning/execution on Opus (planner, executor), Sonnet for checker/verifier/reviewer (Quality profile).
- Notable: a single-phase milestone still exercised the full GSD pipeline (map → new-project → plan → execute → review → fix → complete) end-to-end without manual code edits by the orchestrator.

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v1.0 | 1 | 1 | Baseline — full GSD pipeline on a brownfield single-feature milestone |

### Cumulative Quality

| Milestone | Tests | Integration Tests | Zero-Dep Additions |
|-----------|-------|-------------------|--------------------|
| v1.0 | unit + integration green | real-git harness, all 5 derive criteria | yes (no new deps; extended existing `Manager.Add`) |

### Top Lessons (Verified Across Milestones)

1. Front-loaded design decisions reduce downstream planning overhead. *(to be re-validated in v1.1+)*
2. Explicit sub-agent git guardrails prevent branch/worktree deviations. *(to be re-validated in v1.1+)*
