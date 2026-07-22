package cmd

import (
	"fmt"
	"os"
)

// cdFileFlag is where the TUI's Enter-selection is written for shell cd
// wrappers. Registered as a persistent flag so wrappers can pass it on every
// invocation; only the bare TUI command ever writes it.
var cdFileFlag string

// emitSelection reports the worktree picked in the TUI: the path is printed so
// the user can cd by hand, and written to --cd-file when given so a shell
// wrapper can cd for them (a child process cannot change its parent's cwd).
func emitSelection(sel string) error {
	if sel == "" {
		return nil
	}
	fmt.Println(sel)
	return writeCdFile(cdFileFlag, sel)
}

// escapeDeadCwd transports the user's shell to the repo root when the
// directory the command started in no longer exists (it was removed by this
// very command, e.g. rm/kill-em-all run from inside a worktree). Without the
// shell wrapper this just prints the root path.
func escapeDeadCwd(cwd, repoRoot string, rootErr error) error {
	if rootErr != nil {
		return nil
	}
	if _, err := os.Stat(cwd); err == nil {
		return nil
	}
	return emitSelection(repoRoot)
}

// writeCdFile writes dir into file for the shell wrapper to consume; an empty
// file name is a no-op.
func writeCdFile(file, dir string) error {
	if file == "" {
		return nil
	}
	if err := os.WriteFile(file, []byte(dir+"\n"), 0o644); err != nil {
		return fmt.Errorf("write --cd-file: %w", err)
	}
	return nil
}
