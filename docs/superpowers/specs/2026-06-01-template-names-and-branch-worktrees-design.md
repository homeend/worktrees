# Named templates + worktrees from existing branches — design

**Date:** 2026-06-01
**Status:** Approved (pending spec review)
**Depends on:** [configurable branch prefix](./2026-06-01-configurable-branch-prefix-design.md)
(templates render *into* the configured prefix)

## Problem

Three related gaps in `wt new`:

1. **No reusable branch-naming templates.** Teams repeatedly type structured
   branch names (e.g. `autofix/<ticket>`). They want named, predefined templates
   selected at creation time with variables filled in.
2. **No way to create a worktree from an existing branch.** `wt new` always cuts
   a *new* branch. Users want to check out an *existing* branch into a fresh
   worktree — with lifecycle hooks running so the workspace is initialized.
3. **No way to see defined templates** from either surface.

## Goals

1. `wt new -t <ref> k:v …` — render a named template (by name or 1-based number)
   with CLI-supplied variables, prepend the configured prefix, create the worktree.
2. `wt new --from-branch <branch>` (and TUI `b`) — create a worktree that checks
   out an existing branch, running hooks.
3. `wt templates` (and TUI `t`) — list defined templates.

## Non-goals (YAGNI)

- No template **creation/editing** via CLI (edit `config.yaml`, or a future
  `wt set`-style command).
- No remote-branch handling for `--from-branch` (local branches only).
- No template variable input *in the TUI* (creating-from-template stays CLI-only;
  the TUI `t` view is read-only).
- No built-in fields (`{{.Date}}` etc.) inside templates — user variables only.

## Decisions (confirmed with user)

| Decision | Choice |
| --- | --- |
| Template definition | `templates:` — a list of `{name, template}`. |
| Template reference | by **name** (`autofix`) or **1-based number** (`1`). |
| Variable syntax | Go `text/template` (`{{.ticketName}}`), `missingkey=error`. |
| Variable passing | positional `name:value` pairs after `-t`. |
| Template flag | `-t` / `--template <ref>` (cobra-idiomatic; the literal `-tm`/`--template-name:1` forms aren't valid cobra). |
| From-branch | CLI `wt new --from-branch <branch>`; TUI a **separate `b` key** (`n` stays instant). |
| Listing | `wt templates` command + TUI `t` key. |

## Reference resolution rule

`-t <ref>`: if `<ref>` parses as a positive integer, it is a **1-based index**
into the `templates` list; otherwise it is matched against template **names**.
(Consequence: a template literally named `"2"` is only reachable by index —
documented, acceptable.)

## Architecture & components

Everything flows through the existing layering
(`cmd → pkg/worktree → internal/{config,git,naming,tui}`).

### 1. `internal/config`

```go
type Template struct {
	Name     string `yaml:"name"`
	Template string `yaml:"template"`
}
// Config gains:
Templates []Template `yaml:"templates"`
```

`Templates` is a slice — `Resolve(lo, hi)` overrides it only when `hi.Templates`
is non-nil (file/env layering keeps the same "non-empty wins" spirit). Templates
are **not** part of the `WT_BRANCH_PREFIX`/`Set` write path.

### 2. `pkg/worktree` — types & interface

To avoid a `pkg/worktree → internal/config` import cycle, define a local type
and expose it through `ConfigProvider`:

```go
// types.go
type Template struct {
	Name     string
	Template string
}

// AddOptions gains:
FromBranch string // when set: check out this existing branch instead of cutting a new one

// interfaces.go — ConfigProvider gains:
Templates() []Template
```

`cfgAdapter` (cmd/wt) maps `[]config.Template` → `[]worktree.Template`.

### 3. `internal/naming` — template rendering

```go
// RenderTemplate renders a user template against string variables, erroring on
// any referenced-but-missing variable (missingkey=error). Mirrors GenerateFrom.
func RenderTemplate(tmpl string, vars map[string]string) (string, error)
```

### 4. `pkg/worktree.Manager` — template resolution & listing

```go
// Templates returns the configured templates (for `wt templates` / TUI).
func (m *Manager) Templates() []Template { return m.cfg.Templates() }

// ResolveTemplate finds a template by name or 1-based number and renders it with
// vars. The rendered string becomes AddOptions.Name (the prefix is applied by
// the normal Add flow). Unknown ref or missing variable is an error.
func (m *Manager) ResolveTemplate(ref string, vars map[string]string) (string, error)
```

### 5. `pkg/worktree.Manager.Add` — from-branch path

`Add` gains a localized branch for `opts.FromBranch != ""`. The shared hook
transaction (repo-root resolve, container/target paths, `HookContext`,
pre/post-create hooks, result) is **unchanged and reused** — only three points
differ:

| Step | New-branch (today) | From-branch (new) |
| --- | --- | --- |
| name/branch | `resolveNames` (prefix applied) | branch = `opts.FromBranch` verbatim; name = `SanitizeDir(branch, prefix)` |
| validation | `CheckRefFormat` + reject if branch **exists** + verify base ref | require branch **exists** (error if not) |
| git add | `AddWorktree(…, -b branch, base)` | `AddWorktreeExisting(…, branch)` |

Template selection and `FromBranch` are **mutually exclusive** (enforced at the
CLI; a template just produces a `Name`, so the Manager sees only one of the two).

### 6. `internal/git`

```go
// AddWorktreeExisting checks out an existing branch into a new worktree:
//   git worktree add <path> <branch>   (no -b)
func (r *Runner) AddWorktreeExisting(dir, path, branch string) error
```

Add to the `GitRunner` interface, `gitAdapter`, and both git fakes
(`fakes_test.go`, `manager_integration_test.go`).

### 7. `cmd/wt/new.go`

- Flags: `-t, --template <ref>` and `--from-branch <branch>` (no short — `-b` is
  taken by `--branch`).
- `Args` becomes `cobra.ArbitraryArgs`; validation in `RunE`:
  - `--template` **and** `--from-branch` together → error.
  - `--template` set → positionals are `name:value` pairs (parsed via a
    `parseVars` helper using `SplitN(s, ":", 2)`; empty key or no colon → error);
    the rendered template is the `Name`.
  - `--from-branch` set → no positional name; `FromBranch` is passed through.
  - neither → at most one positional `name` (current behavior).

### 8. `cmd/wt/templates.go` (new)

`wt templates` — `managerForWorkdir` → `m.Templates()` → tab-aligned
`index  name  template`; prints "no templates defined" when empty.

### 9. `internal/tui`

- `tui.Run` fetches `m.Templates()` once and passes them into `newModel`
  (new field `templates []Template`; the `lister` interface is **not** widened).
- New modes: `modeInputBranch` (the TUI's first text entry) and `modeTemplates`
  (read-only list).
- `b` → `modeInputBranch`: a rune buffer; `KeyRunes`/space append, backspace
  deletes, Enter dispatches `runAction("new", "--from-branch", buf, "--repo", dir)`
  (empty buffer cancels), Esc cancels.
- `t` → `modeTemplates`: render the templates list; any key / Esc returns.
- `view.go` hint line gains `b from-branch • t templates`.

## Data flow

```
wt new -t autofix ticketName:ZX-12
  └─ parseVars(["ticketName:ZX-12"]) -> {ticketName: ZX-12}
  └─ m.ResolveTemplate("autofix", vars) -> "autofix/ZX-12"      (name)
  └─ Add{Name:"autofix/ZX-12"} -> branch = prefix+name = "mrutkowski/autofix/ZX-12"
                                -> dir = SanitizeDir -> "autofix-ZX-12"  + hooks

wt new --from-branch feature/login   (TUI: b -> "feature/login")
  └─ Add{FromBranch:"feature/login"}
       require branch exists -> AddWorktreeExisting(path, branch) -> hooks
       dir = SanitizeDir("feature/login", prefix) -> "feature-login"

wt templates   (TUI: t)  ->  index name template
```

## Error handling

| Situation | Behavior |
| --- | --- |
| Unknown template ref (bad name / out-of-range index) | error: list valid refs |
| Missing template variable | error from `missingkey=error` |
| Malformed `k:v` (no colon / empty key) | error with the offending token |
| `--template` + `--from-branch` together | error (mutually exclusive) |
| `--from-branch` branch not found locally | error before any worktree/hook |
| rendered branch invalid as a git ref | existing `CheckRefFormat` error |

## Testing

- **config:** `templates` parses into `[]Template`; `Resolve` overrides on non-nil.
- **naming:** `RenderTemplate` happy path; missing-var error; malformed template error.
- **worktree:** `ResolveTemplate` by name and by 1-based number; unknown ref;
  `Add` from-branch path (fake git `AddWorktreeExisting` called, hooks run, dir
  derived, missing branch errors); template + from-branch never both reach Add.
- **cmd:** `parseVars` (pairs, bad token); `new -t` end-to-end (rendered name →
  prefixed branch); `new --from-branch`; `--template`+`--from-branch` rejected;
  `wt templates` output and empty case.
- **tui:** `b` enters `modeInputBranch`, typing + Enter dispatches the right
  `--from-branch` args, Esc cancels; `t` enters `modeTemplates` and renders.

## Risks

- **TUI text input is a new capability** — kept minimal (single-line rune
  buffer, no editing beyond backspace). If richer input is needed later,
  consider `bubbles/textinput`.
- **Rendered template → directory name** sanitizes slashes but not arbitrary
  characters; an exotic variable value could still fail `CheckRefFormat` (caught)
  or yield an odd dir name. Acceptable; values are user-supplied.
- **`Add` gains a second path.** Mitigated by branching only at three points and
  reusing the single hook transaction, keeping the function cohesive.
