package naming

import (
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

func testCtx() Ctx {
	return Ctx{
		ParentBranch: "feature/login",
		Repo:         "myrepo",
		Seqs:         map[string]int{"wt": 4},
		Now:          func() time.Time { return time.Date(2026, 7, 22, 21, 30, 5, 0, time.UTC) },
		Rand:         rand.New(rand.NewPCG(1, 2)),
	}
}

func TestResolve_BuiltinTokens(t *testing.T) {
	cases := []struct {
		tmpl, want string
	}{
		{"fix/<repo>-x", "fix/myrepo-x"},
		{"<parent-branch>-hotfix", "feature/login-hotfix"},
		{"d-<date>", "d-2026-07-22"},
		{"d-<date:yyyyMMdd-HHmm>", "d-20260722-2130"},
		{"s-<seq:wt>", "s-4"},
		{"s-<seq:wt:3>", "s-004"},
		{"s-<seq:other>", "s-0"},
	}
	for _, c := range cases {
		got, err := Resolve(c.tmpl, nil, testCtx())
		if err != nil {
			t.Errorf("Resolve(%q): %v", c.tmpl, err)
			continue
		}
		if got != c.want {
			t.Errorf("Resolve(%q) = %q, want %q", c.tmpl, got, c.want)
		}
	}
}

func TestResolve_UserInputs(t *testing.T) {
	got, err := Resolve("fix/<user:ticket>-review", map[string]string{"ticket": "GH-42"}, testCtx())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "fix/GH-42-review" {
		t.Errorf("got %q, want fix/GH-42-review", got)
	}
	if _, err := Resolve("fix/<user:ticket>", nil, testCtx()); err == nil {
		t.Error("missing user input should error")
	}
}

func TestResolve_RandomTokens(t *testing.T) {
	got, err := Resolve("<random-alpha:4>-<random-num:3>", nil, testCtx())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	parts := strings.SplitN(got, "-", 2)
	if len(parts[0]) != 4 || len(parts[1]) != 3 {
		t.Errorf("lengths wrong in %q", got)
	}
}

func TestResolve_Errors(t *testing.T) {
	for _, tmpl := range []string{"<bogus>", "<seq>", "<user>", "<random-alpha>", "<seq:x:-1>"} {
		if _, err := Resolve(tmpl, nil, testCtx()); err == nil {
			t.Errorf("Resolve(%q) should error", tmpl)
		}
	}
}

func TestUserLabelsAndSeqNames(t *testing.T) {
	tmpl := "f/<user:ticket>-<seq:wt:3>-<user:who>-<user:ticket>-<seq:other>"
	if got := UserLabels(tmpl); len(got) != 2 || got[0] != "ticket" || got[1] != "who" {
		t.Errorf("UserLabels = %v, want [ticket who]", got)
	}
	if got := SeqNames(tmpl); len(got) != 2 || got[0] != "wt" || got[1] != "other" {
		t.Errorf("SeqNames = %v, want [wt other]", got)
	}
}

func TestSanitizeSegment(t *testing.T) {
	cases := []struct {
		in, want string
		goos     string
	}{
		{"feature/login-v2", "feature-login-v2", "linux"},
		{`a\b`, "a-b", "linux"},
		{"x:y", "x-y", "windows"},
		{"con", "con_", "windows"},
		{"name. ", "name", "windows"},
		{"", "tag", "linux"},
	}
	for _, c := range cases {
		if got := SanitizeSegmentFor(c.in, c.goos); got != c.want {
			t.Errorf("SanitizeSegmentFor(%q, %s) = %q, want %q", c.in, c.goos, got, c.want)
		}
	}
}
