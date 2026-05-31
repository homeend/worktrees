package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/code-drill/wt/pkg/worktree"
)

// mode is the TUI's interaction state.
type mode int

const (
	modeNormal mode = iota
	modeConfirmDelete
)

// lister is the subset of *worktree.Manager the TUI needs to refresh its view.
type lister interface {
	List(dir string) ([]worktree.WorktreeInfo, error)
}

// actionFinishedMsg is delivered when a foreground `wt` action (run via
// tea.ExecProcess) completes. logPath, when set, is where the action's combined
// output was written so the user can inspect it (especially on failure).
type actionFinishedMsg struct {
	err     error
	logPath string
}

// reloadMsg carries a refreshed worktree list.
type reloadMsg struct {
	items []worktree.WorktreeInfo
	err   error
}

type model struct {
	store  lister
	dir    string
	items  []worktree.WorktreeInfo
	cursor int
	mode   mode
	status string

	// runAction launches a `wt` subcommand in the foreground, handing the
	// terminal over so hook output renders cleanly, then reports completion.
	// Injectable so tests can assert the command without spawning a process.
	runAction func(args ...string) tea.Cmd
}

func newModel(store lister, dir string, items []worktree.WorktreeInfo) model {
	return model{store: store, dir: dir, items: items, runAction: defaultRunAction}
}

// loggedExec adapts an *exec.Cmd to tea.ExecCommand. The Bubble Tea runtime
// calls SetStdin/SetStdout/SetStderr with the real terminal streams and then
// Run; in Run we tee the child's combined output to a log file and print a
// banner naming that log before the child starts — so the location is visible
// above the live hook output and the output is preserved for later inspection.
type loggedExec struct {
	cmd     *exec.Cmd
	log     *os.File
	logPath string
}

func (l *loggedExec) SetStdin(r io.Reader)  { l.cmd.Stdin = r }
func (l *loggedExec) SetStdout(w io.Writer) { l.cmd.Stdout = w }
func (l *loggedExec) SetStderr(w io.Writer) { l.cmd.Stderr = w }

func (l *loggedExec) Run() error {
	// Capture the terminal input for the post-run pause before tee'ing wraps the
	// streams (which only touch stdout/stderr).
	in := l.cmd.Stdin
	out := l.cmd.Stdout

	if l.log != nil {
		// Default stderr to stdout if the runtime left it unset.
		if l.cmd.Stderr == nil {
			l.cmd.Stderr = l.cmd.Stdout
		}
		if l.cmd.Stdout != nil {
			l.cmd.Stdout = io.MultiWriter(l.cmd.Stdout, l.log)
		}
		if l.cmd.Stderr != nil {
			l.cmd.Stderr = io.MultiWriter(l.cmd.Stderr, l.log)
		}
		if l.cmd.Stdout != nil && l.logPath != "" {
			fmt.Fprintf(l.cmd.Stdout, "wt: logging this action to %s\n\n", l.logPath)
		}
	}

	runErr := l.cmd.Run()

	// Keep the action's output on screen until the user acknowledges, so it
	// isn't lost when the TUI repaints. The result line goes only to the
	// terminal (not the log).
	if out != nil {
		if runErr != nil {
			fmt.Fprintf(out, "\nwt: action failed: %v\n", runErr)
		} else {
			fmt.Fprintf(out, "\nwt: done.\n")
		}
		if l.logPath != "" {
			fmt.Fprintf(out, "wt: log saved to %s\n", l.logPath)
		}
		fmt.Fprint(out, "\nPress Enter to return to the list…")
		if in != nil {
			bufio.NewReader(in).ReadString('\n')
		}
	}

	return runErr
}

// defaultRunAction re-invokes this binary as `wt <args>` via tea.Exec, which
// suspends the TUI and restores the normal terminal for the duration so the
// subcommand's hook output and messages display live, then resumes. Output is
// tee'd to a temp log file; the path is announced before the run and reported
// back so the status line can show it on completion.
func defaultRunAction(args ...string) tea.Cmd {
	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}
	le := &loggedExec{cmd: exec.Command(self, args...)}
	if f, ferr := os.CreateTemp("", "wt-action-*.log"); ferr == nil {
		le.log = f
		le.logPath = f.Name()
	}
	return tea.Exec(le, func(err error) tea.Msg {
		if le.log != nil {
			le.log.Close()
		}
		return actionFinishedMsg{err: err, logPath: le.logPath}
	})
}

func (m model) Init() tea.Cmd { return nil }

// current returns the worktree under the cursor, if any.
func (m model) current() (worktree.WorktreeInfo, bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return worktree.WorktreeInfo{}, false
	}
	return m.items[m.cursor], true
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case reloadMsg:
		if msg.err != nil {
			m.status = "reload failed: " + msg.err.Error()
			return m, nil
		}
		m.items = msg.items
		m.clampCursor()
		return m, nil
	case actionFinishedMsg:
		switch {
		case msg.err != nil && msg.logPath != "":
			m.status = "action failed: " + msg.err.Error() + " — see " + msg.logPath
		case msg.err != nil:
			m.status = "action failed: " + msg.err.Error()
		case msg.logPath != "":
			m.status = "done — log: " + msg.logPath
		default:
			m.status = ""
		}
		return m, m.reloadCmd()
	case tea.KeyMsg:
		if m.mode == modeConfirmDelete {
			return m.updateConfirm(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m *model) clampCursor() {
	if m.cursor > len(m.items)-1 {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "n":
		// Create a worktree with an auto-generated name.
		m.status = "creating worktree…"
		return m, m.runAction("new", "--repo", m.dir)
	case "d":
		it, ok := m.current()
		if !ok {
			return m, nil
		}
		if it.IsMain {
			m.status = "cannot delete the main worktree"
			return m, nil
		}
		m.mode = modeConfirmDelete
		m.status = ""
	}
	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.mode = modeNormal
		it, ok := m.current()
		if !ok {
			return m, nil
		}
		name := filepath.Base(it.Path)
		m.status = "removing " + name + "…"
		return m, m.runAction("rm", name, "--repo", m.dir)
	case "n", "N", "esc":
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}

// reloadCmd fetches a fresh worktree list off the main update loop.
func (m model) reloadCmd() tea.Cmd {
	store, dir := m.store, m.dir
	return func() tea.Msg {
		items, err := store.List(dir)
		return reloadMsg{items: items, err: err}
	}
}
