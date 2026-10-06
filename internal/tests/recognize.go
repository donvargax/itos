package tests

import (
	"regexp"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/value"
)

// word is a `{pattern}` in a recognize template: one shell word, double-quoted
// without `$`, a backquote, `"` or `\`, single-quoted, or bare. Its three
// groups are the pattern each way, so the first that matched is the pattern.
// The bare word stops at JavaScript's whitespace, as the TypeScript's `\s`
// does, where RE2's `\s` is ASCII alone.
const word = `(?:"([^"$` + "`" + `\\]+)"|'([^']+)'|([^` + value.SpaceChars + `"'$` + "`" + `\\]+))`

// templateRegexp is a recognize template as a pattern a whole command
// matches: each of its words as written, a `{pattern}` in one standing for
// one shell word, the words separated by any run of whitespace.
func templateRegexp(command string) (*regexp.Regexp, error) {
	words := value.Fields(value.Trim(command))
	for i, w := range words {
		parts := strings.Split(w, "{pattern}")
		for j, p := range parts {
			parts[j] = regexp.QuoteMeta(p)
		}
		words[i] = strings.Join(parts, word)
	}
	return regexp.Compile("^" + strings.Join(words, value.Space+"+") + "$")
}

// Recognize reads a task check back as a selection of the kind (tests.ts's
// recognize), and false for a check that is not one of its runs (other
// flags, a pattern the shell would expand): that check runs as it is. Each
// of the kind's recognize templates is tried in order on each of the check's
// readings (config.Readings), so one written for itos reads a check that
// calls hooks.bin. `as: whole` is every test, `as: smoke` the smoke set given
// (its IDs), and any other the pattern its `{pattern}` matched; a template
// without one matches nothing that way, and the next is tried.
func Recognize(cfg *config.Loaded, name, command string, smoke []string) (Selection, bool, error) {
	k, err := KindOf(cfg, name)
	if err != nil {
		return Selection{}, false, err
	}
	forms := cfg.Readings(command)
	for _, rule := range k.Recognize {
		template, err := templateRegexp(rule.Command)
		if err != nil {
			return Selection{}, false, err
		}
		var m []string
		for _, form := range forms {
			if m = template.FindStringSubmatch(form); m != nil {
				break
			}
		}
		if m == nil {
			continue
		}
		switch rule.As {
		case "whole":
			return Whole(), true, nil
		case "smoke":
			return IDs(smoke...), true, nil
		}
		// A group that did not match is "" in Go, undefined in JavaScript; a
		// group that did holds one character at least.
		for _, group := range m[1:min(len(m), 4)] {
			if group != "" {
				return Pattern(group), true, nil
			}
		}
	}
	return Selection{}, false, nil
}
