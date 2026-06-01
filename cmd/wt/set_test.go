package cmd

import (
	"testing"

	"github.com/code-drill/wt/internal/config"
)

func TestRunSet_Writes(t *testing.T) {
	dir := t.TempDir()
	if err := runSet(dir, "branch_prefix", "feature", false); err != nil {
		t.Fatalf("runSet error: %v", err)
	}
	cfg, err := config.LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "feature/" {
		t.Errorf("BranchPrefix = %q, want %q", cfg.BranchPrefix, "feature/")
	}
}

func TestRunSet_SafeBehavior(t *testing.T) {
	dir := t.TempDir()
	if err := runSet(dir, "branch_prefix", "old", false); err != nil {
		t.Fatal(err)
	}
	if err := runSet(dir, "branch_prefix", "new", true); err == nil {
		t.Error("expected --safe to error on a different existing value")
	}
	if err := runSet(dir, "branch_prefix", "old", true); err != nil {
		t.Errorf("--safe with equal value should succeed, got %v", err)
	}
}

func TestRunSet_UnknownKey(t *testing.T) {
	if err := runSet(t.TempDir(), "nope", "x", false); err == nil {
		t.Error("expected error for unknown key")
	}
}
