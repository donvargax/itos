package tests

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

// An area is a stem the kind's ID- pattern takes; slice, bug, a stem the
// pattern does not take, a pattern that is no ID- one, a kind with none
// and a pattern that does not compile (which config.Load refuses) are not.
func TestAreaIsAStemTheKindsIDPatternTakes(t *testing.T) {
	cfg := load(t, `version: 1
tests:
  scenario: { root: features, id: "^ID-[A-Z]+-\\d+$" }
  lower: { root: features, id: "[a-z]+-\\d+" }
  none: { root: features }
`)
	for _, c := range []struct {
		kind, stem string
		want       bool
	}{
		{"scenario", "ID-A", true},
		{"scenario", "ID-NEW", true},
		{"scenario", "slice", false},
		{"scenario", "bug", false},
		{"scenario", "ID-a", false},
		{"lower", "slice", false},
		{"none", "ID-A", false},
	} {
		k, err := KindOf(cfg, c.kind)
		if err != nil {
			t.Fatal(err)
		}
		if got := Area(k, c.stem); got != c.want {
			t.Errorf("Area(%s, %q) = %v, want %v", c.kind, c.stem, got, c.want)
		}
	}
	broken := "ID-("
	if Area(config.Kind{ID: &broken}, "ID-A") {
		t.Error("Area took a stem by a pattern that does not compile")
	}
}

// A kind behind a command adapter has no tags to read: the IDs its listing
// gives at the working tree count, and a listing that fails stops it.
func TestNextTagCountsACommandKindsListedIDs(t *testing.T) {
	t.Chdir(t.TempDir())
	lists := `printf '%s\n' '{"protocol":1,"tests":[{"id":"U-3","file":"a_test.go","live":true}],"files":["a_test.go"]}'` + "\n"
	if err := os.WriteFile("lists.sh", []byte(lists), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := load(t, "version: 1\ntests:\n  unit: { adapter: { command: sh lists.sh }, tag_prefix: '' }\n  broken: { adapter: { command: \"exit 3\" } }\n")
	if got, err := NextTag(cfg, "unit", "U", []string{"U-1"}); err != nil || got != "U-4" {
		t.Errorf("NextTag of a listed U-3 = %q, %v, want U-4", got, err)
	}
	if got, err := NextTag(cfg, "broken", "U", nil); err == nil {
		t.Errorf("NextTag of a kind whose listing fails = %q, want an error", got)
	}
}

// A Gherkin kind's tags and the others are counted, a kind the config lacks
// and a feature file that cannot be read stop the ids from being gathered,
// and NextTag with them.
func TestNextTagCountsAGherkinKindsTagsAndStopsAtWhatItCannotRead(t *testing.T) {
	if _, err := NextTag(&config.Loaded{}, "scenario", "ID-A", nil); err == nil {
		t.Error("NextTag of a kind the config lacks: no error")
	}
	pattern := `ID-[A-Z]+-\d+`
	kindOver := func(root string) *config.Loaded {
		k := config.Kind{Root: &root, ID: &pattern, TagPrefix: "@"}
		k.Adapter.Name = "gherkin"
		cfg := &config.Loaded{}
		cfg.Tests.Keys = []string{"scenario"}
		cfg.Tests.Values = map[string]config.Kind{"scenario": k}
		return cfg
	}
	good := t.TempDir()
	if err := os.WriteFile(filepath.Join(good, "a.feature"), []byte("Feature: A\n\n  @ID-A-01 @slice-9\n  Scenario: one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := NextTag(kindOver(good), "scenario", "ID-A", []string{"ID-A-03"}); err != nil || got != "@ID-A-04" {
		t.Errorf("NextTag of the file's ID-A-01 and another's ID-A-03 = %q, %v, want @ID-A-04", got, err)
	}
	held, err := HeldIDs(kindOver(good), "scenario", kindOver(good).Tests.Values["scenario"], []string{"slice-3"})
	if err != nil || !slices.Equal(held, []string{"slice-3", "ID-A-01", "slice-9"}) {
		t.Errorf("HeldIDs = %q, %v, want the registry's ids then the file's tags", held, err)
	}
	noRoot := kindOver(good)
	k := noRoot.Tests.Values["scenario"]
	k.Root = nil
	noRoot.Tests.Values["scenario"] = k
	if got, err := NextTag(noRoot, "scenario", "ID-A", nil); err == nil {
		t.Errorf("NextTag of a kind with no root = %q, want an error", got)
	}
	dangling := t.TempDir()
	if err := os.Symlink(filepath.Join(dangling, "nothing"), filepath.Join(dangling, "b.feature")); err != nil {
		t.Skip("no symbolic links here:", err)
	}
	if got, err := NextTag(kindOver(dangling), "scenario", "ID-A", nil); err == nil {
		t.Errorf("NextTag over a feature file that cannot be read = %q, want an error", got)
	}
}

// A bare stem is a mistaken area when the pattern's text begins with ID- (a
// ^ dropped), the stem is no area and ID-<stem> is, it is neither slice nor
// bug, and no held id is <stem>-<digits>; each condition alone keeps it a
// preview.
func TestMistakenIsABareStemWhoseFullStemIsAnArea(t *testing.T) {
	cfg := load(t, `version: 1
tests:
  scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
  anchored: { root: features, id: "^ID-[A-Z]+-\\d+$" }
  lower: { root: features, id: "ID-[A-Za-z]+-\\d+" }
  alternate: { root: features, id: "T-\\d+-[a-z]+|ID-[A-Z]+-\\d+" }
  none: { root: features }
`)
	for _, c := range []struct {
		kind, stem string
		held       []string
		want       string
	}{
		{"scenario", "SEC", nil, "ID-SEC"},
		{"scenario", "SEC", []string{"ID-SEC-01", "SECX-1", "SEC-x", "SEC-"}, "ID-SEC"},
		{"anchored", "SEC", nil, "ID-SEC"},
		{"lower", "sec", []string{"batch-7"}, "ID-sec"},
		{"scenario", "SEC", []string{"SEC-1"}, ""},
		{"scenario", "ID-SEC", nil, ""},
		{"scenario", "sec", nil, ""},
		{"lower", "slice", nil, ""},
		{"lower", "bug", nil, ""},
		{"lower", "batch", []string{"batch-7"}, ""},
		{"alternate", "SEC", nil, ""},
		{"none", "SEC", nil, ""},
	} {
		k, err := KindOf(cfg, c.kind)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := Mistaken(k, c.stem, c.held)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("Mistaken(%s, %q, %q) = %q, %v, want %q", c.kind, c.stem, c.held, got, ok, c.want)
		}
	}
}
