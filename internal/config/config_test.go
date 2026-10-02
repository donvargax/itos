package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func load(t *testing.T, text string) (*Config, error) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(file)
}

func TestLoadReadsRequires(t *testing.T) {
	c, err := load(t, "version: 1\nrequires: \">=0.1.0\"\n")
	if err != nil || !c.HasRequires || c.Requires != ">=0.1.0" {
		t.Fatalf("got %+v, %v", c, err)
	}
	c, err = load(t, "version: 1.0\n")
	if err != nil || c.HasRequires {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestLoadProblems(t *testing.T) {
	cases := map[string][]string{
		"version: 2\n":                   {"version 2 is not 1"},
		"version: 1.5\n":                 {"version 1.5 is not 1"},
		"requires: x\n":                  {"version is missing"},
		"version: one\nrequires: [a]\n":  {"version should be a number, not string", "requires should be a string, not a list"},
		"- version: 1\n":                 {"the file should be a mapping, not a list"},
		"":                               {"the file should be a mapping, not null"},
		"version: 1\nrequires: {a: b}\n": {"requires should be a string, not object"},
	}
	for text, want := range cases {
		_, err := load(t, text)
		var e *Error
		if !errors.As(err, &e) {
			t.Errorf("%q: got %v, want a config error", text, err)
			continue
		}
		if len(e.Problems) != len(want) {
			t.Errorf("%q: got %v, want %v", text, e.Problems, want)
			continue
		}
		for i, p := range e.Problems {
			if p.Message != want[i] {
				t.Errorf("%q: problem %d is %q, want %q", text, i, p.Message, want[i])
			}
		}
	}
}

func TestPath(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	if Path() != "itos.yaml" {
		t.Errorf("Path() = %q without ITOS_CONFIG", Path())
	}
	t.Setenv("ITOS_CONFIG", "other.yaml")
	if Path() != "other.yaml" {
		t.Errorf("Path() = %q with ITOS_CONFIG=other.yaml", Path())
	}
}
