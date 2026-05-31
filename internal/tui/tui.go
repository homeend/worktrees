package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/code-drill/wt/pkg/worktree"
)

// Run launches the interactive TUI listing worktrees for the given dir.
func Run(m *worktree.Manager, dir string) error {
	items, err := m.List(dir)
	if err != nil {
		return err
	}
	// WithAltScreen renders in the terminal's alternate buffer so quitting
	// restores the original screen and prompt cleanly. It also makes the
	// terminal handover for create/delete actions (tea.Exec) leave and re-enter
	// the alt buffer, so live hook output shows on the normal screen and the
	// TUI's own frame is never left half-drawn on exit.
	p := tea.NewProgram(newModel(m, dir, items), tea.WithAltScreen())
	_, err = p.Run()
	return err
}
