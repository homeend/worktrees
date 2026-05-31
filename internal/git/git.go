package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Runner executes git commands with a stable, parseable environment.
type Runner struct {
	bin string
}

// New returns a Runner that invokes the `git` binary on PATH.
func New() *Runner { return &Runner{bin: "git"} }

// Run executes git in dir and returns stdout. On non-zero exit it returns an
// error whose message includes stderr. LC_ALL=C makes messages locale-stable.
func (r *Runner) Run(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(r.bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// Version is a parsed git version.
type Version struct {
	Major, Minor, Patch int
}

// Version probes `git --version`.
func (r *Runner) Version() (Version, error) {
	out, err := r.Run("", "--version")
	if err != nil {
		return Version{}, err
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return Version{}, fmt.Errorf("cannot parse git version: %q", out)
	}
	parts := strings.SplitN(fields[2], ".", 3)
	v := Version{}
	if len(parts) > 0 {
		v.Major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		v.Minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		v.Patch, _ = strconv.Atoi(leadingInt(parts[2]))
	}
	return v, nil
}

// EnsureMinVersion returns an error if git is older than major.minor.
func (r *Runner) EnsureMinVersion(major, minor int) error {
	v, err := r.Version()
	if err != nil {
		return err
	}
	if versionLess(v, major, minor) {
		return fmt.Errorf("git %d.%d+ required, found %d.%d", major, minor, v.Major, v.Minor)
	}
	return nil
}

// versionLess reports whether version a is older than major.minor.
func versionLess(a Version, major, minor int) bool {
	return a.Major < major || (a.Major == major && a.Minor < minor)
}

func leadingInt(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return "0"
	}
	return s[:i]
}
