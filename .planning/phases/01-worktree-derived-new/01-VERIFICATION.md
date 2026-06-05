---
phase: 01-worktree-derived-new
verified: 2026-06-05T22:10:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Phase 01: Worktree-Derived `wt new` Verification Report

**Phase Goal:** Running `wt new` from inside a worktree creates a new branch and worktree derived from the current worktree's branch — auto-named `<branch>-vNNN` or with a caller-supplied suffix — while main-root behavior stays exactly as today.
**Verified:** 2026-06-05T22:10:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                                                                                                                                    | Status     | Evidence                                                                                                                                                                                                                          |
|----|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1  | Running `wt new` with no token from inside a worktree creates branch+worktree `<current-branch>-v001`, branched off the committed tip of the current branch, in the main container (DETECT-01, DERIVE-01, DERIVE-02, NAME-01) | ✓ VERIFIED | `TestAdd_DeriveMode_AutoV001` (unit): Add from wtDir with empty opts → branch `wt/feature-login-v001`, baseRef `wt/feature-login`, path under main container. Integration: `TestManager_DeriveFromWorktree_Integration_RealGit` criterion (1) asserts `v1Tip == parentTip` via real `rev-parse`. |
| 2  | Running `wt new` repeatedly picks the lowest free `-vNNN` ≥ 1, skipping existing branches and filling gaps (NAME-02)                                                                                      | ✓ VERIFIED | `TestAdd_DeriveMode_GapFill` (unit): -v001 + -v002 exist → -v003; only -v002 exists → -v001. Integration criterion (2): creates -v003 by hand, next Add yields -v002, then -v004.                                               |
| 3  | Running `wt new -- "-patch01"` inside a worktree creates `<current-branch>-patch01`; if that branch exists it fails with a clear collision error, no rename/auto-bump (NAME-03, NAME-04)                | ✓ VERIFIED | `TestAdd_DeriveMode_CustomToken` and `TestAdd_DeriveMode_CustomTokenCollisionErrors` (unit): error names the derived branch, does NOT say "pass a different --branch", `len(g.added)==0`. Integration criterion (3): second Add with same token returns non-nil error and worktree count is unchanged. |
| 4  | A derived branch keeps the parent branch's prefix verbatim; `--no-prefix`/`--branch-prefix` in derive mode do not alter the inherited prefix (DERIVE-03)                                                  | ✓ VERIFIED | `TestAdd_DeriveMode_IgnoresPrefixAndBaseOverrides` (unit): NoPrefix=true, PrefixOverride="x/", BaseRef="develop" → branch still `wt/feature-login-v001`, baseRef still `wt/feature-login`. Integration criterion (4): same flags → branch still has `wt/feature-login` prefix and tip matches parent tip. |
| 5  | Running `wt new` from the main repo root branches off base_ref/HEAD exactly as today, unchanged naming and placement (DETECT-02)                                                                          | ✓ VERIFIED | `TestAdd_DeriveMode_MainRootRegression` (unit): dir==MainRoot, Name="feat" → branch `wt/feat`, no -vNNN. Integration criterion (5): `root.Branch=="wt/from-root"`, tip equals `HEAD` (not the worktree branch tip). `go test ./... -count=1` passes all pre-existing tests without regression. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact                                     | Expected                                                     | Status     | Details                                                                                                                                                                        |
|----------------------------------------------|--------------------------------------------------------------|------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `pkg/worktree/manager.go`                    | Derive-mode detection + name/branch/baseRef resolution in Add | ✓ VERIFIED | Contains `currentWorktreeBranch` (path-separator-bounded longest-prefix match, ok=false for main or detached), `nextFreeVersion` (gap-filling loop from n=1), and derive fork in `Add` gated on `!fromExisting && opts.Branch=="" && !opts.FromTemplate`. |
| `pkg/worktree/types.go`                      | `AddOptions.FromTemplate bool` field                         | ✓ VERIFIED | Field present with doc comment explaining its purpose in derive mode.                                                                                                          |
| `cmd/wt/new.go`                              | Positional token passthrough + FromTemplate signal + Long help | ✓ VERIFIED | `buildAddOptions` sets `opts.FromTemplate=true` in the `tmpl != ""` case; default positional case sets `opts.Name = args[0]` verbatim; `newCmd.Long` documents derive mode and the `wt new -- -patch01` form. |
| `pkg/worktree/manager_test.go`               | Unit tests for all derive-mode behaviors on fakes            | ✓ VERIFIED | 11 derive-mode tests: AutoV001, SubdirOfWorktree, SharedPrefixNoFalseMatch, GapFill, CustomToken, CustomTokenCollisionErrors, IgnoresPrefixAndBaseOverrides, MainRootRegression, TemplateFallThrough, ExplicitBranchFallThrough, plus CLI tests in new_test.go. |
| `pkg/worktree/manager_integration_test.go`   | Real-git end-to-end coverage of all 5 success criteria       | ✓ VERIFIED | `//go:build integration` on line 1; `TestManager_DeriveFromWorktree_Integration_RealGit` executes under `-tags integration` — confirmed RUN + PASS output (not a silent skip). Covers all 5 criteria including `rev-parse` tip equality for DERIVE-01. |
| `cmd/wt/new_test.go`                         | CLI unit tests for token passthrough and FromTemplate signal  | ✓ VERIFIED | `TestBuildAddOptions_LeadingDashTokenPassthrough`: args=["-patch01"] → Name=="-patch01", FromTemplate==false. `TestBuildAddOptions_TemplateSetsFromTemplate`: template path → FromTemplate==true. `TestBuildAddOptions_PlainNameNotFromTemplate`: regression case. |

### Key Link Verification

| From                          | To                          | Via                                                                | Status     | Details                                                                                                                                             |
|-------------------------------|-----------------------------|--------------------------------------------------------------------|------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| `Manager.Add` (derive mode)   | `GitRunner.ListWorktrees`   | `currentWorktreeBranch` called inside the derive-mode gate         | ✓ WIRED    | Gate at manager.go:155–159: calls `m.currentWorktreeBranch(dir, repoRoot)` which calls `m.git.ListWorktrees(dir)`. Only reached when cheap conditions pass. |
| `Manager.Add` (derive mode)   | `GitRunner.BranchExists`    | `nextFreeVersion` loop + custom-token collision check              | ✓ WIRED    | `nextFreeVersion` at manager.go:125–132 calls `m.git.BranchExists(repoRoot, candidate)`. Custom-token collision check at manager.go:193 calls `m.git.BranchExists(repoRoot, branch)`. |
| `buildAddOptions` (template)  | `AddOptions.FromTemplate`   | `opts.FromTemplate = true` in the `case tmpl != ""` branch        | ✓ WIRED    | new.go:83: `opts.FromTemplate = true` set immediately after `opts.Name = name` from `ResolveTemplate`. Default positional case leaves it false.        |
| `Manager.Add` derive gate     | `opts.FromTemplate`         | `!opts.FromTemplate` in the derive-mode predicate                  | ✓ WIRED    | manager.go:155: `if !fromExisting && opts.Branch == "" && !opts.FromTemplate` — the signal correctly suppresses derive mode for template-named adds. |

### Data-Flow Trace (Level 4)

Not applicable. This phase adds CLI + domain logic with no rendered data components. The artifacts are a domain orchestrator (`Manager.Add`), types, and tests — no data-rendering component that requires a Level 4 trace.

### Behavioral Spot-Checks

| Behavior                              | Command                                                                                    | Result                                                              | Status  |
|---------------------------------------|--------------------------------------------------------------------------------------------|---------------------------------------------------------------------|---------|
| Unit tests pass (all packages)        | `go test ./... -count=1`                                                                   | All 7 packages pass, 0 failures                                     | ✓ PASS  |
| Integration test genuinely runs       | `go test -tags integration ./pkg/worktree/ -run Integration -v -count=1`                   | `=== RUN TestManager_DeriveFromWorktree_Integration_RealGit` + PASS | ✓ PASS  |
| Build clean                           | `go build ./...`                                                                            | No output (success)                                                 | ✓ PASS  |
| go vet clean                          | `go vet ./...`                                                                              | No output (success)                                                 | ✓ PASS  |
| gofmt clean                           | `gofmt -l pkg/worktree cmd/wt`                                                             | No output (no files need formatting)                                | ✓ PASS  |

### Probe Execution

No probes declared in PLAN or SUMMARY. Step skipped (no `scripts/*/tests/probe-*.sh` present).

### Requirements Coverage

| Requirement | Source Plan | Description                                                                                              | Status      | Evidence                                                                                                      |
|-------------|-------------|----------------------------------------------------------------------------------------------------------|-------------|---------------------------------------------------------------------------------------------------------------|
| DETECT-01   | 01-01-PLAN  | `wt new` inside a managed worktree derives from that worktree's branch                                  | ✓ SATISFIED | `currentWorktreeBranch` detects the enclosing worktree; `deriveMode` gate activates; unit + integration prove it. |
| DETECT-02   | 01-01-PLAN  | `wt new` from the main repo root is unchanged                                                            | ✓ SATISFIED | `ok=false` when `best==repoRoot` in `currentWorktreeBranch`; `TestAdd_DeriveMode_MainRootRegression` + integration criterion (5). |
| DERIVE-01   | 01-01-PLAN  | New branch cut from the committed tip of the current branch (not working-tree state)                     | ✓ SATISFIED | `baseRef = parentBranch` in derive mode; integration test compares `rev-parse wt/feature-login-v001` to parent tip. |
| DERIVE-02   | 01-01-PLAN  | New worktree created in the main repo container, not nested under current worktree                       | ✓ SATISFIED | `worktreePath(repoRoot, branch)` anchors under main container; `TestAdd_DeriveMode_AutoV001` asserts `res.Path` under `.worktrees`. |
| DERIVE-03   | 01-01-PLAN  | Derived branch keeps parent prefix verbatim; `--no-prefix`/`--branch-prefix` have no effect in derive mode | ✓ SATISFIED | Derive case skips `effectivePrefix`/`resolveNames` entirely; `TestAdd_DeriveMode_IgnoresPrefixAndBaseOverrides` + integration criterion (4). |
| NAME-01     | 01-01-PLAN  | No token → `<current-branch>-vNNN` (zero-padded 3-digit)                                                | ✓ SATISFIED | `nextFreeVersion` uses `fmt.Sprintf("%s-v%03d", parentBranch, n)`; `TestAdd_DeriveMode_AutoV001`. |
| NAME-02     | 01-01-PLAN  | `-vNNN` is the lowest free value ≥ 1 (skips existing, fills gaps)                                       | ✓ SATISFIED | `nextFreeVersion` iterates from n=1; `TestAdd_DeriveMode_GapFill` covers both skipping and gap-filling. |
| NAME-03     | 01-01-PLAN  | Positional token appended literally as suffix (e.g. `-patch01`)                                          | ✓ SATISFIED | `branch = parentBranch + opts.Name`; `TestAdd_DeriveMode_CustomToken` asserts `wt/feature-login-patch01`. |
| NAME-04     | 01-01-PLAN  | Custom-token branch that already exists → clear error, no rename/auto-bump                               | ✓ SATISFIED | Separate `BranchExists` check + `fmt.Errorf("derived branch %q already exists", branch)`; test asserts error names the branch, no generic message, `len(g.added)==0`. |

### Anti-Patterns Found

No TBD/FIXME/XXX debt markers found in any modified file. No TODO/HACK/PLACEHOLDER markers in implementation files. No stub patterns (`return null`, empty handler, hardcoded empty returns) found in modified files.

**Code review warnings (from 01-REVIEW.md) — impact on goal achievement:**

| Finding | Severity in Review | Goal-Achievement Impact |
|---------|--------------------|-------------------------|
| WR-01: Non-dash token glues to parent branch with no separator (e.g. `wt new fix` → `wt/feature-loginfix`) | Warning | Not a goal blocker. NAME-03 specifies "appended literally" and the design explicitly puts separator responsibility on the caller ("the user's leading dash is the separator"). The spec-defined contract is implemented as designed. |
| WR-02: Derive detection silently disabled with relative `--repo` or symlinked cwd | Warning | Not a goal blocker on the happy path (Linux `os.Getwd()` returns absolute resolved paths). The integration test runs with real git on Linux and passes. The edge case is a robustness gap, not a requirement failure. |
| WR-03: `BranchExists` collapses git errors to "absent" | Warning | Not a goal blocker. Pre-existing behavior used by existing code. The derive path depends on it no more heavily than prior paths. |
| IN-01: `name` field uses configured prefix, not inherited parent prefix | Info | Cosmetic only — does not affect branch, path, or any requirement. |
| IN-02: `nextFreeVersion` loop is unbounded but gap-bounded in practice | Info | No code risk. |

All three review warnings are robustness/edge-case concerns, not requirement failures. None block the stated phase goal.

### Human Verification Required

None. All five success criteria are fully verifiable by running the test suite (unit + integration) and confirmed green. No visual/real-time/external-service behavior is introduced.

---

_Verified: 2026-06-05T22:10:00Z_
_Verifier: Claude (gsd-verifier)_
