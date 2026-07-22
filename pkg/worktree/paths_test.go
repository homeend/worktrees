package worktree

import "testing"

// git-for-Windows emits absolute paths with forward slashes and varying
// drive-letter case (e.g. "T:/others/repo.worktrees/xxxx"), while paths built
// via path/filepath use backslashes on Windows. These tests pin the
// windows-mode comparison semantics on any host OS.

func TestNormPathOSWindows(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`T:\others\test-1`, `t:/others/test-1`},
		{`T:/others/test-1`, `t:/others/test-1`},
		{`t:\Others\Test-1\`, `t:/others/test-1`},
		{`T:/others//test-1/.`, `t:/others/test-1`},
	}
	for _, c := range cases {
		if got := normPathOS(c.in, true); got != c.want {
			t.Errorf("normPathOS(%q, windows) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormPathOSPosixKeepsBackslashes(t *testing.T) {
	// On POSIX a backslash is a legal filename character, not a separator.
	if got := normPathOS(`/a\b`, false); got != `/a\b` {
		t.Errorf("normPathOS(/a\\b, posix) = %q, want /a\\b", got)
	}
}

func TestPathsEqualOS(t *testing.T) {
	cases := []struct {
		a, b    string
		windows bool
		want    bool
	}{
		{`T:/others/test-1`, `T:\others\test-1`, true, true},
		{`t:\x`, `T:/x`, true, true},
		{`T:/others/test-1`, `T:\others\test-2`, true, false},
		{`/a/b`, `/a/b`, false, true},
		{`/a\b`, `/a/b`, false, false},
	}
	for _, c := range cases {
		if got := pathsEqualOS(c.a, c.b, c.windows); got != c.want {
			t.Errorf("pathsEqualOS(%q, %q, windows=%v) = %v, want %v", c.a, c.b, c.windows, got, c.want)
		}
	}
}

func TestHasPathPrefixOS(t *testing.T) {
	cases := []struct {
		p, root string
		windows bool
		want    bool
	}{
		// The reported bug: git path (forward slashes) vs filepath-built
		// container (backslashes).
		{`T:/others/test-1.worktrees/xxxx`, `T:\others\test-1.worktrees`, true, true},
		{`T:/others/test-1.worktrees/wt/deep/leaf`, `T:\others\test-1.worktrees`, true, true},
		{`t:/others/test-1.worktrees/xxxx`, `T:\Others\test-1.worktrees`, true, true},
		{`T:/others/test-1.worktrees`, `T:\others\test-1.worktrees`, true, true},
		// Sibling sharing a leading string is not a path prefix.
		{`T:/others/test-1.worktrees-extra/x`, `T:\others\test-1.worktrees`, true, false},
		{`/a/feat-extra`, `/a/feat`, false, false},
		{`/a/feat/sub`, `/a/feat`, false, true},
		{`/a/feat`, `/a/feat`, false, true},
	}
	for _, c := range cases {
		if got := hasPathPrefixOS(c.p, c.root, c.windows); got != c.want {
			t.Errorf("hasPathPrefixOS(%q, %q, windows=%v) = %v, want %v", c.p, c.root, c.windows, got, c.want)
		}
	}
}

func TestRelUnderOS(t *testing.T) {
	cases := []struct {
		p, root string
		windows bool
		want    string
	}{
		{`T:/others/test-1.worktrees/wt/foo`, `T:\others\test-1.worktrees`, true, `wt/foo`},
		{`T:/others/test-1.worktrees/xxxx`, `T:\others\test-1.worktrees`, true, `xxxx`},
		// Equal to root or outside it: no relative path.
		{`T:/others/test-1.worktrees`, `T:\others\test-1.worktrees`, true, ``},
		{`T:/elsewhere/x`, `T:\others\test-1.worktrees`, true, ``},
		{`/c/repo.worktrees/wt/foo`, `/c/repo.worktrees`, false, `wt/foo`},
		{`/c/other/x`, `/c/repo.worktrees`, false, ``},
	}
	for _, c := range cases {
		if got := relUnderOS(c.p, c.root, c.windows); got != c.want {
			t.Errorf("relUnderOS(%q, %q, windows=%v) = %q, want %q", c.p, c.root, c.windows, got, c.want)
		}
	}
}
