package tests

import (
	"strings"
	"testing"
)

// A run warns once per kind on its own writer; another run warns again, and
// nil warns nowhere.
func TestWarningsOncePerRun(t *testing.T) {
	var first, second strings.Builder
	run := NewWarnings(&first)
	run.cannotList("unit", "HEAD")
	run.cannotList("unit", "index")
	run.cannotList("e2e", "HEAD")
	want := "tests.unit cannot list a tree; HEAD is read as the working tree\n" +
		"tests.e2e cannot list a tree; HEAD is read as the working tree\n"
	if first.String() != want {
		t.Errorf("one run warned %q, want %q", first.String(), want)
	}
	NewWarnings(&second).cannotList("unit", "index")
	if want := "tests.unit cannot list a tree; index is read as the working tree\n"; second.String() != want {
		t.Errorf("another run warned %q, want %q", second.String(), want)
	}
	var none *Warnings
	none.cannotList("unit", "HEAD")
}
