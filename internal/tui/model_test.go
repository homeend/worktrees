package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/homeend/worktrees/pkg/worktree"
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
	m := newModel(&fakeLister{items: items}, "/repo", items, nil)
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
	m := newModel(&fakeLister{}, "/repo", nil, nil)
	if m.View() == "" {
		t.Fatal("view should render even with no worktrees")
	}
}

func TestNew_AsksForNameThenDispatches(t *testing.T) {
	m, rec := newTestModel(sample())
	cur, cmd := m.Update(key("n"))
	if cmd != nil {
		t.Fatal("n must not create immediately (names are never generated)")
	}
	if cur.(model).mode != modeInputName {
		t.Fatalf("n should enter name input mode, got %v", cur.(model).mode)
	}
	for _, ch := range []string{"f", "e", "a", "t"} {
		cur, _ = cur.Update(key(ch))
	}
	done, cmd := cur.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should dispatch the create action")
	}
	if done.(model).mode != modeNormal {
		t.Errorf("mode should return to normal after Enter")
	}
	if len(*rec) != 1 || (*rec)[0] != "new feat --repo /repo" {
		t.Errorf("runAction = %v, want [new feat --repo /repo]", *rec)
	}
}

func TestNew_EmptyNameCancels(t *testing.T) {
	m, rec := newTestModel(sample())
	cur, _ := m.Update(key("n"))
	done, cmd := cur.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || done.(model).mode != modeNormal || len(*rec) != 0 {
		t.Errorf("empty name should cancel without dispatching, rec=%v", *rec)
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

func TestDelete_ConfirmForceRemovesWithForceFlags(t *testing.T) {
	m, rec := newTestModel(sample())
	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown}) // move to /repo.worktrees/feat
	conf, _ := down.(model).Update(key("d"))
	if conf.(model).mode != modeConfirmDelete {
		t.Fatalf("d should enter confirm mode, got %v", conf.(model).mode)
	}
	done, cmd := conf.(model).Update(key("f"))
	if cmd == nil {
		t.Fatal("f should return an action command")
	}
	if done.(model).mode != modeNormal {
		t.Errorf("mode should return to normal after force confirm")
	}
	if len(*rec) != 1 || (*rec)[0] != "rm feat --force --force-branch --repo /repo" {
		t.Errorf("runAction = %v, want [rm feat --force --force-branch --repo /repo]", *rec)
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

func TestDeleteAndKillAll_EscapeCwdBeforeDispatch(t *testing.T) {
	m, _ := newTestModel(sample())
	escaped := 0
	m.escapeCwd = func() { escaped++ }
	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	conf, _ := down.(model).Update(key("d"))
	if _, cmd := conf.(model).Update(key("y")); cmd == nil {
		t.Fatal("y should dispatch")
	}
	if escaped != 1 {
		t.Errorf("delete confirm should escape cwd first, escaped=%d", escaped)
	}

	m2, _ := newTestModel(sample())
	m2.escapeCwd = func() { escaped++ }
	conf2, _ := m2.Update(key("K"))
	if _, cmd := conf2.(model).Update(key("y")); cmd == nil {
		t.Fatal("kill-all y should dispatch")
	}
	if escaped != 2 {
		t.Errorf("kill-all confirm should escape cwd first, escaped=%d", escaped)
	}
}

func TestKillAll_KeyEntersConfirm(t *testing.T) {
	m, _ := newTestModel(sample())
	updated, _ := m.Update(key("K"))
	if updated.(model).mode != modeConfirmKillAll {
		t.Errorf("mode = %v, want modeConfirmKillAll", updated.(model).mode)
	}
}

func TestKillAll_ConfirmYesDispatches(t *testing.T) {
	m, rec := newTestModel(sample())
	conf, _ := m.Update(key("K"))
	done, cmd := conf.(model).Update(key("y"))
	if cmd == nil {
		t.Fatal("y should return an action command")
	}
	if done.(model).mode != modeNormal {
		t.Errorf("mode should return to normal after confirm")
	}
	if len(*rec) != 1 || (*rec)[0] != "kill-em-all --yes --repo /repo" {
		t.Errorf("runAction = %v, want [kill-em-all --yes --repo /repo]", *rec)
	}
}

func TestKillAll_ConfirmNoCancels(t *testing.T) {
	m, rec := newTestModel(sample())
	conf, _ := m.Update(key("K"))
	cancel, _ := conf.(model).Update(key("n"))
	if cancel.(model).mode != modeNormal {
		t.Errorf("n should cancel back to normal mode")
	}
	if len(*rec) != 0 {
		t.Errorf("cancel should run no action, got %v", *rec)
	}
}

func TestNameInput_EscCancels(t *testing.T) {
	m, rec := newTestModel(sample())
	cur, _ := m.Update(key("n"))
	cur, _ = cur.(model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cur.(model).mode != modeNormal {
		t.Errorf("Esc should cancel to normal mode")
	}
	if len(*rec) != 0 {
		t.Errorf("cancel should run no action, got %v", *rec)
	}
}

func TestTemplates_DigitOnNoVarTemplateDispatches(t *testing.T) {
	m, rec := newTestModel(sample())
	m.templates = []worktree.Template{{Name: "spike", Template: "spike/<date>"}}
	cur, _ := m.Update(key("t"))
	done, cmd := cur.(model).Update(key("1"))
	if cmd == nil {
		t.Fatal("digit on a var-less template should dispatch immediately")
	}
	if done.(model).mode != modeNormal {
		t.Errorf("mode should return to normal, got %v", done.(model).mode)
	}
	if len(*rec) != 1 || (*rec)[0] != "new -t spike --repo /repo" {
		t.Errorf("runAction = %v, want [new -t spike --repo /repo]", *rec)
	}
}

func TestTemplates_DigitPromptsForVarsThenDispatches(t *testing.T) {
	m, rec := newTestModel(sample())
	m.templates = []worktree.Template{{Name: "fix", Template: "fix/<user:ticket>-<user:who>"}}
	cur, _ := m.Update(key("t"))
	cur, cmd := cur.(model).Update(key("1"))
	if cmd != nil {
		t.Fatal("template with vars must prompt, not dispatch")
	}
	if cur.(model).mode != modeInputVar {
		t.Fatalf("mode = %v, want modeInputVar", cur.(model).mode)
	}
	// First label: ticket
	for _, ch := range []string{"4", "2"} {
		cur, _ = cur.Update(key(ch))
	}
	cur, cmd = cur.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("one label left — must keep prompting")
	}
	// Second label: who
	cur, _ = cur.Update(key("m"))
	done, cmd := cur.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("last label collected — should dispatch")
	}
	if done.(model).mode != modeNormal {
		t.Errorf("mode should return to normal after dispatch")
	}
	if len(*rec) != 1 || (*rec)[0] != "new -t fix ticket=42 who=m --repo /repo" {
		t.Errorf("runAction = %v, want [new -t fix ticket=42 who=m --repo /repo]", *rec)
	}
}

func TestTemplates_VarInputEscCancels(t *testing.T) {
	m, rec := newTestModel(sample())
	m.templates = []worktree.Template{{Name: "fix", Template: "fix/<user:ticket>"}}
	cur, _ := m.Update(key("t"))
	cur, _ = cur.(model).Update(key("1"))
	done, _ := cur.(model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if done.(model).mode != modeNormal {
		t.Errorf("Esc should cancel the var prompt")
	}
	if len(*rec) != 0 {
		t.Errorf("cancel should run no action, got %v", *rec)
	}
}

func TestTemplates_KeyShowsList(t *testing.T) {
	m, _ := newTestModel(sample())
	m.templates = []worktree.Template{{Name: "autofix", Template: "autofix/{{.t}}"}}
	updated, _ := m.Update(key("t"))
	mm := updated.(model)
	if mm.mode != modeTemplates {
		t.Fatalf("mode = %v, want modeTemplates", mm.mode)
	}
	if !strings.Contains(mm.View(), "autofix") {
		t.Errorf("templates view should list 'autofix':\n%s", mm.View())
	}
}

func TestTemplates_AnyKeyReturns(t *testing.T) {
	m, _ := newTestModel(sample())
	m.templates = []worktree.Template{{Name: "autofix", Template: "autofix/{{.t}}"}}
	shown, _ := m.Update(key("t"))
	back, _ := shown.(model).Update(key("x"))
	if back.(model).mode != modeNormal {
		t.Errorf("any key should return to normal mode")
	}
}

func TestEnter_SelectsWorktreeAndQuits(t *testing.T) {
	m, rec := newTestModel(sample())
	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown}) // move to /repo.worktrees/feat
	done, cmd := down.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := done.(model).selected; got != "/repo.worktrees/feat" {
		t.Errorf("selected = %q, want /repo.worktrees/feat", got)
	}
	if cmd == nil {
		t.Fatal("Enter should return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("Enter command should produce tea.QuitMsg, got %T", cmd())
	}
	if len(*rec) != 0 {
		t.Errorf("Enter should not dispatch an action, got %v", *rec)
	}
}

func TestEnter_OnMainSelectsRepoRoot(t *testing.T) {
	m, _ := newTestModel(sample()) // cursor starts on /repo (main)
	done, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := done.(model).selected; got != "/repo" {
		t.Errorf("selected = %q, want /repo", got)
	}
	if cmd == nil {
		t.Fatal("Enter on main should still quit (cd to repo root)")
	}
}

func TestEnter_EmptyListDoesNothing(t *testing.T) {
	m := newModel(&fakeLister{}, "/repo", nil, nil)
	done, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := done.(model).selected; got != "" {
		t.Errorf("selected = %q, want empty", got)
	}
	if cmd != nil {
		t.Error("Enter with no items should not quit")
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

func TestActionFinished_SuccessShowsLogPath(t *testing.T) {
	m, _ := newTestModel(sample())
	m.status = "creating worktree…"
	updated, _ := m.Update(actionFinishedMsg{logPath: "/tmp/wt-action-9.log"})
	st := updated.(model).status
	if !strings.Contains(st, "/tmp/wt-action-9.log") {
		t.Errorf("success should show the log path, got %q", st)
	}
}

func TestActionFinished_SuccessNoLogClearsStatus(t *testing.T) {
	m, _ := newTestModel(sample())
	m.status = "creating worktree…"
	updated, _ := m.Update(actionFinishedMsg{})
	if updated.(model).status != "" {
		t.Errorf("success with no log should clear status, got %q", updated.(model).status)
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
