package tui

import (
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
// tea.ExecProcess) completes.
type actionFinishedMsg struct{ err error }

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

// defaultRunAction re-invokes this binary as `wt <args>` via tea.ExecProcess,
// which suspends the TUI, restores the normal terminal for the duration (so the
// subcommand's hook output and messages display correctly), then resumes.
func defaultRunAction(args ...string) tea.Cmd {
	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}
	c := exec.Command(self, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return actionFinishedMsg{err: err}
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
		if msg.err != nil {
			m.status = "action failed: " + msg.err.Error()
		} else {
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
