package cmd

import (
	"bytes"
	"testing"

	"github.com/homeend/worktrees/pkg/worktree"
)

func TestPrintTemplates_Lists(t *testing.T) {
	var out bytes.Buffer
	err := printTemplates(&out, []worktree.Template{
		{Name: "autofix", Template: "autofix/{{.ticketName}}"},
		{Name: "feature", Template: "feat/{{.ticketName}}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("autofix")) || !bytes.Contains(out.Bytes(), []byte("feat/{{.ticketName}}")) {
		t.Errorf("output missing templates:\n%s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("1")) || !bytes.Contains(out.Bytes(), []byte("2")) {
		t.Errorf("output missing 1-based indices:\n%s", out.String())
	}
}

func TestPrintTemplates_Empty(t *testing.T) {
	var out bytes.Buffer
	if err := printTemplates(&out, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("no templates defined")) {
		t.Errorf("empty output = %q", out.String())
	}
}
