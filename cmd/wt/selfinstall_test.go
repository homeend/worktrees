package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsFullNameInvocation(t *testing.T) {
	yes := []string{"/home/u/go/bin/worktrees", `C:\Users\u\go\bin\worktrees.exe`, "/x/WORKTREES.EXE"}
	no := []string{"/opt/bin/wt.bin", `C:\x\wt.bin.exe`, "/usr/local/bin/wt", "/x/worktrees2"}
	for _, p := range yes {
		if !isFullNameInvocation(p) {
			t.Errorf("isFullNameInvocation(%q) = false, want true", p)
		}
	}
	for _, p := range no {
		if isFullNameInvocation(p) {
			t.Errorf("isFullNameInvocation(%q) = true, want false", p)
		}
	}
}

func TestSelfInstallAt_Posix(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "worktrees")
	if err := os.WriteFile(exe, []byte("BINARY"), 0o755); err != nil {
		t.Fatal(err)
	}

	changed, err := selfInstallAt(exe, "linux")
	if err != nil {
		t.Fatalf("selfInstallAt: %v", err)
	}
	if !changed {
		t.Error("first run should install")
	}
	bin, err := os.ReadFile(filepath.Join(dir, "wt.bin"))
	if err != nil || string(bin) != "BINARY" {
		t.Fatalf("wt.bin = %q, %v; want copy of the binary", bin, err)
	}
	info, _ := os.Stat(filepath.Join(dir, "wt.bin"))
	if info.Mode()&0o111 == 0 {
		t.Error("wt.bin must be executable")
	}
	script, err := os.ReadFile(filepath.Join(dir, "wt"))
	if err != nil || !strings.Contains(string(script), "wt.bin") {
		t.Fatalf("wt entry script missing or not pointing at wt.bin: %q, %v", script, err)
	}
	sinfo, _ := os.Stat(filepath.Join(dir, "wt"))
	if sinfo.Mode()&0o111 == 0 {
		t.Error("wt entry script must be executable")
	}

	// Idempotent while nothing changed.
	changed, err = selfInstallAt(exe, "linux")
	if err != nil || changed {
		t.Errorf("second run should be a no-op, changed=%v err=%v", changed, err)
	}

	// A newer binary (fresh go install) refreshes the copies.
	future := time.Now().Add(time.Hour)
	os.Chtimes(exe, future, future)
	changed, err = selfInstallAt(exe, "linux")
	if err != nil || !changed {
		t.Errorf("newer binary should refresh, changed=%v err=%v", changed, err)
	}
}

func TestSelfInstallAt_Windows(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "worktrees.exe")
	if err := os.WriteFile(exe, []byte("PE"), 0o755); err != nil {
		t.Fatal(err)
	}
	changed, err := selfInstallAt(exe, "windows")
	if err != nil || !changed {
		t.Fatalf("selfInstallAt: changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wt.bin.exe")); err != nil {
		t.Errorf("wt.bin.exe should be created: %v", err)
	}
	cmdScript, err := os.ReadFile(filepath.Join(dir, "wt.cmd"))
	if err != nil {
		t.Fatalf("wt.cmd should be created: %v", err)
	}
	if !strings.Contains(string(cmdScript), "wt.bin.exe") {
		t.Errorf("wt.cmd should reference wt.bin.exe:\n%s", cmdScript)
	}
	if !strings.Contains(string(cmdScript), "\r\n") {
		t.Error("wt.cmd must use CRLF line endings")
	}
	if _, err := os.Stat(filepath.Join(dir, "wt")); err == nil {
		t.Error("posix entry script should not be written on windows")
	}
}
