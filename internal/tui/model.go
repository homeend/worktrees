package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/homeend/worktrees/internal/naming"
	"github.com/homeend/worktrees/pkg/worktree"
)

// mode is the TUI's interaction state.
type mode int

const (
	modeNormal mode = iota
	modeConfirmDelete
	modeConfirmKillAll
	modeInputName
	modeTemplates
	modeInputVar
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
	store     lister
	dir       string
	items     []worktree.WorktreeInfo
	templates []worktree.Template
	cursor    int
	mode      mode
	status    string
	input     string

	// selected is the worktree path chosen with Enter. The CLI layer reads it
	// after the program quits and hands it to a shell cd wrapper (a child
	// process cannot change its parent shell's cwd itself).
	selected string

	// Template-creation flow state: the picked template, its <user:> labels,
	// and the values collected so far (one modeInputVar round per label).
	pendingTmpl worktree.Template
	varLabels   []string
	varVals     []string

	// runAction launches a `wt` subcommand in the foreground, handing the
	// terminal over so hook output renders cleanly, then reports completion.
	// Injectable so tests can assert the command without spawning a process.
	runAction func(args ...string) tea.Cmd

	// escapeCwd moves the TUI process out of the worktree it may be standing
	// in, called before destructive actions: on Windows a directory that is
	// any process's cwd cannot be deleted, and the TUI itself would otherwise
	// block the removal its subprocess performs. Injectable for tests.
	escapeCwd func()
}

func newModel(store lister, dir string, items []worktree.WorktreeInfo, templates []worktree.Template) model {
	return model{store: store, dir: dir, items: items, templates: templates,
		runAction: defaultRunAction, escapeCwd: func() {}}
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
		switch m.mode {
		case modeConfirmDelete:
			return m.updateConfirm(msg)
		case modeConfirmKillAll:
			return m.updateConfirmKillAll(msg)
		case modeInputName:
			return m.updateInputName(msg)
		case modeTemplates:
			return m.updateTemplates(msg)
		case modeInputVar:
			return m.updateInputVar(msg)
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
	case "enter":
		// Select the worktree under the cursor and quit so the shell wrapper
		// can cd into it.
		it, ok := m.current()
		if !ok {
			return m, nil
		}
		m.selected = it.Path
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
		// Names are never generated: ask for one.
		m.mode = modeInputName
		m.input = ""
		m.status = ""
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
	case "K":
		m.mode = modeConfirmKillAll
		m.status = ""
	case "t":
		m.mode = modeTemplates
		m.status = ""
	}
	return m, nil
}

func (m model) updateInputName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		name := strings.TrimSpace(m.input)
		m.mode = modeNormal
		m.input = ""
		if name == "" {
			return m, nil
		}
		m.status = "creating " + name + "…"
		return m, m.runAction("new", name, "--repo", m.dir)
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input = ""
		return m, nil
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyRunes:
		m.input += string(msg.Runes)
		return m, nil
	}
	return m, nil
}

// updateTemplates handles the template picker: a digit key creates from that
// template (prompting for its <user:> values first), any other key returns.
func (m model) updateTemplates(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	if s == "ctrl+c" {
		return m, tea.Quit
	}
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		idx := int(s[0] - '1')
		if idx < len(m.templates) {
			m.pendingTmpl = m.templates[idx]
			m.varLabels = naming.UserLabels(m.pendingTmpl.Template)
			m.varVals = nil
			if len(m.varLabels) == 0 {
				return m.dispatchTemplate()
			}
			m.mode = modeInputVar
			m.input = ""
			m.status = ""
			return m, nil
		}
	}
	m.mode = modeNormal
	return m, nil
}

// updateInputVar collects one <user:LABEL> value per round; after the last
// label the create action is dispatched.
func (m model) updateInputVar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		v := strings.TrimSpace(m.input)
		if v == "" {
			m.status = "value required (Esc cancels)"
			return m, nil
		}
		m.varVals = append(m.varVals, v)
		m.input = ""
		if len(m.varVals) < len(m.varLabels) {
			return m, nil
		}
		return m.dispatchTemplate()
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input = ""
		return m, nil
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyRunes:
		m.input += string(msg.Runes)
		return m, nil
	}
	return m, nil
}

// dispatchTemplate runs `wt new -t <name> label=value... --repo dir` with the
// collected variable values.
func (m model) dispatchTemplate() (tea.Model, tea.Cmd) {
	args := []string{"new", "-t", m.pendingTmpl.Name}
	for i, l := range m.varLabels {
		args = append(args, l+"="+m.varVals[i])
	}
	args = append(args, "--repo", m.dir)
	m.mode = modeNormal
	m.input = ""
	m.status = "creating from template " + m.pendingTmpl.Name + "…"
	return m, m.runAction(args...)
}

func (m model) updateConfirmKillAll(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.mode = modeNormal
		m.status = "removing all worktrees…"
		m.escapeCwd()
		return m, m.runAction("kill-em-all", "--yes", "--repo", m.dir)
	case "n", "N", "esc":
		m.mode = modeNormal
		return m, nil
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
		m.escapeCwd()
		return m, m.runAction("rm", name, "--repo", m.dir)
	case "f", "F":
		// Force: discard uncommitted changes and force-delete the branch even
		// if unmerged (`wt rm --force --force-branch`).
		m.mode = modeNormal
		it, ok := m.current()
		if !ok {
			return m, nil
		}
		name := filepath.Base(it.Path)
		m.status = "force-removing " + name + "…"
		m.escapeCwd()
		return m, m.runAction("rm", name, "--force", "--force-branch", "--repo", m.dir)
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
