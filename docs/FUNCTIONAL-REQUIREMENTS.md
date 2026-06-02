# `wt` — Functional Requirements

**Status:** Living document — the source of truth for *what `wt` does and how it
behaves*. Update it whenever behavior changes. It describes observable behavior
(WHAT/HOW-it-behaves), not implementation (for design/architecture see
`docs/superpowers/specs/2026-05-31-wt-git-worktree-utility-design.md`).

Each requirement has a stable ID (`FR-…`) so future work can reference it.
"Required" = intended behavior; a change to it is a deliberate product decision,
not an incidental refactor.

Last verified against the codebase: 2026-06-01.

---

## 1. Purpose & scope

- **FR-1.1** `wt` creates, lists, and removes **git worktrees** for an existing
  repository, with lifecycle **hooks** so project-specific setup/teardown (e.g.
  copying gitignored files like `.env`) runs automatically.
- **FR-1.2** `wt` exposes the same capabilities through two surfaces over one
  shared core: a **CLI** (scriptable) and an interactive **TUI**.
- **FR-1.3** `wt` operates on the git repository it is pointed at; it never
  requires a network/remote and performs no network I/O of its own.

## 2. Environment & preconditions

- **FR-2.1** `wt` requires the `git` CLI on `PATH`, **git ≥ 2.30**. On an older
  or missing git, a command fails fast with a clear message (exit code 1).
- **FR-2.2** `wt` operates inside a git repository. When the target directory is
  not a git repo, the command fails with a "not a git repository" error
  (exit code 2). (`completion` and `--help` are exempt — they need no repo.)
- **FR-2.3** Building/installing `wt` requires **Go ≥ 1.25**. (The module's
  source-compatibility floor is Go 1.23; the toolchain in use is newer.)

## 3. Repository, container & layout

- **FR-3.1 (main-repo anchoring).** All paths anchor to the **main** working
  tree, even when `wt` is invoked from inside a linked worktree. Running `wt`
  from a worktree must not nest containers.
- **FR-3.2 (container & layout).** New worktrees live in a **sibling container**
  of the main repo root named `<repo>.worktrees/`. Each worktree's directory
  **mirrors its full branch ref**: `<repo>.worktrees/<branch>/`, with the
  branch's `/` becoming nested subdirectories and the prefix included (e.g.
  branch `mrutkowski/autofix/X` → `<repo>.worktrees/mrutkowski/autofix/X`). The
  default-prefix case nests too (branch `wt/<gen>` → `<repo>.worktrees/wt/<gen>`).
- **FR-3.3 (container override).** If `container` is configured (see §9), that
  path is used **verbatim** (absolute, or relative to the repo root) — the
  `<repo>.worktrees` default is not applied on top of it.

## 4. Naming & branching

- **FR-4.1 (generated names).** When no name is supplied, `wt` generates a
  **date-first** name: `YYYY-MM-DD_HH-mm-<adjective>-<noun>-NNNN` (NNNN is
  random, zero-padded). Date-first ordering makes worktrees sort chronologically
  and stale ones easy to spot.
- **FR-4.2 (branch prefix).** Every branch `wt` creates is prefixed with the
  **configured branch prefix** (default `wt/`; see FR-9.5). This applies to
  generated **and** user-supplied names/branches. Re-supplying an
  already-prefixed value must not double-prefix it.
- **FR-4.3 (worktree directory path).** The on-disk path mirrors the **full
  branch ref** under the container — slashes become nested subdirectories and
  the prefix is included (e.g. branch `wt/feature/foo` →
  `<container>/wt/feature/foo`). Git ref validity (FR-4.4) guarantees the path
  is filesystem-safe.
- **FR-4.4 (branch validity).** The final branch name is validated as a legal
  git ref before use; an invalid name fails the command.
- **FR-4.5 (base ref).** A new branch is cut from the configured `base_ref`
  (default `HEAD`) unless overridden per command. The base ref is verified to
  exist **before** any hook runs or worktree is created.
- **FR-4.6 (custom name template).** If `name_template` is configured (see §9),
  generated names are rendered from it (Go `text/template`, fields `{{.Date}}`,
  `{{.Adjective}}`, `{{.Noun}}`, `{{.Digits}}`). An invalid template fails the
  command with a clear message rather than producing a malformed name.
- **FR-4.7 (named templates).** `wt new -t <ref>` selects a configured template
  (see §9) by **name** or **1-based number**, rendering it (Go `text/template`)
  against `name:value` variables given as positional args; a referenced-but-
  missing variable, an unknown ref, or a malformed `name:value` token is an
  error. The rendered string is used as the worktree name, so the configured
  branch prefix is prepended (e.g. template `autofix/{{.ticketName}}` + prefix
  `mrutkowski/` + `ticketName:ZX-12` → branch `mrutkowski/autofix/ZX-12`).
  `--template`, `--from-branch`, and `--branch` are mutually exclusive.
- **FR-4.8 (from existing branch).** `wt new --from-branch <branch>` checks out
  an existing **local** branch into a new worktree instead of cutting a new one
  (error if the branch does not exist locally; remote-only is not resolved). No
  new branch is created; lifecycle hooks run as for any `new`. The directory
  path mirrors the branch (FR-4.3).
- **FR-4.9 (per-run prefix controls).** `wt new --no-prefix` creates the branch
  without the configured prefix; `wt new --branch-prefix <value>` overrides the
  prefix for that run (normalized with a trailing `/`; empty disables). If both
  are given, `--no-prefix` wins. They do not apply to `--from-branch` (verbatim).

## 5. CLI commands

Global: **FR-5.0** A persistent `-r, --repo <path>` flag points any command at a
different repository; default is the current directory. It is honored by every
command (`new`, `list`, `rm`, `prune`, `path`, `init`).

- **FR-5.1 `wt new [name]`** — create a worktree.
  - Flags: `-b/--branch <name>` (branch name; default derived from the name),
    `--base <ref>` (default config `base_ref` / `HEAD`), `--no-hooks`.
  - Omitting `name` generates one (FR-4.1/FR-4.6).
  - On success prints the created name, branch, and path.
- **FR-5.2 `wt list` / `wt ls`** — list worktrees (see §7 for scope).
  - Default output is a table with a `(main)` marker on the main worktree.
  - `--json` emits machine-readable output (path, branch, head, is_main).
- **FR-5.3 `wt rm <name>`** — remove a worktree and (safely) its branch (§6).
  - Flags: `--force` (remove a worktree with uncommitted changes),
    `-D/--force-branch` (force-delete an unmerged branch),
    `--keep-branch` (keep the branch), `--no-hooks`.
- **FR-5.4 `wt path <name>`** — print a worktree's absolute path (one line),
  suitable for a shell `cd` helper. `wt` cannot change the parent shell's
  directory itself.
- **FR-5.5 `wt prune`** — clear stale worktree administrative state
  (`git worktree prune`) for the resolved repo.
- **FR-5.6 `wt init`** — scaffold the `.worktrees/` convention dir (§8.7/§9).
- **FR-5.7 `wt completion [bash|zsh|fish|powershell]`** — emit a shell
  completion script.
- **FR-5.8 `wt` (no subcommand)** — launch the TUI when stdout is a TTY;
  otherwise print help. It must never hang on non-interactive invocation, and
  must never launch the TUI for an unknown subcommand (that is an error, exit 1).
- **FR-5.9 `wt set <key> <value>`** — persist a config value to
  `.worktrees/config.yaml` (§9.4). Today the only valid key is `branch_prefix`;
  an unknown key or empty value is an error. `--safe` refuses to overwrite an
  existing **different** value (an equal value is a no-op success).
- **FR-5.10 `wt kill-em-all`** — destructive bulk cleanup (§6.6).
- **FR-5.11 `wt templates`** — list configured templates (1-based index, name,
  template string); prints "no templates defined" when none are configured.

## 6. Removal semantics

- **FR-6.1 (resolution).** `<name>` resolves against the actual worktree list,
  matching by branch (with or without the configured prefix), by the
  container-relative directory path, or by the leaf directory name. The full
  branch ref is the unambiguous identifier; not-found is an error.
- **FR-6.2 (refuse main).** The main worktree can never be removed.
- **FR-6.3 (dirty worktree).** Removal refuses a worktree with uncommitted
  changes unless `--force`; the message explains how to override (exit code 5).
- **FR-6.4 (safe branch delete).** By default the branch is deleted **safely**
  (`git branch -d`). If the branch is **unmerged**, safe-delete is refused — the
  worktree is **still removed**, the branch is **kept**, and `wt` reports the
  kept branch and how to force-delete it (`--force-branch` / `git branch -D`).
  This is non-fatal.
- **FR-6.5 (force / keep).** `--force-branch` (`-D`) force-deletes an unmerged
  branch; `--keep-branch` never deletes the branch.
- **FR-6.7 (prune empty parents).** After a worktree is removed, now-empty parent
  directories created by the mirrored layout (FR-3.2) are pruned up to — but not
  including — the container. A still-populated parent stops the prune.
- **FR-6.6 (kill-em-all — bulk cleanup).** `wt kill-em-all` force-removes **every
  non-main worktree** in the container **and** force-deletes **every branch
  matching the configured prefix** (incl. orphan branches with no worktree).
  - The main worktree and non-prefixed branches are **never** touched.
  - Removal is forced regardless of committed/uncommitted or merged state.
  - **Lifecycle hooks are skipped**, and a notice saying so is printed.
  - Without `--yes`: when stdout is a TTY it prints the plan and asks `y/N`;
    when **not** a TTY it refuses with an error (instructing `--yes`).
  - It is **best-effort** — a per-item failure is collected and does not stop
    the rest. A summary is printed; if any item failed, the exit code is `6`
    (§12). A stale-state `git worktree prune` runs at the end.
  - Available in the TUI via the `K` key (with its own `y/n` confirmation).

## 7. Listing scope

- **FR-7.1** `list` (and the TUI list) show the **main** worktree plus worktrees
  that live **inside this repo's container**. Linked worktrees git knows about
  but that live elsewhere are omitted — `wt` only manages its container.
- **FR-7.2** The main worktree is flagged distinctly (`is_main` / `(main)`).

## 8. Hooks

- **FR-8.1 (convention dir).** Hooks live in `.worktrees/<event>` in the source
  repo and are committed with the project.
- **FR-8.2 (events, cwd).** Four events, each run with a defined working dir:

  | Event         | When                              | Working directory          |
  |---------------|-----------------------------------|----------------------------|
  | `pre-create`  | before the worktree is created    | source repo root           |
  | `post-create` | after the worktree is created     | the new worktree root      |
  | `pre-remove`  | before the worktree is removed    | the worktree being removed |
  | `post-remove` | after the worktree is removed     | source repo root           |

- **FR-8.3 (enablement).** A hook runs only if its file **exists and is
  executable**. An absent or non-executable hook is a silent no-op. The
  interpreter is chosen by the script's shebang (POSIX).
- **FR-8.4 (environment).** Every hook receives: `WT_SOURCE_ROOT`,
  `WT_TARGET_ROOT`, `WT_NAME` (no `wt/` prefix), `WT_BRANCH` (with prefix),
  `WT_BASE_REF`, `WT_CONTAINER`, `WT_REPO_NAME`, `WT_HOOK` (the event name).
- **FR-8.5 (failure policy — strict, no rollback).**
  - `pre-create` failure → abort; nothing is created.
  - `post-create` failure → error, but the worktree is **left in place** (no
    rollback) so it can be inspected.
  - `pre-remove` failure → abort the removal.
  - `post-remove` failure → error (the worktree is already removed).
- **FR-8.6 (output).** Hook stdout/stderr stream live to the terminal.
- **FR-8.7 (bypass).** `--no-hooks` skips all hooks for a command.
- **FR-8.8 (trust).** Hooks are arbitrary executables from the repo; this is a
  documented supply-chain consideration. `--no-hooks` is the escape hatch.

## 9. Configuration

- **FR-9.1 (location & precedence).** Config lives at
  `.worktrees/config.yaml`. Effective values are **CLI flags > config file >
  built-in defaults**. A missing config file is not an error. (`branch_prefix`
  additionally has an environment layer — see FR-9.5.)
- **FR-9.2 (keys).** `base_ref` (default `HEAD`), `container` (default empty →
  sibling container; used verbatim when set), `name_template` (default empty →
  built-in date-first pattern), `branch_prefix` (default `wt/`; see FR-9.5),
  `templates` (a list of `{name, template}`; see FR-4.7).
- **FR-9.4 (`wt set`).** `wt set <key> <value>` persists a key to the config
  file via a comment-preserving upsert (creating `.worktrees/config.yaml` if
  absent). Only `branch_prefix` is valid today. `--safe` errors when a
  **different** value already exists; an equal value is a no-op success.
- **FR-9.5 (branch prefix resolution).** The branch prefix is resolved
  highest-wins from: `WT_BRANCH_PREFIX` env var, then `branch_prefix` in the
  config file, then the built-in default `wt/`. A trailing `/` is appended
  automatically (so `feature` and `feature/` are equivalent); an empty value is
  treated as "use the default".
- **FR-9.3 (`wt init` scaffolding).** Creates `.worktrees/` with a commented
  `config.yaml` and four **executable** hook stubs (`pre-create`,
  `post-create`, `pre-remove`, `post-remove`), each a valid no-op
  (`#!/usr/bin/env bash` … `exit 0`). `init` is **idempotent** — it never
  clobbers an existing file.

## 10. Interactive TUI

- **FR-10.1 (launch).** Opening `wt` with no args on a TTY shows a list of
  worktrees (scope per §7), rendered in the terminal's alternate screen so the
  original screen/prompt is restored cleanly on exit.
- **FR-10.2 (navigation).** `↑`/`↓` (or `k`/`j`) move the selection; `q` or
  `Ctrl+C` quits.
- **FR-10.3 (create — `n`).** Pressing `n` immediately creates a worktree with a
  generated name (no text entry).
- **FR-10.4 (delete — `d`).** Pressing `d` on a worktree shows an inline
  `Delete <name>? (y/n/f)` confirmation; `y` removes it (safe), `f` **force**-
  removes it (runs `wt rm --force --force-branch`: discards uncommitted changes
  and force-deletes the branch even if unmerged), `n`/`Esc` cancels. The main
  worktree is refused (with a status message, no prompt).
- **FR-10.4a (kill-em-all — `K`).** Pressing `K` shows an inline confirmation
  (counting the worktrees and noting hooks are skipped); `y` runs
  `wt kill-em-all --yes`, `n`/`Esc` cancels (§6.6).
- **FR-10.4b (from-branch — `b`).** Pressing `b` opens a single-line text input;
  typing a branch name and pressing Enter runs `wt new --from-branch <name>`
  (empty input or `Esc` cancels). This is the TUI's text-entry mode (§4.8).
- **FR-10.4c (templates — `t`).** Pressing `t` shows a read-only list of
  configured templates; any key returns to the list (§4.7). Creating *from* a
  template is CLI-only.
- **FR-10.5 (action execution).** Create/delete in the TUI perform the **same**
  operations as `wt new` / `wt rm`, including running hooks. The TUI hands the
  terminal over for the action so hook output and any interactive prompts
  display **live**.
- **FR-10.6 (output visibility — required).** After a TUI action finishes, its
  output **remains on screen** with a `Press Enter to return to the list…`
  prompt; the list reloads only after the user acknowledges. Output must not
  flash by and disappear.
- **FR-10.7 (post-action result line).** When an action completes, `wt` prints a
  result line on the terminal — `wt: done.` on success or
  `wt: action failed: <err>` on failure — followed by the log path.
- **FR-10.8 (status line).** After returning to the list, the status line shows
  the outcome and the log path: `done — log: <path>` on success, or
  `action failed: <err> — see <path>` on failure.
- **FR-10.9 (auto-refresh).** The worktree list refreshes automatically after a
  create/delete completes.

## 11. Action logging (TUI)

- **FR-11.1** Every TUI create/delete tees its combined stdout/stderr to a
  temporary log file named `wt-action-*.log`.
- **FR-11.2** The log path is surfaced in three places: a banner at the **start**
  of the action's output (`wt: logging this action to <path>`), the **result
  line** when it completes (FR-10.7), and the **status line** afterward
  (FR-10.8). The location is never hidden.
- **FR-11.3** The log file persists after the action so it can be inspected
  later (notably on failure).

## 12. Exit codes (CLI)

- **FR-12.1** Stable process exit codes: `0` success; `1` generic/unknown
  failure (incl. unusable/too-old git); `2` not a git repository; `3` name
  collision (branch already exists); `4` hook failed; `5` dirty worktree;
  `6` kill-em-all completed with one or more failures (§6.6).

## 13. Quality requirements

- **FR-13.1** No shell-string interpolation: git and hook invocations use
  argument vectors, not constructed shell command strings.
- **FR-13.2** git command output is parsed from stable machine formats
  (porcelain `-z`) under a fixed locale (`LC_ALL=C`); success/failure is decided
  by exit status, not by scraping human-readable text.
- **FR-13.3** Every package ships unit **and** integration tests; integration
  tests run against real throwaway git repos behind a build tag.

## 14. Non-goals (current) / future directions

Not implemented today; recorded so scope stays clear:

- **FR-14.1** No per-worktree status/lifecycle metadata (e.g. "ready for
  merge") and no `wt done`/`wt ready`.
- **FR-14.2** No merge/PR integration.
- **FR-14.3** TUI is create/delete + list only — no in-TUI rename, branch
  selection, or config editing.
- **FR-14.4** Hooks are POSIX/shebang-based; no Windows-native hook execution.
- **FR-14.5** No remove-time confirmation beyond the TUI `d` prompt; the CLI
  `rm` is non-interactive by design (guarded by `--force`/`--force-branch`).
