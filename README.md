# wt — fast git worktrees

`wt` creates, lists, and removes git worktrees in a sibling container, with
lifecycle hooks for copying gitignored files (like `.env`) into new worktrees.

Each worktree's directory **mirrors its full branch** under
`<repo>.worktrees/` (slashes become nested subdirectories), so branch
`wt/2026-…-foo` lives at `<repo>.worktrees/wt/2026-…-foo`. Branches are prefixed
`wt/` by default. Generated names are date-first
(`YYYY-MM-DD_HH-mm-<adjective>-<noun>-NNNN`) so they sort chronologically and
stale branches are easy to spot.

## Requirements

- **Go 1.25+** (to build/install)
- **git 2.30+** on your `PATH`

---

## Install

`wt` is installed with the Go toolchain. The installed binary lands in your Go
bin directory — `$(go env GOPATH)/bin` (typically `~/go/bin`), or `$GOBIN` if set.

**Make sure that directory is on your `PATH`** so you can run `wt` from anywhere:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"   # add to ~/.bashrc or ~/.zshrc
```

### Option A — install from this repository (works today)

This repo is not published to a module proxy yet, so install it from the local
checkout:

```sh
cd ~/worktrees     # the directory containing go.mod
go install .
```

That builds and installs the `wt` binary to `~/go/bin/wt`. Verify:

```sh
wt --help
```

### Option B — install via module path (once published)

After this module is pushed to its remote (`github.com/homeend/worktrees`), anyone
can install it without cloning:

```sh
go install github.com/homeend/worktrees@latest
```

Until then, use Option A.

### Last resort — run straight from the folder (no install)

If you don't want to install a binary, run it via `go run` from inside the repo.
Note: `go run` operates on **its own working directory**, so point `wt` at your
target repo with `--repo`:

```sh
cd ~/worktrees
go run . --repo /path/to/your/repo list
go run . --repo /path/to/your/repo new my-feature
```

---

## Quick start

```sh
# 1. Install (see above)
cd ~/worktrees && go install .

# 2. Go to any git repository you want worktrees for
cd ~/projects/myrepo

# 3. (optional) scaffold hooks + config in this repo
wt init                      # creates ./.worktrees/ with config + hook stubs

# 4. Create a worktree (auto-generated name)
wt new
# -> Created worktree "2026-05-31_14-30-eager-canyon-4821"
#      branch: wt/2026-05-31_14-30-eager-canyon-4821
#      path:   ~/projects/myrepo.worktrees/2026-05-31_14-30-eager-canyon-4821

# ...or give it a name:
wt new my-feature           # branch wt/my-feature, dir myrepo.worktrees/my-feature

# 5. See what you have
wt list                     # table; add --json for machine output

# 6. Jump into one (see the cd helper below)
cd "$(wt path my-feature)"

# 7. When done, remove it (and its branch, safely)
wt rm my-feature
```

### Shell `cd` helper

A program can't change its parent shell's directory, so `wt` prints the path and
you `cd` to it. Add this function to your `~/.bashrc` / `~/.zshrc`:

```sh
wtcd() { cd "$(wt path "$1")"; }
```

Then `wtcd my-feature` jumps straight into the worktree.

---

## Interactive TUI

Run `wt` with no arguments in a terminal to open the interactive list:

```sh
wt
```

Keys:

| Key       | Action                                              |
|-----------|-----------------------------------------------------|
| `↑`/`↓` (or `k`/`j`) | move the cursor                          |
| `n`       | create a new worktree (auto-generated name)         |
| `b`       | create a worktree from an existing branch (type the branch name, Enter) |
| `t`       | view configured branch templates                    |
| `d`       | delete the selected worktree (asks `y`/`n`/`f` to confirm — `f` **force**-deletes, discarding uncommitted changes and removing an unmerged branch; the main worktree is refused) |
| `K`       | **kill-em-all** — remove every worktree and prefixed branch (asks `y`/`n`; hooks skipped) |
| `q` / `Ctrl+C` | quit                                           |

`n` and `d` run the same `wt new` / `wt rm` underneath, so **hooks run and their
output is shown live** while the action runs. When the action finishes, the
output stays on screen with a `Press Enter to return to the list…` prompt — so
you can read it — and pressing Enter returns to the (refreshed) list. Each
action's combined output is also tee'd to a temporary `wt-action-*.log`: the
path is printed at the top of the action's output (`wt: logging this action
to …`), repeated when it completes, and shown in the status line afterward —
`done — log: …` on success, `action failed: … — see …` on failure — so you can
always find it.

> The TUI only opens when stdout is a real terminal; piped/non-interactive
> invocation prints help instead.

---

## Commands

```
wt new [name]        Create a worktree (generated name if omitted)
wt new -t <ref> k:v  Create from a named/numbered template (var:value pairs)
wt new --from-branch Create a worktree from an existing local branch
wt list | wt ls      List worktrees (--json for machine output)
wt rm <name>         Remove a worktree and its branch
wt path <name>       Print a worktree's absolute path
wt prune             Clear stale worktree state (git worktree prune)
wt set <key> <val>   Set a config value (e.g. branch_prefix); --safe
wt templates         List configured branch templates
wt kill-em-all       Remove ALL worktrees + prefixed branches (--yes)
wt init              Scaffold .worktrees/ (config + hook stubs)
wt completion <sh>   Generate shell completion (bash|zsh|fish|powershell)
wt                   Interactive TUI (when run in a terminal)
```

Common flags:

- `-r, --repo <path>` — operate on another repository (default: current dir).
  Works with every command.
- `wt new`: `-b/--branch <name>` (branch name; default derived from the name),
  `--base <ref>` (ref to branch from; default config `base_ref` / `HEAD`),
  `-t/--template <ref>` (render the branch from a template — see below),
  `--from-branch <branch>` (check out an existing local branch), `--no-hooks`,
  `--no-prefix` (don't prepend the configured prefix), `--branch-prefix <value>`
  (override the prefix for this run; `--no-prefix` wins).
  `--template`, `--from-branch`, and `--branch` are mutually exclusive.
- `wt rm`: `--force` (remove a worktree with uncommitted changes),
  `-D/--force-branch` (force-delete an unmerged branch),
  `--keep-branch` (keep the branch), `--no-hooks`.

### Removing worktrees

`wt rm <name>` removes the worktree and **safely** deletes its branch
(`git branch -d`). If the branch is unmerged, the worktree is still removed and
the branch is **kept**, with a message telling you how to force-delete it:

```
Removed worktree "my-feature" (.../myrepo.worktrees/my-feature)
Kept branch wt/my-feature (unmerged). Delete with: wt rm my-feature --force-branch, or git branch -D wt/my-feature
```

### Removing everything — `wt kill-em-all`

`wt kill-em-all` is a destructive clean-slate cleanup: it **force-removes every
worktree** in the repo's container and **force-deletes every branch matching the
configured prefix** (default `wt/`), *including orphan branches with no
worktree*. Removal is forced regardless of committed/uncommitted state. The main
worktree and any non-prefixed branch (e.g. `main`) are never touched.

- **Lifecycle hooks are skipped** (a notice is printed).
- Without `--yes`, it prints what will be removed and asks `y/N` when stdout is a
  terminal; with no terminal it refuses and tells you to pass `--yes`.
- It is **best-effort**: a failure on one item is reported in the summary and
  does not stop the rest. If anything failed, the process exits with code `6`.
- Also available in the TUI via the `K` key.

```sh
wt kill-em-all          # prompts for confirmation
wt kill-em-all --yes    # no prompt (for scripts)
```

### Creating from a template

Define reusable branch templates in `.worktrees/config.yaml` and select one at
`new` time by **name or 1-based number**, filling variables with `name:value`
pairs. Templates are Go `text/template` strings (`{{.var}}`); a missing variable
is an error. The rendered string is appended to the configured branch prefix.

```yaml
# .worktrees/config.yaml
templates:
  - name: autofix
    template: "autofix/{{.ticketName}}"
  - name: feature
    template: "feat/{{.ticketName}}"
```

```sh
wt templates                              # list them (index, name, template)
wt new -t autofix ticketName:ZXXXX-12121  # branch <prefix>autofix/ZXXXX-12121
wt new -t 1 ticketName:ZXXXX-12121        # same template, by number
```

With prefix `mrutkowski/`, that yields branch `mrutkowski/autofix/ZXXXX-12121`
and worktree dir `<repo>.worktrees/mrutkowski/autofix/ZXXXX-12121` (the directory
mirrors the full branch). Use `--no-prefix` to skip the prefix, or
`--branch-prefix <value>` to override it for one run.

### Creating from an existing branch

`wt new --from-branch <branch>` (or the TUI `b` key) checks out an existing
**local** branch into a new worktree and runs the lifecycle hooks, so the
workspace is initialized just like a fresh `wt new`. It does not create a new
branch; if the branch doesn't exist locally, it errors.

```sh
wt new --from-branch feature/login
```

---

## Hooks

Hooks let you run scripts around worktree creation/removal — e.g. copy
gitignored files (`.env`, local certs) into a fresh worktree, or tear down a dev
container before removing one.

### Setup

```sh
cd ~/projects/myrepo
wt init
```

This creates a `.worktrees/` directory (commit it to your repo) containing a
`config.yaml` and four executable hook stubs:

```
.worktrees/
├── config.yaml
├── pre-create     # runs in the SOURCE repo, before the worktree is created
├── post-create    # runs in the NEW worktree, after it is created
├── pre-remove     # runs in the worktree being removed, before removal
└── post-remove    # runs in the SOURCE repo, after removal
```

A hook runs only if it **exists and is executable** (`chmod +x`). Any
interpreter works via the shebang line. Skip all hooks for one command with
`--no-hooks`.

### When each hook runs and its working directory

| Hook          | Runs                              | Working directory          |
|---------------|-----------------------------------|----------------------------|
| `pre-create`  | before `git worktree add`         | source repo root           |
| `post-create` | after the worktree is created     | the new worktree root      |
| `pre-remove`  | before the worktree is removed    | the worktree being removed |
| `post-remove` | after the worktree is removed     | source repo root           |

**Failure policy (strict, no rollback):**
- A `pre-create` failure aborts — nothing is created.
- A `post-create` failure returns an error but **leaves the worktree in place**
  so you can inspect it.
- A `pre-remove` failure aborts the removal.

### Environment variables

Every hook receives these variables:

| Variable          | Meaning                                          |
|-------------------|--------------------------------------------------|
| `WT_SOURCE_ROOT`  | the main repository root                         |
| `WT_TARGET_ROOT`  | the worktree's root directory                    |
| `WT_NAME`         | worktree name (no `wt/` prefix)                  |
| `WT_BRANCH`       | branch name (includes the `wt/` prefix)          |
| `WT_BASE_REF`     | the ref the branch was cut from                  |
| `WT_CONTAINER`    | the container directory (`<repo>.worktrees`)     |
| `WT_REPO_NAME`    | the repository's basename                        |
| `WT_HOOK`         | which hook is running (`pre-create`, …)           |

### Example hooks

**`post-create` — copy gitignored env files into the new worktree:**

```bash
#!/usr/bin/env bash
set -euo pipefail
# Copy local-only files that aren't committed but are needed to run the project.
cp "$WT_SOURCE_ROOT/.env"            "$WT_TARGET_ROOT/.env"            2>/dev/null || true
cp "$WT_SOURCE_ROOT/.env.local"      "$WT_TARGET_ROOT/.env.local"      2>/dev/null || true
cp -r "$WT_SOURCE_ROOT/certs"        "$WT_TARGET_ROOT/certs"           2>/dev/null || true
echo "Seeded $WT_NAME from $WT_REPO_NAME"
```

**`pre-create` — mint a token in the source repo before the worktree exists:**

```bash
#!/usr/bin/env bash
set -euo pipefail
# Generate a per-worktree secret that post-create can copy across.
openssl rand -hex 16 > "$WT_SOURCE_ROOT/.worktree-token"
```

**`pre-remove` — stop a dev container tied to the worktree:**

```bash
#!/usr/bin/env bash
set -euo pipefail
if [ -f "$WT_TARGET_ROOT/docker-compose.yml" ]; then
  docker compose -f "$WT_TARGET_ROOT/docker-compose.yml" down 2>/dev/null || true
fi
```

**One script branching on the event** (via `WT_HOOK`):

```bash
#!/usr/bin/env bash
case "$WT_HOOK" in
  post-create) cp "$WT_SOURCE_ROOT/.env" "$WT_TARGET_ROOT/.env" ;;
  pre-remove)  echo "tearing down $WT_NAME" ;;
esac
```

> **Security note:** hooks are arbitrary executables stored in the repo. Review
> them before running `wt` against a repository you don't trust, or pass
> `--no-hooks`.

---

## Config

`.worktrees/config.yaml` (CLI flags override these values):

```yaml
base_ref: HEAD          # default ref new branches are cut from
container: ""           # override the container path; used verbatim
name_template: ""       # Go text/template for generated names
branch_prefix: "wt/"    # prefix for branches created by `wt new`
```

### Branch prefix

Branches created by `wt new` are prefixed (default `wt/`). The prefix is
resolved in this order (highest wins):

1. the `WT_BRANCH_PREFIX` environment variable,
2. `branch_prefix` in `.worktrees/config.yaml`,
3. the built-in default `wt/`.

A trailing `/` is appended automatically, so `feature` and `feature/` are
equivalent. Set it without hand-editing the file using `wt set`:

```sh
wt set branch_prefix feature          # branches become feature/<name>
wt set branch_prefix feature --safe   # error if a *different* value is already set
WT_BRANCH_PREFIX=hotfix wt new        # one-off override via env
```

`--safe` is a no-op when the existing value already equals the new one; it only
errors when a different value is already persisted.

`name_template` is a Go `text/template` with these fields:
`{{.Date}}` `{{.Adjective}}` `{{.Noun}}` `{{.Digits}}`. For example:

```yaml
name_template: "{{.Date}}-{{.Adjective}}-{{.Noun}}-{{.Digits}}"   # the default
# name_template: "{{.Adjective}}-{{.Noun}}"                        # short names
```

An invalid template (unknown field or syntax error) makes `wt new` fail with a
clear message instead of producing a bad name.

---

## Maintenance — update after the code changes

When you change `wt`'s source (or pull new commits), reinstall the binary:

```sh
cd ~/worktrees
git pull                 # if you track an upstream
go install .             # rebuild + replace ~/go/bin/wt
wt --help                # confirm the new build is in use
```

Once the module is published, updating an installed copy is just:

```sh
go install github.com/homeend/worktrees@latest
```

To remove the installed binary:

```sh
rm "$(go env GOPATH)/bin/wt"
```

---

## Development

```sh
go test ./...                      # unit tests
go test -tags integration ./...    # integration tests (real git in temp repos)
go build ./...                     # compile everything
go vet ./...                       # static checks
```

The architecture is layered: `pkg/worktree` holds the reusable `Manager` and the
`GitRunner`/`HookRunner`/`ConfigProvider` interfaces; `internal/git`,
`internal/config`, `internal/hooks`, and `internal/naming` provide the default
implementations; `cmd/wt` wires them together for the CLI and `internal/tui`
renders the interactive view.

## Documentation

- **[Functional requirements](docs/FUNCTIONAL-REQUIREMENTS.md)** — the source of
  truth for *what `wt` does and how it behaves* (feature-by-feature, with stable
  `FR-…` IDs). Update it whenever behavior changes.
- [Design spec](docs/superpowers/specs/2026-05-31-wt-git-worktree-utility-design.md)
  — the original design decisions and architecture (HOW it's built).
- [Implementation plan](docs/superpowers/plans/2026-05-31-wt-git-worktree-utility.md)
  — the phased build plan.
