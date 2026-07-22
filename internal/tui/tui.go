package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/homeend/worktrees/pkg/worktree"
)

// Run launches the interactive TUI listing worktrees for the given dir. It
// returns the worktree path selected with Enter, or "" when the user quit
// without selecting one.
func Run(m *worktree.Manager, dir string) (string, error) {
	items, err := m.List(dir)
	if err != nil {
		return "", err
	}
	// WithAltScreen renders in the terminal's alternate buffer so quitting
	// restores the original screen and prompt cleanly. It also makes the
	// terminal handover for create/delete actions (tea.Exec) leave and re-enter
	// the alt buffer, so live hook output shows on the normal screen and the
	// TUI's own frame is never left half-drawn on exit.
	p := tea.NewProgram(newModel(m, dir, items, m.Templates()), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	if fm, ok := final.(model); ok {
		return fm.selected, nil
	}
	return "", nil
}
