package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendShellInit_AppendsAndIsIdempotent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("# existing config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	line := `eval "$(/opt/bin/wt shell-init zsh)"`

	added, err := appendShellInit(rc, line, "zsh")
	if err != nil {
		t.Fatalf("first install: %v", err)
	}
	if !added {
		t.Error("first install should report added=true")
	}

	added, err = appendShellInit(rc, line, "zsh")
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if added {
		t.Error("second install should be a no-op (added=false)")
	}

	b, _ := os.ReadFile(rc)
	if got := strings.Count(string(b), "shell-init zsh"); got != 1 {
		t.Errorf("rc should contain the line exactly once, found %d:\n%s", got, b)
	}
	if !strings.HasPrefix(string(b), "# existing config\n") {
		t.Errorf("existing content should be preserved:\n%s", b)
	}
}

func TestAppendShellInit_DetectsExistingInstallWithOtherPath(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	old := `eval "$(/somewhere/else/wt shell-init zsh)"   # wt integration` + "\n"
	if err := os.WriteFile(rc, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	added, err := appendShellInit(rc, `eval "$(/opt/bin/wt shell-init zsh)"`, "zsh")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if added {
		t.Error("an existing wt shell-init line (any path) should count as installed")
	}
}

func TestAppendShellInit_CreatesMissingRcFile(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".bashrc")
	added, err := appendShellInit(rc, `eval "$(/opt/bin/wt shell-init bash)"`, "bash")
	if err != nil {
		t.Fatalf("install into missing rc: %v", err)
	}
	if !added {
		t.Error("install into missing rc should report added=true")
	}
	if _, err := os.Stat(rc); err != nil {
		t.Errorf("rc file should have been created: %v", err)
	}
}

func TestRcFileFor(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("ZDOTDIR", "")
	if got, _ := rcFileFor("zsh"); got != "/home/u/.zshrc" {
		t.Errorf("zsh rc = %q, want /home/u/.zshrc", got)
	}
	if got, _ := rcFileFor("bash"); got != "/home/u/.bashrc" {
		t.Errorf("bash rc = %q, want /home/u/.bashrc", got)
	}
	t.Setenv("ZDOTDIR", "/home/u/cfg")
	if got, _ := rcFileFor("zsh"); got != "/home/u/cfg/.zshrc" {
		t.Errorf("zsh rc with ZDOTDIR = %q, want /home/u/cfg/.zshrc", got)
	}
	if _, err := rcFileFor("fish"); err == nil {
		t.Error("unsupported shell should error")
	}
}
