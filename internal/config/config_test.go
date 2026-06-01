package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestDefaults_BranchPrefix(t *testing.T) {
	if got := Defaults().BranchPrefix; got != "wt/" {
		t.Errorf("Defaults().BranchPrefix = %q, want %q", got, "wt/")
	}
}

func TestResolve_BranchPrefixOverrides(t *testing.T) {
	lo := Config{BranchPrefix: "wt/"}
	hi := Config{BranchPrefix: "feature/"}
	if got := Resolve(lo, hi).BranchPrefix; got != "feature/" {
		t.Errorf("Resolve BranchPrefix = %q, want %q", got, "feature/")
	}
	if got := Resolve(lo, Config{}).BranchPrefix; got != "wt/" {
		t.Errorf("Resolve with empty hi = %q, want %q", got, "wt/")
	}
}

func TestNormalizePrefix(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"feature":  "feature/",
		"feature/": "feature/",
		"wt":       "wt/",
	}
	for in, want := range cases {
		if got := NormalizePrefix(in); got != want {
			t.Errorf("NormalizePrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadFile_MissingReturnsZero(t *testing.T) {
	cfg, err := LoadFile(t.TempDir())
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}
	if cfg.BranchPrefix != "" {
		t.Errorf("missing file BranchPrefix = %q, want empty", cfg.BranchPrefix)
	}
}

func TestLoad_EnvOverridesFileAndNormalizes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".worktrees"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "branch_prefix: \"file/\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".worktrees", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("WT_BRANCH_PREFIX", "")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "file/" {
		t.Errorf("file BranchPrefix = %q, want %q", cfg.BranchPrefix, "file/")
	}

	t.Setenv("WT_BRANCH_PREFIX", "env")
	cfg, err = Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "env/" {
		t.Errorf("env BranchPrefix = %q, want %q", cfg.BranchPrefix, "env/")
	}
}

func TestLoad_DefaultWhenNothingSet(t *testing.T) {
	t.Setenv("WT_BRANCH_PREFIX", "")
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "wt/" {
		t.Errorf("default BranchPrefix = %q, want %q", cfg.BranchPrefix, "wt/")
	}
}

func TestSet_WritesAndReadsBack(t *testing.T) {
	dir := t.TempDir()
	if err := Set(dir, "branch_prefix", "feature"); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	cfg, err := LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BranchPrefix != "feature/" {
		t.Errorf("BranchPrefix = %q, want %q (normalized)", cfg.BranchPrefix, "feature/")
	}
}

func TestSet_UpdatesExistingAndPreservesComments(t *testing.T) {
	dir := t.TempDir()
	wt := filepath.Join(dir, ".worktrees")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# a helpful comment\nbranch_prefix: \"old/\"\n"
	if err := os.WriteFile(filepath.Join(wt, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Set(dir, "branch_prefix", "new"); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(wt, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# a helpful comment") {
		t.Errorf("comment not preserved:\n%s", s)
	}
	if strings.Contains(s, "old/") {
		t.Errorf("old value not replaced:\n%s", s)
	}
	if !strings.Contains(s, "branch_prefix: \"new/\"") {
		t.Errorf("new value missing:\n%s", s)
	}
}

func TestSet_RejectsUnknownKeyAndEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := Set(dir, "nope", "x"); err == nil {
		t.Error("expected error for unknown key")
	}
	if err := Set(dir, "branch_prefix", ""); err == nil {
		t.Error("expected error for empty value")
	}
}

func TestLoad_ReadsTemplates(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "templates:\n  - name: autofix\n    template: \"autofix/{{.ticketName}}\"\n  - name: feature\n    template: \"feat/{{.ticketName}}\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Templates) != 2 {
		t.Fatalf("Templates = %+v, want 2", cfg.Templates)
	}
	if cfg.Templates[0].Name != "autofix" || cfg.Templates[0].Template != "autofix/{{.ticketName}}" {
		t.Errorf("Templates[0] = %+v", cfg.Templates[0])
	}
}

func TestResolve_TemplatesOverrideOnlyWhenSet(t *testing.T) {
	lo := Config{Templates: []Template{{Name: "a", Template: "a/{{.x}}"}}}
	if got := Resolve(lo, Config{}).Templates; len(got) != 1 {
		t.Errorf("empty hi should not clear templates, got %+v", got)
	}
	hi := Config{Templates: []Template{{Name: "b", Template: "b/{{.x}}"}}}
	if got := Resolve(lo, hi).Templates; len(got) != 1 || got[0].Name != "b" {
		t.Errorf("non-nil hi should override, got %+v", got)
	}
}
