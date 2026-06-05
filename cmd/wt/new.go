package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/code-drill/wt/internal/config"
	"github.com/code-drill/wt/pkg/worktree"
)

var (
	newBranch       string
	newBase         string
	newNoHooks      bool
	newTemplate     string
	newFromBranch   string
	newNoPrefix     bool
	newBranchPrefix string
)

// worktreeAddOptions builds AddOptions from flag values (extracted for testing).
func worktreeAddOptions(name, branch, base string, noHooks bool) worktree.AddOptions {
	return worktree.AddOptions{Name: name, Branch: branch, BaseRef: base, NoHooks: noHooks}
}

// addResolver renders a template ref into a branch name. *worktree.Manager
// satisfies it; a fake is used in tests.
type addResolver interface {
	ResolveTemplate(ref string, vars map[string]string) (string, error)
}

// parseVars turns ["k:v", ...] into a map. The value may contain colons (split
// on the first only). A missing colon or empty key is an error.
func parseVars(args []string) (map[string]string, error) {
	vars := make(map[string]string, len(args))
	for _, a := range args {
		k, v, ok := strings.Cut(a, ":")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid variable %q (expected name:value)", a)
		}
		vars[k] = v
	}
	return vars, nil
}

// buildAddOptions resolves new's flags/args into AddOptions. --template,
// --from-branch, and --branch are mutually exclusive (each defines the branch).
func buildAddOptions(r addResolver, args []string, tmpl, fromBranch, branch, base string, noHooks, noPrefix bool, prefixOverride string) (worktree.AddOptions, error) {
	set := 0
	for _, s := range []string{tmpl, fromBranch, branch} {
		if s != "" {
			set++
		}
	}
	if set > 1 {
		return worktree.AddOptions{}, fmt.Errorf("--template, --from-branch, and --branch are mutually exclusive")
	}

	opts := worktreeAddOptions("", branch, base, noHooks)
	opts.NoPrefix = noPrefix
	if !noPrefix {
		opts.PrefixOverride = config.NormalizePrefix(prefixOverride)
	}
	switch {
	case fromBranch != "":
		if len(args) > 0 {
			return worktree.AddOptions{}, fmt.Errorf("--from-branch takes no positional arguments")
		}
		opts.FromBranch = fromBranch
	case tmpl != "":
		vars, err := parseVars(args)
		if err != nil {
			return worktree.AddOptions{}, err
		}
		name, err := r.ResolveTemplate(tmpl, vars)
		if err != nil {
			return worktree.AddOptions{}, err
		}
		opts.Name = name
		// Signal that Name came from a template so derive mode (Add run from
		// inside a worktree) does not reinterpret it as a literal suffix token.
		opts.FromTemplate = true
	default:
		if len(args) > 1 {
			return worktree.AddOptions{}, fmt.Errorf("expected at most one name argument, got %d", len(args))
		}
		if len(args) == 1 {
			opts.Name = args[0]
		}
	}
	return opts, nil
}

var newCmd = &cobra.Command{
	Use:   "new [name | suffix | var:value ...]",
	Short: "Create a new worktree (derives from the current worktree's branch when run inside one)",
	Long: `Create a new worktree.

Run from the main repo root, ` + "`wt new`" + ` branches off the configured base
ref (base_ref / HEAD) exactly as before.

Run from inside a managed worktree, ` + "`wt new`" + ` instead derives the new branch
from the current worktree's branch, cutting it from that branch's committed tip
and placing the new worktree in the main repo's container:

  wt new            # <current-branch>-vNNN (lowest free, zero-padded)
  wt new -- -patch01  # <current-branch>-patch01 (literal suffix)

A custom suffix token that starts with a dash must be passed after ` + "`--`" + `
(e.g. ` + "`wt new -- -patch01`" + `); otherwise cobra parses the leading-dash
argument as a flag. The token is appended verbatim — its leading dash is the
separator. If the resulting branch already exists, the command fails rather than
auto-renaming. The parent branch's prefix is inherited as-is; --no-prefix /
--branch-prefix / --base have no effect in derive mode.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}
		opts, err := buildAddOptions(m, args, newTemplate, newFromBranch, newBranch, newBase, newNoHooks, newNoPrefix, newBranchPrefix)
		if err != nil {
			return err
		}
		res, err := m.Add(cwd, opts)
		if err != nil {
			return err
		}
		fmt.Printf("Created worktree %q\n  branch: %s\n  path:   %s\n", res.Name, res.Branch, res.Path)
		return nil
	},
}

func init() {
	newCmd.Flags().StringVarP(&newBranch, "branch", "b", "", "branch name (default: derived from name)")
	newCmd.Flags().StringVar(&newBase, "base", "", "base ref to branch from (default: config base_ref / HEAD)")
	newCmd.Flags().BoolVar(&newNoHooks, "no-hooks", false, "skip lifecycle hooks")
	newCmd.Flags().StringVarP(&newTemplate, "template", "t", "", "render the branch from a named/numbered template")
	newCmd.Flags().StringVar(&newFromBranch, "from-branch", "", "create a worktree from an existing local branch")
	newCmd.Flags().BoolVar(&newNoPrefix, "no-prefix", false, "do not prepend the configured branch prefix")
	newCmd.Flags().StringVar(&newBranchPrefix, "branch-prefix", "", "override the branch prefix for this run (empty disables; --no-prefix wins)")
	rootCmd.AddCommand(newCmd)
}
