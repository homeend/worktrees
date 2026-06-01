# Configurable branch prefix — design

**Date:** 2026-06-01
**Status:** Approved (pending spec review)

## Problem

The branch prefix for worktrees created with `wt new` is hardcoded as `wt/` in
three places:

- `pkg/worktree/manager.go:61` — derives the branch name (`"wt/" + ...`)
- `pkg/worktree/manager.go:185` — matches existing worktrees by branch
- `internal/naming/naming.go:61` — strips the prefix when building the on-disk
  directory name

Users want to choose their own prefix (e.g. `feature/`, `dev/`) through the
config file, an environment variable, and a CLI command.

## Goals

1. Make the `new` branch prefix configurable via **config file**, **environment
   variable**, and a **CLI `set` command**.
2. `wt set branch_prefix <value>` writes the value to the config file.
3. `wt set branch_prefix <value> --safe` refuses to overwrite an existing,
   different value.

## Non-goals (YAGNI)

- No per-invocation `--branch-prefix` flag on `wt new` (the existing `--branch`
  flag already lets you pass a full branch name for one-offs).
- No env-var layer for the other config fields (`base_ref`, `container`,
  `name_template`) — only `branch_prefix` gets one, per scope decision.
- No `wt get` command.

## Decisions (confirmed with user)

| Decision | Choice |
| --- | --- |
| `set` command shape | Generic `set <key> <value>`; today only `branch_prefix` is a valid key (unknown key → error). |
| Env-var scope | `WT_BRANCH_PREFIX` only. |
| Value normalization | **Auto-append `/`** — `feature` is stored/used as `feature/`. Empty value is rejected. |
| `--safe` when value equals existing | **Succeed (no-op)** — errors only when the existing value *differs*. |
| CLI syntax (matched to cobra conventions) | `wt set branch_prefix <value> [--safe]` — a subcommand, not a `-set` flag. |

## Resolution & precedence

At `wt new` time the prefix is resolved in this order (highest wins):

```
WT_BRANCH_PREFIX (env)  >  branch_prefix (config file)  >  default "wt/"
```

- The default is `wt/`, preserving today's behavior when nothing is configured.
- An empty/unset layer falls through to the next (consistent with how
  `base_ref`/`container`/`name_template` already resolve).
- The resolved value is normalized (trailing `/` appended if missing).

`--safe` compares against the **config-file** value only (not env), because
`set` persists to the file and env is a runtime override.

## Components & changes

### 1. `internal/config/config.go`

- Add field: `BranchPrefix string `+"`yaml:\"branch_prefix\"`"+`.
- `Defaults()` returns `BranchPrefix: "wt/"` (in addition to `BaseRef: "HEAD"`).
- `Resolve(lo, hi)` gains a clause: non-empty `hi.BranchPrefix` overrides.
- New helper `NormalizePrefix(s string) string`: returns `""` for empty;
  otherwise appends `/` if absent. Single source of truth for normalization.
- New `LoadFile(repoRoot) (Config, error)`: reads **only** the YAML file and
  returns the raw parsed config (missing file → zero `Config`, nil error). Used
  by `--safe` to inspect the persisted value without defaults/env mixed in.
- `Load(repoRoot)` becomes: `Defaults()` → merge file (`LoadFile`) → merge env
  (`WT_BRANCH_PREFIX`) → normalize `BranchPrefix`. Env is read here, scoped to
  the prefix only.
- New writer `Set(repoRoot, key, value string) error`:
  - Validates `key` (only `branch_prefix` today; unknown → error).
  - Normalizes the value; rejects empty.
  - **Line-based upsert** into `.worktrees/config.yaml`: if an uncommented
    `branch_prefix:` line exists, replace it; otherwise append
    `branch_prefix: "<value>"`. This preserves the comment template that `init`
    writes (a full YAML re-marshal would discard those hints). The value is
    quoted for safety. This is sound because config keys are flat scalars.
  - If the file/dir is absent, create `.worktrees/` and write a file containing
    just the `branch_prefix:` line. (No cross-package coupling to the `init`
    template constant; the full commented template remains `wt init`'s job.)

### 2. `pkg/worktree/interfaces.go`

- Add `BranchPrefix() string` to the `ConfigProvider` interface.

### 3. `cmd/wt/root.go`

- Add `func (a cfgAdapter) BranchPrefix() string { return a.c.BranchPrefix }`.

### 4. `pkg/worktree/manager.go`

- `resolveNames`: replace the hardcoded literal with the configured prefix:
  ```go
  prefix := m.cfg.BranchPrefix()
  branch = prefix + strings.TrimPrefix(base, prefix)
  ```
  (Works even if `prefix == ""`, yielding `base` unchanged.)
- `resolveWorktree`: build `wantBranch` from the configured prefix:
  ```go
  prefix := m.cfg.BranchPrefix()
  wantBranch := "refs/heads/" + prefix + strings.TrimPrefix(name, prefix)
  ```
  Directory-basename matching remains the primary match, so worktrees created
  under an older prefix still resolve by their on-disk name.

### 5. `internal/naming/naming.go`

- Change `SanitizeDir(name string)` → `SanitizeDir(name, prefix string)`, trimming
  the supplied prefix instead of the literal `"wt/"`. Both call sites in
  `manager.go` pass `m.cfg.BranchPrefix()`. This keeps directory names
  prefix-free (e.g. `2026-…-foo`, not `feature-2026-…-foo`).

### 6. `cmd/wt/set.go` (new)

A cobra subcommand mirroring the existing command style:

```go
Use:   "set <key> <value>"
Short: "Set a configuration value (e.g. branch_prefix)"
Args:  cobra.ExactArgs(2)
```

- Flag: `--safe` (`BoolVar`, kebab-case, matching `--force`/`--json`).
- `RunE`:
  1. Resolve repo root (reuse the `git.New()` → `EnsureMinVersion` → `MainRoot`
     pattern from `buildManager`).
  2. If `--safe`: load `config.LoadFile`, compare `NormalizePrefix(existing)` to
     `NormalizePrefix(value)`. If existing is non-empty and differs → error.
     Equal or unset → proceed.
  3. Call `config.Set(repoRoot, key, value)`.
  4. Print confirmation: `Set branch_prefix = "feature/"`.
- Registered with `rootCmd.AddCommand(setCmd)` in `init()`.

### 7. Docs

- `cmd/wt/init.go` config template: add a `# branch_prefix: "wt/"` hint line.
- `README.md` + `docs/FUNCTIONAL-REQUIREMENTS.md`: document the new key, the
  `WT_BRANCH_PREFIX` env var, the `wt set` command, and `--safe`.

## Error handling

| Situation | Behavior |
| --- | --- |
| Unknown key (`wt set foo bar`) | Error: `unknown config key "foo" (supported: branch_prefix)`. |
| Empty value (`wt set branch_prefix ""`) | Error: `branch_prefix cannot be empty`. |
| `--safe` with a different existing value | Error: `branch_prefix already set to "x/"; refusing to overwrite with "y/" (--safe)`. |
| `--safe` with an equal value | Success, no-op. |
| Not a git repo | Existing `ErrNotARepo` path. |

## Data flow

```
wt set branch_prefix feature --safe
  └─ resolve repoRoot
  └─ LoadFile(repoRoot).BranchPrefix  ─(--safe compare, normalized)→ ok?
  └─ config.Set(repoRoot, "branch_prefix", "feature")
        └─ normalize → "feature/"
        └─ upsert into .worktrees/config.yaml (comments preserved)

wt new
  └─ config.Load(repoRoot): defaults("wt/") ◁ file ◁ WT_BRANCH_PREFIX, normalized
  └─ Manager.resolveNames: branch = prefix + name
  └─ Manager.worktreePath → naming.SanitizeDir(branch, prefix)
```

## Testing

- **`internal/config`**: `NormalizePrefix` cases; `LoadFile` (present/absent);
  `Load` precedence with `WT_BRANCH_PREFIX` set/unset; `Resolve` honors
  `BranchPrefix`; `Set` round-trip (write→read), idempotency, updating an
  existing line, comment preservation, empty/unknown-key rejection.
- **`pkg/worktree`**: extend `fakeConfig` with a `branchPrefix` field +
  `BranchPrefix()` method (default `"wt/"` in existing tests). New tests:
  custom prefix in `resolveNames`; default unchanged; `resolveWorktree` matches
  under a custom prefix.
- **`internal/naming`**: `SanitizeDir` with explicit prefix (strip + slash
  replacement); update existing `TestSanitizeDir_StripsPrefixAndSlashes`.
- **`cmd/wt`**: integration coverage for `wt set` — writes value, `--safe`
  errors on difference, `--safe` no-ops on equal, unknown key errors, empty
  errors; end-to-end `WT_BRANCH_PREFIX` honored by `wt new`.

## Risks

- **Line-based YAML upsert** is intentionally not a full parser. It is safe for
  this flat scalar config; if the config schema ever gains nested structures,
  revisit with a `yaml.Node` round-trip.
- **Changing an existing prefix** does not rename already-created branches or
  directories. Old worktrees still resolve via directory-basename matching;
  branch-name matching uses the current prefix. This is acceptable and noted in
  docs.
