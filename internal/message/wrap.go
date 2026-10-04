package message

// The body wrapping itos commit does before git sees a message (slice 58,
// features/commit-command.feature): a body line longer than the header
// lint's body-max-line-length is broken at word boundaries, so a message
// typed or generated as long lines is not refused for them. Only lines the
// lint would hold to the limit and could be broken without changing what
// they mean are touched: the header, the footers (from the first line the
// lint reads as one, as it reads the rest after it as footer too), comment
// lines, indented lines and lines within the limit are left as written, and
// lines are never joined, so a message already wrapped comes back unchanged.

import (
	"regexp"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
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
// first character is commentChar are left as written ("" for none).
func Wrap(parts []string, limit int, commentChar string) []string {
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
				out = append(out, wrapLine(l, limit)...)
			}
			at++
		}
		at++ // the blank line between two paragraphs
		wrapped[i] = strings.Join(out, "\n")
	}
	return wrapped
}

// keptLines are which lines of a message stay as written: everything up to
// the header, its first line with text; every line from the first the lint
// reads as a footer or a breaking-change note on; comment lines, indented
// lines and lines within the limit.
func keptLines(lines []string, limit int, commentChar string) []bool {
	keep := make([]bool, len(lines))
	header, footer := false, false
	for i, l := range lines {
		comment := commentChar != "" && strings.HasPrefix(l, commentChar)
		switch {
		case !header:
			header = strings.TrimSpace(l) != "" && !comment
			keep[i] = true
		case footer || footerToken.MatchString(l) || noteLine.MatchString(l):
			footer = true
			keep[i] = true
		default:
			keep[i] = comment || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t") || length(l) <= limit
		}
	}
	return keep
}

// wrapLine is one line broken at its spaces into lines of at most limit, a
// list item's continuation lines indented to its text; a word longer than
// the limit has a line of its own, whole.
func wrapLine(l string, limit int) []string {
	marker := listMarker.FindString(l)
	words := strings.Fields(l[len(marker):])
	if len(words) == 0 {
		return []string{l}
	}
	indent := strings.Repeat(" ", length(marker))
	var out []string
	current := marker + words[0]
	for _, w := range words[1:] {
		if length(current)+1+length(w) <= limit {
			current += " " + w
			continue
		}
		out = append(out, current)
		current = indent + w
	}
	return append(out, current)
}
