package cmd

import (
	"testing"
)

type fakeResolver struct {
	name string
	err  error
}

func (f fakeResolver) ResolveTemplate(string, map[string]string) (string, error) {
	return f.name, f.err
}

func TestParseVars(t *testing.T) {
	vars, err := parseVars([]string{"a:1", "b:2:3"})
	if err != nil {
		t.Fatal(err)
	}
	if vars["a"] != "1" || vars["b"] != "2:3" {
		t.Errorf("vars = %+v", vars)
	}
	if _, err := parseVars([]string{"nocolon"}); err == nil {
		t.Error("missing colon should error")
	}
	if _, err := parseVars([]string{":v"}); err == nil {
		t.Error("empty key should error")
	}
}

func TestBuildAddOptions_Template(t *testing.T) {
	opts, err := buildAddOptions(fakeResolver{name: "autofix/ZX-12"},
		[]string{"ticketName:ZX-12"}, "autofix", "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Name != "autofix/ZX-12" {
		t.Errorf("Name = %q, want autofix/ZX-12", opts.Name)
	}
}

func TestBuildAddOptions_FromBranch(t *testing.T) {
	opts, err := buildAddOptions(fakeResolver{}, nil, "", "feature/login", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if opts.FromBranch != "feature/login" {
		t.Errorf("FromBranch = %q", opts.FromBranch)
	}
	if _, err := buildAddOptions(fakeResolver{}, []string{"x"}, "", "feature/login", "", "", false); err == nil {
		t.Error("--from-branch with a positional arg should error")
	}
}

func TestBuildAddOptions_MutualExclusion(t *testing.T) {
	if _, err := buildAddOptions(fakeResolver{}, nil, "autofix", "", "feat", "", false); err == nil {
		t.Error("--template + --branch should be mutually exclusive")
	}
	if _, err := buildAddOptions(fakeResolver{}, nil, "autofix", "feature/x", "", "", false); err == nil {
		t.Error("--template + --from-branch should be mutually exclusive")
	}
}

func TestBuildAddOptions_PlainName(t *testing.T) {
	opts, err := buildAddOptions(fakeResolver{}, []string{"hotfix"}, "", "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Name != "hotfix" {
		t.Errorf("Name = %q, want hotfix", opts.Name)
	}
	if _, err := buildAddOptions(fakeResolver{}, []string{"a", "b"}, "", "", "", "", false); err == nil {
		t.Error("more than one positional name should error")
	}
}
