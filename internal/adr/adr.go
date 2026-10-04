// Package adr is the architecture decision records itos ask record writes
// (slice 69, features/ask.feature): an answered question written as a
// record in adr-tools' format (npryce/adr-tools, read 2026-10-04), so a
// repository that already keeps records with adr-tools or log4brains keeps
// its own and itos adds to them. A record is a Markdown file NNNN-slug.md,
// its first line "# N. Title", then "Date: YYYY-MM-DD" and the sections
// Status, Context, Decision and Consequences, as adr-tools' template.md
// lays them out; links between records are lines of the Status section,
// "Superseded by [N. Title](file)" and "Supersedes [N. Title](file)".
// adr-tools spells them "Superceded" and "Supercedes": itos writes the
// right spelling and reads both. The folder's README.md holds, between
// itos's markers, the index of the decisions still live.
//
// The functions here are text in, text out, but for Dir, List and Next,
// which read the folder; internal/cli/ask.go writes and commits.
package adr

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// DefaultDir is where the records go when no .adr-dir names a folder.
const DefaultDir = "docs/adr"

// DirFile is the file that names the records' folder, as adr-tools reads it.
const DirFile = ".adr-dir"

// IndexName is the index's file in the records' folder.
const IndexName = "README.md"

// The markers the index is written between; the text outside them is kept.
const (
	Begin = "<!-- itos:decisions:begin -->"
	End   = "<!-- itos:decisions:end -->"
)

// Accepted is a new record's status, and the status a record superseded
// loses.
const Accepted = "Accepted"

// The links a supersede writes: the old record gains the first, the new one
// the second.
const (
	SupersededBy = "Superseded by"
	Supersedes   = "Supersedes"
)

// Dir is the records' folder of the work tree at top: the folder its
// .adr-dir names, relative to top, as adr-tools' _adr_dir reads it, else
// DefaultDir. adr-tools walks up from where it is run; itos runs at the
// repository's top, and looks there alone. A .adr-dir that names no folder
// inside the work tree is an error.
func Dir(top string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(top, DirFile))
	if err == nil {
		named, _, _ := strings.Cut(string(raw), "\n")
		named = strings.TrimSpace(named)
		if named == "" || !filepath.IsLocal(filepath.FromSlash(named)) {
			return "", fmt.Errorf("%s names %q, which is no folder inside the repository", DirFile, named)
		}
		return filepath.ToSlash(filepath.Clean(named)), nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return DefaultDir, nil
}

// Record is a record in the folder: its number, its file's name and its
// text.
type Record struct {
	Number int
	File   string
	Text   string
}

// Title is the record's title as adr-tools' _adr_title reads it, its first
// line without the "# ": "7. Use Go".
func (r Record) Title() string {
	first, _, _ := strings.Cut(r.Text, "\n")
	return strings.TrimPrefix(strings.TrimRight(first, "\r"), "# ")
}

// Live is whether the record still stands: no line of its Status section
// says it is superseded, in either spelling.
func (r Record) Live() bool {
	for _, line := range statusLines(r.Text) {
		if strings.HasPrefix(line, SupersededBy+" ") || strings.HasPrefix(line, "Superceded by ") {
			return false
		}
	}
	return true
}

// statusLines are the lines of the text's Status section.
func statusLines(text string) []string {
	var lines []string
	in := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "##") {
			in = line == "## Status"
			continue
		}
		if in {
			lines = append(lines, line)
		}
	}
	return lines
}

// numbered is a file name's leading number, as adr-tools reads it
// (grep -Eo '^[0-9]+'), and whether it has one.
var numbered = regexp.MustCompile(`^[0-9]+`)

func numberOf(name string) (int, bool) {
	digits := numbered.FindString(name)
	if digits == "" {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	return n, err == nil
}

// List is the records in the folder, by number: its Markdown files whose
// names start with a number. A folder that is not there holds none.
func List(dir string) ([]Record, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []Record
	for _, e := range entries {
		n, ok := numberOf(e.Name())
		if !ok || e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		records = append(records, Record{Number: n, File: e.Name(), Text: string(raw)})
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].Number < records[j].Number })
	return records, nil
}

// Next is the next record's number, as adr-tools' adr-new gives it: one past
// the highest number any name in the folder starts with, 1 when there is
// none or no folder.
func Next(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	highest := 0
	for _, e := range entries {
		if n, ok := numberOf(e.Name()); ok {
			highest = max(highest, n)
		}
	}
	return highest + 1, nil
}

// Slug is the title as adr-tools makes a file name of it: lowercase, each
// run of characters that are not letters or digits one dash, none at
// either end. adr-tools' tr reads bytes, so it makes a dash of every letter
// past ASCII; itos keeps them, lowercased.
func Slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		dash = true
	}
	return b.String()
}

// FileName is the record's file name: its number in four digits, then the
// title's slug.
func FileName(n int, title string) string {
	return fmt.Sprintf("%04d-%s.md", n, Slug(title))
}

// New is what a new record says.
type New struct {
	Number                                      int
	Title, Date, Context, Decision, Consequence string
}

// Text is the new record in adr-tools' template, its status Accepted.
func Text(n New) string {
	return fmt.Sprintf("# %d. %s\n\nDate: %s\n\n## Status\n\n%s\n\n## Context\n\n%s\n\n## Decision\n\n%s\n\n## Consequences\n\n%s\n",
		n.Number, n.Title, n.Date, Accepted, strings.TrimSpace(n.Context), strings.TrimSpace(n.Decision),
		strings.TrimSpace(n.Consequence))
}

// AddLink is the text with a link to the target added at the end of its
// Status section, as adr-tools' _adr_add_link adds one: before the next
// heading, the link line and a blank line. A Status section that ends the
// file, where adr-tools adds nothing, gets it at the end.
func AddLink(text, kind string, target Record) string {
	link := fmt.Sprintf("%s [%s](%s)", kind, target.Title(), target.File)
	lines := strings.SplitAfter(text, "\n")
	var b strings.Builder
	in := false
	for _, line := range lines {
		bare := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(bare, "##") {
			if in {
				b.WriteString(link + "\n\n")
			}
			in = bare == "## Status"
		}
		b.WriteString(line)
	}
	if in {
		out := b.String()
		if !strings.HasSuffix(out, "\n") && out != "" {
			out += "\n"
		}
		return out + "\n" + link + "\n"
	}
	return b.String()
}

// RemoveStatus is the text with the status line removed from its Status
// section, as adr-tools' _adr_remove_status removes it, and the blank lines
// left there one at most in a row.
func RemoveStatus(text, status string) string {
	lines := strings.SplitAfter(text, "\n")
	var b strings.Builder
	in, afterBlank := false, false
	for _, line := range lines {
		bare := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(bare, "##") {
			in = false
		}
		if bare == "## Status" {
			in = true
		}
		switch {
		case in && strings.TrimSpace(bare) == "":
			if !afterBlank {
				b.WriteString(line)
			}
			afterBlank = true
			continue
		case in && bare == status:
			continue
		case in:
			afterBlank = false
		}
		b.WriteString(line)
	}
	return b.String()
}

// Index is the index's text: the text it had, its part between the markers
// made the list of the live records, the rest kept; the markers and the list
// added at its end when it has none, and a heading for an index that is
// new ("" before).
func Index(before string, records []Record) string {
	var list strings.Builder
	list.WriteString(Begin + "\n")
	for _, r := range records {
		if r.Live() {
			fmt.Fprintf(&list, "- [%s](%s)\n", r.Title(), r.File)
		}
	}
	list.WriteString(End)
	if start := strings.Index(before, Begin); start >= 0 {
		if end := strings.Index(before[start:], End); end >= 0 {
			return before[:start] + list.String() + before[start+end+len(End):]
		}
	}
	if before == "" {
		before = "# Decisions\n\nThe decisions that still stand, one record each. A record superseded keeps its\nfile and leaves this list, which itos ask record writes.\n"
	}
	if !strings.HasSuffix(before, "\n") {
		before += "\n"
	}
	return before + "\n" + list.String() + "\n"
}
