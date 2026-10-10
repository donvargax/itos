package draft

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSaveThenLoadKeepsTheDraftsInOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "itos", Folder, FileName)
	want := File{Drafts: []Draft{
		{ID: "say-new", Message: "docs: say new\n\nWhy it is new.", Paths: []string{"notes.md"}},
		{ID: "say-more", Message: "docs: say more", Paths: []string{"notes.md"}, Base: "0123abc"},
		{ID: "add-thing", Command: []string{"work", "add", "p1-thing", "--title", "A thing", "--why", "Because."}},
		{ID: "add-slice", Command: []string{"work", "add", "--kind", "slice", "--title", "A slice", "--why", "Because."}, Minted: "slice-2"},
	}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Drafts) != 4 || got.Drafts[0].ID != "say-new" || got.Drafts[1].ID != "say-more" || got.Drafts[2].ID != "add-thing" ||
		got.Drafts[2].Minted != "" || got.Drafts[3].Minted != "slice-2" {
		t.Fatalf("loaded %+v", got)
	}
	if got.Drafts[0].IsEdit() || !got.Drafts[1].IsEdit() || got.Drafts[1].Base != "0123abc" || got.Drafts[2].IsEdit() {
		t.Fatalf("loaded %+v: only say-more is an edit draft, from 0123abc", got)
	}
	if got.Drafts[0].Message != want.Drafts[0].Message || strings.Join(got.Drafts[2].Command, "|") != strings.Join(want.Drafts[2].Command, "|") {
		t.Fatalf("loaded %+v, not %+v", got, want)
	}
	// Windows keeps no Unix mode: a file there reads as 0666 whatever it
	// was made with.
	if info, err := os.Stat(path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("the list's mode is %v (%v), not 0600", info.Mode().Perm(), err)
	}
}

func TestLoadRefusesAListItosDidNotWrite(t *testing.T) {
	for name, text := range map[string]string{
		"empty":           "",
		"unknown key":     "drafts: []\nmore: 1\n",
		"two ids alike":   "drafts:\n  - {id: a, command: [work]}\n  - {id: a, command: [work]}\n",
		"no id":           "drafts:\n  - {command: [work]}\n",
		"both kinds":      "drafts:\n  - {id: a, command: [work], message: m, paths: [x]}\n",
		"a change half":   "drafts:\n  - {id: a, message: m}\n",
		"a based command": "drafts:\n  - {id: a, command: [work], base: abc}\n",
		"a minted change": "drafts:\n  - {id: a, message: m, paths: [x], minted: slice-2}\n",
		"two documents":   "drafts: []\n---\ndrafts: []\n",
		"an id with room": "drafts:\n  - {id: 'a b', command: [work]}\n",
	} {
		path := filepath.Join(t.TempDir(), FileName)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("%s: Load gave %v, not an error naming the file", name, err)
		}
	}
}

func TestLineQuotesWhatAShellWouldRead(t *testing.T) {
	d := Draft{Command: []string{"work", "add", "p1-thing", "--title", "A thing", "--why", "It's so."}}
	if got, want := d.Line(), `itos work add p1-thing --title 'A thing' --why 'It'\''s so.'`; got != want {
		t.Fatalf("Line() = %s, not %s", got, want)
	}
}

func TestHeaderIsTheMessagesFirstLine(t *testing.T) {
	if got := (Draft{Message: "docs: say new\n\nThe body."}).Header(); got != "docs: say new" {
		t.Fatalf("Header() = %q", got)
	}
}
