package worktree

import (
	"path"
	"runtime"
	"strings"
)

// Path comparisons in the manager mix two sources: paths emitted by git
// (git-for-Windows always uses forward slashes and an arbitrary drive-letter
// case, e.g. "T:/others/repo.worktrees/xxxx") and paths built with
// path/filepath (backslashes on Windows). Comparing them byte-for-byte fails
// on Windows, so every comparison goes through a normalized form: cleaned,
// forward slashes, lowercased on Windows (the filesystem is case-insensitive
// there). On POSIX a backslash is a legal filename character and is left
// untouched. The *OS variants take windows explicitly so the Windows
// semantics stay unit-testable on any host.

var onWindows = runtime.GOOS == "windows"

// normPathOS returns the canonical comparison form of p.
func normPathOS(p string, windows bool) string {
	if windows {
		p = strings.ToLower(strings.ReplaceAll(p, `\`, `/`))
	}
	return path.Clean(p)
}

// pathsEqualOS reports whether a and b denote the same path.
func pathsEqualOS(a, b string, windows bool) bool {
	return normPathOS(a, windows) == normPathOS(b, windows)
}

// hasPathPrefixOS reports whether p is root itself or lies below it. The
// prefix is separator-bounded, so siblings sharing a leading string (e.g.
// "feat" vs "feat-extra") never match.
func hasPathPrefixOS(p, root string, windows bool) bool {
	np, nr := normPathOS(p, windows), normPathOS(root, windows)
	return np == nr || strings.HasPrefix(np, nr+"/")
}

// relUnderOS returns p's normalized slash-separated path below root, or ""
// when p is root itself or lies outside it.
func relUnderOS(p, root string, windows bool) string {
	np, nr := normPathOS(p, windows), normPathOS(root, windows)
	if !strings.HasPrefix(np, nr+"/") {
		return ""
	}
	return np[len(nr)+1:]
}

// pathsEqual, hasPathPrefix and relUnder are the host-OS forms used by the
// manager.
func pathsEqual(a, b string) bool       { return pathsEqualOS(a, b, onWindows) }
func hasPathPrefix(p, root string) bool { return hasPathPrefixOS(p, root, onWindows) }
func relUnder(p, root string) string    { return relUnderOS(p, root, onWindows) }
