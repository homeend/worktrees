package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const configTemplate = `# wt configuration. CLI flags override these values.
# base_ref: HEAD          # default ref new branches are cut from
# container: ""           # override container path; used verbatim
# name_template: ""       # override the default generated name pattern
`

const hookTemplate = `#!/usr/bin/env bash
# wt %s hook.
# Available environment variables:
#   WT_SOURCE_ROOT WT_TARGET_ROOT WT_NAME WT_BRANCH
#   WT_BASE_REF WT_CONTAINER WT_REPO_NAME WT_HOOK
# Example: cp "$WT_SOURCE_ROOT/.env" "$WT_TARGET_ROOT/.env"
exit 0
`

// scaffold creates .worktrees/ with a config and executable hook stubs. It never
// clobbers an existing file (idempotent).
func scaffold(repoRoot string) error {
	dir := filepath.Join(repoRoot, ".worktrees")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeIfAbsent(filepath.Join(dir, "config.yaml"), configTemplate, 0o644); err != nil {
		return err
	}
	for _, ev := range []string{"pre-create", "post-create", "pre-remove", "post-remove"} {
		body := fmt.Sprintf(hookTemplate, ev)
		if err := writeIfAbsent(filepath.Join(dir, ev), body, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func writeIfAbsent(path, content string, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), mode)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold .worktrees/ with config and hook stubs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := scaffold(cwd); err != nil {
			return err
		}
		fmt.Println("Initialized .worktrees/ (config.yaml + hook stubs).")
		return nil
	},
}

func init() { rootCmd.AddCommand(initCmd) }
