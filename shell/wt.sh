# wt shell integration (bash/zsh): make Enter in the `wt` TUI cd your shell
# into the selected worktree.
#
# Install: source this file from ~/.bashrc or ~/.zshrc:
#   source /path/to/bin/wt.sh    # build.sh copies it next to the wt binary
# then use plain `wt`. Invoking the binary by path (e.g. ./bin/wt) bypasses
# the function and cannot cd your shell.
#
# The real binary is resolved once at source time: a `wt` binary sitting next
# to this file wins (the materialized bin/ layout), PATH otherwise.
#
# How it works: a child process can never change its parent shell's cwd, so
# the function passes --cd-file to wt; when you press Enter on a worktree, wt
# writes the chosen path there and this function cd's after wt exits.

if [ -n "${BASH_SOURCE:-}" ]; then
  _WT_SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
elif [ -n "${ZSH_VERSION:-}" ]; then
  _WT_SRC_DIR="$(cd "$(dirname "${(%):-%N}")" >/dev/null 2>&1 && pwd)"
else
  _WT_SRC_DIR=""
fi
if [ -n "$_WT_SRC_DIR" ] && [ -x "$_WT_SRC_DIR/wt" ]; then
  _WT_BIN="$_WT_SRC_DIR/wt"
else
  _WT_BIN="wt"
fi
unset _WT_SRC_DIR

wt() {
  local tmp dir code
  tmp="$(mktemp "${TMPDIR:-/tmp}/wt-cd.XXXXXX")" || { command "$_WT_BIN" "$@"; return $?; }
  command "$_WT_BIN" --cd-file "$tmp" "$@"
  code=$?
  if [ -s "$tmp" ]; then
    IFS= read -r dir <"$tmp"
    if [ -n "$dir" ] && [ -d "$dir" ]; then
      cd "$dir" || code=$?
    fi
  fi
  rm -f "$tmp"
  return $code
}
