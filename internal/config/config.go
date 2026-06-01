package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds resolved worktree settings.
type Config struct {
	BaseRef      string `yaml:"base_ref"`
	Container    string `yaml:"container"`
	NameTemplate string `yaml:"name_template"`
	BranchPrefix string `yaml:"branch_prefix"`
}

// Defaults returns the built-in defaults.
func Defaults() Config {
	return Config{BaseRef: "HEAD", BranchPrefix: "wt/"}
}

// NormalizePrefix returns the branch prefix with a single trailing slash. An
// empty prefix is returned unchanged (callers treat "" as "use the default").
func NormalizePrefix(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasSuffix(s, "/") {
		return s
	}
	return s + "/"
}

// LoadFile reads only <repoRoot>/.worktrees/config.yaml and returns the raw
// parsed config. A missing file yields a zero Config and a nil error. No
// defaults or env layering are applied — callers that want the resolved config
// use Load; --safe inspects the persisted value via LoadFile.
func LoadFile(repoRoot string) (Config, error) {
	path := filepath.Join(repoRoot, ".worktrees", "config.yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return Config{}, err
	}
	return fileCfg, nil
}

// envCfg reads the environment-variable config layer. Today only
// WT_BRANCH_PREFIX is supported; other fields stay file-only.
func envCfg() Config {
	return Config{BranchPrefix: os.Getenv("WT_BRANCH_PREFIX")}
}

// Load resolves configuration in precedence order (highest wins):
// WT_BRANCH_PREFIX env > .worktrees/config.yaml > built-in defaults. The
// resolved branch prefix is normalized to carry a single trailing slash.
func Load(repoRoot string) (Config, error) {
	fileCfg, err := LoadFile(repoRoot)
	if err != nil {
		return Defaults(), err
	}
	cfg := Resolve(Resolve(Defaults(), fileCfg), envCfg())
	cfg.BranchPrefix = NormalizePrefix(cfg.BranchPrefix)
	return cfg, nil
}

// Resolve layers higher-priority over lower: any non-empty field in hi wins.
func Resolve(lo, hi Config) Config {
	out := lo
	if hi.BaseRef != "" {
		out.BaseRef = hi.BaseRef
	}
	if hi.Container != "" {
		out.Container = hi.Container
	}
	if hi.NameTemplate != "" {
		out.NameTemplate = hi.NameTemplate
	}
	if hi.BranchPrefix != "" {
		out.BranchPrefix = hi.BranchPrefix
	}
	return out
}

// Set persists a single config key to <repoRoot>/.worktrees/config.yaml,
// creating the directory/file if absent. Only branch_prefix is supported today;
// an unknown key or empty value is rejected. The write is a line-based upsert:
// an existing top-level "<key>:" line is replaced, otherwise the line is
// appended — preserving any surrounding comments. This is sound only because
// every config key is a flat top-level scalar.
func Set(repoRoot, key, value string) error {
	if key != "branch_prefix" {
		return fmt.Errorf("unknown config key %q (supported: branch_prefix)", key)
	}
	value = NormalizePrefix(value)
	if value == "" {
		return fmt.Errorf("branch_prefix cannot be empty")
	}

	dir := filepath.Join(repoRoot, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "config.yaml")

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	line := fmt.Sprintf("%s: %q", key, value)
	out := upsertLine(string(existing), key, line)
	return os.WriteFile(path, []byte(out), 0o644)
}

// upsertLine replaces the first uncommented "<key>:" line in content, or appends
// the line if none exists. It preserves all other lines (including comments).
func upsertLine(content, key, line string) string {
	lines := strings.Split(content, "\n")
	prefix := key + ":"
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, prefix) {
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}
	// Append. Drop a single trailing empty element so we don't add blank lines.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	lines = append(lines, line, "")
	return strings.Join(lines, "\n")
}
