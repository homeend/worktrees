package cmd

import (
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
