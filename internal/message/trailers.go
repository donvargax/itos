package message

// The footers as git reads a message's trailers (bug 110): trailer.c's
// find_trailer_block_start and its parsing, as git log's %(trailers) and
// git commit --trailer read a commit's message, with no --- divider. Read
// here rather than by a git process a message, since the hook, verify and
// CI's range readers read thousands; the rules that would make it fragile
// (the divider, the scissors line, configured trailer keys) are either off
// for a commit's message or read the same way here.
//
// It departs from trailer.c in one place: "BREAKING CHANGE:" is a trailer,
// its space and all, where git reads no key with a space in it. The
// Conventional Commits spec allows it as a footer's token, so a last
// paragraph holding it beside the other footers stays the footers.

import (
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
)

// trailer is one footer: its key and its value, both trimmed, a
// continuation line's break and indent kept in the value.
type trailer struct{ key, value string }

// gitGenerated are the trailers git writes itself: a last paragraph with
// one is read as trailers when a quarter of its lines are.
var gitGenerated = []string{"Signed-off-by: ", "(cherry picked from commit "}

// breakingKey is the one key with a space the footers read.
const breakingKey = "BREAKING CHANGE"

// gitSpace is whether a byte is a space as git's isspace reads it.
func gitSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// GitTrim is a text trimmed as git trims a trailer's key and value: of its
// isspace alone, the space, tab, line feed and carriage return, never of the
// form feed, the vertical tab or a character above 127 that value.Trim,
// String.trim's set, drops.
func GitTrim(s string) string { return strings.Trim(s, " \t\n\r") }

func blankLine(l string) bool { return strings.TrimLeft(l, " \t\n\r") == "" }

// commentLine is whether a line is a comment, "" reading none as one.
func commentLine(l, comment string) bool { return comment != "" && strings.HasPrefix(l, comment) }

// separatorAt is where a line's trailer separator, its colon, stands: after a
// key of ASCII letters, digits and hyphens, spaces allowed before the colon,
// or after BREAKING CHANGE; -1 when the line has none, or begins with a URL
// (find_separator).
func separatorAt(l string) int {
	if strings.HasPrefix(l, breakingKey+":") {
		return len(breakingKey)
	}
	spaced := false
	for i := 0; i < len(l); i++ {
		c := l[i]
		switch {
		case c == ':' && !spaced && strings.HasPrefix(l[i:], "://"):
			return -1
		case c == ':':
			return i
		case !spaced && (c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'):
		case i > 0 && (c == ' ' || c == '\t'):
			spaced = true
		default:
			return -1
		}
	}
	return -1
}

// trailerLine is whether a line is one git reads as a trailer's first.
func trailerLine(l string) bool { return separatorAt(l) >= 1 && !gitSpace(l[0]) }

// messageEnd is where a message's lines end for its trailers: before git's
// scissors line, and before the blank and comment lines at the end.
func messageEnd(lines []string, comment string) int {
	end := len(lines)
	if at := indexOf(lines, comment+" "+scissor); comment != "" && at >= 0 {
		end = at
	}
	for end > 0 && (blankLine(lines[end-1]) || commentLine(lines[end-1], comment)) {
		end--
	}
	return end
}

func indexOf(lines []string, l string) int {
	for i, s := range lines {
		if s == l {
			return i
		}
	}
	return -1
}

// trailerBlock is where a message's trailers stand among its lines, start
// to end; start is end when it has none. They are its last paragraph, never
// its first, when each of its lines is a trailer or a trailer's indented
// continuation, or when it holds a trailer git writes itself and a quarter
// of its lines are trailers. Lines beginning with comment are passed over.
func trailerBlock(lines []string, comment string) (start, end int) {
	end = messageEnd(lines, comment)
	title := end
	for i, l := range lines[:end] {
		if !commentLine(l, comment) && blankLine(l) {
			title = i
			break
		}
	}
	generated := false
	trailers, others, continuing := 0, 0, 0
	for i := end - 1; i >= title; i-- {
		l := lines[i]
		switch {
		case commentLine(l, comment):
			others += continuing
			continuing = 0
		case blankLine(l):
			others += continuing
			if generated && trailers*3 >= others || trailers > 0 && others == 0 {
				return i + 1, end
			}
			return end, end
		case hasGenerated(l):
			generated = true
			trailers++
			continuing = 0
		case trailerLine(l):
			trailers++
			continuing = 0
		case gitSpace(l[0]):
			continuing++
		default:
			others += 1 + continuing
			continuing = 0
		}
	}
	return end, end
}

func hasGenerated(l string) bool {
	for _, p := range gitGenerated {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}

// parseTrailers are the trailers among a block's lines: an indented line
// continues the trailer above it, and a line that is no trailer, or a
// comment, is passed over.
func parseTrailers(block []string, comment string) []trailer {
	var found []trailer
	last := -1
	for _, l := range block {
		if last >= 0 && l != "" && gitSpace(l[0]) {
			found[last].value += "\n" + l
			continue
		}
		last = -1
		at := separatorAt(l)
		if commentLine(l, comment) || at < 1 {
			continue
		}
		found = append(found, trailer{key: GitTrim(l[:at]), value: l[at+1:]})
		last = len(found) - 1
	}
	for i := range found {
		found[i].value = GitTrim(found[i].value)
	}
	return found
}

// trailers are a message's footers, as git reads its trailers, the comment
// lines passed over.
func trailers(message, comment string) []trailer {
	lines := strings.Split(message, "\n")
	start, end := trailerBlock(lines, comment)
	return parseTrailers(lines[start:end], comment)
}

// linked are the footers a commit's links stand in: its message's, or under
// a stealth config its note's, every line of which is a footer, the
// repository's comment lines passed over.
func linked(cfg *config.Loaded, links string) []trailer {
	if cfg.Stealth {
		return parseTrailers(strings.Split(links, "\n"), cfg.CommentChar())
	}
	return trailers(links, cfg.CommentChar())
}

// unfolded is a trailer's value on one line, each line break and the indent
// after it one space, as git's unfold reads it, trimmed as git trims it.
func unfolded(v string) string {
	parts := strings.Split(v, "\n")
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.TrimLeft(parts[i], " \t\r")
	}
	return GitTrim(strings.Join(parts, " "))
}
