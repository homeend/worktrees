package cmd

import (
	"bytes"
	"testing"

	"github.com/homeend/worktrees/internal/config"
	"github.com/homeend/worktrees/pkg/worktree"
)

func TestPrintTemplates_ListsNamed(t *testing.T) {
	var out bytes.Buffer
	err := printTemplates(&out, []worktree.Template{
		{Name: "fix", Template: "fix/<user:ticket>"},
		{Name: "spike", Template: "spike/<date>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("fix/<user:ticket>")) || !bytes.Contains(out.Bytes(), []byte("spike")) {
		t.Errorf("output missing templates:\n%s", out.String())
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

func TestTemplateSlice_SortedByName(t *testing.T) {
	cfg := config.Config{Templates: map[string]string{
		"zeta": "z/<date>",
		"alfa": "a/<date>",
	}}
	got := templateSlice(cfg)
	if len(got) != 2 || got[0].Name != "alfa" || got[1].Name != "zeta" {
		t.Errorf("templateSlice = %+v, want sorted by name", got)
	}
}
