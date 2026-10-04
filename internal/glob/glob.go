// Package glob is the path globs (config.ts's globToRegExp and matchesAny),
// ported as written (PLAN.md's risks): `*` does not cross `/`, `**` does, `**/`
// may match nothing, `{a,b}` is either, a glob matches the whole path, and a
// glob with no `/` matches at the root only.
//
// The TypeScript turns a glob into a regular expression by five replacements
// in turn, and so does Source, in the same order, so a glob means in Go what
// it means there, quirks included (`?` stays a regular expression's `?`, and
// a `*` right after a `.` is left alone, so `a.*` is `a` and any dots). RE2
// forced two changes of construction, neither of meaning: the last
// replacement's lookbehind (a `*` not after a `.`) is a scan, since RE2 has
// no lookbehind; and every `.` the replacements write is JavaScript's dot,
// any character but a line terminator, written out as a class, since RE2's
// dot leaves out `\n` alone.
package glob

import (
	"regexp"
	"regexp/syntax"
	"strings"
	"sync"
)

var (
	special = regexp.MustCompile(`[.+^$()|[\]\\]`)
	braces  = regexp.MustCompile(`\{([^}]*)\}`)
)

// jsDot is JavaScript's `.` without the s flag: anything but a line
// terminator.
const jsDot = `[^\n\r\x{2028}\x{2029}]`

// Source is the glob's regular expression, as globToRegExp writes it, its
// dots JavaScript's.
func Source(glob string) string {
	s := special.ReplaceAllString(glob, `\$0`)
	s = braces.ReplaceAllStringFunc(s, func(m string) string {
		return "(?:" + strings.ReplaceAll(m[1:len(m)-1], ",", "|") + ")"
	})
	s = strings.ReplaceAll(s, "**/", "(?:.*/)?")
	s = strings.ReplaceAll(s, "**", ".*")
	// .replace(/(?<!\.)\*/g, "[^/]*"): each `*` the character before it in
	// this string is not a `.`.
	var star strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '*' && (i == 0 || s[i-1] != '.') {
			star.WriteString("[^/]*")
		} else {
			star.WriteByte(s[i])
		}
	}
	return "^" + jsDots(star.String()) + "$"
}

// jsDots writes each `.` that is not escaped as JavaScript's dot. Every
// literal dot of the glob was escaped by the first replacement, so the ones
// left are those the replacements wrote.
func jsDots(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s):
			b.WriteString(s[i : i+2])
			i++
		case s[i] == '.':
			b.WriteString(jsDot)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// Error is a glob whose regular expression JavaScript would refuse, worded
// as V8 words it, where the reason is one a glob can give (a `?` with
// nothing before it to repeat).
type Error struct {
	// Source is the expression as globToRegExp writes it (JavaScript's dots
	// as `.`).
	Source, Reason string
}

func (e *Error) Error() string {
	return "Invalid regular expression: /" + e.Source + "/: " + e.Reason
}

var compiled sync.Map // glob → *regexp.Regexp or *Error

// Compile is the glob's regular expression, compiled once per glob; an error
// when JavaScript would refuse it. RE2 takes `^?`, a `?` repeating the start,
// which JavaScript refuses, so a glob starting with `?` is refused here too.
func Compile(glob string) (*regexp.Regexp, error) {
	if c, ok := compiled.Load(glob); ok {
		if re, ok := c.(*regexp.Regexp); ok {
			return re, nil
		}
		return nil, c.(*Error)
	}
	source := Source(glob)
	re, err := regexp.Compile(source)
	if err == nil && strings.HasPrefix(source, "^?") {
		err = &syntax.Error{Code: syntax.ErrMissingRepeatArgument}
	}
	if err != nil {
		e := &Error{Source: strings.ReplaceAll(source, jsDot, "."), Reason: reason(err)}
		compiled.Store(glob, e)
		return nil, e
	}
	compiled.Store(glob, re)
	return re, nil
}

// reason is V8's words for RE2's refusal: a repetition with nothing to
// repeat, or repeating a repetition, is "Nothing to repeat"; any other
// (which no glob reaches) keeps RE2's.
func reason(err error) string {
	if s, ok := err.(*syntax.Error); ok {
		switch s.Code {
		case syntax.ErrMissingRepeatArgument, syntax.ErrInvalidRepeatOp:
			return "Nothing to repeat"
		}
		return string(s.Code)
	}
	return err.Error()
}

// Match is whether the glob matches the whole path.
func Match(glob, path string) (bool, error) {
	re, err := Compile(glob)
	if err != nil {
		return false, err
	}
	return re.MatchString(path), nil
}

// MatchesAny is whether any of the globs matches the path (matchesAny),
// tried in order up to the first that does, as `some` tries them: a glob
// after a match is not compiled, so it is no error.
func MatchesAny(path string, globs []string) (bool, error) {
	for _, g := range globs {
		if ok, err := Match(g, path); ok || err != nil {
			return ok, err
		}
	}
	return false, nil
}
