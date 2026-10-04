// Package adr is the architecture decision records itos ask record writes
// (features/ask.feature): an answered question written as a record in MADR
// 4's format (adr/madr 4.0.0, its bare-minimal template), the maintained
// one under the adr organisation (slice 71, which replaced slice 69's
// adr-tools records). A record is a Markdown file NNNN-slug.md: YAML
// frontmatter holding its status and date, which no section repeats; the
// title as "# Title", with no number; then the sections Context and Problem
// Statement, Considered Options, Decision Outcome and its Consequences, and
// More Information when it supersedes another. A record superseded says so
// in its status, "superseded by ADR-NNNN", as MADR's template spells it.
// The folder's README.md holds, between itos's markers, the index of the
// records whose status is accepted.
//
// A record is read by its structure, never its bytes: the status is the
// frontmatter's, parsed as YAML, the title the first "# " heading after it,
// so a record a person or a formatter rewrote reads the same. Everything
// written here is in the form a Markdown formatter leaves alone (bug 17).
//
// The functions here are text in, text out, but for List and Next, which
// read the folder; internal/cli/ask.go writes and commits.
package adr

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
)

// IndexName is the index's file in the records' folder.
const IndexName = "README.md"

// The markers the index is written between; the text outside them is kept.
const (
	Begin = "<!-- itos:decisions:begin -->"
	End   = "<!-- itos:decisions:end -->"
)

// Accepted is a new record's status, the one the index lists.
const Accepted = "accepted"

// Ref is how MADR names a record from another: "ADR-0007".
func Ref(n int) string { return fmt.Sprintf("ADR-%04d", n) }

// SupersededBy is the status of a record the record n supersedes.
func SupersededBy(n int) string { return "superseded by " + Ref(n) }

// Record is a record in the folder: its number, its file's name and its
// text.
type Record struct {
	Number int
	File   string
	Text   string
}

// frontmatter splits the text into its YAML frontmatter's lines, without
// the fences, and the line index the body starts at; ok is false when the
// text has none: no "---" first line, or no closing "---" or "...".
func frontmatter(lines []string) (yamlLines []string, body int, ok bool) {
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t\r") != "---" {
		return nil, 0, false
	}
	for i := 1; i < len(lines); i++ {
		if fence := strings.TrimRight(lines[i], " \t\r"); fence == "---" || fence == "..." {
			return lines[1:i], i + 1, true
		}
	}
	return nil, 0, false
}

// Status is the record's status as its frontmatter gives it, trimmed; ""
// when it has no frontmatter, no status or frontmatter that is not YAML.
func (r Record) Status() string {
	front, _, ok := frontmatter(strings.Split(r.Text, "\n"))
	if !ok {
		return ""
	}
	var meta map[string]any
	if yaml.Unmarshal([]byte(strings.Join(front, "\n")), &meta) != nil {
		return ""
	}
	status, _ := meta["status"].(string)
	return strings.TrimSpace(status)
}

// Accepted is whether the record's status is accepted, the records the
// index lists.
func (r Record) Accepted() bool { return strings.EqualFold(r.Status(), Accepted) }

// Title is the record's title: its first "# " heading after the
// frontmatter, outside a fenced code block; "" when it has none.
func (r Record) Title() string {
	lines := strings.Split(r.Text, "\n")
	_, start, _ := frontmatter(lines)
	fence := ""
	for _, line := range lines[start:] {
		line = strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimLeft(line, " ")
		switch {
		case fence != "":
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			fence = trimmed[:3]
		case strings.HasPrefix(line, "# "):
			return strings.TrimSpace(line[2:])
		}
	}
	return ""
}

// SetStatus is the text with its frontmatter's status set: the status line
// replaced, one added at the frontmatter's end when it has none, and
// frontmatter holding it added at the top of a text with none.
func SetStatus(text, status string) string {
	lines := strings.Split(text, "\n")
	front, body, ok := frontmatter(lines)
	line := "status: " + status
	if !ok {
		return "---\n" + line + "\n---\n\n" + text
	}
	for i, l := range front {
		if strings.HasPrefix(l, "status:") {
			eol := ""
			if strings.HasSuffix(l, "\r") {
				eol = "\r"
			}
			lines[1+i] = line + eol
			return strings.Join(lines, "\n")
		}
	}
	out := append(append(slices.Clone(lines[:body-1]), line), lines[body-1:]...)
	return strings.Join(out, "\n")
}

// numbered is a file name's leading number, and whether it has one.
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

// Next is the next record's number: one past the highest number any name in
// the folder starts with, 1 when there is none or no folder.
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

// Slug is the title as a file name: lowercase, each run of characters that
// are not letters or digits one dash, none at either end; letters past
// ASCII are kept, lowercased.
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

// New is what a new record says: the options are one line each, none for a
// question that names its own; Supersedes is the number of the record it
// supersedes, 0 for none.
type New struct {
	Title, Date, Context, Decision, Consequences string
	Options                                      []string
	Supersedes                                   int
}

// NoOptions is Considered Options' text when no option is given.
const NoOptions = "The options are those the question names."

// NoConsequences is Consequences' text when none is given.
const NoConsequences = "None recorded."

// Text is the new record, its status accepted: MADR's bare-minimal
// sections under its optional frontmatter, and More Information naming the
// record it supersedes.
func Text(n New) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nstatus: %s\ndate: %s\n---\n\n# %s\n\n", Accepted, n.Date, n.Title)
	fmt.Fprintf(&b, "## Context and Problem Statement\n\n%s\n\n## Considered Options\n\n", strings.TrimSpace(n.Context))
	if len(n.Options) == 0 {
		b.WriteString(NoOptions + "\n")
	}
	for _, option := range n.Options {
		fmt.Fprintf(&b, "- %s\n", option)
	}
	consequences := strings.TrimSpace(n.Consequences)
	if consequences == "" {
		consequences = NoConsequences
	}
	fmt.Fprintf(&b, "\n## Decision Outcome\n\n%s\n\n### Consequences\n\n%s\n", strings.TrimSpace(n.Decision), consequences)
	if n.Supersedes != 0 {
		fmt.Fprintf(&b, "\n## More Information\n\nSupersedes %s.\n", Ref(n.Supersedes))
	}
	return b.String()
}

// Index is the index's text: the text it had, its part between the markers
// made the list of the accepted records, by number and title, the rest
// kept; the markers and the list added at its end when it has none, and a
// heading for an index that is new ("" before). A blank line follows the
// begin marker and comes before the end one, the form a Markdown formatter
// leaves alone (bug 17): a list against a comment is one it rewrites.
func Index(before string, records []Record) string {
	var list strings.Builder
	list.WriteString(Begin + "\n\n")
	for _, r := range records {
		if r.Accepted() {
			fmt.Fprintf(&list, "- [%s: %s](%s)\n", Ref(r.Number), r.Title(), r.File)
		}
	}
	if list.Len() > len(Begin)+2 {
		list.WriteString("\n")
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
