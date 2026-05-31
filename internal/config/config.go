package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds resolved worktree settings.
type Config struct {
	BaseRef      string `yaml:"base_ref"`
	Container    string `yaml:"container"`
	NameTemplate string `yaml:"name_template"`
}

// Defaults returns the built-in defaults.
func Defaults() Config {
	return Config{BaseRef: "HEAD"}
}

// Load reads <repoRoot>/.worktrees/config.yaml. A missing file is not an error;
// defaults are returned. The file's set fields override defaults.
func Load(repoRoot string) (Config, error) {
	cfg := Defaults()
	path := filepath.Join(repoRoot, ".worktrees", "config.yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return cfg, err
	}
	return Resolve(cfg, fileCfg), nil
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
	return out
}
