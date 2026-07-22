package cmd

import (
	"strings"
	"testing"
)

func TestRootLongFor_PosixMentionsShellInit(t *testing.T) {
	long := rootLongFor("linux")
	if !strings.Contains(long, "shell-init") {
		t.Errorf("posix root help should mention shell-init:\n%s", long)
	}
	if strings.Contains(long, "wt.cmd") {
		t.Errorf("posix root help should not carry the cmd.exe instructions:\n%s", long)
	}
}

func TestRootLongFor_WindowsOmitsShellInit(t *testing.T) {
	long := rootLongFor("windows")
	if strings.Contains(long, "shell-init") {
		t.Errorf("windows root help must not mention shell-init (not registered there):\n%s", long)
	}
	if !strings.Contains(long, "wt.cmd") {
		t.Errorf("windows root help should explain the wt.cmd wrapper:\n%s", long)
	}
}
