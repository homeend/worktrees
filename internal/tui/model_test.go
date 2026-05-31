package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/code-drill/wt/pkg/worktree"
)

func TestModel_QuitOnQ(t *testing.T) {
	m := newModel([]worktree.WorktreeInfo{{Path: "/r", Branch: "refs/heads/main", IsMain: true}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("pressing q should return a command (tea.Quit)")
	}
}

func TestModel_CursorMovesDown(t *testing.T) {
	m := newModel([]worktree.WorktreeInfo{
		{Path: "/r", Branch: "refs/heads/main", IsMain: true},
		{Path: "/r.worktrees/feat", Branch: "refs/heads/wt/feat"},
	})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm := updated.(model)
	if mm.cursor != 1 {
		t.Errorf("cursor = %d, want 1", mm.cursor)
	}
}

func TestModel_CursorClampsAtTopAndBottom(t *testing.T) {
	m := newModel([]worktree.WorktreeInfo{
		{Path: "/r", Branch: "refs/heads/main", IsMain: true},
		{Path: "/r.worktrees/feat", Branch: "refs/heads/wt/feat"},
	})
	// up at top stays at 0
	up, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if up.(model).cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", up.(model).cursor)
	}
	// down twice clamps at last index (1)
	d1, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	d2, _ := d1.(model).Update(tea.KeyMsg{Type: tea.KeyDown})
	if d2.(model).cursor != 1 {
		t.Errorf("cursor should clamp at 1, got %d", d2.(model).cursor)
	}
}

func TestView_RendersBranches(t *testing.T) {
	m := newModel([]worktree.WorktreeInfo{{Path: "/r.worktrees/feat", Branch: "refs/heads/wt/feat"}})
	out := m.View()
	if out == "" {
		t.Fatal("view should render non-empty output")
	}
}

func TestView_EmptyListStillRenders(t *testing.T) {
	m := newModel(nil)
	if m.View() == "" {
		t.Error("view should render even with no worktrees")
	}
}
