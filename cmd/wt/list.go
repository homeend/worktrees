package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/code-drill/wt/pkg/worktree"
)

var listJSON bool

// renderListJSON serializes worktrees as indented JSON (extracted for testing).
func renderListJSON(items []worktree.WorktreeInfo) (string, error) {
	type row struct {
		Path   string `json:"path"`
		Branch string `json:"branch"`
		HEAD   string `json:"head"`
		IsMain bool   `json:"is_main"`
	}
	rows := make([]row, len(items))
	for i, w := range items {
		rows[i] = row{Path: w.Path, Branch: w.Branch, HEAD: w.HEAD, IsMain: w.IsMain}
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List worktrees",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}
		items, err := m.List(cwd)
		if err != nil {
			return err
		}
		if listJSON {
			out, err := renderListJSON(items)
			if err != nil {
				return err
			}
			fmt.Println(out)
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "BRANCH\tPATH")
		for _, it := range items {
			marker := ""
			if it.IsMain {
				marker = " (main)"
			}
			fmt.Fprintf(w, "%s%s\t%s\n", it.Branch, marker, it.Path)
		}
		return w.Flush()
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(listCmd)
}
