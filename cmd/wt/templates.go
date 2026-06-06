package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/homeend/worktrees/pkg/worktree"
)

// printTemplates writes a tab-aligned table of templates (extracted for testing).
func printTemplates(out io.Writer, tmpls []worktree.Template) error {
	if len(tmpls) == 0 {
		fmt.Fprintln(out, "no templates defined")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "#\tNAME\tTEMPLATE")
	for i, t := range tmpls {
		fmt.Fprintf(w, "%d\t%s\t%s\n", i+1, t.Name, t.Template)
	}
	return w.Flush()
}

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "List configured branch templates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, _, err := managerForWorkdir()
		if err != nil {
			return err
		}
		return printTemplates(os.Stdout, m.Templates())
	},
}

func init() { rootCmd.AddCommand(templatesCmd) }
