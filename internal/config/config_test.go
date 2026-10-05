package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/donvargax/itos/v4/internal/value"
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
	if c.Work.Registry != "work/work-items.yaml" || c.Hooks.Bin != "itos" || !c.CI.StopAtFirstFailure {
		t.Errorf("registry %q, bin %q, stop %v", c.Work.Registry, c.Hooks.Bin, c.CI.StopAtFirstFailure)
	}
	if c.Work.Asks != "work/asks.yaml" {
		t.Errorf("asks %q, not beside the registry", c.Work.Asks)
	}
	k, ok := c.Tests.Get("k")
	if !ok || *k.Root != "f" || *k.Run.Whole != "w" || k.Run.Join.Sep != "|" || k.TagPrefix != "@" || k.Adapter.Name != "gherkin" || !k.Smoke.EveryFile {
		t.Errorf("kind: %+v", k)
	}
	if !c.HasSection("ledger") || c.HasSection("ci") || c.Section("ci") == nil {
		t.Error("sections")
	}
}

// work.asks is beside the registry the file names, wherever that is, unless
// the file names it too (slice 62).
func TestAsksBesideTheRegistry(t *testing.T) {
	for text, want := range map[string]string{
		"version: 1\n": "tasks/asks.yaml",
		"version: 1\nwork: { registry: work-items.yaml }\n":                     "asks.yaml",
		"version: 1\nwork: { registry: data/reg.yaml }\n":                       "data/asks.yaml",
		"version: 1\nwork: { registry: data/reg.yaml, asks: questions.yaml }\n": "questions.yaml",
	} {
		c, err := load(t, text)
		if err != nil {
			t.Fatal(err)
		}
		if c.Work.Asks != want {
			t.Errorf("%q: asks %q, want %q", text, c.Work.Asks, want)
		}
	}
}

// hooks.bin as a command's first word is also read as itos, and only then.
func TestReadings(t *testing.T) {
	c, err := load(t, "version: 1\nci: { steps: [], cost: { static: [\"^itos work check$\"] } }\nhooks: { bin: tools/bin/itos }\n")
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

func TestKnownKey(t *testing.T) {
	for _, key := range []string{
		"version", "hooks.bin", "hooks.commit_msg.check_timeout", "tests.scenario.root",
		"tests.scenario.adapter.command", "commits.footers.Task.source", "ci.steps", "pin",
	} {
		if known, _ := KnownKey(key); !known {
			t.Errorf("%s is not known", key)
		}
	}
	cases := map[string][]string{
		"hooks.no_such_key": {"manager", "bin", "pre_push", "commit_msg"},
		"hooks.bin.x":       nil,
		"ci.steps.0":        nil,
		"hooks.":            {"manager", "bin", "pre_push", "commit_msg"},
	}
	for key, want := range cases {
		known, keys := KnownKey(key)
		if known || !slices.Equal(keys, want) {
			t.Errorf("%s: got %v %v, want false %v", key, known, keys, want)
		}
	}
}

func TestGet(t *testing.T) {
	c, err := load(t, "version: 1\nhooks: { bin: bin/mine }\ntests: { scenario: { root: features } }\n")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]any{
		"hooks.bin":                      c.Get("hooks.bin"),
		"hooks.commit_msg.check_timeout": c.Get("hooks.commit_msg.check_timeout"),
		"tests.scenario.wip_tag":         c.Get("tests.scenario.wip_tag"),
		"ledger.id":                      c.Get("ledger.id"),
		"hooks.bin.x":                    c.Get("hooks.bin.x"),
	}
	want := map[string]any{
		"hooks.bin":                      "bin/mine",
		"hooks.commit_msg.check_timeout": 60.0,
		"tests.scenario.wip_tag":         "@wip",
		"ledger.id":                      value.Undefined,
		"hooks.bin.x":                    value.Undefined,
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s: got %v, want %v", key, got[key], w)
		}
	}
}

// A footer's in_place_of names other footers of IDs, for commit types
// (slice 63); a footer of free text neither stands in nor is stood in for.
func TestInPlaceOfProblems(t *testing.T) {
	const head = "version: 1\ncommits:\n  types: [test, docs]\n  footers:\n" +
		"    Task: { source: ledger, required_for: [test] }\n" +
		"    Upgrading: { source: text }\n"
	cases := map[string][]string{
		"    Item: { source: registry, in_place_of: { Task: [test, docs] } }\n": nil,
		"    Item: { source: registry, in_place_of: { Task: all } }\n":          nil,
		"    Item: { source: registry, in_place_of: { Item: [test] } }\n": {
			"commits.footers.Item.in_place_of names Item itself",
		},
		"    Item: { source: registry, in_place_of: { Tusk: [test] } }\n": {
			"commits.footers.Item.in_place_of names Tusk, which is not one of commits.footers",
		},
		"    Item: { source: registry, in_place_of: { Upgrading: [test] } }\n": {
			"commits.footers.Item.in_place_of names Upgrading, a footer of free text, which no ID stands in for",
		},
		"    Item: { source: registry, in_place_of: { Task: [chore] } }\n": {
			"commits.footers.Item.in_place_of.Task names chore, which is not one of commits.types",
		},
		"    Note: { source: text, in_place_of: { Task: [test] } }\n": {
			"commits.footers.Note.in_place_of is for a footer of IDs, and commits.footers.Note is free text (source: text)",
		},
	}
	for footer, want := range cases {
		_, err := load(t, head+footer)
		if want == nil {
			if err != nil {
				t.Errorf("%q: got %v, want none", footer, err)
			}
			continue
		}
		if got := messages(t, err); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", footer, got, want)
		}
	}
}

// A ci section may hold the watch alone (bug 26); a tool that needs its
// steps asks for ci.steps, a key below the section, by its dotted path.
func TestSectionReadsADottedKey(t *testing.T) {
	c, err := load(t, "version: 1\nci:\n  watch: { provider: github }\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Section("ci"); err != nil {
		t.Fatalf("ci: %v", err)
	}
	if got := messages(t, c.Section("ci.steps")); !slices.Equal(got, []string{"ci.steps is missing"}) {
		t.Fatalf("ci.steps: %v", got)
	}
	if got := messages(t, c.Section("ledger.files")); !slices.Equal(got, []string{"ledger.files is missing"}) {
		t.Fatalf("ledger.files: %v", got)
	}
	c, err = load(t, "version: 1\nci:\n  steps: []\n")
	if err != nil || c.Section("ci.steps") != nil {
		t.Fatalf("ci.steps written: %v", err)
	}
}
