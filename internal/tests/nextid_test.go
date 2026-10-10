package tests

import (
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
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
