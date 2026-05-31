package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/code-drill/wt/pkg/worktree"
)

type model struct {
	items  []worktree.WorktreeInfo
	cursor int
}

func newModel(items []worktree.WorktreeInfo) model {
	return model{items: items}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
		}
	}
	return m, nil
}
