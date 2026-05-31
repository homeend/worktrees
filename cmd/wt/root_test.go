package cmd

import (
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
