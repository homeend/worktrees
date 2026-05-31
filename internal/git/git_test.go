package git

import "testing"

func TestParsePorcelainZ_BasicAndDetachedAndBare(t *testing.T) {
	input := "worktree /repo\x00HEAD abc123\x00branch refs/heads/main\x00\x00" +
		"worktree /repo.worktrees/feat\x00HEAD def456\x00detached\x00\x00" +
		"worktree /bare\x00bare\x00\x00"
	got, err := parsePorcelainZ([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 worktrees, got %d", len(got))
	}
	if got[0].Path != "/repo" || got[0].HEAD != "abc123" || got[0].Branch != "refs/heads/main" {
		t.Errorf("record 0 wrong: %+v", got[0])
	}
	if got[1].Branch != "" || !got[1].Detached {
		t.Errorf("record 1 should be detached with no branch: %+v", got[1])
	}
	if !got[2].Bare {
		t.Errorf("record 2 should be bare: %+v", got[2])
	}
}

func TestParsePorcelainZ_LockedAndPrunableWithReasons(t *testing.T) {
	input := "worktree /a\x00HEAD a1\x00branch refs/heads/x\x00locked needs disk\x00\x00" +
		"worktree /b\x00HEAD b1\x00branch refs/heads/y\x00prunable gitdir gone\x00\x00"
	got, err := parsePorcelainZ([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got[0].Locked || got[0].LockedReason != "needs disk" {
		t.Errorf("record 0 lock wrong: %+v", got[0])
	}
	if !got[1].Prunable || got[1].PrunableReason != "gitdir gone" {
		t.Errorf("record 1 prunable wrong: %+v", got[1])
	}
}

func TestParsePorcelainZ_PathWithSpaces(t *testing.T) {
	input := "worktree /home/me/my repo.worktrees/cool feature\x00HEAD a1\x00branch refs/heads/z\x00\x00"
	got, err := parsePorcelainZ([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Path != "/home/me/my repo.worktrees/cool feature" {
		t.Errorf("path with spaces mangled: %q", got[0].Path)
	}
}

func TestParsePorcelainZ_Empty(t *testing.T) {
	got, err := parsePorcelainZ([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 worktrees, got %d", len(got))
	}
}
