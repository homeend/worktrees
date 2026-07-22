package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/internal/config"
)

var setSafe bool

// runSet applies a config write to the repo's .wt.toml with optional --safe
// protection. Extracted from the cobra command for testing. With safe=true it
// refuses to overwrite an existing, different value (an equal value is a
// no-op success).
func runSet(repoRoot, key, value string, safe bool) error {
	if safe {
		fileCfg, err := config.LoadRepoFile(repoRoot)
		if err != nil {
			return err
		}
		existing := ""
		switch key {
		case "base_ref":
			existing = fileCfg.BaseRef
		case "container":
			existing = fileCfg.Container
		}
		if existing != "" && existing != value {
			return fmt.Errorf("%s already set to %q; refusing to overwrite with %q (--safe)", key, existing, value)
		}
	}
	return config.Set(repoRoot, key, value)
}

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value in .wt.toml (base_ref, container)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := workdir()
		if err != nil {
			return err
		}
		repoRoot, err := repoRootFor(cwd)
		if err != nil {
			return err
		}
		key, value := args[0], args[1]
		if err := runSet(repoRoot, key, value, setSafe); err != nil {
			return err
		}
		fmt.Printf("Set %s = %q\n", key, value)
		return nil
	},
}

func init() {
	setCmd.Flags().BoolVar(&setSafe, "safe", false, "error if the key already has a different value")
	rootCmd.AddCommand(setCmd)
}
