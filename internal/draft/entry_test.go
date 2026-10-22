package draft

import (
	"slices"
	"testing"
)

// A change's entry is its header and paths, a command's its arguments and
// the id it mints.
func TestEntryOf(t *testing.T) {
	change := EntryOf(Draft{ID: "d-1", Message: "docs: say it\n\nWhy.\n", Paths: []string{"a.md"}})
	if change.ID != "d-1" || change.Kind != "change" || change.Header != "docs: say it" || !slices.Equal(change.Paths, []string{"a.md"}) ||
		change.Command != nil || change.Minted != "" {
		t.Errorf("a change: %+v", change)
	}
	command := EntryOf(Draft{ID: "d-2", Command: []string{"work", "add", "x"}, Minted: "T-9"})
	if command.ID != "d-2" || command.Kind != "command" || command.Header != "" || command.Paths != nil ||
		!slices.Equal(command.Command, []string{"work", "add", "x"}) || command.Minted != "T-9" {
		t.Errorf("a command: %+v", command)
	}
}

// The lines say a change's header and paths, a command's line and the id it
// makes, indented two spaces, ids aligned.
func TestLines(t *testing.T) {
	got := Lines([]Draft{
		{ID: "d-1", Message: "docs: say it", Paths: []string{"a.md", "b.md"}},
		{ID: "d-10", Command: []string{"work", "add", "a b"}, Minted: "T-9"},
		{ID: "d-2", Command: []string{"work", "done", "T-1"}},
	})
	want := []string{
		"  d-1   docs: say it  (a.md, b.md)",
		"  d-10  itos work add 'a b'  (makes T-9)",
		"  d-2   itos work done T-1",
	}
	if !slices.Equal(got, want) {
		t.Errorf("%q\nnot\n%q", got, want)
	}
	if got := Lines([]Draft{{Command: []string{"go"}}}); !slices.Equal(got, []string{"    itos go"}) {
		t.Errorf("no id: %q", got)
	}
	if got := Lines(nil); len(got) != 0 {
		t.Errorf("no drafts: %q", got)
	}
}
