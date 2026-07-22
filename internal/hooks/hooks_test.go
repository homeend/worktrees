package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/homeend/worktrees/pkg/worktree"
)

func writeHook(t *testing.T, dir, name, body string, exec bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mode := os.FileMode(0o644)
	if exec {
		mode = 0o755
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestRun_AbsentHookIsNoop(t *testing.T) {
	repo := t.TempDir()
	r := New(repo)
	err := r.Run(worktree.HookContext{Event: worktree.PreCreate, Cwd: repo})
	if err != nil {
		t.Errorf("absent hook should be a no-op, got %v", err)
	}
}

func TestRun_ExportsEnvAndRuns(t *testing.T) {
	repo := t.TempDir()
	target := t.TempDir()
	writeHook(t, filepath.Join(repo, ".wt"), "post-create",
		"#!/usr/bin/env bash\necho \"$WT_TARGET_ROOT\" > \"$WT_TARGET_ROOT/marker\"\n", true)

	r := New(repo)
	err := r.Run(worktree.HookContext{
		Event:      worktree.PostCreate,
		SourceRoot: repo,
		TargetRoot: target,
		Name:       "feat",
		Branch:     "wt/feat",
		Cwd:        target,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "marker"))
	if err != nil {
		t.Fatalf("marker not written: %v", err)
	}
	if string(got) != target+"\n" {
		t.Errorf("WT_TARGET_ROOT env wrong: %q", got)
	}
}

func TestRun_ExportsAllEnvVars(t *testing.T) {
	repo := t.TempDir()
	target := t.TempDir()
	// The hook dumps every WT_* var to a marker file so we can assert each
	// field-to-variable mapping (catches a typo in env() that the single-var
	// test would miss).
	writeHook(t, filepath.Join(repo, ".wt"), "post-create",
		"#!/usr/bin/env bash\n{\n"+
			"echo \"$WT_SOURCE_ROOT\"\n"+
			"echo \"$WT_TARGET_ROOT\"\n"+
			"echo \"$WT_NAME\"\n"+
			"echo \"$WT_BRANCH\"\n"+
			"echo \"$WT_BASE_REF\"\n"+
			"echo \"$WT_CONTAINER\"\n"+
			"echo \"$WT_REPO_NAME\"\n"+
			"echo \"$WT_HOOK\"\n"+
			"} > \"$WT_TARGET_ROOT/env.txt\"\n", true)

	r := New(repo)
	err := r.Run(worktree.HookContext{
		Event:      worktree.PostCreate,
		SourceRoot: repo,
		TargetRoot: target,
		Name:       "feat",
		Branch:     "wt/feat",
		BaseRef:    "HEAD",
		Container:  "/cont",
		RepoName:   "myrepo",
		Cwd:        target,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "env.txt"))
	if err != nil {
		t.Fatalf("env.txt not written: %v", err)
	}
	want := repo + "\n" + target + "\nfeat\nwt/feat\nHEAD\n/cont\nmyrepo\npost-create\n"
	if string(got) != want {
		t.Errorf("env vars wrong:\n got %q\nwant %q", got, want)
	}
}

func TestRun_NonExecutableIsSkipped(t *testing.T) {
	repo := t.TempDir()
	writeHook(t, filepath.Join(repo, ".wt"), "pre-create",
		"#!/usr/bin/env bash\nexit 7\n", false)
	r := New(repo)
	if err := r.Run(worktree.HookContext{Event: worktree.PreCreate, Cwd: repo}); err != nil {
		t.Errorf("non-executable hook should be skipped, got %v", err)
	}
}

func TestRun_FailingHookReturnsError(t *testing.T) {
	repo := t.TempDir()
	writeHook(t, filepath.Join(repo, ".wt"), "pre-create",
		"#!/usr/bin/env bash\nexit 3\n", true)
	r := New(repo)
	err := r.Run(worktree.HookContext{Event: worktree.PreCreate, Cwd: repo})
	if err == nil {
		t.Fatal("failing hook must return an error")
	}
}

func TestRun_AllFourEventsRouteByFilename(t *testing.T) {
	repo := t.TempDir()
	target := t.TempDir()
	for _, ev := range []worktree.HookEvent{
		worktree.PreCreate, worktree.PostCreate, worktree.PreRemove, worktree.PostRemove,
	} {
		// Each hook writes a marker named after the event into target.
		writeHook(t, filepath.Join(repo, ".wt"), string(ev),
			"#!/usr/bin/env bash\ntouch \"$WT_TARGET_ROOT/$WT_HOOK\"\n", true)
		r := New(repo)
		if err := r.Run(worktree.HookContext{
			Event: ev, TargetRoot: target, Cwd: target,
		}); err != nil {
			t.Fatalf("event %s: %v", ev, err)
		}
		if _, err := os.Stat(filepath.Join(target, string(ev))); err != nil {
			t.Errorf("event %s did not run (no marker): %v", ev, err)
		}
	}
}
