package tests

import (
	"os"
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
