package message

// The body wrapping itos commit does before git sees a message (slice 58,
// features/commit-command.feature): a body line longer than the header
// lint's body-max-line-length is broken at word boundaries, so a message
// typed or generated as long lines is not refused for them. Only lines the
// lint would hold to the limit and could be broken without changing what
// they mean are touched: the header, the footers (the message's trailers,
// as git and the lint read them), comment lines, indented lines and lines
// within the limit are left as written, and lines are never joined, so a
// message already wrapped comes back unchanged.
//
// A break never starts a line with what something reads as other than body
// (bug 15): a footer token or a breaking-change note, which
// conventional-commits-parser reads as the footer from there on, a
// configured footer key and its colon, which ends the last paragraph as a
// footer glued to the body, or git's comment char, a line strip cleanup
// drops. Such a break moves back a word; with none left before it, the line
// runs over the limit, for the lint to judge.

import (
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/donvargax/itos/v8/internal/config"
)

// BodyLimit is the longest a body line may be under the config's header
// lint: the built-in lint's body-max-line-length. 0, wrap nothing, when the
// config has no header lint or delegates it to a command, whose limit itos
// cannot read.
func BodyLimit(cfg *config.Loaded) int {
	if cfg != nil && cfg.Commits.HeaderLint.Builtin() {
		return maxLength
	}
	return 0
}

// listMarker is a list item's marker and the spaces after it, which its
// continuation lines are indented by: -, * or +, or a number and . or ).
var listMarker = regexp.MustCompile(`^(?:[-*+]|\d+[.)]) +`)

// Wrap is a message, given as the paragraphs git joins with a blank line
// (each -m, or the one -F text), with each body line longer than limit
// broken at word boundaries, each paragraph back in its place. Lines whose
// first character is commentChar are left as written ("" for none), and no
// break starts a line with it, with a footer token or note, or with one of
// keys, the configured footers', and its colon.
func Wrap(parts []string, limit int, commentChar string, keys []string) []string {
	if limit <= 0 {
		return parts
	}
	var all []string
	for i, part := range parts {
		if i > 0 {
			all = append(all, "")
		}
		all = append(all, strings.Split(part, "\n")...)
	}
	keep := keptLines(all, limit, commentChar)
	wrapped := make([]string, len(parts))
	at := 0
	for i, part := range parts {
		var out []string
		for _, l := range strings.Split(part, "\n") {
			if keep[at] {
				out = append(out, l)
			} else {
				out = append(out, wrapLine(l, limit, func(lead, rest string) bool {
					return opensFooter(lead, rest, commentChar, keys)
				})...)
			}
			at++
		}
		at++ // the blank line between two paragraphs
		wrapped[i] = strings.Join(out, "\n")
	}
	return wrapped
}

// keptLines are which lines of a message stay as written: everything up to
// the header, its first line with text; the trailers; comment lines,
// indented lines and lines within the limit.
func keptLines(lines []string, limit int, commentChar string) []bool {
	keep := make([]bool, len(lines))
	header := false
	start, _ := trailerBlock(lines, commentChar)
	for i, l := range lines {
		comment := commentChar != "" && strings.HasPrefix(l, commentChar)
		switch {
		case !header:
			header = strings.TrimSpace(l) != "" && !comment
			keep[i] = true
		case i >= start:
			keep[i] = true
		default:
			keep[i] = comment || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t") || length(l) <= limit
		}
	}
	return keep
}

// opensFooter is whether a line starting with a text is read as other than
// body: a footer token or a breaking-change note, as the parser reads them,
// a footer key and its colon, or a comment. The
// line is lead and then rest, read as one text without being built (see
// wrapLine): every test is anchored at its start, so it reads only as far as
// it must.
func opensFooter(lead, rest, commentChar string, keys []string) bool {
	if footerToken.MatchReader(&twoParts{lead, rest}) || noteLine.MatchReader(&twoParts{lead, rest}) ||
		commentChar != "" && startsWith(lead, rest, commentChar) {
		return true
	}
	return slices.ContainsFunc(keys, func(key string) bool { return startsWith(lead, rest, key+":") })
}

// startsWith is strings.HasPrefix(lead+rest, prefix), without building it.
func startsWith(lead, rest, prefix string) bool {
	if len(prefix) <= len(lead) {
		return strings.HasPrefix(lead, prefix)
	}
	return strings.HasPrefix(prefix, lead) && strings.HasPrefix(rest, prefix[len(lead):])
}

// twoParts reads lead and then rest as one text, a character at a time, for a
// pattern to match. lead ends on a whole character, as an indent does.
type twoParts struct{ lead, rest string }

func (p *twoParts) ReadRune() (rune, int, error) {
	s := &p.lead
	if *s == "" {
		s = &p.rest
	}
	if *s == "" {
		return 0, 0, io.EOF
	}
	r, n := utf8.DecodeRuneInString(*s)
	*s = (*s)[n:]
	return r, n, nil
}

// BreakLong is a body with each line longer than limit and holding no URL
// (the lint allows one) broken inside its words (bug 132), for the body of a
// commit itos makes of its own files, where a word longer than the line (a
// path, a long id) would be refused whole; Wrap leaves it so for itos commit,
// whose message is the person's. A line is cut after its last "/" that
// leaves a piece within limit, else at the limit, no character lost and none
// split; the lint and limit count UTF-16 units. No cut starts a line with
// what Wrap's breaks bar (opensFooter): it moves to an earlier "/" or
// character, and a line with no cut allowed is left as it is.
func BreakLong(text string, limit int, commentChar string, keys []string) string {
	barred := func(rest string) bool { return opensFooter("", rest, commentChar, keys) }
	lines := strings.Split(text, "\n")
	var out []string
	for _, l := range lines {
		for length(l) > limit && !url.MatchString(l) {
			at := cutAt(l, limit, barred)
			if at == 0 {
				break
			}
			out = append(out, l[:at])
			l = l[at:]
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// cutAt is where to cut a line of more than limit: the last byte offset
// after a "/" with the line before it within limit, else the last offset
// with one, and the rest after it not barred; 0 when none is.
func cutAt(l string, limit int, barred func(rest string) bool) int {
	var ends []int
	for i, units := 0, 0; i < len(l); {
		r, w := utf8.DecodeRuneInString(l[i:])
		if units += utf16Len(r); units > limit {
			break
		}
		i += w
		ends = append(ends, i)
	}
	for _, slash := range []bool{true, false} {
		for k := len(ends) - 1; k >= 0; k-- {
			if at := ends[k]; (!slash || l[at-1] == '/') && !barred(l[at:]) {
				return at
			}
		}
	}
	return 0
}

// wrapLine is one line broken at its spaces into lines of at most limit, a
// list item's continuation lines indented to its text; a word longer than
// the limit has a line of its own, whole. A break is never made where the
// line after it would start with a text barred holds to (given as the
// indent and the rest of the line after it): it moves back a word, and when
// no break is left before it the line runs over the limit to the next break
// that is allowed.
//
// It takes time linear in the line's length (T-105): the words are joined
// once, each line written and each rest asked of barred is a slice of that,
// and a line's length is summed from its words', so no step builds or
// measures the rest of the line, which made a body of megabytes take minutes.
func wrapLine(l string, limit int, barred func(lead, rest string) bool) []string {
	marker := listMarker.FindString(l)
	words := strings.Fields(l[len(marker):])
	if len(words) == 0 {
		return []string{l}
	}
	indent := strings.Repeat(" ", length(marker))
	// joined is the words a space apart; words[i] starts at start[i] in it,
	// and the words before it are sum[i] long.
	joined := strings.Join(words, " ")
	start := make([]int, len(words)+1)
	sum := make([]int, len(words)+1)
	for i, w := range words {
		start[i+1] = start[i] + len(w) + 1
		sum[i+1] = sum[i] + length(w)
	}
	lead := func(from int) string {
		if from == 0 {
			return marker
		}
		return indent
	}
	// text is the line of words[from:to], the marker before the first word,
	// and width its length.
	text := func(from, to int) string {
		return lead(from) + joined[start[from]:start[to]-1]
	}
	width := func(from, to int) int {
		return length(lead(from)) + sum[to] - sum[from] + to - from - 1
	}
	var out []string
	from := 0
	for to := 1; to < len(words); {
		if width(from, to+1) <= limit {
			to++
			continue
		}
		at := to
		for at > from && barred(indent, joined[start[at]:]) {
			at--
		}
		if at == from {
			to++
			continue
		}
		out = append(out, text(from, at))
		from = at
	}
	return append(out, text(from, len(words)))
}
