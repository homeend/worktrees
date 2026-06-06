package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/internal/config"
)

var setSafe bool

// runSet applies a config write with optional --safe protection. Extracted from
// the cobra command for testing. With safe=true it refuses to overwrite an
// existing, different value (an equal value is a no-op success).
func runSet(repoRoot, key, value string, safe bool) error {
	if safe && key == "branch_prefix" {
		fileCfg, err := config.LoadFile(repoRoot)
		if err != nil {
			return err
		}
		existing := config.NormalizePrefix(fileCfg.BranchPrefix)
		want := config.NormalizePrefix(value)
		if existing != "" && existing != want {
			return fmt.Errorf("branch_prefix already set to %q; refusing to overwrite with %q (--safe)", existing, want)
		}
	}
	return config.Set(repoRoot, key, value)
}

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value (e.g. branch_prefix)",
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
		fmt.Printf("Set %s = %q\n", key, config.NormalizePrefix(value))
		return nil
	},
}

func init() {
	setCmd.Flags().BoolVar(&setSafe, "safe", false, "error if the key already has a different value")
	rootCmd.AddCommand(setCmd)
}
