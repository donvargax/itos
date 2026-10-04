package message

import (
	"slices"
	"strings"
	"testing"
)

// The body's long lines broken at a limit of 20, everything else as
// written: the header, a short line, an indented one, a comment, a word
// longer than the limit, and every line from the first footer on.
func TestWrap(t *testing.T) {
	long := "chore: a header that is longer than twenty"
	text := strings.Join([]string{
		long,
		"",
		"one two three four five six seven",
		"short line",
		"    indented and longer than twenty",
		"# a comment longer than twenty",
		"- an item that goes on and on",
		"12. a numbered item that goes on",
		"averyveryverylongwordindeed and more",
		"",
		"Task: T-001",
		"a footer's continuation that is long",
	}, "\n")
	want := strings.Join([]string{
		long,
		"",
		"one two three four",
		"five six seven",
		"short line",
		"    indented and longer than twenty",
		"# a comment longer than twenty",
		"- an item that goes",
		"  on and on",
		"12. a numbered item",
		"    that goes on",
		"averyveryverylongwordindeed",
		"and more",
		"",
		"Task: T-001",
		"a footer's continuation that is long",
	}, "\n")
	if got := Wrap([]string{text}, 20, "#")[0]; got != want {
		t.Errorf("Wrap:\n%s\nwant:\n%s", got, want)
	}
	if got := Wrap([]string{want}, 20, "#")[0]; got != want {
		t.Errorf("Wrap changed a wrapped message:\n%s", got)
	}
	if got := Wrap([]string{text}, 0, "#")[0]; got != text {
		t.Errorf("Wrap with no limit changed the message:\n%s", got)
	}
}

// The -m paragraphs are read as one message, joined by a blank line: the
// header is the first one's, a footer in the last ends the body, and each
// paragraph comes back in its place.
func TestWrapParagraphs(t *testing.T) {
	got := Wrap([]string{"chore: a header that is longer than twenty", "one two three four five six", "Task: T-001 and a long footer"}, 20, "#")
	want := []string{"chore: a header that is longer than twenty", "one two three four\nfive six", "Task: T-001 and a long footer"}
	if !slices.Equal(got, want) {
		t.Errorf("Wrap %q", got)
	}
}
