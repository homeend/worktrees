package cmd

import (
	"strings"
	"testing"
)

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
