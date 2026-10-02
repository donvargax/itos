package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func load(t *testing.T, text string) (*Loaded, error) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(file)
}

func TestLoadReadsRequires(t *testing.T) {
	c, err := load(t, "version: 1\nrequires: \">=0.1.0\"\n")
	if err != nil || c.Requires == nil || *c.Requires != ">=0.1.0" {
		t.Fatalf("got %+v, %v", c, err)
	}
	c, err = load(t, "version: 1.0\n")
	if err != nil || c.Requires != nil {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func messages(t *testing.T, err error) []string {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v, want a config error", err)
	}
	var found []string
	for _, p := range e.Problems {
		found = append(found, p.Message)
	}
	return found
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
		// YAML 1.1's forms are text to the core schema: 1_0 is no number.
		"version: 1_0\n": {"version should be a number, not string"},
		"version: 1\nhooks: { bim: x, shims: true }\n": {
			"unknown key hooks.bim", "unknown key hooks.shims",
		},
		"version: 1\ncommits: { scopes: { docs: { only: [$prose] } } }\n": {
			"commits.scopes.docs.only names $prose, which commits.path_sets does not have",
		},
	}
	for text, want := range cases {
		_, err := load(t, text)
		if got := messages(t, err); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

// An unknown key's fix renames it to the key it misspells, else lists the
// keys there are.
func TestUnknownKeyFix(t *testing.T) {
	_, err := load(t, "version: 1\nhooks: { bim: x, shims: true }\n")
	var e *Error
	errors.As(err, &e)
	if e.Problems[0].Fix != "rename hooks.bim to bin" {
		t.Errorf("near: %q", e.Problems[0].Fix)
	}
	if e.Problems[1].Fix != "remove hooks.shims; the keys here are manager, bin, pre_push, commit_msg" {
		t.Errorf("far: %q", e.Problems[1].Fix)
	}
}

// The file is laid over the one table: what it leaves out is the table's,
// work.registry beside its ledger, each kind over the per-kind defaults.
func TestLoadLaysTheFileOverTheDefaults(t *testing.T) {
	c, err := load(t, "version: 1\nledger: { files: \"work/phase-{group}.yaml\", check: { timeout: 5 } }\ntests: { k: { root: f, run: { whole: w } } }\n")
	if err != nil {
		t.Fatal(err)
	}
	if c.Ledger.Check.Timeout != 5 || c.Ledger.Group.Label != "phase" || c.Ledger.ID != nil {
		t.Errorf("ledger: %+v", c.Ledger)
	}
	if c.Work.Registry != "work/work-items.yaml" || c.Hooks.Bin != "tools/bin/itos" || !c.CI.StopAtFirstFailure {
		t.Errorf("registry %q, bin %q, stop %v", c.Work.Registry, c.Hooks.Bin, c.CI.StopAtFirstFailure)
	}
	k, ok := c.Tests.Get("k")
	if !ok || *k.Root != "f" || *k.Run.Whole != "w" || k.Run.Join.Sep != "|" || k.TagPrefix != "@" || k.Adapter.Name != "gherkin" || !k.Smoke.EveryFile {
		t.Errorf("kind: %+v", k)
	}
	if !c.HasSection("ledger") || c.HasSection("ci") || c.Section("ci") == nil {
		t.Error("sections")
	}
}

// hooks.bin as a command's first word is also read as itos, and only then.
func TestReadings(t *testing.T) {
	c, err := load(t, "version: 1\nci: { steps: [], cost: { static: [\"^itos work check$\"] } }\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Readings("  tools/bin/itos   work check "); !slices.Equal(got, []string{"tools/bin/itos   work check", "itos   work check"}) {
		t.Errorf("readings: %q", got)
	}
	if !c.MatchesStatic("tools/bin/itos work  check") || c.MatchesStatic("other/itos work check") {
		t.Error("matches")
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
