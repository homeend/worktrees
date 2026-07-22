package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_DefaultsWhenNoFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseRef != "HEAD" {
		t.Errorf("BaseRef = %q, want HEAD", cfg.BaseRef)
	}
}

func TestLoad_RepoOverlaysUserOverlaysDefaults(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	writeFile(t, filepath.Join(userDir, "wt", "config.toml"), `
base_ref = "main"
[templates]
fix = "fix/<user:ticket>"
feat = "feature/<user:ticket>"
`)
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".wt.toml"), `
base_ref = "develop"
`)
	cfg, err := Load(repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseRef != "develop" {
		t.Errorf("repo base_ref should win, got %q", cfg.BaseRef)
	}
	if cfg.Templates["fix"] != "fix/<user:ticket>" {
		t.Errorf("user templates should survive when repo defines none, got %v", cfg.Templates)
	}
}

func TestLoad_RepoTemplatesReplaceUserTemplates(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	writeFile(t, filepath.Join(userDir, "wt", "config.toml"), `
[templates]
fix = "fix/<user:ticket>"
`)
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".wt.toml"), `
[templates]
spike = "spike/<date>"
`)
	cfg, err := Load(repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := cfg.Templates["fix"]; ok {
		t.Error("repo [templates] should replace the user table (gg overlay rule)")
	}
	if cfg.Templates["spike"] != "spike/<date>" {
		t.Errorf("Templates = %v", cfg.Templates)
	}
}

func TestSet_UpsertsIntoRepoTomlBeforeTables(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".wt.toml"), `# repo config
base_ref = "main"

[templates]
fix = "fix/<user:ticket>"
`)
	if err := Set(repo, "base_ref", "develop"); err != nil {
		t.Fatalf("Set replace: %v", err)
	}
	if err := Set(repo, "container", "/tmp/wts"); err != nil {
		t.Fatalf("Set insert: %v", err)
	}
	cfg, err := Load(repo)
	if err != nil {
		t.Fatalf("Load after Set: %v", err)
	}
	if cfg.BaseRef != "develop" || cfg.Container != "/tmp/wts" {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Templates["fix"] == "" {
		t.Error("templates section should be preserved by Set")
	}
	b, _ := os.ReadFile(filepath.Join(repo, ".wt.toml"))
	if !strings.Contains(string(b), "# repo config") {
		t.Error("comments should be preserved by Set")
	}
}

func TestSet_CreatesFileAndRejectsUnknownKey(t *testing.T) {
	repo := t.TempDir()
	if err := Set(repo, "base_ref", "main"); err != nil {
		t.Fatalf("Set on missing file: %v", err)
	}
	cfg, _ := Load(repo)
	if cfg.BaseRef != "main" {
		t.Errorf("BaseRef = %q", cfg.BaseRef)
	}
	if err := Set(repo, "branch_prefix", "x"); err == nil {
		t.Error("unknown key should be rejected")
	}
}

func TestSeqState_PeekAndBump(t *testing.T) {
	commonDir := t.TempDir()
	if got := PeekSeq(commonDir, "wt"); got != 1 {
		t.Errorf("fresh Peek = %d, want 1", got)
	}
	n, err := BumpSeq(commonDir, "wt")
	if err != nil || n != 1 {
		t.Fatalf("first Bump = %d, %v; want 1", n, err)
	}
	if got := PeekSeq(commonDir, "wt"); got != 2 {
		t.Errorf("Peek after bump = %d, want 2", got)
	}
	n, err = BumpSeq(commonDir, "wt")
	if err != nil || n != 2 {
		t.Fatalf("second Bump = %d, %v; want 2", n, err)
	}
	if _, err := BumpSeq("", "wt"); err == nil {
		t.Error("Bump with empty common dir must refuse to write")
	}
	if got := PeekSeq(commonDir, "other"); got != 1 {
		t.Errorf("independent counter Peek = %d, want 1", got)
	}
}
