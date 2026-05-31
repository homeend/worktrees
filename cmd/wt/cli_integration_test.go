//go:build integration

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func newRepoForCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(a ...string) {
		c := exec.Command("git", a...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", a, err, out)
		}
	}
	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644)
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func TestNewCommand_CreatesWorktree(t *testing.T) {
	repo := newRepoForCLI(t)
	m, err := buildManager(repo)
	if err != nil {
		t.Fatalf("buildManager: %v", err)
	}
	res, err := m.Add(repo, worktreeAddOptions("feat", "", "", true))
	if err != nil {
		t.Fatalf("Add via manager: %v", err)
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Errorf("worktree not created: %v", err)
	}
}
