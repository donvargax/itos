package message

// The messages commitlint passes without linting them (@commitlint/is-ignored's
// wildcards, which config-conventional keeps): a merge's, a revert's or a
// reapply's as git writes them, an amend!, fixup! or squash! commit, a
// version alone, and the merges hosts write. The built-in lint passes the
// same ones, so the two give the same verdict on a history that has them.

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v4/internal/value"
)

var (
	// Matched against any line of the message, as the /m wildcards are.
	mergeLine = regexp.MustCompile(`^(?:Merge pull request|Merge .* into |Merge branch |Merge tag )`)
	// Matched against the message's start.
	ignoredStart = regexp.MustCompile(`^(?:[Rr]evert |[Rr]eapply |(?:amend|fixup|squash)!|` +
		`Merged ` + line + `*?(?:in|into) |Merged PR ` + line + `*: |Merge remote-tracking branch|` +
		`Automatic merge|Auto-merged ` + line + `*? into )`)
	terminators = regexp.MustCompile(`[\n\r\x{2028}\x{2029}]`)
	// What isSemver takes off the first line before asking semver.
	chorePrefix = regexp.MustCompile(`^chore(?:\([^)]+\))?:`)
	skip        = `(?:[Ss][Kk][Ii][Pp]|[Cc][Ii])(?:-|` + value.Space + `)(?:[Cc][Ii]|[Ss][Kk][Ii][Pp])`
	skipBracket = regexp.MustCompile(`\[` + skip + `\]`)
	skipParen   = regexp.MustCompile(`\(` + skip + `\)`)
	// semver's FULL: a version, as semver.valid reads it strictly.
	numeric  = `(0|[1-9]\d*)`
	ident    = `(?:\d*[a-zA-Z-][a-zA-Z0-9-]*|0|[1-9]\d*)`
	semverRe = regexp.MustCompile(`^v?` + numeric + `\.` + numeric + `\.` + numeric +
		`(?:-` + ident + `(?:\.` + ident + `)*)?(?:\+[a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)*)?$`)
)

// ignored is whether commitlint leaves a message, its end trimmed, unlinted.
func ignored(message string) bool {
	for _, l := range terminators.Split(message, -1) {
		if mergeLine.MatchString(l) {
			return true
		}
	}
	return ignoredStart.MatchString(message) || isVersion(message)
}

// replaceFirst is a text with the pattern's first match taken out, as
// String.replace takes out one.
func replaceFirst(re *regexp.Regexp, text string) string {
	if m := re.FindStringIndex(text); m != nil {
		return text[:m[0]] + text[m[1]:]
	}
	return text
}

// isVersion is is-ignored's isSemver: the first line, without a chore type
// or a skip-ci mark, is a version semver takes.
func isVersion(message string) bool {
	first, _, _ := strings.Cut(message, "\n")
	stripped := replaceFirst(chorePrefix, first)
	stripped = replaceFirst(skipBracket, stripped)
	stripped = value.Trim(replaceFirst(skipParen, stripped))
	if length(stripped) > 256 {
		return false
	}
	m := semverRe.FindStringSubmatch(stripped)
	if m == nil {
		return false
	}
	for _, part := range m[1:4] {
		if n, err := strconv.ParseFloat(part, 64); err != nil || n > 9007199254740991 {
			return false
		}
	}
	return true
}
