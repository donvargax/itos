package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// Show and Log read a commit whose message holds the byte 0x1e whole, the
// commits around it too (bug-157).
func TestLogReadsAMessageHoldingTheRecordSeparator(t *testing.T) {
	dir := filepath.Join(alone(t), "r")
	gitIn(t, "", "", "init", "-q", "-b", "main", dir)
	message := "b: tidy\n\nA line with \x1e in it.\n\nTask: T-1\n"
	for _, m := range []string{"a", message, "c"} {
		gitIn(t, dir, "", "commit", "-q", "--allow-empty", "-m", m)
	}
	gitIn(t, dir, "", "notes", "--ref="+port.NotesRef, "add", "-m", "Note \x1e here", "HEAD~1")
	g := Git{Dir: dir}
	logged, err := g.Log(t.Context(), port.Range{To: "HEAD"})
	if err != nil || len(logged) != 3 || logged[0].Subject != "a" || logged[2].Subject != "c" {
		t.Fatalf("Log is %+v, %v; want a, b and c", logged, err)
	}
	shown, err := g.Show(t.Context(), "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range []port.Entry{logged[1], shown} {
		if got.Subject != "b: tidy" || got.Message != message || got.Note != "Note \x1e here\n" {
			t.Errorf("the commit holding the byte is read as %+v", got)
		}
	}
}

// Commits and Log read a branch whose name is also a folder of the work tree
// as the branch.
func TestLogReadsABranchNamedAsAFolder(t *testing.T) {
	dir := filepath.Join(alone(t), "r")
	gitIn(t, "", "", "init", "-q", "-b", "main", dir)
	gitIn(t, dir, "", "commit", "-q", "--allow-empty", "-m", "a")
	gitIn(t, dir, "", "branch", "docs")
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	g := Git{Dir: dir}
	if shas, err := g.Commits(t.Context(), port.Range{To: "docs"}); err != nil || len(shas) != 1 {
		t.Errorf("Commits of docs is %v, %v; want its one commit", shas, err)
	}
	if logged, err := g.Log(t.Context(), port.Range{To: "docs"}); err != nil || len(logged) != 1 || logged[0].Subject != "a" {
		t.Errorf("Log of docs is %+v, %v; want a", logged, err)
	}
}

// record is a record as logArgs prints it of the commit sha, its note note.
func record(sha, note string) string {
	return strings.Join([]string{sha, sha[:2], "s", "t", "t@t", "1 +0000", "", "s\n", note}, "\x00") + "\x00"
}

// entries reads the records of the commits named, in their order, and makes
// any record that is not its commit's, whole, an error naming the commit.
func TestEntriesReadOnlyTheRecordsOfTheCommitsNamed(t *testing.T) {
	got, err := entries(record("aa", "n\n")+record("bb", ""), []string{"aa", "bb"})
	if err != nil || len(got) != 2 || got[0].SHA != "aa" || got[0].Note != "n\n" || got[1].SHA != "bb" || got[1].Message != "s\n" {
		t.Errorf("two sound records are %+v, %v", got, err)
	}
	if got, err := entries("", nil); err != nil || got != nil {
		t.Errorf("no commit is %+v, %v", got, err)
	}
	if got, err := entries(record("aa", ""), nil); err == nil || !strings.Contains(err.Error(), "record of aa") || got != nil {
		t.Errorf("a record of no commit named is %+v, %v; want an error naming aa", got, err)
	}
	for _, c := range []struct{ name, out, blamed string }{
		{"a note holding NUL before another record", record("aa", "n\x00m") + record("bb", ""), "aa"},
		{"a note holding NUL in the last record", record("aa", "") + record("bb", "n\x00"), "bb"},
		{"a note holding NUL at the end of the last record", record("aa", "") + record("bb", "n\x00m"), "bb"},
		{"a log of another commit", record("cc", "") + record("bb", ""), "aa"},
		{"a log of no commit", "", "aa"},
		{"a log missing the last commit", record("aa", ""), "aa"},
		{"a log cut within a record", strings.TrimSuffix(record("aa", ""), "\x00"), "aa"},
		{"a log ending without its last NUL", record("aa", "") + strings.TrimSuffix(record("bb", ""), "\x00"), "bb"},
	} {
		got, err := entries(c.out, []string{"aa", "bb"})
		if err == nil || !strings.Contains(err.Error(), c.blamed+" no record") || got != nil {
			t.Errorf("%s: %+v, %v; want an error naming %s", c.name, got, err, c.blamed)
		}
	}
}
