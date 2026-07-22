package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCdFile_WritesSelectedPath(t *testing.T) {
	f := filepath.Join(t.TempDir(), "cd")
	if err := writeCdFile(f, "/repo.worktrees/feat"); err != nil {
		t.Fatalf("writeCdFile: %v", err)
	}
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(b) != "/repo.worktrees/feat\n" {
		t.Errorf("content = %q, want path + newline", string(b))
	}
}

func TestWriteCdFile_EmptyFileArgIsNoop(t *testing.T) {
	if err := writeCdFile("", "/repo.worktrees/feat"); err != nil {
		t.Errorf("no --cd-file should be a no-op, got %v", err)
	}
}
