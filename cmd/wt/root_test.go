package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/code-drill/wt/pkg/worktree"
)

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, 0},
		{ErrNotARepo, 2},
		{ErrNameCollision, 3},
		{ErrHookFailed, 4},
		{ErrDirtyWorktree, 5},
	}
	for _, c := range cases {
		if got := exitCodeFor(c.err); got != c.want {
			t.Errorf("exitCodeFor(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

func TestClassify_MapsKnownSignatures(t *testing.T) {
	hook := classify(errInjectedErr("post-create hook failed (worktree left in place): boom"))
	if exitCodeFor(hook) != 4 {
		t.Errorf("hook failure should map to exit 4, got %d", exitCodeFor(hook))
	}
	dirty := classify(errInjectedErr("git worktree remove: fatal: contains modified or untracked files"))
	if exitCodeFor(dirty) != 5 {
		t.Errorf("dirty worktree should map to exit 5, got %d", exitCodeFor(dirty))
	}
	plain := classify(errInjectedErr("some other error"))
	if exitCodeFor(plain) != 1 {
		t.Errorf("unknown error should map to exit 1, got %d", exitCodeFor(plain))
	}
	collision := classify(errInjectedErr("branch \"wt/feat\" already exists; pass a different --branch"))
	if exitCodeFor(collision) != 3 {
		t.Errorf("name collision should map to exit 3, got %d", exitCodeFor(collision))
	}
	if classify(nil) != nil {
		t.Error("classify(nil) must be nil")
	}
}

func errInjectedErr(s string) error { return fmt.Errorf("%s", s) }

func TestListJSON_Marshals(t *testing.T) {
	items := []worktree.WorktreeInfo{
		{Path: "/r", Branch: "refs/heads/main", IsMain: true},
		{Path: "/r.worktrees/feat", Branch: "refs/heads/wt/feat"},
	}
	out, err := renderListJSON(items)
	if err != nil {
		t.Fatalf("renderListJSON: %v", err)
	}
	if !strings.Contains(out, "\"branch\": \"refs/heads/wt/feat\"") {
		t.Errorf("json missing branch: %s", out)
	}
}

func TestInitScaffold_CreatesFilesIdempotently(t *testing.T) {
	repo := t.TempDir()
	if err := scaffold(repo); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	wt := filepath.Join(repo, ".worktrees")
	for _, f := range []string{"config.yaml", "pre-create", "post-create", "pre-remove", "post-remove"} {
		if _, err := os.Stat(filepath.Join(wt, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	info, _ := os.Stat(filepath.Join(wt, "pre-create"))
	if info.Mode()&0o111 == 0 {
		t.Error("hook stub should be executable")
	}
	custom := filepath.Join(wt, "config.yaml")
	os.WriteFile(custom, []byte("base_ref: develop\n"), 0o644)
	if err := scaffold(repo); err != nil {
		t.Fatalf("second scaffold: %v", err)
	}
	got, _ := os.ReadFile(custom)
	if string(got) != "base_ref: develop\n" {
		t.Errorf("scaffold clobbered existing config: %q", got)
	}
}

func TestShouldLaunchTUI_RespectsTTY(t *testing.T) {
	if shouldLaunchTUI(false) {
		t.Error("must not launch TUI when stdout is not a TTY")
	}
	if !shouldLaunchTUI(true) {
		t.Error("should launch TUI when stdout is a TTY")
	}
}
