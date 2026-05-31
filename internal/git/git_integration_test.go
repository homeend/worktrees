//go:build integration

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestRunner_VersionAndRun(t *testing.T) {
	r := New()
	repo := newTestRepo(t)
	out, err := r.Run(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if got := string(out); got != "main\n" {
		t.Errorf("want branch main, got %q", got)
	}
	v, err := r.Version()
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}
	if v.Major < 2 {
		t.Errorf("unexpected git major version: %+v", v)
	}
}

func TestResolve_MainRootFromWorktree(t *testing.T) {
	r := New()
	repo := newTestRepo(t)
	wtPath := repo + ".wt-test"
	if _, err := r.Run(repo, "worktree", "add", wtPath); err != nil {
		t.Fatalf("setup worktree add: %v", err)
	}
	t.Cleanup(func() { _, _ = r.Run(repo, "worktree", "remove", "--force", wtPath) })
	got, err := r.MainRoot(wtPath)
	if err != nil {
		t.Fatalf("MainRoot: %v", err)
	}
	want, _ := r.TopLevel(repo)
	if got != want {
		t.Errorf("MainRoot from worktree = %q, want main repo root %q", got, want)
	}
}

func TestResolve_VerifyRefAndCheckRefFormat(t *testing.T) {
	r := New()
	repo := newTestRepo(t)
	if err := r.VerifyRef(repo, "HEAD"); err != nil {
		t.Errorf("HEAD should verify: %v", err)
	}
	if err := r.VerifyRef(repo, "no-such-ref"); err == nil {
		t.Error("bogus ref should not verify")
	}
	if err := r.CheckRefFormat("wt/2026-05-31_10-00-snowy-beach-4821"); err != nil {
		t.Errorf("valid ref rejected: %v", err)
	}
	if err := r.CheckRefFormat("bad..ref"); err == nil {
		t.Error("invalid ref accepted")
	}
}

func TestWorktree_AddListRemovePrune(t *testing.T) {
	r := New()
	repo := newTestRepo(t)
	wtPath := filepath.Join(t.TempDir(), "feature")
	if err := r.AddWorktree(repo, wtPath, "wt/feature", "HEAD"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	list, err := r.ListWorktrees(repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	found := false
	for _, w := range list {
		if w.Path == wtPath && w.Branch == "refs/heads/wt/feature" {
			found = true
		}
	}
	if !found {
		t.Errorf("new worktree not in list: %+v", list)
	}
	if err := r.RemoveWorktree(repo, wtPath, false); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if err := r.Prune(repo); err != nil {
		t.Fatalf("Prune: %v", err)
	}
}
