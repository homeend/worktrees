package cmd

import (
	"bufio"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/homeend/worktrees/internal/config"
	"github.com/homeend/worktrees/internal/git"
	"github.com/homeend/worktrees/internal/naming"
	"github.com/homeend/worktrees/pkg/worktree"
)

var newTemplate string

// parseVars turns ["k=v", ...] into a map. The value may contain '=' (split
// on the first only). A missing '=' or empty key is an error.
func parseVars(args []string) (map[string]string, error) {
	vars := make(map[string]string, len(args))
	for _, a := range args {
		k, v, ok := strings.Cut(a, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid variable %q (expected name=value)", a)
		}
		vars[k] = v
	}
	return vars, nil
}

// promptMissing collects values for template <user:LABEL> tokens not already
// present in vars. Interactive mode asks on out/in; otherwise a missing
// label is an error naming the k=v syntax.
func promptMissing(vars map[string]string, labels []string, in io.Reader, out io.Writer, interactive bool) error {
	reader := bufio.NewReader(in)
	for _, l := range labels {
		if _, ok := vars[l]; ok {
			continue
		}
		if !interactive {
			return fmt.Errorf("missing value for <user:%s>: pass %s=<value> (no TTY to ask interactively)", l, l)
		}
		fmt.Fprintf(out, "%s: ", l)
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return fmt.Errorf("read value for <user:%s>: %w", l, err)
		}
		v := strings.TrimSpace(line)
		if v == "" {
			return fmt.Errorf("empty value for <user:%s>", l)
		}
		vars[l] = v
	}
	return nil
}

// lookupTemplate finds a named template, erroring with the available names.
func lookupTemplate(cfg config.Config, name string) (string, error) {
	if tmpl, ok := cfg.Templates[name]; ok {
		return tmpl, nil
	}
	names := make([]string, 0, len(cfg.Templates))
	for n := range cfg.Templates {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "", fmt.Errorf("unknown template %q (none configured; add a [templates] table to .wt.toml or the user config)", name)
	}
	return "", fmt.Errorf("unknown template %q (available: %s)", name, strings.Join(names, ", "))
}

var newCmd = &cobra.Command{
	Use:   "new [name] [var=value ...]",
	Short: "Create a new worktree (derives from the current worktree's branch when run inside one)",
	Long: `Create a new worktree. A branch name is never generated: pass a name, use a
named template (-t), or run it from inside a worktree.

From the main repo root:

  wt new fix-login          # branch fix-login, cut from base_ref
  wt new -t fix ticket=42   # branch from the "fix" template; missing
                            # <user:...> values are asked interactively

From inside a managed worktree the new branch derives from that worktree's
branch, cut from its committed tip:

  wt new                    # <current-branch>-vNNN (lowest free number)
  wt new fix                # <current-branch>-fix (literal suffix)

Templates use the gg token syntax: <user:LABEL>, <seq:NAME:PAD>,
<date:yyyy-MM-dd>, <repo>, <parent-branch>, <random-alpha:N>, <random-num:N>.
Worktree directories are the branch sanitized into one flat segment
('/' becomes '-') inside the sibling <repo>.worktrees container.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, cfg, cwd, err := managerForWorkdir()
		if err != nil {
			return err
		}

		var opts worktree.AddOptions
		var bumpSeqs []string
		var commonDir string

		if newTemplate == "" {
			if len(args) > 1 {
				return fmt.Errorf("expected at most one name argument, got %d", len(args))
			}
			if len(args) == 1 {
				opts.Name = args[0]
			}
		} else {
			tmpl, err := lookupTemplate(cfg, newTemplate)
			if err != nil {
				return err
			}
			vars, err := parseVars(args)
			if err != nil {
				return err
			}
			interactive := term.IsTerminal(int(os.Stdin.Fd()))
			if err := promptMissing(vars, naming.UserLabels(tmpl), os.Stdin, os.Stdout, interactive); err != nil {
				return err
			}

			repoRoot, err := repoRootFor(cwd)
			if err != nil {
				return err
			}
			commonDir, err = git.New().CommonDir(cwd)
			if err != nil {
				return err
			}
			bumpSeqs = naming.SeqNames(tmpl)
			parent, _ := m.ParentBranch(cwd)
			branch, err := naming.Resolve(tmpl, vars, naming.Ctx{
				Repo:         filepath.Base(repoRoot),
				ParentBranch: parent,
				Seqs:         config.PeekSeqs(commonDir, bumpSeqs),
				Now:          time.Now,
				Rand:         rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
			})
			if err != nil {
				return err
			}
			opts.Branch = branch
		}

		res, err := m.Add(cwd, opts)
		if err != nil {
			return err
		}
		// Consume the <seq:> counters only after a successful create, so a
		// failed attempt does not burn numbers.
		for _, name := range bumpSeqs {
			if _, err := config.BumpSeq(commonDir, name); err != nil {
				fmt.Fprintf(os.Stderr, "warning: bump seq %q: %v\n", name, err)
			}
		}
		fmt.Printf("Created worktree %q\n  branch: %s\n  path:   %s\n", res.Name, res.Branch, res.Path)
		return nil
	},
}

func init() {
	newCmd.Flags().StringVarP(&newTemplate, "template", "t", "", "render the branch from a named template")
	rootCmd.AddCommand(newCmd)
}
