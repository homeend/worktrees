package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// shellInitCmd prints a wrapper function for the user's shell, bound to this
// binary by absolute path. Eval'ing it from the rc file is the whole install:
//
//	eval "$(/path/to/wt shell-init zsh)"
//
// The function passes --cd-file to wt and cd's to the path selected with
// Enter in the TUI after wt exits (a child process cannot change its parent
// shell's cwd itself).
var shellInitCmd = &cobra.Command{
	Use:   "shell-init <bash|zsh>",
	Short: "Print or install the shell function that cd's to the worktree selected in the TUI",
	Long: `Print (or install with --install) the shell function that makes Enter in
the wt TUI cd your shell into the selected worktree.

A child process can never change its parent shell's directory, so the
function wraps wt: it passes --cd-file, and after wt exits it cds to the
path wt wrote there. The function is bound to this binary by absolute path —
no PATH setup is needed.

--install appends the eval line to your rc file (~/.zshrc honoring ZDOTDIR,
or ~/.bashrc), creating it if missing. It is idempotent: an existing wt
shell-init line is detected and never duplicated, so it is safe to run on
every new machine or user account.`,
	Example: `  # one-time setup on a new machine or user account
  wt shell-init zsh --install && exec zsh

  # or wire it manually in ~/.zshrc / ~/.bashrc
  eval "$(/path/to/bin/wt shell-init zsh)"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exe, err := os.Executable()
		if err != nil {
			exe = os.Args[0]
		}
		shell := args[0]
		script, err := shellInitScript(shell, exe)
		if err != nil {
			return err
		}
		if !shellInitInstall {
			fmt.Print(script)
			return nil
		}
		rc, err := rcFileFor(shell)
		if err != nil {
			return err
		}
		line := fmt.Sprintf(`eval "$('%s' shell-init %s)"   # wt: cd to the worktree selected with Enter in the TUI`, exe, shell)
		added, err := appendShellInit(rc, line, shell)
		if err != nil {
			return err
		}
		if added {
			fmt.Printf("Installed — added to %s:\n  %s\nRestart your shell or run: source %s\n", rc, line, rc)
		} else {
			fmt.Printf("Already installed: %s has a wt shell-init %s line; nothing changed.\n", rc, shell)
		}
		return nil
	},
}

var shellInitInstall bool

func init() {
	shellInitCmd.Flags().BoolVar(&shellInitInstall, "install", false,
		"append the eval line to your shell rc file (idempotent) instead of printing the function")
	// shell-init is POSIX-shell integration. On Windows cd-on-Enter is handled
	// by the wt.cmd wrapper next to wt.bin.exe, so the command is not
	// registered there and never shows up in help.
	if runtime.GOOS != "windows" {
		rootCmd.AddCommand(shellInitCmd)
	}
}

// rcFileFor returns the rc file to install into. zsh honors ZDOTDIR.
func rcFileFor(shell string) (string, error) {
	switch shell {
	case "zsh":
		if z := os.Getenv("ZDOTDIR"); z != "" {
			return filepath.Join(z, ".zshrc"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return filepath.Join(home, ".zshrc"), nil
	case "bash":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return filepath.Join(home, ".bashrc"), nil
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: bash, zsh)", shell)
	}
}

// appendShellInit appends line to rc unless a wt shell-init line for this
// shell is already present (any binary path). It reports whether it added
// the line. The rc file is created when missing.
func appendShellInit(rc, line, shell string) (bool, error) {
	existing, err := os.ReadFile(rc)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", rc, err)
	}
	for _, l := range strings.Split(string(existing), "\n") {
		if lineIsWtShellInit(l, shell) {
			return false, nil
		}
	}
	prefix := ""
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		prefix = "\n"
	}
	f, err := os.OpenFile(rc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", rc, err)
	}
	defer f.Close()
	if _, err := f.WriteString(prefix + line + "\n"); err != nil {
		return false, fmt.Errorf("append to %s: %w", rc, err)
	}
	return true, nil
}

// lineIsWtShellInit reports whether l already evals wt's shell-init for
// shell. The binary token directly before "shell-init" must be wt or wt.bin
// (optionally quote-terminated), so other tools' shell-init lines (e.g. gg)
// and paths merely containing "wt" never match.
func lineIsWtShellInit(l, shell string) bool {
	if !strings.Contains(l, "shell-init "+shell) {
		return false
	}
	for _, tok := range []string{
		"wt shell-init", "wt' shell-init", `wt" shell-init`,
		"wt.bin shell-init", "wt.bin' shell-init", `wt.bin" shell-init`,
	} {
		if strings.Contains(l, tok) {
			return true
		}
	}
	return false
}

// shellInitScript returns the wrapper function for shell, binding it to the
// wt binary at exe so no PATH setup is needed. bash and zsh share one POSIX
// body.
func shellInitScript(shell, exe string) (string, error) {
	switch shell {
	case "bash", "zsh":
		return fmt.Sprintf(`wt() {
  local tmp dir code
  tmp="$(mktemp "${TMPDIR:-/tmp}/wt-cd.XXXXXX")" || { command %[1]q "$@"; return $?; }
  command %[1]q --cd-file "$tmp" "$@"
  code=$?
  if [ -s "$tmp" ]; then
    IFS= read -r dir <"$tmp"
    if [ -n "$dir" ] && [ -d "$dir" ]; then
      cd "$dir" || code=$?
    fi
  fi
  rm -f "$tmp"
  return $code
}
`, exe), nil
	case "cmd":
		return "", fmt.Errorf("cmd.exe cannot eval functions; add the bin directory to PATH and type `wt` — it resolves to the wt.cmd wrapper, which launches the sibling wt.bin.exe")
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: bash, zsh)", shell)
	}
}
