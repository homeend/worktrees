package cmd

import (
	"testing"

	"github.com/homeend/worktrees/internal/config"
)

func TestRunSet_Writes(t *testing.T) {
	dir := t.TempDir()
	if err := runSet(dir, "base_ref", "develop", false); err != nil {
		t.Fatalf("runSet error: %v", err)
	}
	cfg, err := config.LoadRepoFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseRef != "develop" {
		t.Errorf("BaseRef = %q, want develop", cfg.BaseRef)
	}
}

func TestRunSet_SafeBehavior(t *testing.T) {
	dir := t.TempDir()
	if err := runSet(dir, "base_ref", "old", false); err != nil {
		t.Fatal(err)
	}
	if err := runSet(dir, "base_ref", "new", true); err == nil {
		t.Error("expected --safe to error on a different existing value")
	}
	if err := runSet(dir, "base_ref", "old", true); err != nil {
		t.Errorf("--safe with equal value should succeed, got %v", err)
	}
}

func TestRunSet_UnknownKey(t *testing.T) {
	if err := runSet(t.TempDir(), "branch_prefix", "x", false); err == nil {
		t.Error("expected error for unknown key (branch_prefix was removed)")
	}
}
