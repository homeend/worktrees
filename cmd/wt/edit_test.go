package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorCommand_ResolutionOrder(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	if got := editorCommand("linux", env(map[string]string{"VISUAL": "code --wait", "EDITOR": "vim"})); strings.Join(got, " ") != "code --wait" {
		t.Errorf("VISUAL should win and split into fields, got %v", got)
	}
	if got := editorCommand("linux", env(map[string]string{"EDITOR": "nano"})); strings.Join(got, " ") != "nano" {
		t.Errorf("EDITOR fallback, got %v", got)
	}
	if got := editorCommand("linux", env(nil)); strings.Join(got, " ") != "vi" {
		t.Errorf("unix default should be vi, got %v", got)
	}
	if got := editorCommand("windows", env(nil)); strings.Join(got, " ") != "notepad" {
		t.Errorf("windows default should be notepad, got %v", got)
	}
}

func TestEnsureConfigFile_ScaffoldsOnlyWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", ".wt.toml")
	if err := ensureConfigFile(path, "# fresh template\n"); err != nil {
		t.Fatalf("ensureConfigFile: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "# fresh template\n" {
		t.Fatalf("scaffold content = %q, %v", b, err)
	}
	if err := ensureConfigFile(path, "# other\n"); err != nil {
		t.Fatalf("second ensureConfigFile: %v", err)
	}
	b, _ = os.ReadFile(path)
	if string(b) != "# fresh template\n" {
		t.Errorf("existing file must not be clobbered, got %q", b)
	}
}
