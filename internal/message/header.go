package message

// The built-in header lint (commits.header_lint.use: builtin): what
// commitlint does with @commitlint/config-conventional, in Go, so that a
// project needs no Node to judge its headers. Each part follows the package
// that does it there, as this repository's node_modules hold them:
// conventional-commits-parser 7 with the conventionalcommits preset's
// patterns reads the message into a header, its type, scope and subject, a
// body and a footer (parse); @commitlint/is-ignored passes the messages
// commitlint leaves alone (ignored); @commitlint/rules judges them, at the
// levels config-conventional sets (headerRules), each problem under the
// rule's id and in its words. type-enum's list is commits.types, or
// config-conventional's own when the config has none.

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/value"
)

// conventionalTypes are config-conventional's type-enum, for a config that
// lists no commits.types.
var conventionalTypes = []string{"build", "chore", "ci", "docs", "feat", "fix", "perf", "refactor", "revert", "style", "test"}

// maxLength is config-conventional's limit for the header and for each line
// of the body and the footer.
const maxLength = 100

// line is one character of a line as JavaScript's dot reads it: anything but
// its four line terminators (RE2's dot stops only at \n).
const line = `[^\n\r\x{2028}\x{2029}]`

var (
	// The conventionalcommits preset's header patterns, the breaking one
	// tried first, as the parser tries them.
	breakingHeader = regexp.MustCompile(`^(\w*)(?:\((` + line + `*)\))?!: (` + line + `*)$`)
	headerPattern  = regexp.MustCompile(`^(\w*)(?:\((` + line + `*)\))?!?: (` + line + `*)$`)
	// A footer's first line and a breaking-change note, case-insensitive
	// as JavaScript's /i reads ASCII; # is the preset's issue prefix.
	breaking    = `[Bb][Rr][Ee][Aa][Kk][Ii][Nn][Gg]`
	change      = `[Cc][Hh][Aa][Nn][Gg][Ee]`
	footerToken = regexp.MustCompile(`^(?:` + breaking + ` ` + change + `|[\w-]+)(?::` + value.Space + `+|` + value.Space + `+#)` + line + `+`)
	noteLine    = regexp.MustCompile(`^(?:\*` + value.Space + `+)?(?:` + breaking + `[ -]` + change + `):`)
	gpgLine     = regexp.MustCompile(`^` + value.Space + `*gpg:`)
	newline     = regexp.MustCompile(`\r?\n`)
	// A line with a URL may be as long as it likes (@commitlint/ensure).
	url = regexp.MustCompile(`\bhttps?://[^` + value.SpaceChars + `]+`)
)

// scissor is the line below which git leaves what the editor showed.
const scissor = "------------------------ >8 ------------------------"

// parsed is a message as the parser reads it: nil where it has none.
type parsed struct {
	raw                  string
	header, body, footer *string
	typ, scope, subject  string // "" where the header gives none
}

func truthy(s *string) bool { return s != nil && *s != "" }

// lines is a text split as /\r?\n/ splits it.
func lines(text string) []string { return newline.Split(text, -1) }

// trimNewLines is the text without the line breaks at either end.
func trimNewLines(text string) string {
	return strings.TrimRight(strings.TrimLeft(text, "\r\n"), "\r\n")
}

// appendLine is the parser's: a line after a text, or alone when the text
// is empty.
func appendLine(text *string, l string) *string {
	if truthy(text) {
		l = *text + "\n" + l
	}
	return &l
}

// parser reads a message's lines into its parts (CommitParser).
type parser struct {
	lines []string
	at    int
	p     parsed
}

func (ps *parser) available() bool { return ps.at < len(ps.lines) }

// notes reads a breaking-change note and the lines that continue it into
// the footer, up to and with the next footer line.
func (ps *parser) notes() bool {
	if !ps.available() || !noteLine.MatchString(ps.lines[ps.at]) {
		return false
	}
	ps.p.footer = appendLine(ps.p.footer, ps.lines[ps.at])
	ps.at++
	for ps.available() {
		if ps.notes() {
			return true
		}
		token := footerToken.MatchString(ps.lines[ps.at])
		ps.p.footer = appendLine(ps.p.footer, ps.lines[ps.at])
		ps.at++
		if token {
			break
		}
	}
	return true
}

// bodyOrFooter reads one line into the body while it still is one, else the
// footer; whether the body goes on.
func (ps *parser) bodyOrFooter(isBody bool) bool {
	if !ps.available() {
		return isBody
	}
	still := isBody && !footerToken.MatchString(ps.lines[ps.at])
	if still {
		ps.p.body = appendLine(ps.p.body, ps.lines[ps.at])
	} else {
		ps.p.footer = appendLine(ps.p.footer, ps.lines[ps.at])
	}
	ps.at++
	return still
}

// parse is the message read as commitlint reads it: comment lines (those
// starting with commentChar, and everything below git's scissor line) left
// out when commentChar is given, as commitlint --edit does, and gpg's lines
// always.
func parse(raw, commentChar string) parsed {
	ps := &parser{lines: kept(raw, commentChar), p: parsed{raw: raw}}
	if ps.available() {
		header := ps.lines[0]
		ps.at++
		if header != "" {
			ps.p.header = &header
			m := breakingHeader.FindStringSubmatch(header)
			if m == nil {
				m = headerPattern.FindStringSubmatch(header)
			}
			if m != nil {
				ps.p.typ, ps.p.scope, ps.p.subject = m[1], m[2], m[3]
			}
		}
	}
	isBody := true
	for ps.available() {
		if ps.notes() {
			isBody = false
		}
		if !ps.bodyOrFooter(isBody) {
			isBody = false
		}
	}
	for _, part := range []**string{&ps.p.body, &ps.p.footer} {
		if truthy(*part) {
			trimmed := trimNewLines(**part)
			*part = &trimmed
		}
	}
	return ps.p
}

// kept are the lines of a message the parser reads: the line breaks at
// either end trimmed, then comment lines and everything from git's scissor
// line down left out when commentChar is given, and gpg's lines always.
func kept(raw, commentChar string) []string {
	found := []string{}
	for _, l := range lines(trimNewLines(raw)) {
		if commentChar != "" && l == commentChar+" "+scissor {
			break
		}
		if commentChar != "" && strings.HasPrefix(l, commentChar) || gpgLine.MatchString(l) {
			continue
		}
		found = append(found, l)
	}
	return found
}

// Cleaned is the message the commit-msg hook's file holds as the header lint
// reads it (bug 36): its kept lines, so that its first line is the header the
// lint judges, whatever blank or comment lines an editor left above it. The
// hook judges the type, the paths and the footers on it, never on the file's
// own first line, which would hide a commit's type behind a comment.
func Cleaned(raw, commentChar string) string {
	return strings.Join(kept(raw, commentChar), "\n")
}

// length is a text's length as JavaScript counts it, in UTF-16 code units.
func length(text string) int {
	n := 0
	for _, r := range text {
		n += utf16Len(r)
	}
	return n
}

func utf16Len(r rune) int {
	if r > 0xffff {
		return 2
	}
	return 1
}

// linesFit is whether each line of a text is at most max long, or has a URL.
func linesFit(text string) bool {
	for _, l := range lines(text) {
		if !url.MatchString(l) && length(l) > maxLength {
			return false
		}
	}
	return true
}

// blankBefore is whether the raw line before index is empty, the index read
// as JavaScript's slice reads a negative one.
func blankBefore(raw []string, index int) bool {
	start := index - 1
	if start < 0 {
		start = max(len(raw)+start, 0)
	}
	return start < len(raw) && raw[start] == ""
}

// rule is one of config-conventional's rules: its id, whether it is a
// warning rather than an error, and its judgement, "" when the message
// passes it, else why not.
type rule struct {
	name    string
	warning bool
	judge   func(p parsed, types []string) string
}

// headerRules are config-conventional's, in the order it lists them, which
// is the order commitlint reports them in, errors before warnings.
var headerRules = []rule{
	{"body-leading-blank", true, func(p parsed, _ []string) string {
		raw := lines(p.raw)
		if !truthy(p.body) || len(raw) > 1 && raw[1] == "" {
			return ""
		}
		return "body must have leading blank line"
	}},
	{"body-max-line-length", false, func(p parsed, _ []string) string {
		if !truthy(p.body) || linesFit(*p.body) {
			return ""
		}
		return "body's lines must not be longer than " + strconv.Itoa(maxLength) + " characters"
	}},
	{"footer-leading-blank", true, func(p parsed, _ []string) string {
		if !truthy(p.footer) {
			return ""
		}
		raw := lines(p.raw)
		first := lines(*p.footer)[0]
		index := -1
		for i, l := range raw {
			if l == first {
				index = i
				break
			}
		}
		if blankBefore(raw, index) {
			return ""
		}
		return "footer must have leading blank line"
	}},
	{"footer-max-line-length", false, func(p parsed, _ []string) string {
		if !truthy(p.footer) || linesFit(*p.footer) {
			return ""
		}
		return "footer's lines must not be longer than " + strconv.Itoa(maxLength) + " characters"
	}},
	{"header-max-length", false, func(p parsed, _ []string) string {
		current := "undefined"
		if p.header != nil {
			if length(*p.header) <= maxLength {
				return ""
			}
			current = strconv.Itoa(length(*p.header))
		}
		return "header must not be longer than " + strconv.Itoa(maxLength) + " characters, current length is " + current
	}},
	{"header-trim", false, func(p parsed, _ []string) string {
		if !truthy(p.header) {
			return ""
		}
		first, _ := utf8.DecodeRuneInString(*p.header)
		last, _ := utf8.DecodeLastRuneInString(*p.header)
		starts, ends := value.IsSpace(first), value.IsSpace(last)
		switch {
		case starts && ends:
			return "header must not be surrounded by whitespace"
		case starts:
			return "header must not start with whitespace"
		case ends:
			return "header must not end with whitespace"
		}
		return ""
	}},
	{"subject-case", false, func(p parsed, _ []string) string {
		if cases := subjectCases(p.subject); len(cases) > 0 {
			return "subject must not be " + strings.Join(cases, ", ")
		}
		return ""
	}},
	{"subject-empty", false, func(p parsed, _ []string) string {
		if p.subject != "" {
			return ""
		}
		return "subject may not be empty"
	}},
	// On the header, not the subject, as commitlint has it: a header that
	// ends at its type's colon has no subject to end.
	{"subject-full-stop", false, func(p parsed, _ []string) string {
		if p.header == nil {
			return ""
		}
		h := *p.header
		if colon := strings.Index(h, ":"); colon > 0 && colon == len(h)-1 {
			return ""
		}
		if !strings.HasSuffix(h, ".") || strings.HasSuffix(h, "...") {
			return ""
		}
		return "subject may not end with full stop"
	}},
	{"type-case", false, func(p parsed, _ []string) string {
		if p.typ == "" || inCase(p.typ, "lower-case") {
			return ""
		}
		return "type must be lower-case"
	}},
	{"type-empty", false, func(p parsed, _ []string) string {
		if p.typ != "" {
			return ""
		}
		return "type may not be empty"
	}},
	{"type-enum", false, func(p parsed, types []string) string {
		if p.typ == "" || value.Includes(types, p.typ) {
			return ""
		}
		return "type must be one of [" + strings.Join(types, ", ") + "]"
	}},
}

// blankRules judge a blank message, which has no header to measure: its
// type and subject are missing.
var blankRules = named("subject-empty", "type-empty")

// named are the header rules of those names, in their order.
func named(names ...string) []rule {
	var found []rule
	for _, r := range headerRules {
		if value.Includes(names, r.name) {
			found = append(found, r)
		}
	}
	return found
}

// HeaderProblems are the built-in header lint's problems with a message,
// errors then warnings, each with its level and fix. commentChar is the
// comment character whose lines are left out ("" for none: a message from
// stdin or a commit). A message commitlint ignores (a merge's, a revert's,
// a fixup's, a version's) has none, and so does one with nothing left once
// its comments are out. A blank message has none in the hook's reading,
// where commitlint --edit lints nothing and git then aborts the commit for
// its empty message; on stdin it has neither type nor subject, where
// commitlint refuses it as no input at all.
func HeaderProblems(cfg *config.Loaded, raw, commentChar string) ([]Leveled, error) {
	rules := headerRules
	var p parsed
	if value.Trim(raw) == "" && commentChar != "" {
		return []Leveled{}, nil
	} else if value.Trim(raw) == "" {
		rules = blankRules
	} else if ignored(strings.TrimRightFunc(raw, value.IsSpace)) {
		return []Leveled{}, nil
	} else if p = parse(raw, commentChar); p.header == nil && p.body == nil && p.footer == nil {
		return []Leveled{}, nil
	}
	types := Types(cfg)
	var errors, warnings []Leveled
	for _, r := range rules {
		why := r.judge(p, types)
		if why == "" {
			continue
		}
		fix, err := fixFor(cfg, r.name, why)
		if err != nil {
			return nil, err
		}
		if r.warning {
			warnings = append(warnings, Leveled{Rule: r.name, Message: why, Fix: fix, Level: "warning"})
		} else {
			errors = append(errors, Leveled{Rule: r.name, Message: why, Fix: fix, Level: "error"})
		}
	}
	return append(append([]Leveled{}, errors...), warnings...), nil
}

// Types are the commit types type-enum takes: commits.types, or
// config-conventional's own when the config lists none.
func Types(cfg *config.Loaded) []string {
	if cfg.Commits.Types == nil {
		return conventionalTypes
	}
	return cfg.Commits.Types
}

// CommentChar is git's comment character in this repository
// (core.commentChar), # when it sets none, which commitlint --edit leaves
// out of the message file as git will.
func CommentChar() string {
	out, _ := git.Output("config", "core.commentChar")
	if c := value.Trim(out); c != "" {
		return c
	}
	return "#"
}

// hasError is whether a list of problems holds an error, not only warnings.
func hasError(found []Leveled) bool {
	for _, p := range found {
		if p.Level == "error" {
			return true
		}
	}
	return false
}
