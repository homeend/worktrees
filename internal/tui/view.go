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
			b.WriteString(promptStyle.Render(fmt.Sprintf(
				"Delete %s? (y)es  (n)o  (f)orce — force discards uncommitted changes & deletes the branch",
				filepath.Base(it.Path))) + "\n")
		}
	case modeConfirmKillAll:
		n := 0
		for _, it := range m.items {
			if !it.IsMain {
				n++
			}
		}
		b.WriteString(promptStyle.Render(
			fmt.Sprintf("Remove ALL %d worktrees and their branches? Hooks skipped. (y/n)", n)) + "\n")
	case modeInputName:
		b.WriteString(promptStyle.Render("New worktree name (Enter create, Esc cancel):") + "\n")
		b.WriteString("  " + m.input + "_\n")
	case modeTemplates:
		b.WriteString(titleStyle.Render("Templates") + "\n")
		if len(m.templates) == 0 {
			b.WriteString("  (none defined)\n")
		}
		for i, tpl := range m.templates {
			b.WriteString(fmt.Sprintf("  %d  %s  %s\n", i+1, tpl.Name, tpl.Template))
		}
		b.WriteString("\n" + statusStyle.Render("press 1-9 to create from a template • any other key to return") + "\n")
	case modeInputVar:
		label := ""
		if len(m.varVals) < len(m.varLabels) {
			label = m.varLabels[len(m.varVals)]
		}
		b.WriteString(promptStyle.Render(fmt.Sprintf(
			"template %s — %s (Enter next, Esc cancel):", m.pendingTmpl.Name, label)) + "\n")
		b.WriteString("  " + m.input + "_\n")
	default:
		b.WriteString("↑/↓ move • enter cd • n new • t templates • d delete • K kill-all • q quit\n")
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
