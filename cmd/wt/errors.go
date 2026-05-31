package cmd

import "errors"

// Sentinel errors mapped to stable process exit codes.
var (
	ErrNotARepo      = errors.New("not a git repository")
	ErrNameCollision = errors.New("name collision")
	ErrHookFailed    = errors.New("hook failed")
	ErrDirtyWorktree = errors.New("worktree has uncommitted changes")
)

// exitCodeFor maps an error to a process exit code. Unknown non-nil errors -> 1.
func exitCodeFor(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrNotARepo):
		return 2
	case errors.Is(err, ErrNameCollision):
		return 3
	case errors.Is(err, ErrHookFailed):
		return 4
	case errors.Is(err, ErrDirtyWorktree):
		return 5
	default:
		return 1
	}
}
