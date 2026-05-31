package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Underline(true)
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
	b.WriteString("\n↑/↓ move • q quit\n")
	return b.String()
}
