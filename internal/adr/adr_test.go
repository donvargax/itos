package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A new record in MADR 4's bare-minimal sections, its status and date in the
// frontmatter alone.
const useGo = `---
status: accepted
date: 2026-10-04
---

# Use Go

## Context and Problem Statement

Go or Rust?

Asked as q-1.

## Considered Options

- Go
- Rust

## Decision Outcome

Go.

### Consequences

None recorded.
`

func TestText(t *testing.T) {
	got := Text(New{Title: "Use Go", Date: "2026-10-04", Context: "Go or Rust?\n\nAsked as q-1.", Options: []string{"Go", "Rust"},
		Decision: "Go."})
	if got != useGo {
		t.Errorf("the record:\n%s\nwant:\n%s", got, useGo)
	}
	bare := Text(New{Title: "Use Rust", Date: "2026-10-05", Context: "c", Decision: "d", Consequences: "e", Supersedes: 1})
	for _, want := range []string{"## Considered Options\n\n" + NoOptions + "\n\n", "### Consequences\n\ne\n",
		"\n## More Information\n\nSupersedes ADR-0001.\n"} {
		if !strings.Contains(bare, want) {
			t.Errorf("the record should say %q:\n%s", want, bare)
		}
	}
	r := Record{Text: useGo}
	if r.Title() != "Use Go" || r.Status() != "accepted" || !r.Live() {
		t.Errorf("read back: %q %q", r.Title(), r.Status())
	}
}

// A record is read by its structure: a quoted status, CRLF line endings, a
// heading in a code block and no frontmatter at all read as they mean.
func TestReadByStructure(t *testing.T) {
	quoted := Record{Text: "---\r\ndate: 2020-01-01\r\nstatus: \"Accepted\"\r\n---\r\n\r\n```\r\n# not it\r\n```\r\n\r\n# Use Go\r\n"}
	if quoted.Status() != "Accepted" || quoted.Title() != "Use Go" {
		t.Errorf("quoted: %q %q", quoted.Status(), quoted.Title())
	}
	bare := Record{Text: "# Use Go\n\nstatus: accepted\n"}
	if bare.Status() != "" || bare.Title() != "Use Go" {
		t.Errorf("no frontmatter: %q %q", bare.Status(), bare.Title())
	}
	broken := Record{Text: "---\nstatus: [accepted\n---\n# X\n"}
	if broken.Status() != "" {
		t.Error("frontmatter that is not YAML has no status")
	}
}

// A record is live unless its status says superseded, deprecated or
// rejected, in any case: one with no frontmatter, or with frontmatter but no
// status, is live (bug 19).
func TestLive(t *testing.T) {
	for text, want := range map[string]bool{
		"# A\n":                                             true,
		"---\ndate: 2020-01-01\n---\n\n# A\n":               true,
		"---\nstatus: accepted\n---\n\n# A\n":               true,
		"---\nstatus: Proposed\n---\n\n# A\n":               true,
		"---\nstatus: [accepted\n---\n# A\n":                true,
		"---\nstatus: superseded by ADR-0002\n---\n\n# A\n": false,
		"---\nstatus: Superseded by ADR-0002\n---\n\n# A\n": false,
		"---\nstatus: deprecated\n---\n\n# A\n":             false,
		"---\nstatus: REJECTED\n---\n\n# A\n":               false,
	} {
		if got := (Record{Text: text}).Live(); got != want {
			t.Errorf("Live() = %v, want %v, for:\n%s", got, want, text)
		}
	}
}

// Superseding sets the status: the line replaced, added to frontmatter
// without one, and frontmatter added to a record with none.
func TestSetStatus(t *testing.T) {
	got := SetStatus(useGo, SupersededBy(2))
	if want := strings.Replace(useGo, "status: accepted", "status: superseded by ADR-0002", 1); got != want {
		t.Errorf("replaced:\n%s", got)
	}
	if (Record{Text: got}).Live() || (Record{Text: got}).Status() != "superseded by ADR-0002" {
		t.Errorf("the status read back: %q", (Record{Text: got}).Status())
	}
	added := SetStatus("---\ndate: 2020-01-01\n---\n\n# A\n", SupersededBy(12))
	if added != "---\ndate: 2020-01-01\nstatus: superseded by ADR-0012\n---\n\n# A\n" {
		t.Errorf("added:\n%s", added)
	}
	if made := SetStatus("# A\n", SupersededBy(3)); made != "---\nstatus: superseded by ADR-0003\n---\n\n# A\n" {
		t.Errorf("made:\n%s", made)
	}
}

// Slugs: lowercase, each run of other characters a dash, none at the ends.
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

// The next number is past the highest name, the records listed by number.
func TestListNext(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs/decisions")
	if n, err := Next(dir); err != nil || n != 1 {
		t.Errorf("next in no folder: %d, %v", n, err)
	}
	write(t, filepath.Join(dir, "0007-use-go.md"), "# Use Go\n")
	write(t, filepath.Join(dir, "0010-later.md"), "---\nstatus: accepted\n---\n\n# Later\n")
	write(t, filepath.Join(dir, "README.md"), "index\n")
	write(t, filepath.Join(dir, "0012-notes.txt"), "not a record\n")
	if n, err := Next(dir); err != nil || n != 13 {
		t.Errorf("next: %d, %v", n, err)
	}
	records, err := List(dir)
	if err != nil || len(records) != 2 || records[0].Number != 7 || records[1].Title() != "Later" {
		t.Errorf("list: %+v, %v", records, err)
	}
}

// The index lists the live records between the markers, by number and
// title, a blank line inside each (bug 17), keeps the text outside them,
// and is made with a heading when there was none.
func TestIndex(t *testing.T) {
	records := []Record{
		{Number: 1, File: "0001-a.md", Text: "---\nstatus: superseded by ADR-0002\n---\n\n# A\n"},
		{Number: 2, File: "0002-b.md", Text: "---\nstatus: accepted\n---\n\n# B\n"},
		{Number: 3, File: "0003-c.md", Text: "# C\n"},
	}
	made := Index("", records)
	if !strings.HasPrefix(made, "# Decisions\n") || !strings.Contains(made, Begin+"\n\n- [ADR-0002: B](0002-b.md)\n- [ADR-0003: C](0003-c.md)\n\n"+End+"\n") ||
		strings.Contains(made, "0001-a.md") {
		t.Errorf("a new index:\n%s", made)
	}
	kept := Index("# Ours\n\n"+Begin+"\n- old\n"+End+"\n\nMore.\n", records)
	if kept != "# Ours\n\n"+Begin+"\n\n- [ADR-0002: B](0002-b.md)\n- [ADR-0003: C](0003-c.md)\n\n"+End+"\n\nMore.\n" {
		t.Errorf("an index with markers:\n%s", kept)
	}
	added := Index("# Ours", records)
	if added != "# Ours\n\n"+Begin+"\n\n- [ADR-0002: B](0002-b.md)\n- [ADR-0003: C](0003-c.md)\n\n"+End+"\n" {
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

// Config check's rules (slice 74): a number two records share names both
// files, the second renumbered past the highest; a record superseded by a
// number no record has names it, its status read in any case; a record
// superseded by one the folder has, or with no status, holds.
func TestProblems(t *testing.T) {
	records := []Record{
		{Number: 1, File: "0001-a.md", Text: "---\nstatus: Superseded by adr-0003\n---\n\n# A\n"},
		{Number: 2, File: "0002-b.md", Text: "---\nstatus: superseded by ADR-0009\n---\n\n# B\n"},
		{Number: 3, File: "0003-c.md", Text: "# C\n"},
		{Number: 3, File: "0003-d.md", Text: "---\nstatus: deprecated\n---\n\n# D\n"},
	}
	found := Problems("docs/decisions", records)
	if len(found) != 2 {
		t.Fatalf("problems: %+v", found)
	}
	if found[0].Rule != "decisions-number-twice" ||
		found[0].Message != "docs/decisions/0003-c.md and docs/decisions/0003-d.md share the number 0003" ||
		!strings.Contains(found[0].Fix, "renumber docs/decisions/0003-d.md to 0004") {
		t.Errorf("a number twice: %+v", found[0])
	}
	if found[1].Rule != "decisions-superseded-by-missing" ||
		found[1].Message != "docs/decisions/0002-b.md is superseded by ADR-0009, which no record in docs/decisions has" ||
		!strings.Contains(found[1].Fix, "ADR-0009") {
		t.Errorf("a supersede link to no record: %+v", found[1])
	}
	if sound := Problems("docs/decisions", records[:1:1]); len(sound) != 1 {
		t.Errorf("a record superseded by one not listed: %+v", sound)
	}
	if none := Problems("docs/decisions", nil); len(none) != 0 {
		t.Errorf("no records: %+v", none)
	}
	if n, ok := (Record{Text: "---\nstatus: superseded\n---\n"}).SupersededByNumber(); ok {
		t.Errorf("a status naming no record: %d", n)
	}
}
