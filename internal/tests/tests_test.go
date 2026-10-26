package tests

import (
	"os"
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

// A command adapter that cannot list a tree is asked for the working tree
// instead, and the listing warns on the run's warnings; at the working tree
// it warns nothing.
func TestListTestsWarnsOnTheRunsWarnings(t *testing.T) {
	t.Chdir(t.TempDir())
	adapter := `[ "$3" = worktree ] || exit 1
printf '{"protocol":1,"tests":[],"files":["a.test.ts"]}\n'
`
	if err := os.WriteFile("adapter.sh", []byte(adapter), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := load(t, "version: 1\ntests:\n  unit: { adapter: { command: \"sh adapter.sh\", supports_at: false }, root: src }\n")
	var warned strings.Builder
	ws := NewWarnings(&warned)
	if list, err := ListTests(cfg, "unit", "worktree", ws); err != nil || len(list.Files) != 1 || warned.Len() != 0 {
		t.Fatalf("at the working tree: %+v, %v, warned %q", list, err, warned.String())
	}
	if list, err := ListTests(cfg, "unit", "HEAD", ws); err != nil || len(list.Files) != 1 {
		t.Fatalf("at HEAD: %+v, %v", list, err)
	}
	if want := "tests.unit cannot list a tree; HEAD is read as the working tree\n"; warned.String() != want {
		t.Errorf("warned %q, want %q", warned.String(), want)
	}
}

// A Gherkin kind, the default adapter, is read by the built-in reader and
// lists its feature's scenario; an adapter that is neither a command nor
// gherkin is refused, naming it.
func TestListTestsReadsOnlyTheBuiltInGherkinAdapter(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("features", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("features/a.feature", []byte("Feature: a\n\n  @ID-A-01\n  Scenario: one\n    Given x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := load(t, "version: 1\ntests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\n  other: { adapter: nope, root: features, id: X }\n")
	if list, err := ListTests(cfg, "scenario", "worktree", nil); err != nil || len(list.Tests) != 1 || list.Tests[0].ID != "ID-A-01" {
		t.Errorf("the gherkin kind: %+v, %v", list, err)
	}
	if _, err := ListTests(cfg, "other", "worktree", nil); err == nil || !strings.Contains(err.Error(), "tests.other.adapter nope is not built in") {
		t.Errorf("an adapter of another name: %v", err)
	}
}
