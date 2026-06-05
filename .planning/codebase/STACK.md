# Technology Stack

**Analysis Date:** 2026-06-05

## Languages

**Primary:**
- Go 1.25.0 - All application code (`main.go`, `cmd/`, `internal/`, `pkg/`)

**Secondary:**
- YAML - Configuration file format (`.worktrees/config.yaml`)
- Go `text/template` - Branch name templating (`internal/naming/naming.go`)

## Runtime

**Environment:**
- Go runtime (compiled binary, no interpreter)
- Minimum: Go 1.25.0 (declared in `go.mod`)

**Package Manager:**
- Go modules (`go mod`)
- Lockfile: `go.sum` present and committed

## Frameworks

**Core:**
- `github.com/spf13/cobra v1.10.2` - CLI command framework (`cmd/wt/root.go`, all `cmd/wt/*.go`)
- `github.com/charmbracelet/bubbletea v1.3.10` - TUI framework (`internal/tui/tui.go`, `internal/tui/model.go`)

**TUI Rendering:**
- `github.com/charmbracelet/lipgloss v1.1.0` - TUI styling (`internal/tui/view.go`)
- `github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc` - Terminal color detection (indirect)

**Testing:**
- Go standard library `testing` package - No external test framework
- Integration tests use real git repos: `internal/git/git_integration_test.go`, `pkg/worktree/manager_integration_test.go`

**Build/Dev:**
- Go toolchain only — no Makefile, task runner, or build scripts detected

## Key Dependencies

**Critical (direct):**
- `github.com/spf13/cobra v1.10.2` - CLI entrypoint and command routing
- `gopkg.in/yaml.v3 v3.0.1` - Parses `.worktrees/config.yaml` (`internal/config/config.go`)
- `github.com/charmbracelet/bubbletea v1.3.10` - Interactive TUI mode (launched when stdout is a TTY with no subcommand)

**Infrastructure (indirect, pulled by bubbletea/lipgloss):**
- `github.com/charmbracelet/x/ansi v0.10.1` - ANSI escape code handling
- `github.com/charmbracelet/x/term v0.2.1` - Terminal state management
- `github.com/muesli/cancelreader v0.2.2` - Cancellable stdin reader
- `github.com/muesli/termenv v0.16.0` - Terminal environment detection
- `github.com/mattn/go-isatty v0.0.20` - TTY detection
- `golang.org/x/term v0.43.0` - TTY check in `cmd/wt/root.go` (`term.IsTerminal`)
- `github.com/rivo/uniseg v0.4.7` - Unicode segmentation
- `github.com/lucasb-eyer/go-colorful v1.2.0` - Color conversion

**Standard library used heavily:**
- `os/exec` - Shell-out to `git` binary (`internal/git/git.go`, `internal/hooks/hooks.go`)
- `crypto/rand` - Secure random digits for name generation (`cmd/wt/root.go`)
- `text/template` - Branch name and user templates (`internal/naming/naming.go`)
- `path/filepath`, `os`, `io/fs` - File and directory operations throughout

## Configuration

**Environment variables:**
- `WT_BRANCH_PREFIX` - Overrides `branch_prefix` from config file (highest precedence)
- `LC_ALL=C` - Set by the git runner to ensure locale-stable git output (not user-configurable)
- `GIT_TERMINAL_PROMPT=0` - Set by the git runner to disable git credential prompts

**Per-repo config file:**
- Location: `<repo-root>/.worktrees/config.yaml`
- Read by: `internal/config/config.go`
- Supported keys: `base_ref`, `container`, `name_template`, `branch_prefix`, `templates`
- Precedence: `WT_BRANCH_PREFIX` env > `config.yaml` > built-in defaults
- Built-in defaults: `base_ref: HEAD`, `branch_prefix: wt/`

**Build:**
- No external build config. Build with `go build ./...`
- No `Dockerfile`, `Makefile`, or CI config detected in repo root

## Platform Requirements

**Development:**
- Go 1.25.0+
- `git` binary on PATH (minimum git 2.30, enforced at runtime in `cmd/wt/root.go`)
- POSIX-compatible terminal for TUI mode

**Production:**
- Single compiled binary (`wt`)
- `git` 2.30+ on PATH at runtime
- Any OS supported by Go (Linux, macOS, Windows — though some git invocation paths are Unix-idiomatic)

---

*Stack analysis: 2026-06-05*
