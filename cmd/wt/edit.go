package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/internal/config"
)

var editUserCfg bool

const userConfigTemplate = `# wt user-level configuration (<UserConfigDir>/wt/config.toml).
# Applies to every repo; a repo's committed .wt.toml overlays it
# field-by-field (a [templates] table there replaces this one wholesale).
#
# base_ref = "HEAD"     # ref new branches are cut from (outside derive mode)
# container = ""        # override the default sibling <repo>.worktrees dir
#
# [templates]
# fix = "fix/<user:ticket>-<seq:fix:3>"
# spike = "spike/<date>-<random-alpha:4>"
`

// editorCommand returns the editor invocation: $VISUAL, then $EDITOR (both
// may carry arguments, e.g. "code --wait"), then a platform default.
func editorCommand(goos string, getenv func(string) string) []string {
	for _, v := range []string{"VISUAL", "EDITOR"} {
		if e := getenv(v); e != "" {
			return strings.Fields(e)
		}
	}
	if goos == "windows" {
		return []string{"notepad"}
	}
	return []string{"vi"}
}

// ensureConfigFile scaffolds a commented template at path when the file does
// not exist yet, so the editor always opens a documented skeleton.
func ensureConfigFile(path, template string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(template), 0o644)
}

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the config in your editor (.wt.toml; --user for the user config)",
	Long: `Open the repo's .wt.toml — or with --user the user-level config at
<UserConfigDir>/wt/config.toml — in your default editor ($VISUAL, then
$EDITOR, then vi / notepad). A missing file is created from a commented
template first. After the editor exits the file is parsed, so syntax errors
surface immediately instead of on the next command.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var path, template string
		if editUserCfg {
			p, err := config.UserConfigPath()
			if err != nil {
				return err
			}
			path, template = p, userConfigTemplate
		} else {
			cwd, err := workdir()
			if err != nil {
				return err
			}
			repoRoot, err := repoRootFor(cwd)
			if err != nil {
				return err
			}
			path, template = config.RepoConfigPath(repoRoot), repoConfigTemplate
		}
		if err := ensureConfigFile(path, template); err != nil {
			return err
		}

		ed := editorCommand(runtime.GOOS, os.Getenv)
		c := exec.Command(ed[0], append(ed[1:], path)...)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("editor %q: %w", strings.Join(ed, " "), err)
		}

		// Surface syntax errors now, not on the next wt invocation.
		if err := checkConfigFile(path); err != nil {
			return fmt.Errorf("%s saved but has a problem: %w", path, err)
		}
		fmt.Printf("Updated %s\n", path)
		return nil
	},
}

// checkConfigFile parses the edited file so mistakes surface immediately.
func checkConfigFile(path string) error {
	_, err := config.CheckFile(path)
	return err
}

func init() {
	editCmd.Flags().BoolVar(&editUserCfg, "user", false, "edit the user-level config instead of the repo's .wt.toml")
	rootCmd.AddCommand(editCmd)
}
