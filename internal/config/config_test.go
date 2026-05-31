package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if cfg.BaseRef != "HEAD" {
		t.Errorf("default BaseRef = %q, want HEAD", cfg.BaseRef)
	}
	if cfg.Container != "" || cfg.NameTemplate != "" {
		t.Errorf("unset fields should be empty: %+v", cfg)
	}
}

func TestLoad_ReadsYAML(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "base_ref: develop\ncontainer: /tmp/wts\nname_template: custom\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseRef != "develop" || cfg.Container != "/tmp/wts" || cfg.NameTemplate != "custom" {
		t.Errorf("loaded config wrong: %+v", cfg)
	}
}

func TestResolve_FlagsOverrideFileOverrideDefaults(t *testing.T) {
	file := Config{BaseRef: "develop", Container: "/from/file"}
	flags := Config{BaseRef: "feature"} // Container unset -> file wins
	got := Resolve(file, flags)
	if got.BaseRef != "feature" {
		t.Errorf("flag should override: BaseRef=%q", got.BaseRef)
	}
	if got.Container != "/from/file" {
		t.Errorf("file should win when flag unset: Container=%q", got.Container)
	}
}

func TestLoad_MalformedYAMLErrors(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("base_ref: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(repo); err == nil {
		t.Error("expected error for malformed YAML")
	}
}

func TestLoad_EmptyFileYieldsDefaults(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(repo)
	if err != nil {
		t.Fatalf("empty file should not error: %v", err)
	}
	if cfg.BaseRef != "HEAD" {
		t.Errorf("empty file should preserve default BaseRef=HEAD, got %q", cfg.BaseRef)
	}
}

func TestLoad_PartialConfigPreservesDefaults(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("container: /tmp/wts\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseRef != "HEAD" {
		t.Errorf("BaseRef should remain default HEAD when only container is set, got %q", cfg.BaseRef)
	}
	if cfg.Container != "/tmp/wts" {
		t.Errorf("Container = %q, want /tmp/wts", cfg.Container)
	}
	if cfg.NameTemplate != "" {
		t.Errorf("NameTemplate should be empty, got %q", cfg.NameTemplate)
	}
}
