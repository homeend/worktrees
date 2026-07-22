# wt shell integration (bash/zsh): make Enter in the `wt` TUI cd your shell
# into the selected worktree.
#
# Install: source this file from ~/.bashrc or ~/.zshrc:
#   source /path/to/shell/wt.sh
# The real wt binary must be on PATH (it is resolved with `command wt`).
#
# How it works: a child process can never change its parent shell's cwd, so
# the function passes --cd-file to wt; when you press Enter on a worktree, wt
# writes the chosen path there and this function cd's after wt exits.
wt() {
  local tmp dir code
  tmp="$(mktemp "${TMPDIR:-/tmp}/wt-cd.XXXXXX")" || { command wt "$@"; return $?; }
  command wt --cd-file "$tmp" "$@"
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
