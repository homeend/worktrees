package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Worktrees") + "\n\n")
	if len(m.items) == 0 {
		b.WriteString("  (no worktrees)\n")
	}
	for i, it := range m.items {
		line := fmt.Sprintf("%s  %s", it.Branch, it.Path)
		if it.IsMain {
			line += "  (main)"
		}
		if i == m.cursor {
			line = "> " + selectedStyle.Render(line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")

	switch m.mode {
	case modeConfirmDelete:
		if it, ok := m.current(); ok {
			b.WriteString(promptStyle.Render(fmt.Sprintf("Delete %s? (y/n)", filepath.Base(it.Path))) + "\n")
		}
	default:
		b.WriteString("↑/↓ move • n new • d delete • q quit\n")
	}

	if m.status != "" {
		b.WriteString("\n" + statusStyle.Render(m.status) + "\n")
	}
	return b.String()
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	promptStyle   = lipgloss.NewStyle().Bold(true)
	statusStyle   = lipgloss.NewStyle().Faint(true)
)
