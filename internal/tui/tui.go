package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/homeend/worktrees/pkg/worktree"
)

// Run launches the interactive TUI listing worktrees for the given dir. It
// returns the worktree path selected with Enter, or "" when the user quit
// without selecting one. tmpls are the configured named templates offered by
// the template picker.
func Run(m *worktree.Manager, dir string, tmpls []worktree.Template) (string, error) {
	items, err := m.List(dir)
	if err != nil {
		return "", err
	}
	// WithAltScreen renders in the terminal's alternate buffer so quitting
	// restores the original screen and prompt cleanly. It also makes the
	// terminal handover for create/delete actions (tea.Exec) leave and re-enter
	// the alt buffer, so live hook output shows on the normal screen and the
	// TUI's own frame is never left half-drawn on exit.
	mdl := newModel(m, dir, items, tmpls)
	// Destructive actions first move this process out of any worktree it
	// stands in, so its cwd never blocks a removal (Windows locks a directory
	// that is any process's cwd).
	mdl.escapeCwd = func() { m.EscapeCwd(dir) }
	p := tea.NewProgram(mdl, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	if fm, ok := final.(model); ok {
		return fm.selected, nil
	}
	return "", nil
}
