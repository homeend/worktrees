package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const repoConfigTemplate = `# wt per-repo configuration (.wt.toml, committed). Overlays the user-level
# config at <UserConfigDir>/wt/config.toml field-by-field; only keys set here
# win. Same layering as gg's .gg.toml.
#
# base_ref = "HEAD"     # ref new branches are cut from (outside derive mode)
# container = ""        # override the default sibling <repo>.worktrees dir
#
# Named branch templates for 'wt new -t <name>'. Tokens:
#   <user:LABEL>        value asked interactively (or passed as LABEL=value)
#   <seq:NAME:PAD>      per-repo counter, zero-padded (state in .git/wt/)
#   <date:yyyy-MM-dd>   current date/time (yyyy MM dd HH mm ss tokens)
#   <repo>              repository directory name
#   <parent-branch>     branch of the worktree wt runs in (empty at root)
#   <random-alpha:N>    N random lowercase letters
#   <random-num:N>      N random digits
#
# [templates]
# fix = "fix/<user:ticket>-<seq:fix:3>"
# spike = "spike/<date>-<random-alpha:4>"
`

const hookTemplate = `#!/usr/bin/env bash
# wt %s hook.
# Available environment variables:
#   WT_SOURCE_ROOT WT_TARGET_ROOT WT_NAME WT_BRANCH
#   WT_BASE_REF WT_CONTAINER WT_REPO_NAME WT_HOOK
# Example: cp "$WT_SOURCE_ROOT/.env" "$WT_TARGET_ROOT/.env"
exit 0
`

// scaffold creates .wt/ with executable hook stubs and a commented .wt.toml
// at the repo root. It never clobbers an existing file (idempotent).
func scaffold(repoRoot string) error {
	dir := filepath.Join(repoRoot, ".wt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeIfAbsent(filepath.Join(repoRoot, ".wt.toml"), repoConfigTemplate, 0o644); err != nil {
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
	Short: "Scaffold .wt/ hook stubs and a commented .wt.toml",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := workdir()
		if err != nil {
			return err
		}
		if err := scaffold(cwd); err != nil {
			return err
		}
		fmt.Println("Initialized .wt.toml and .wt/ hook stubs.")
		return nil
	},
}

func init() { rootCmd.AddCommand(initCmd) }
