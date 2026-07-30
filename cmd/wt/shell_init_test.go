package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// brewLayout builds <root>/Cellar/<formula>/<version>/bin/wt plus the
// upgrade-stable <root>/opt/<formula>/bin/wt symlink target, mirroring a
// Homebrew prefix, and returns both paths.
func brewLayout(t *testing.T, formula, version string) (cellarExe, optExe string) {
	t.Helper()
	root := t.TempDir()
	cellarExe = filepath.Join(root, "Cellar", formula, version, "bin", "wt")
	optExe = filepath.Join(root, "opt", formula, "bin", "wt")
	for _, p := range []string{cellarExe, optExe} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return cellarExe, optExe
}

func TestStabilizeExePath_RewritesCellarToOpt(t *testing.T) {
	for _, version := range []string{"1.2.3", "0.5.0_1"} {
		cellarExe, optExe := brewLayout(t, "wt", version)
		if got := stabilizeExePath(cellarExe); got != optExe {
			t.Errorf("stabilizeExePath(%q) = %q, want %q", cellarExe, got, optExe)
		}
	}
}

func TestStabilizeExePath_LeavesNonBrewPathsAlone(t *testing.T) {
	for _, exe := range []string{
		"/home/u/go/bin/wt.bin",
		"/usr/local/bin/wt",
		"/home/u/Cellar/mytools/bin/wt", // dir named Cellar, but "bin" is no version
	} {
		if got := stabilizeExePath(exe); got != exe {
			t.Errorf("stabilizeExePath(%q) = %q, want unchanged", exe, got)
		}
	}
}

func TestStabilizeExePath_RequiresExistingOptTarget(t *testing.T) {
	cellarExe, optExe := brewLayout(t, "wt", "1.0.0")
	if err := os.Remove(optExe); err != nil {
		t.Fatal(err)
	}
	if got := stabilizeExePath(cellarExe); got != cellarExe {
		t.Errorf("without an opt target the Cellar path must pass through, got %q", got)
	}
}

func TestShellInitScript_ZshEmitsFunctionBoundToExe(t *testing.T) {
	s, err := shellInitScript("zsh", "/opt/tools/bin/wt")
	if err != nil {
		t.Fatalf("shellInitScript: %v", err)
	}
	for _, want := range []string{"wt() {", "/opt/tools/bin/wt", "--cd-file", "cd "} {
		if !strings.Contains(s, want) {
			t.Errorf("script should contain %q:\n%s", want, s)
		}
	}
}

func TestShellInitScript_BashMatchesZsh(t *testing.T) {
	z, err1 := shellInitScript("zsh", "/opt/tools/bin/wt")
	b, err2 := shellInitScript("bash", "/opt/tools/bin/wt")
	if err1 != nil || err2 != nil {
		t.Fatalf("errors: %v, %v", err1, err2)
	}
	if z != b {
		t.Errorf("bash and zsh scripts should be identical (POSIX body)")
	}
}

func TestShellInitScript_UnknownShellErrors(t *testing.T) {
	if _, err := shellInitScript("fish", "/opt/tools/bin/wt"); err == nil {
		t.Error("unknown shell should error")
	}
	if _, err := shellInitScript("cmd", "/opt/tools/bin/wt"); err == nil {
		t.Error("cmd should error with a pointer to the batch wrapper")
	}
}
