---
phase: 01-worktree-derived-new
reviewed: 2026-06-05T20:05:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - pkg/worktree/types.go
  - pkg/worktree/manager.go
  - cmd/wt/new.go
  - pkg/worktree/manager_test.go
  - pkg/worktree/manager_integration_test.go
  - cmd/wt/new_test.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: resolved
fixes_applied: 2026-06-06
---

> **Fixes applied 2026-06-06** via `/gsd:code-review 1 --fix` (Critical+Warning scope):
> WR-01 (separator inserted for non-dash derive token, `a421f4e`), WR-02 (`dir` normalized
> with `filepath.Abs`+`EvalSymlinks` in `currentWorktreeBranch`, `b6a97df`), WR-03 (doc-comment
> note on `nextFreeVersion`; `BranchExists` semantics intentionally unchanged, `4061b35`).
> Info findings IN-01/IN-02 deferred (not in `--fix` scope without `--all`). All gates green
> (build, vet, gofmt, unit + integration tests).

# Phase 1: Code Review Report

**Reviewed:** 2026-06-05T20:05:00Z
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

This phase adds worktree-derive mode to `Manager.Add`: when `wt new` runs from
inside a managed (non-main) worktree with no explicit branch source, the new
branch is cut from the current worktree's branch with a lowest-free `-vNNN`
suffix (or a literal caller-supplied token). The path-separator-bounded matching
in `currentWorktreeBranch` is correct (shared-prefix siblings like `feat` vs
`feat-extra` do not false-match, subdirectories resolve), the `CheckRefFormat`
gate runs before any mutation, the gap-filling numbering is right, and the
main-root no-regression path is preserved and well tested.

No critical (BLOCKER) defects were found — there is no injection vector, secret,
crash, or data-loss path. However there are three correctness/robustness gaps
worth fixing before ship: a custom token without a leading dash is concatenated
to the parent branch with **no separator** (silently producing a malformed-looking
branch name), and derive detection silently disables itself when `dir` is not a
git-canonical absolute path (relative `--repo`, or a symlinked `os.Getwd()`).

## Warnings

### WR-01: Non-dash positional token glues to parent branch with no separator

**File:** `pkg/worktree/manager.go:170-172`
**Issue:** In derive mode, a custom token is appended verbatim:

```go
} else {
    branch = parentBranch + opts.Name
}
```

The design assumes the user supplies the separator as a leading dash
(`wt new -- -patch01` → `wt/feature-login-patch01`). But nothing enforces or
documents the no-dash case. Running `wt new fix` from inside `wt/feature-login`
yields `opts.Name = "fix"` and therefore `branch = "wt/feature-loginfix"` — a
valid ref (passes `CheckRefFormat`) with no collision, so it is silently created
with a name the user almost certainly did not intend. The help text only ever
shows the dash form and states "the leading dash is the separator," so this is
an undocumented, unguarded footgun. No test exercises a non-dash positional in
derive mode (`TestAdd_DeriveMode_CustomToken` uses `-patch01`).

**Fix:** Decide and enforce a contract. Either reject a token that does not begin
with `-` in derive mode, or normalize by inserting the separator:

```go
} else {
    sep := opts.Name
    if !strings.HasPrefix(sep, "-") {
        sep = "-" + sep
    }
    branch = parentBranch + sep
}
```

Add a derive-mode test for a bare token (e.g. `Name: "fix"`) asserting the
resulting branch (`wt/feature-login-fix`) or the rejection error.

### WR-02: Derive detection silently disabled when `dir` is not a git-canonical absolute path

**File:** `pkg/worktree/manager.go:99-120`
**Issue:** `currentWorktreeBranch` matches the raw `dir` against `w.Path` values
that git emits via `--show-toplevel` (always absolute and symlink-resolved):

```go
if dir != w.Path && !strings.HasPrefix(dir, w.Path+sep) {
    continue
}
```

`dir` originates from `managerForWorkdir` → `workdir()` (`cmd/wt/root.go:47-52`),
which returns `repoFlag` **verbatim** or `os.Getwd()`. Two real divergences make
the prefix match fail, silently dropping out of derive mode and falling back to
base-ref behavior with no error:

1. **Relative `--repo`:** `wt new --repo . ` (or `-r ../wt`) from inside a
   worktree passes a relative `dir`; `strings.HasPrefix("/abs/...", "...")`
   against `"."` never matches.
2. **Symlinked cwd:** on platforms where `os.Getwd()` returns an unresolved
   symlink path (e.g. macOS `/var` vs `/private/var`) while git returns the
   resolved path, the match fails on the *default* path too.

Because derive mode newly depends on this path comparison, the gap is newly
load-bearing even though `--repo` was always passed verbatim.

**Fix:** Normalize `dir` to a canonical absolute path at the top of
`currentWorktreeBranch` before comparing:

```go
if abs, err := filepath.Abs(dir); err == nil {
    dir = abs
}
if resolved, err := filepath.EvalSymlinks(dir); err == nil {
    dir = resolved
}
```

(`filepath.EvalSymlinks` requires the path to exist, which it does here — it is
the cwd.) Add a test feeding a relative `dir` and asserting derive still triggers.

### WR-03: `BranchExists` collapses operational git failures to "absent," weakening the derive collision guard

**File:** `pkg/worktree/manager.go:125-132, 193` (relies on
`internal/git/branch.go:25-28`)
**Issue:** `nextFreeVersion` and the custom-token collision check both gate on
`m.git.BranchExists(...)`. Per its own doc comment, `BranchExists` "collapses any
verification failure (including operational git errors) to false." If a transient
git failure (lock contention, corrupt ref) is misread as "absent," `nextFreeVersion`
can return a candidate that actually exists, or the custom-token guard can skip a
real collision — and the subsequent `git worktree add` then fails with a less clear
downstream error. This is a robustness/clarity gap, not a security issue, but the
derive path leans on `BranchExists` for correctness more heavily than prior code.

**Fix:** Out of scope to change `BranchExists` here, but worth a follow-up: have
the git layer distinguish "absent" from "git errored" (e.g. return `(bool, error)`)
so derive-mode numbering and collision detection fail loudly instead of silently
picking a colliding name. At minimum, note this assumption in `nextFreeVersion`'s
doc comment.

## Info

### IN-01: Derive-mode `name` uses configured prefix, not the inherited parent prefix

**File:** `pkg/worktree/manager.go:173`
**Issue:** `name = strings.TrimPrefix(branch, m.cfg.BranchPrefix())` strips the
*currently configured* prefix from the derived branch. In derive mode the prefix
is inherited from the parent branch verbatim and is deliberately independent of
config (the docs say `--branch-prefix` has no effect). If the configured prefix
was changed after the parent worktree was created, `TrimPrefix` is a no-op and
`name` (which flows into `AddResult.Name` and `WT_NAME` for hooks) still carries
the prefix. Cosmetic only — does not affect the created branch or path.

**Fix:** Strip the parent branch's actual prefix instead of the configured one,
or document that `Name` mirrors the configured-prefix view. Low priority.

### IN-02: Numbering loop is intentionally unbounded but undocumented as terminating

**File:** `pkg/worktree/manager.go:125-132`
**Issue:** `nextFreeVersion` is an unbounded `for n := 1; ;` loop. It terminates
at the first free slot (gap-filling guarantees a bound equal to the number of
existing `-vNNN` branches), so it is not an infinite-loop risk in practice — but
the unbounded form invites a future reader to assume otherwise.

**Fix:** No code change required. Consider a brief comment noting the loop is
bounded by the count of existing siblings.

---

_Reviewed: 2026-06-05T20:05:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
