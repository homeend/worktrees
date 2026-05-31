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
	p := tea.NewProgram(newModel(m, dir, items))
	_, err = p.Run()
	return err
}
