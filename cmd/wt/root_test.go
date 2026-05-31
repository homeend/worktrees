package cmd

import "testing"

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
