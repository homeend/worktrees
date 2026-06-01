# Branch-mirrored worktree layout + per-run prefix controls — design

**Date:** 2026-06-01
**Status:** Approved (pending spec review)
**Amends:** FR-3.2, FR-3.3, FR-4.3, FR-6.x in `docs/FUNCTIONAL-REQUIREMENTS.md`
**Related:** [configurable branch prefix](./2026-06-01-configurable-branch-prefix-design.md),
[named templates + from-branch](./2026-06-01-template-names-and-branch-worktrees-design.md)

## Problem

Today the worktree directory is a single flattened segment with the prefix
stripped: branch `mrutkowski/autofix/MTRH-2132` → dir
`<container>/autofix-MTRH-2132`. With templates (which contain `/`) this is
confusing — the prefix vanishes and the structure is lost. Users expect the
directory to **mirror the full branch ref**:
`<container>/mrutkowski/autofix/MTRH-2132`. Separately, users want to override
or disable the prefix for a single `wt new`.

## Decisions (confirmed with user)

| Decision | Choice |
| --- | --- |
| Directory layout | **Mirror the full branch**: `dir = <container>/<branch>`, slashes as nested subdirectories, **prefix included**. |
| Empty parents on `rm` | **Prune** now-empty parent dirs up to (not including) the container. |
| Per-run prefix off | `wt new --no-prefix` (bool) creates the branch without the configured prefix. |
| Per-run prefix override | `wt new --branch-prefix <value>` overrides the prefix for this run (normalized; empty disables). |
| Precedence of the two | `--no-prefix` **wins** over `--branch-prefix` if both are given. |

Consequence (intended): the default case nests too — a generated branch
`wt/2026-06-01_14-30-foo-1234` lives at
`<container>/wt/2026-06-01_14-30-foo-1234`. Existing worktrees are unaffected;
this changes only newly-created ones.

## Architecture & components

### 1. `pkg/worktree/manager.go`

**`worktreePath`** mirrors the branch instead of flattening:

```go
func (m *Manager) worktreePath(repoRoot, branch string) string {
	return filepath.Join(m.containerPath(repoRoot), filepath.FromSlash(branch))
}
```

(No more `naming.SanitizeDir` here. Git ref rules already forbid the characters
that would make path mirroring unsafe — `..`, control chars, leading/trailing
`/`, etc. — and `CheckRefFormat` runs before this.)

**`resolveNames`** computes the effective prefix from the new options:

```go
prefix := m.cfg.BranchPrefix()
if opts.NoPrefix {
	prefix = ""
} else if opts.PrefixOverride != "" {
	prefix = opts.PrefixOverride // already normalized by the CLI
}
branch = prefix + strings.TrimPrefix(base, prefix)
```

**`Add` (from-branch path)** sets the display name without the prefix and lets
`worktreePath` mirror the (verbatim) branch:

```go
name = strings.TrimPrefix(branch, m.cfg.BranchPrefix())
```

(`--no-prefix`/`--branch-prefix` are no-ops for `--from-branch`, which uses the
branch verbatim.)

**`resolveWorktree`** must match nested paths. It matches a worktree when **any**
of these hold (prefix = `m.cfg.BranchPrefix()`, `nameNoPrefix =
TrimPrefix(name, prefix)`):
- branch equals `refs/heads/<prefix><nameNoPrefix>` or `refs/heads/<name>`;
- the container-relative path (slash-normalized) equals `name`,
  `<prefix><nameNoPrefix>`, or `nameNoPrefix`;
- the leaf basename equals the last segment of `name`.

The main worktree is still refused; not-found still errors. (The full branch ref
is the unambiguous identifier; a bare leaf may match the first of several
branches sharing it — documented.)

**`Remove`** prunes empty parents after the worktree is removed, before the
post-remove hook:

```go
container := m.containerPath(repoRoot)
for parent := filepath.Dir(w.Path); parent != container &&
	strings.HasPrefix(parent, container+string(filepath.Separator)); parent = filepath.Dir(parent) {
	if err := os.Remove(parent); err != nil {
		break // non-empty or error → stop
	}
}
```

(`os.Remove` only deletes empty dirs; the first non-empty parent stops the walk.
Requires adding `os` to the manager imports.)

### 2. `pkg/worktree/types.go`

`AddOptions` gains:

```go
	NoPrefix       bool   // skip the configured branch prefix
	PrefixOverride string // override the configured prefix for this run (normalized)
```

### 3. `internal/naming`

`SanitizeDir` is no longer used in production (the layout no longer flattens).
Remove `SanitizeDir` and its test `TestSanitizeDir_StripsPrefixAndSlashes`
(`RenderTemplate`/`Generate*` stay).

### 4. `cmd/wt/new.go`

- New flags: `--no-prefix` (bool) and `--branch-prefix <value>` (string).
- The override is normalized with `config.NormalizePrefix` before being passed.
- `buildAddOptions` gains `noPrefix bool, prefixOverride string`, sets
  `opts.NoPrefix`/`opts.PrefixOverride`. `--no-prefix` wins (when set, the
  override is ignored). These are independent of the
  `--template`/`--from-branch`/`--branch` mutual-exclusion group.

### 5. Docs

- `README.md`: update the Config/Branch-prefix section and the `wt new` example
  paths to the mirrored layout; document `--no-prefix` and `--branch-prefix`.
- `docs/FUNCTIONAL-REQUIREMENTS.md`: rewrite FR-3.2/FR-4.3 (mirrored layout), add
  the prune-empty-parents requirement to §6, and add an FR for the two flags.

## Data flow

```
wt new -t junie issue:MTRH-2132     (branch_prefix "mrutkowski/")
  └─ ResolveTemplate -> "autofix/MTRH-2132"  (Name)
  └─ resolveNames: branch = "mrutkowski/autofix/MTRH-2132"
  └─ worktreePath = <container>/mrutkowski/autofix/MTRH-2132   (nested)

wt new -t junie issue:X --no-prefix
  └─ branch = "autofix/X"  -> <container>/autofix/X

wt new -t junie issue:X --branch-prefix team/
  └─ branch = "team/autofix/X" -> <container>/team/autofix/X

wt rm mrutkowski/autofix/MTRH-2132
  └─ resolve by branch -> remove leaf -> prune empty mrutkowski/autofix, mrutkowski
```

## Error handling

| Situation | Behavior |
| --- | --- |
| Rendered/explicit branch invalid as a ref | existing `CheckRefFormat` error (before any dir is created) |
| `rm <name>` not found | existing not-found error |
| Parent dir non-empty during prune | stop pruning silently (another worktree still lives there) |

## Testing

- **manager:** `worktreePath` mirrors a slashed branch; `resolveNames` honors
  `NoPrefix` and `PrefixOverride` (and `--no-prefix` wins); `Add` from-branch
  name = branch w/o prefix; `resolveWorktree` resolves by full branch and by leaf.
- **integration (`manager_integration_test.go`):** create a worktree on a
  slashed branch → dir nested; `Remove` deletes it and prunes empty parents.
- **naming:** remove the obsolete `SanitizeDir` test.
- **cmd:** `buildAddOptions` sets `NoPrefix`/`PrefixOverride`; `--no-prefix` wins.

## Risks

- **Layout change** for newly-created worktrees (existing ones unaffected). The
  default case now nests under the prefix dir (e.g. `wt/`). Documented.
- **Leaf-name resolution ambiguity** if two branches share a trailing segment;
  the full branch ref is the canonical identifier.
- **Path mirroring safety** relies on git ref validity (`CheckRefFormat`).
