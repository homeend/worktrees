# wt — fast git worktrees

`wt` creates, lists, and removes git worktrees in a sibling container, with
lifecycle hooks for copying gitignored files (like `.env`) into new worktrees.

Each worktree lives in the sibling `<repo>.worktrees/` container, in a
directory that is the branch name sanitized into **one flat segment**
(`/` becomes `-`): branch `fix/GH-42` lives at `<repo>.worktrees/fix-GH-42`.
Branch names are exactly what you type or what a named template renders
(gg-style `<token>` templates) — never generated, never prefixed. Run
`wt new` from inside a worktree and it derives a sibling iteration
(`<branch>-v001`, `-v002`, …) from that worktree's branch.

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
worktrees          # first run: self-installs the wt entry points
```

`go install` names the binary `worktrees`. On its first run under that full
name it bootstraps the `wt` layout next to itself in `~/go/bin`: a copy of
the binary as `wt.bin` (`wt.bin.exe` on Windows) plus the `wt` entry point
(`wt.cmd` on Windows — deliberately no `wt.exe`, which would shadow the
wrapper in cmd's lookup). A later `go install` upgrade is picked up the next
time you run `worktrees`. Verify, then wire up cd-on-Enter:

```sh
wt --help
wt shell-init zsh --install    # bash/zsh; Windows cmd needs nothing extra
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
cd ~/worktrees && go install . && worktrees   # first run self-installs wt

# 2. Go to any git repository you want worktrees for
cd ~/projects/myrepo

# 3. (optional) scaffold hooks + config in this repo
wt init                      # creates .wt.toml + ./.wt/ hook stubs

# 4. Create a worktree — a name is always required (never generated):
wt new my-feature           # branch my-feature, dir myrepo.worktrees/my-feature

# ...or render one from a named template (see Config below):
wt new -t fix ticket=42     # e.g. branch fix/42-001, dir myrepo.worktrees/fix-42-001
wt new -t fix               # missing <user:...> values are asked interactively

# ...or, from INSIDE a worktree, derive a sibling iteration:
wt new                      # <current-branch>-v001 (lowest free number)
wt new attempt2             # <current-branch>-attempt2

# 5. See what you have
wt list                     # table; add --json for machine output

# 6. Jump into one (see the cd helper below)
cd "$(wt path my-feature)"

# 7. When done, remove it (and its branch, safely)
wt rm my-feature
```

### Shell `cd` helper

The full integration is [shell integration — cd on Enter](#shell-integration--cd-on-enter)
(`wt shell-init zsh --install`): plain `wt` then transports your shell on
Enter. If all you want is a quick jump by name, this one-liner also works:

```sh
wtcd() { cd "$(wt path "$1")"; }   # wtcd my-feature
```

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
| `Enter`   | select the worktree and quit — prints its path, and with the [shell wrapper](#shell-integration--cd-on-enter) installed your shell cd's into it |
| `n`       | create a new worktree (asks for the name — names are never generated) |
| `t`       | template picker — press `1`-`9` to create from a template; its `<user:...>` values are prompted one by one |
| `d`       | delete the selected worktree (asks `y`/`n`/`f` to confirm — `f` **force**-deletes, discarding uncommitted changes and removing an unmerged branch; the main worktree is refused) |
| `K`       | **kill-em-all** — remove every container worktree and its branch (asks `y`/`n`; hooks skipped) |
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

### Shell integration — cd on Enter

A child process can never change its parent shell's directory, so the
"transport me there" step needs a tiny wrapper: `wt` is invoked with
`--cd-file <tmpfile>`, writes the path you selected with Enter into it, and
the wrapper cd's after `wt` exits (the same pattern fzf, lazygit and yazi
use).

The build produces `bin/` with the real binaries named `wt.bin` /
`wt.bin.exe` and thin `wt` entry points in front of them (`bin/wt` script,
`bin\wt.cmd` batch), so `wt` is the name you use on both platforms.

**bash / zsh** — let wt install itself (appends one line to your rc file;
idempotent, so rerunning is safe — on a new machine this is the whole setup):

```sh
/path/to/bin/wt shell-init zsh --install    # or: shell-init bash --install
source ~/.zshrc                             # or open a new terminal
```

The line it adds evals the function emitted by the binary itself, bound to
it by absolute path, so no PATH setup is needed:

```sh
# ~/.bashrc or ~/.zshrc (zsh honors ZDOTDIR)
eval "$('/path/to/bin/wt.bin' shell-init zsh)"
```

Then type plain `wt` anywhere: the function opens the TUI and cd's your
shell on Enter. (Invoking `bin/wt` by path still works for everything else,
but an executed script cannot cd your shell — only the function can.)
Alternatively source the equivalent static file `shell/wt.sh` (also copied
to `bin/wt.sh` by the build scripts).

**cmd.exe** — add the `bin\` directory to `PATH`; that's the whole install.
The build deliberately names the Windows binary `wt.bin.exe` instead of
`wt.exe`: within one directory cmd prefers `.exe` over `.cmd`, so a `wt.exe`
would shadow the wrapper. With no `wt.exe` around, typing `wt` resolves to
`bin\wt.cmd`, which launches the `wt.bin.exe` sitting next to it and cd's
after it exits. (Remove any old `wt.exe` copies from `PATH` directories or
they will shadow the wrapper.)

Without a wrapper, Enter still quits the TUI and prints the selected path so
you can cd by hand.

---

## Commands

```
wt new <name>        Create a worktree (branch = name; name is required)
wt new -t <name> k=v Create from a named template; missing <user:> values
                     are asked interactively
wt new [suffix]      Inside a worktree: derive <branch>-vNNN / <branch>-suffix
wt list | wt ls      List worktrees (--json for machine output)
wt rm <name>         Remove a worktree and its branch
wt path <name>       Print a worktree's absolute path
wt prune             Clear stale worktree state (git worktree prune)
wt set <key> <val>   Set a .wt.toml value (base_ref, container); --safe
wt edit              Open .wt.toml in $VISUAL/$EDITOR (--user for user config)
wt templates         List configured branch templates
wt kill-em-all       Remove ALL container worktrees + their branches (--yes)
wt init              Scaffold .wt.toml + .wt/ hook stubs
wt shell-init <sh>   Print/install the cd-on-Enter shell function (--install)
wt completion <sh>   Generate shell completion (bash|zsh|fish|powershell)
wt                   Interactive TUI (when run in a terminal)
```

Common flags:

- `-r, --repo <path>` — operate on another repository (default: current dir).
  Works with every command.
- `wt new`: `-t/--template <name>` (render the branch from a named template;
  remaining args are `var=value` pairs for its `<user:...>` tokens).
- `wt rm`: `--force` (remove a worktree with uncommitted changes),
  `-D/--force-branch` (force-delete an unmerged branch),
  `--keep-branch` (keep the branch), `--no-hooks`.

### Removing worktrees

`wt rm <name>` removes the worktree and **safely** deletes its branch
(`git branch -d`). If the branch is unmerged, the worktree is still removed and
the branch is **kept**, with a message telling you how to force-delete it:

```
Removed worktree "my-feature" (.../myrepo.worktrees/my-feature)
Kept branch my-feature (unmerged). Delete with: wt rm my-feature --force-branch, or git branch -D my-feature
```

### Removing everything — `wt kill-em-all`

`wt kill-em-all` is a destructive clean-slate cleanup: it **force-removes every
worktree** in the repo's container and **force-deletes each one's branch**.
Removal is forced regardless of committed/uncommitted state. The main worktree
and its branch are never touched; branches without a container worktree are
not swept (there is no branch prefix to identify them by).

- **Lifecycle hooks are skipped** (a notice is printed).
- Safe to run from **inside** a worktree: the process moves itself out first
  (a directory that is any process's cwd cannot be deleted on Windows), and
  with the shell wrapper installed your shell is transported to the repo root
  when the directory it stood in was removed.
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

Define **named** branch templates in `.wt.toml` (or the user config) and pick
one with `-t <name>`. Templates use the same `<token>` syntax as gg:

| Token | Meaning |
|-------|---------|
| `<user:LABEL>` | value supplied as `LABEL=value` — or **asked interactively** when missing |
| `<seq:NAME:PAD>` | per-repo counter (state in `.git/wt/state.toml`), zero-padded to PAD; consumed only on a successful create |
| `<date:yyyy-MM-dd>` | current date/time (`yyyy MM dd HH mm ss` tokens; bare `<date>` = `yyyy-MM-dd`) |
| `<repo>` | repository directory name |
| `<parent-branch>` | branch of the worktree wt runs in (empty at the repo root) |
| `<random-alpha:N>` / `<random-num:N>` | N random lowercase letters / digits |

```toml
# .wt.toml
[templates]
fix = "fix/<user:ticket>-<seq:fix:3>"
spike = "spike/<date>-<random-alpha:4>"
```

```sh
wt templates                # list them (name, template)
wt new -t fix ticket=GH-42  # branch fix/GH-42-001
wt new -t fix               # prompts:  ticket: _
```

The worktree directory is the branch sanitized into **one flat segment**
(`/` becomes `-`): branch `fix/GH-42-001` lives at
`<repo>.worktrees/fix-GH-42-001`.

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

This creates a commented `.wt.toml` at the repo root and a `.wt/` directory
(commit both) with four executable hook stubs:

```
.wt.toml           # per-repo config (overlays the user config)
.wt/
├── pre-create     # runs in the SOURCE repo, before the worktree is created
├── post-create    # runs in the NEW worktree, after it is created
├── pre-remove     # runs in the worktree being removed, before removal
└── post-remove    # runs in the SOURCE repo, after removal
```

A hook runs only if it **exists and is executable** (`chmod +x`). Any
interpreter works via the shebang line. `wt rm --no-hooks` skips them for a
removal; `kill-em-all` always skips them.

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
| `WT_NAME`         | the branch name (same as `WT_BRANCH`)            |
| `WT_BRANCH`       | branch name                                      |
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

Configuration is TOML, layered the same way gg does it (highest wins):

1. `<repo>/.wt.toml` — committed per-repo overrides,
2. `<UserConfigDir>/wt/config.toml` — your user-level defaults
   (`~/.config/wt/config.toml` on Linux),
3. built-in defaults (`base_ref = "HEAD"`).

The overlay is field-by-field; a `[templates]` table in a higher layer
replaces the lower one wholesale.

```toml
# .wt.toml (or ~/.config/wt/config.toml)
base_ref = "HEAD"      # ref new branches are cut from (outside derive mode)
container = ""         # override the default sibling <repo>.worktrees dir

[templates]
fix = "fix/<user:ticket>-<seq:fix:3>"
spike = "spike/<date>-<random-alpha:4>"
```

Branches are created with exactly the name you give (or the template
renders) — there is no branch prefix. `<seq:...>` counter state is
machine-local, stored at `<git-common-dir>/wt/state.toml` (shared by all
linked worktrees of a repo, never committed).

Set flat keys without hand-editing the file:

```sh
wt set base_ref develop           # writes into .wt.toml
wt set base_ref develop --safe    # error if a *different* value is already set
```

`--safe` is a no-op when the existing value already equals the new one; it only
errors when a different value is already persisted.

---

## Maintenance — update after the code changes

When you change `wt`'s source (or pull new commits), reinstall the binary:

```sh
cd ~/worktrees
git pull                 # if you track an upstream
go install .             # rebuild + replace ~/go/bin/worktrees
worktrees                # refreshes the wt entry points next to it
wt --help                # confirm the new build is in use
```

Once the module is published, updating an installed copy is just:

```sh
go install github.com/homeend/worktrees@latest && worktrees
```

To remove the installed binaries:

```sh
cd "$(go env GOPATH)/bin" && rm -f worktrees wt wt.bin
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
