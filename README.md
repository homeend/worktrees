# wt — fast git worktrees

`wt` creates, lists, and removes git worktrees in a sibling container, with
lifecycle hooks for copying gitignored files (like `.env`) into new worktrees.

## Install

	go install github.com/code-drill/wt@latest

## Usage

	wt new [name]        # create a worktree (generated name if omitted)
	wt list | wt ls      # list worktrees (--json for machine output)
	wt rm <name>         # remove a worktree and its branch
	wt path <name>       # print a worktree's path
	wt prune             # clear stale worktree state
	wt init              # scaffold .worktrees/ (config + hook stubs)
	wt                   # interactive TUI (in a terminal)

A global `--repo`/`-r` flag points any command at a different repository
(default: the current directory).

Worktrees are placed at `<repo>.worktrees/<name>/`. Branches are prefixed `wt/`.
Generated names are date-first (`YYYY-MM-DD_HH-mm-<adjective>-<noun>-NNNN`) so
they sort chronologically and stale branches are easy to spot.

### Shell `cd` helper

`wt` cannot change your shell's directory; add this function to your shell rc:

	wtcd() { cd "$(wt path "$1")"; }

## Hooks

`wt init` creates `.worktrees/` with executable stubs: `pre-create`,
`post-create`, `pre-remove`, `post-remove`. Each receives `WT_*` environment
variables:

| Variable | Meaning |
|---|---|
| `WT_SOURCE_ROOT` | the main repo root |
| `WT_TARGET_ROOT` | the worktree root |
| `WT_NAME` | worktree name (no `wt/` prefix) |
| `WT_BRANCH` | branch name (incl. `wt/` prefix) |
| `WT_BASE_REF` | base ref the branch was cut from |
| `WT_CONTAINER` | the container directory |
| `WT_REPO_NAME` | repo basename |
| `WT_HOOK` | which hook is running |

Example `post-create`:

	cp "$WT_SOURCE_ROOT/.env" "$WT_TARGET_ROOT/.env"

Hooks are arbitrary executables from the repo — review them before use, or pass
`--no-hooks`. Hook failures are strict: a `pre-create` failure aborts before
anything is created; a `post-create` failure leaves the worktree in place (no
rollback) so you can inspect it.

## Config

`.worktrees/config.yaml` (CLI flags override these values):

	base_ref: HEAD          # default ref new branches are cut from
	container: ""           # override container path; used verbatim
	name_template: ""       # override the default generated name pattern

## Removing worktrees

`wt rm <name>` removes the worktree and safely deletes its branch
(`git branch -d`). If the branch is unmerged, the worktree is still removed and
the branch is **kept**, with a message telling you how to force-delete it
(`wt rm <name> --force-branch`). Use `--keep-branch` to always retain the branch,
or `--force` to remove a worktree with uncommitted changes.

## Requirements

- Go 1.23+ (to build)
- git 2.30+ on `PATH`

## Development

	go test ./...                      # unit tests
	go test -tags integration ./...    # integration tests (real git in temp repos)
	go build ./...

The architecture is layered: `pkg/worktree` holds the reusable `Manager` and the
`GitRunner`/`HookRunner`/`ConfigProvider` interfaces; `internal/git`,
`internal/config`, `internal/hooks`, and `internal/naming` provide the default
implementations; `cmd/wt` wires them together for the CLI and `internal/tui`
renders the interactive view.
