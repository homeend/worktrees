package cmd

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors mapped to stable process exit codes.
var (
	ErrNotARepo       = errors.New("not a git repository")
	ErrNameCollision  = errors.New("name collision")
	ErrHookFailed     = errors.New("hook failed")
	ErrDirtyWorktree  = errors.New("worktree has uncommitted changes")
	ErrPartialCleanup = errors.New("cleanup completed with failures")
)

// classify inspects an error's message for known failure signatures and wraps it
// with the matching sentinel so exitCodeFor can map it to a stable exit code.
// pkg/worktree returns descriptive wrapped errors (not sentinels), so the CLI
// boundary translates them here.
func classify(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "hook failed"), strings.Contains(msg, "hook exited"):
		return fmt.Errorf("%w: %v", ErrHookFailed, err)
	case strings.Contains(msg, "uncommitted") || strings.Contains(msg, "contains modified or untracked"):
		return fmt.Errorf("%w: %v", ErrDirtyWorktree, err)
	case strings.Contains(msg, "already exists"):
		return fmt.Errorf("%w: %v", ErrNameCollision, err)
	}
	return err
}

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
	case errors.Is(err, ErrPartialCleanup):
		return 6
	default:
		return 1
	}
}
