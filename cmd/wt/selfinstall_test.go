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

func TestSelfInstallTo_Posix(t *testing.T) {
	src := t.TempDir()
	exe := filepath.Join(src, "worktrees")
	if err := os.WriteFile(exe, []byte("BINARY"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Nested, not-yet-existing target: an explicit install path is created.
	dir := filepath.Join(t.TempDir(), "tools", "bin")

	if err := selfInstallTo(exe, dir, "linux"); err != nil {
		t.Fatalf("selfInstallTo: %v", err)
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
	// The source dir itself gets nothing: install goes only to the target.
	if _, err := os.Stat(filepath.Join(src, "wt.bin")); err == nil {
		t.Error("nothing may be installed next to the binary")
	}

	// An explicit path always overwrites, even a newer stale copy.
	stale := filepath.Join(dir, "wt.bin")
	os.WriteFile(stale, []byte("STALE"), 0o755)
	future := time.Now().Add(time.Hour)
	os.Chtimes(stale, future, future)
	if err := selfInstallTo(exe, dir, "linux"); err != nil {
		t.Fatalf("selfInstallTo (overwrite): %v", err)
	}
	bin, _ = os.ReadFile(stale)
	if string(bin) != "BINARY" {
		t.Errorf("explicit install must overwrite, got %q", bin)
	}
}

func TestBootstrapUsageFor_Posix(t *testing.T) {
	usage := bootstrapUsageFor("linux")
	for _, want := range []string{"path", "wt.bin", "shell-init", "PATH"} {
		if !strings.Contains(usage, want) {
			t.Errorf("posix usage missing %q:\n%s", want, usage)
		}
	}
	// Never the full CLI help: no subcommand listing.
	for _, reject := range []string{"kill-em-all", "Available Commands"} {
		if strings.Contains(usage, reject) {
			t.Errorf("posix usage must not mention %q:\n%s", reject, usage)
		}
	}
}

func TestBootstrapUsageFor_Windows(t *testing.T) {
	usage := bootstrapUsageFor("windows")
	for _, want := range []string{"path", "wt.cmd", "wt.bin.exe", "PATH"} {
		if !strings.Contains(usage, want) {
			t.Errorf("windows usage missing %q:\n%s", want, usage)
		}
	}
	if strings.Contains(usage, "shell-init") {
		t.Errorf("windows usage must not mention shell-init (POSIX only):\n%s", usage)
	}
}

func TestRunBootstrap_NoPathShowsUsageAndInstallsNothing(t *testing.T) {
	src := t.TempDir()
	exe := filepath.Join(src, "worktrees")
	if err := os.WriteFile(exe, []byte("BINARY"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {}, {"--help"}, {"-h"}, {"a", "b"}} {
		var out, errOut strings.Builder
		code := runBootstrap(exe, args, "linux", &out, &errOut)
		if code == 0 {
			t.Errorf("runBootstrap(%v) = 0, want a usage-error exit", args)
		}
		if !strings.Contains(errOut.String(), "path") {
			t.Errorf("runBootstrap(%v) should explain the required path parameter:\n%s", args, errOut.String())
		}
		if _, err := os.Stat(filepath.Join(src, "wt.bin")); err == nil {
			t.Fatalf("runBootstrap(%v) must not install next to the binary", args)
		}
	}
}

func TestRunBootstrap_WithPathInstallsThere(t *testing.T) {
	src := t.TempDir()
	exe := filepath.Join(src, "worktrees")
	if err := os.WriteFile(exe, []byte("BINARY"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "target")
	var out, errOut strings.Builder
	code := runBootstrap(exe, []string{dir}, "linux", &out, &errOut)
	if code != 0 {
		t.Fatalf("runBootstrap = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "wt.bin")); err != nil {
		t.Errorf("wt.bin should be installed in the target dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wt")); err != nil {
		t.Errorf("wt entry script should be installed in the target dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(src, "wt.bin")); err == nil {
		t.Error("nothing may be installed next to the binary")
	}
	if !strings.Contains(out.String(), dir) || !strings.Contains(out.String(), "shell-init") {
		t.Errorf("stdout should confirm the target dir and shell integration steps:\n%s", out.String())
	}
	if strings.Contains(out.String(), "kill-em-all") {
		t.Errorf("stdout must not carry the full CLI help:\n%s", out.String())
	}
}

func TestSelfInstallTo_Windows(t *testing.T) {
	src := t.TempDir()
	exe := filepath.Join(src, "worktrees.exe")
	if err := os.WriteFile(exe, []byte("PE"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "target")
	if err := selfInstallTo(exe, dir, "windows"); err != nil {
		t.Fatalf("selfInstallTo: %v", err)
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
