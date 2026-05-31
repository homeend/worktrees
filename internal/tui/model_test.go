package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/code-drill/wt/pkg/worktree"
)

// fakeLister stands in for *worktree.Manager in unit tests.
type fakeLister struct {
	items []worktree.WorktreeInfo
	calls int
}

func (f *fakeLister) List(string) ([]worktree.WorktreeInfo, error) {
	f.calls++
	return f.items, nil
}

// newTestModel builds a model whose runAction records invocations instead of
// spawning a subprocess.
func newTestModel(items []worktree.WorktreeInfo) (model, *[]string) {
	rec := &[]string{}
	m := newModel(&fakeLister{items: items}, "/repo", items)
	m.runAction = func(args ...string) tea.Cmd {
		*rec = append(*rec, strings.Join(args, " "))
		return func() tea.Msg { return actionFinishedMsg{} }
	}
	return m, rec
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func sample() []worktree.WorktreeInfo {
	return []worktree.WorktreeInfo{
		{Path: "/repo", Branch: "refs/heads/main", IsMain: true},
		{Path: "/repo.worktrees/feat", Branch: "refs/heads/wt/feat"},
	}
}

func TestModel_QuitOnQ(t *testing.T) {
	m, _ := newTestModel(sample())
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("pressing q should return a command (tea.Quit)")
	}
}

func TestModel_CursorMovesDown(t *testing.T) {
	m, _ := newTestModel(sample())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.(model).cursor != 1 {
		t.Errorf("cursor = %d, want 1", updated.(model).cursor)
	}
}

func TestModel_CursorClampsAtTopAndBottom(t *testing.T) {
	m, _ := newTestModel(sample())
	up, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if up.(model).cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", up.(model).cursor)
	}
	d1, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	d2, _ := d1.(model).Update(tea.KeyMsg{Type: tea.KeyDown})
	if d2.(model).cursor != 1 {
		t.Errorf("cursor should clamp at 1, got %d", d2.(model).cursor)
	}
}

func TestView_RendersBranches(t *testing.T) {
	m, _ := newTestModel(sample())
	if m.View() == "" {
		t.Fatal("view should render non-empty output")
	}
}

func TestView_EmptyListStillRenders(t *testing.T) {
	m := newModel(&fakeLister{}, "/repo", nil)
	if m.View() == "" {
		t.Fatal("view should render even with no worktrees")
	}
}

func TestNew_CreatesWithGeneratedName(t *testing.T) {
	m, rec := newTestModel(sample())
	mm, cmd := m.Update(key("n"))
	if cmd == nil {
		t.Fatal("n should return an action command immediately")
	}
	if mm.(model).mode != modeNormal {
		t.Errorf("n should stay in normal mode (instant create), got %v", mm.(model).mode)
	}
	if len(*rec) != 1 || (*rec)[0] != "new --repo /repo" {
		t.Errorf("runAction = %v, want [new --repo /repo]", *rec)
	}
}

func TestDelete_ConfirmYesRemovesByDirName(t *testing.T) {
	m, rec := newTestModel(sample())
	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown}) // move to /repo.worktrees/feat
	conf, _ := down.(model).Update(key("d"))
	if conf.(model).mode != modeConfirmDelete {
		t.Fatalf("d should enter confirm mode, got %v", conf.(model).mode)
	}
	done, cmd := conf.(model).Update(key("y"))
	if cmd == nil {
		t.Fatal("y should return an action command")
	}
	if done.(model).mode != modeNormal {
		t.Errorf("mode should return to normal after confirm")
	}
	if len(*rec) != 1 || (*rec)[0] != "rm feat --repo /repo" {
		t.Errorf("runAction = %v, want [rm feat --repo /repo]", *rec)
	}
}

func TestDelete_ConfirmNoCancels(t *testing.T) {
	m, rec := newTestModel(sample())
	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	conf, _ := down.(model).Update(key("d"))
	cancel, _ := conf.(model).Update(key("n"))
	if cancel.(model).mode != modeNormal {
		t.Errorf("n should cancel back to normal mode")
	}
	if len(*rec) != 0 {
		t.Errorf("cancel should run no action, got %v", *rec)
	}
}

func TestDelete_RefusesMainWorktree(t *testing.T) {
	m, rec := newTestModel(sample()) // cursor starts on /repo (main)
	u, _ := m.Update(key("d"))
	if u.(model).mode != modeNormal {
		t.Errorf("deleting main should not enter confirm mode")
	}
	if u.(model).status == "" {
		t.Errorf("deleting main should set a status message")
	}
	if len(*rec) != 0 {
		t.Errorf("deleting main should run no action, got %v", *rec)
	}
}

func TestActionFinished_TriggersReload(t *testing.T) {
	m, _ := newTestModel(sample())
	_, cmd := m.Update(actionFinishedMsg{})
	if cmd == nil {
		t.Fatal("actionFinishedMsg should trigger a reload command")
	}
	msg := cmd()
	rl, ok := msg.(reloadMsg)
	if !ok {
		t.Fatalf("reload command should produce reloadMsg, got %T", msg)
	}
	if len(rl.items) != 2 {
		t.Errorf("reloaded items = %d, want 2", len(rl.items))
	}
}

func TestActionFinished_ErrorSurfacesLogPath(t *testing.T) {
	m, _ := newTestModel(sample())
	updated, _ := m.Update(actionFinishedMsg{err: errFake("boom"), logPath: "/tmp/wt-action-123.log"})
	st := updated.(model).status
	if !strings.Contains(st, "/tmp/wt-action-123.log") {
		t.Errorf("status should mention the log path, got %q", st)
	}
	if !strings.Contains(st, "boom") {
		t.Errorf("status should mention the error, got %q", st)
	}
}

func TestActionFinished_SuccessClearsStatus(t *testing.T) {
	m, _ := newTestModel(sample())
	m.status = "creating worktree…"
	updated, _ := m.Update(actionFinishedMsg{logPath: "/tmp/wt-action-9.log"})
	if updated.(model).status != "" {
		t.Errorf("success should clear status, got %q", updated.(model).status)
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

func TestReload_UpdatesItemsAndClampsCursor(t *testing.T) {
	m, _ := newTestModel(sample())
	m.cursor = 1
	updated, _ := m.Update(reloadMsg{items: sample()[:1]}) // only main remains
	mm := updated.(model)
	if len(mm.items) != 1 {
		t.Fatalf("items = %d, want 1", len(mm.items))
	}
	if mm.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", mm.cursor)
	}
}
