package cmd

import (
	"strings"
	"testing"

	"github.com/homeend/worktrees/internal/config"
)

func TestParseVars_EqualsSyntax(t *testing.T) {
	vars, err := parseVars([]string{"ticket=GH-42", "note=a=b"})
	if err != nil {
		t.Fatalf("parseVars: %v", err)
	}
	if vars["ticket"] != "GH-42" {
		t.Errorf("ticket = %q", vars["ticket"])
	}
	if vars["note"] != "a=b" {
		t.Errorf("value may contain '=', got %q", vars["note"])
	}
	if _, err := parseVars([]string{"no-equals"}); err == nil {
		t.Error("missing '=' should error")
	}
	if _, err := parseVars([]string{"=v"}); err == nil {
		t.Error("empty key should error")
	}
}

func TestPromptMissing_InteractiveReadsValues(t *testing.T) {
	vars := map[string]string{"ticket": "GH-1"}
	in := strings.NewReader("alice\n")
	var out strings.Builder
	err := promptMissing(vars, []string{"ticket", "who"}, in, &out, true)
	if err != nil {
		t.Fatalf("promptMissing: %v", err)
	}
	if vars["who"] != "alice" {
		t.Errorf("who = %q, want alice", vars["who"])
	}
	if !strings.Contains(out.String(), "who: ") {
		t.Errorf("prompt should name the label, got %q", out.String())
	}
	if strings.Contains(out.String(), "ticket: ") {
		t.Errorf("already-supplied label must not be prompted, got %q", out.String())
	}
}

func TestPromptMissing_NonInteractiveErrors(t *testing.T) {
	err := promptMissing(map[string]string{}, []string{"ticket"}, strings.NewReader(""), &strings.Builder{}, false)
	if err == nil || !strings.Contains(err.Error(), "ticket") {
		t.Errorf("non-interactive missing label should error naming it, got %v", err)
	}
}

func TestPromptMissing_EmptyValueErrors(t *testing.T) {
	err := promptMissing(map[string]string{}, []string{"ticket"}, strings.NewReader("\n"), &strings.Builder{}, true)
	if err == nil {
		t.Error("empty interactive value should error")
	}
}

func TestLookupTemplate(t *testing.T) {
	cfg := config.Config{Templates: map[string]string{"fix": "fix/<user:ticket>"}}
	tmpl, err := lookupTemplate(cfg, "fix")
	if err != nil || tmpl != "fix/<user:ticket>" {
		t.Fatalf("lookupTemplate = %q, %v", tmpl, err)
	}
	_, err = lookupTemplate(cfg, "nope")
	if err == nil || !strings.Contains(err.Error(), "fix") {
		t.Errorf("unknown template error should list available names, got %v", err)
	}
	_, err = lookupTemplate(config.Config{}, "nope")
	if err == nil || !strings.Contains(err.Error(), "none configured") {
		t.Errorf("no-templates error should say none configured, got %v", err)
	}
}
