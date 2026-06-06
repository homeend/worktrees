package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/homeend/worktrees/pkg/worktree"
)

// fakeKiller implements the killer interface used by runKillEmAll.
type fakeKiller struct {
	plan worktree.RemoveAllPlan
	res  worktree.RemoveAllResult
}

func (f fakeKiller) PlanRemoveAll(string) (worktree.RemoveAllPlan, error) { return f.plan, nil }
func (f fakeKiller) RemoveAll(string) (worktree.RemoveAllResult, error)   { return f.res, nil }

func TestRunKillEmAll_EmptyPlan(t *testing.T) {
	var out bytes.Buffer
	err := runKillEmAll(fakeKiller{}, "/repo", killOpts{yes: true, isTTY: false}, &out)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("nothing to remove")) {
		t.Errorf("output = %q, want 'nothing to remove'", out.String())
	}
}

func TestRunKillEmAll_RefusesWithoutYesNoTTY(t *testing.T) {
	var out bytes.Buffer
	k := fakeKiller{plan: worktree.RemoveAllPlan{Branches: []string{"wt/a"}}}
	err := runKillEmAll(k, "/repo", killOpts{yes: false, isTTY: false}, &out)
	if err == nil {
		t.Error("expected refusal error without --yes and no TTY")
	}
}

func TestRunKillEmAll_PartialFailureReturnsSentinel(t *testing.T) {
	var out bytes.Buffer
	k := fakeKiller{
		plan: worktree.RemoveAllPlan{Branches: []string{"wt/a"}},
		res:  worktree.RemoveAllResult{Failures: []worktree.CleanupFailure{{Kind: "branch", Ref: "wt/a", Err: "boom"}}},
	}
	err := runKillEmAll(k, "/repo", killOpts{yes: true, isTTY: false}, &out)
	if !errors.Is(err, ErrPartialCleanup) {
		t.Errorf("err = %v, want ErrPartialCleanup", err)
	}
}

func TestRunKillEmAll_TTYPromptYesProceeds(t *testing.T) {
	var out bytes.Buffer
	k := fakeKiller{
		plan: worktree.RemoveAllPlan{Branches: []string{"wt/a"}},
		res:  worktree.RemoveAllResult{BranchesDeleted: 1},
	}
	in := bytes.NewBufferString("y\n")
	err := runKillEmAll(k, "/repo", killOpts{yes: false, isTTY: true, in: in}, &out)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("deleted 1 branch")) {
		t.Errorf("output = %q, want summary", out.String())
	}
}

func TestExitCodeFor_PartialCleanup(t *testing.T) {
	err := errors.Join(ErrPartialCleanup, errors.New("2 failures"))
	if code := exitCodeFor(err); code != 6 {
		t.Errorf("exitCodeFor(ErrPartialCleanup) = %d, want 6", code)
	}
}
