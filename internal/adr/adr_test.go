package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adr-tools' template, as adr new writes it for "Use Go" (its body text
// left as the template has it).
const useGo = `# 1. Use Go

Date: 2026-10-04

## Status

Accepted

## Context

The issue motivating this decision.

## Decision

The change.

## Consequences

What becomes easier.
`

// A supersede edits both records as adr-tools' adr new -s does (its output,
// 2026-10-04), with the right spelling: the old record loses Accepted and
// gains the link to the new, the new one gains the link to the old after
// its status.
func TestSupersedeAsAdrTools(t *testing.T) {
	old := Record{Number: 1, File: "0001-use-go.md", Text: useGo}
	newer := Record{Number: 2, File: "0002-use-rust.md", Text: strings.ReplaceAll(strings.ReplaceAll(useGo, "1. Use Go", "2. Use Rust"), "2026-10-04", "2026-10-05")}
	gotOld := RemoveStatus(AddLink(old.Text, SupersededBy, newer), Accepted)
	wantOld := strings.Replace(useGo, "Accepted\n", "Superseded by [2. Use Rust](0002-use-rust.md)\n", 1)
	if gotOld != wantOld {
		t.Errorf("the old record:\n%s\nwant:\n%s", gotOld, wantOld)
	}
	gotNew := AddLink(newer.Text, Supersedes, old)
	wantNew := strings.Replace(newer.Text, "Accepted\n", "Accepted\n\nSupersedes [1. Use Go](0001-use-go.md)\n", 1)
	if gotNew != wantNew {
		t.Errorf("the new record:\n%s\nwant:\n%s", gotNew, wantNew)
	}
	if (Record{Text: gotOld}).Live() || !(Record{Text: gotNew}).Live() {
		t.Error("the old record should be superseded and the new one live")
	}
	if (Record{Text: strings.Replace(useGo, "Accepted", "Superceded by [2. X](0002-x.md)", 1)}).Live() {
		t.Error("adr-tools' spelling should read as superseded")
	}
	if !(Record{Text: strings.Replace(useGo, "The change.", "Superseded by nothing yet.", 1)}).Live() {
		t.Error("a line outside the Status section should not supersede")
	}
}

// A Status section that ends the record gets its link at the end.
func TestAddLinkStatusLast(t *testing.T) {
	got := AddLink("# 3. X\n\n## Status\n\nAccepted", Supersedes, Record{File: "0001-a.md", Text: "# 1. A\n"})
	if want := "# 3. X\n\n## Status\n\nAccepted\n\nSupersedes [1. A](0001-a.md)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Slugs as adr-tools makes them: lowercase, each run of other characters a
// dash, none at the ends.
func TestSlug(t *testing.T) {
	for title, want := range map[string]string{
		"Triage issues with labels": "triage-issues-with-labels",
		"  Use Rust, & C++!  ":      "use-rust-c",
		"Ünïcode kept":              "ünïcode-kept",
		"--":                        "",
	} {
		if got := Slug(title); got != want {
			t.Errorf("Slug(%q) = %q, want %q", title, got, want)
		}
	}
	if got := FileName(12, "Use Go"); got != "0012-use-go.md" {
		t.Errorf("FileName: %s", got)
	}
}

// The folder: a .adr-dir's, else docs/adr; a .adr-dir naming a folder
// outside is refused. The next number is past the
// highest name, the records listed by number.
func TestDirListNext(t *testing.T) {
	top := t.TempDir()
	if dir, err := Dir(top); err != nil || dir != DefaultDir {
		t.Errorf("no folder: %q, %v", dir, err)
	}
	if n, err := Next(filepath.Join(top, DefaultDir)); err != nil || n != 1 {
		t.Errorf("next in no folder: %d, %v", n, err)
	}
	write(t, filepath.Join(top, DirFile), "doc/decisions\n")
	if dir, err := Dir(top); err != nil || dir != "doc/decisions" {
		t.Errorf(".adr-dir: %q, %v", dir, err)
	}
	write(t, filepath.Join(top, DirFile), "../elsewhere\n")
	if _, err := Dir(top); err == nil {
		t.Error("a .adr-dir outside the repository should be refused")
	}
	dir := filepath.Join(top, "doc/decisions")
	write(t, filepath.Join(dir, "0007-use-go.md"), "# 7. Use Go\n")
	write(t, filepath.Join(dir, "0010-later.md"), "# 10. Later\n")
	write(t, filepath.Join(dir, "README.md"), "index\n")
	write(t, filepath.Join(dir, "0012-notes.txt"), "not a record\n")
	if n, err := Next(dir); err != nil || n != 13 {
		t.Errorf("next: %d, %v", n, err)
	}
	records, err := List(dir)
	if err != nil || len(records) != 2 || records[0].Number != 7 || records[1].Title() != "10. Later" {
		t.Errorf("list: %+v, %v", records, err)
	}
}

// The index lists the live records between the markers, a blank line inside
// each (bug 17), keeps the text outside them, and is made with a heading
// when there was none.
func TestIndex(t *testing.T) {
	records := []Record{
		{Number: 1, File: "0001-a.md", Text: "# 1. A\n\n## Status\n\nSuperseded by [2. B](0002-b.md)\n"},
		{Number: 2, File: "0002-b.md", Text: "# 2. B\n\n## Status\n\nAccepted\n"},
	}
	made := Index("", records)
	if !strings.HasPrefix(made, "# Decisions\n") || !strings.Contains(made, Begin+"\n\n- [2. B](0002-b.md)\n\n"+End+"\n") || strings.Contains(made, "[1. A]") {
		t.Errorf("a new index:\n%s", made)
	}
	kept := Index("# Ours\n\n"+Begin+"\n- old\n"+End+"\n\nMore.\n", records)
	if kept != "# Ours\n\n"+Begin+"\n\n- [2. B](0002-b.md)\n\n"+End+"\n\nMore.\n" {
		t.Errorf("an index with markers:\n%s", kept)
	}
	added := Index("# Ours", records)
	if added != "# Ours\n\n"+Begin+"\n\n- [2. B](0002-b.md)\n\n"+End+"\n" {
		t.Errorf("an index without markers:\n%s", added)
	}
	if empty := Index("# Ours\n\n"+Begin+"\n- old\n"+End+"\n", records[:1]); empty != "# Ours\n\n"+Begin+"\n\n"+End+"\n" {
		t.Errorf("an index of no live record:\n%s", empty)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
