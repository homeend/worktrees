// Package config loads wt's configuration the same way gigagit does: a
// user-level TOML file overlaid field-by-field by a committed per-repo file.
// It also manages the machine-local per-repo <seq> counter state (state.go).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Config holds resolved worktree settings. TOML keys are snake_case.
type Config struct {
	// BaseRef is the ref new branches are cut from outside derive mode.
	BaseRef string `toml:"base_ref"`
	// Container overrides the default sibling "<repo>.worktrees" directory.
	Container string `toml:"container"`
	// Templates are the named branch-name templates (gg <token> syntax).
	Templates map[string]string `toml:"templates"`
}

// Defaults returns the built-in defaults.
func Defaults() Config {
	return Config{BaseRef: "HEAD"}
}

// UserConfigPath returns the user-level config file: <UserConfigDir>/wt/config.toml.
func UserConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "wt", "config.toml"), nil
}

// RepoConfigPath returns the committed per-repo config file: <repoRoot>/.wt.toml.
func RepoConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".wt.toml")
}

// loadFile parses one TOML config file. A missing file yields a zero Config
// and nil error.
func loadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// LoadRepoFile reads only the per-repo .wt.toml (no defaults or user-config
// layering). Used by `wt set --safe` to inspect the persisted value.
func LoadRepoFile(repoRoot string) (Config, error) {
	return loadFile(RepoConfigPath(repoRoot))
}

// CheckFile parses a single config file at path (any layer), so callers like
// `wt edit` can validate an edit immediately.
func CheckFile(path string) (Config, error) {
	return loadFile(path)
}

// Load resolves configuration in precedence order (highest wins):
// <repoRoot>/.wt.toml > <UserConfigDir>/wt/config.toml > built-in defaults.
// Overlay is field-by-field; a [templates] table in a higher layer replaces
// the lower one wholesale (the gg overlay rule).
func Load(repoRoot string) (Config, error) {
	cfg := Defaults()
	if userPath, err := UserConfigPath(); err == nil {
		userCfg, err := loadFile(userPath)
		if err != nil {
			return Defaults(), err
		}
		cfg = Resolve(cfg, userCfg)
	}
	repoCfg, err := loadFile(RepoConfigPath(repoRoot))
	if err != nil {
		return Defaults(), err
	}
	return Resolve(cfg, repoCfg), nil
}

// Resolve layers higher-priority over lower: any non-zero field in hi wins.
func Resolve(lo, hi Config) Config {
	out := lo
	if hi.BaseRef != "" {
		out.BaseRef = hi.BaseRef
	}
	if hi.Container != "" {
		out.Container = hi.Container
	}
	if hi.Templates != nil {
		out.Templates = hi.Templates
	}
	return out
}

// settableKeys are the flat top-level keys `wt set` may write.
var settableKeys = map[string]bool{"base_ref": true, "container": true}

// Set persists a single flat config key to <repoRoot>/.wt.toml, creating the
// file if absent. The write is a line-based upsert that preserves comments
// and the [templates] table: an existing "<key> =" line is replaced in
// place; a new key is inserted at the top, before any [table] header (a
// plain append would land inside the last table).
func Set(repoRoot, key, value string) error {
	if !settableKeys[key] {
		return fmt.Errorf("unknown config key %q (supported: base_ref, container)", key)
	}
	if value == "" {
		return fmt.Errorf("%s cannot be empty", key)
	}

	path := RepoConfigPath(repoRoot)
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	line := fmt.Sprintf("%s = %q", key, value)
	out := upsertTomlLine(string(existing), key, line)
	return os.WriteFile(path, []byte(out), 0o644)
}

// upsertTomlLine replaces the first "<key> =" line before any table header,
// or inserts the line at the top when the key is absent.
func upsertTomlLine(content, key, line string) string {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") {
			break // top-level scalars end at the first table header
		}
		if strings.HasPrefix(trimmed, key) {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
			if strings.HasPrefix(rest, "=") {
				lines[i] = line
				return strings.Join(lines, "\n")
			}
		}
	}
	if content == "" {
		return line + "\n"
	}
	return line + "\n" + content
}
