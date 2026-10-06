package message

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/ledger"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/shell"
	"github.com/donvargax/itos/v6/internal/tests"
	"github.com/donvargax/itos/v6/internal/value"
)

// fixes resolve each rule of @commitlint/config-conventional an agent meets.
var fixes = map[string]string{
	"type-enum":              "start the header with one of the commit types, as in `docs: …`",
	"type-case":              "write the type in lower case",
	"type-empty":             "start the header with a type and a colon, as in `fix: …`",
	"subject-empty":          "write a subject after the type's colon",
	"subject-case":           "start the subject with a lower-case letter",
	"subject-full-stop":      "remove the full stop at the end of the subject",
	"header-max-length":      "shorten the header to 100 characters or fewer",
	"header-trim":            "remove the spaces around the header",
	"body-leading-blank":     "leave a blank line between the header and the body",
	"footer-leading-blank":   "leave a blank line before the footers",
	"body-max-line-length":   "wrap the body at 100 characters",
	"footer-max-line-length": "split the footer over several lines of 100 characters or fewer; a footer may repeat",
}

var needs = regexp.MustCompile(` need a `)

// footerFix is a footer rule's fix, by what its sentence says; "" for a rule
// no footer of the config has.
func footerFix(cfg *config.Loaded, rule, message string) (string, error) {
	for _, key := range cfg.Commits.Footers.Keys {
		if strings.ToLower(key)+"-footer" != rule {
			continue
		}
		f := cfg.Commits.Footers.Values[key]
		if cfg.Stealth {
			if fix := stealthFix(cfg, key, f, message); fix != "" {
				return fix, nil
			}
		}
		if f.Text() {
			if needs.MatchString(message) {
				return "add a line `" + key + ": <what a consumer must do>` after a blank line at the end, or `" +
					key + ": none` when a consumer changes nothing", nil
			}
			return "write after " + key + ": what a consumer must do, or none", nil
		}
		src := "the " + f.Source.Tests + " files"
		if f.Registry() {
			src = cfg.Work.Registry
		}
		if isLedger(f) {
			layout, err := ledger.LayoutOf(cfg)
			if err != nil {
				return "", err
			}
			src = layout.Dir + "/"
		}
		switch {
		case needs.MatchString(message):
			return "add a line `" + key + ": <id>` after a blank line at the end, naming what the commit belongs to", nil
		case strings.HasPrefix(message, "unknown "):
			return "name an id " + src + " has, or add it first in a docs commit", nil
		}
		return "make each id the " + key + ": footer names live in the same commit", nil
	}
	return "", nil
}

func fixFor(cfg *config.Loaded, rule, message string) (string, error) {
	if fix, ok := fixes[rule]; ok {
		return fix, nil
	}
	return footerFix(cfg, rule, message)
}

// Leveled is a problem of the header lint's report, with its level: error
// or warning.
type Leveled struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
	Level   string `json:"level"`
}

// reportLine is one problem of a delegate's report: `✖` (error) or `⚠`
// (warning), the sentence, and its rule in brackets at the end. JavaScript's
// dot and \s, spelled out, since RE2's differ.
var reportLine = regexp.MustCompile(`^(✖|⚠)` + value.Space + `+([^\n\r\x{2028}\x{2029}]*[^` + value.SpaceChars + `])` +
	value.Space + `+\[([\w-]+)\]$`)

// ParseReport is the delegate's report, one problem per line that is a
// problem; anything else it printed, when it failed without an error among
// them, becomes one header-lint problem, so a failure never reads as clean.
func ParseReport(cfg *config.Loaded, text string, failed bool) ([]Leveled, error) {
	found := []Leveled{}
	hasError := false
	for _, line := range strings.Split(text, "\n") {
		m := reportLine.FindStringSubmatch(value.Trim(line))
		if m == nil {
			continue
		}
		fix, err := fixFor(cfg, m[3], m[2])
		if err != nil {
			return nil, err
		}
		level := "warning"
		if m[1] == "✖" {
			level, hasError = "error", true
		}
		found = append(found, Leveled{Rule: m[3], Message: m[2], Fix: fix, Level: level})
	}
	if failed && !hasError {
		found = append(found, Leveled{Rule: "header-lint", Message: value.Trim(text), Level: "error"})
	}
	return found, nil
}

// Streams are how a check reports: under JSON one object on Stdout, else the
// delegate's report as it comes and the footer problems on Stderr.
type Streams struct {
	JSON           bool
	Stdout, Stderr io.Writer
}

// Check is `commit check-message` on one message (commit.ts's checkMessage):
// the header lint, built in or the delegate (commits.header_lint.stdin) with
// the message on its stdin, then the footer rules, at the commit r.At names.
// 1 when the message fails either, else 0.
func Check(cfg *config.Loaded, message string, r Reading, s Streams) (int, error) {
	if cfg.Commits.HeaderLint.Builtin() {
		header, err := HeaderProblems(cfg, message, "")
		if err != nil {
			return 0, err
		}
		return builtin(cfg, message, header, r, s)
	}
	delegate := cfg.Commits.HeaderLint.Stdin
	if delegate == nil || *delegate == "" {
		return footersOnly(cfg, message, r, s)
	}
	return delegated(cfg, *delegate, message, r, s)
}

// footersOnly is the footer rules alone, without a delegate.
func footersOnly(cfg *config.Loaded, message string, r Reading, s Streams) (int, error) {
	found, err := FooterProblems(cfg, message, r)
	if err != nil {
		return 0, err
	}
	if s.JSON {
		if err := out.Emit(s.Stdout, out.Field{Key: "ok", Value: len(found) == 0}, out.Field{Key: "problems", Value: found}); err != nil {
			return 0, err
		}
	} else {
		PrintFooters(s.Stderr, found)
	}
	if len(found) > 0 {
		return 1, nil
	}
	return 0, nil
}

// builtin is the built-in header lint's problems, then the footer rules':
// printed on Stderr as commitlint prints a problem, the header's first, or
// under JSON one list, each with its level. Its warnings alone do not fail
// the message.
func builtin(cfg *config.Loaded, message string, header []Leveled, r Reading, s Streams) (int, error) {
	found, err := FooterProblems(cfg, message, r)
	if err != nil {
		return 0, err
	}
	ok := !hasError(header) && len(found) == 0
	if s.JSON {
		problems := append([]Leveled{}, header...)
		for _, p := range found {
			problems = append(problems, Leveled{Rule: p.Rule, Message: p.Message, Fix: p.Fix, Level: "error"})
		}
		if err := out.Emit(s.Stdout, out.Field{Key: "ok", Value: ok}, out.Field{Key: "problems", Value: problems}); err != nil {
			return 0, err
		}
	} else {
		PrintLeveled(s.Stderr, header)
		PrintFooters(s.Stderr, found)
	}
	if ok {
		return 0, nil
	}
	return 1, nil
}

// PrintLeveled prints the header lint's problems as commitlint prints them:
// ✖ for an error, ⚠ for a warning.
func PrintLeveled(w io.Writer, found []Leveled) {
	for _, p := range found {
		sign := "⚠"
		if p.Level == "error" {
			sign = "✖"
		}
		fmt.Fprintf(w, "%s   %s [%s]\n", sign, p.Message, p.Rule)
	}
}

// delegated is the delegate, its report printed as it comes or read into
// problems, then the footer rules; under JSON one list, the delegate's
// problems first. The delegate reads the commit from ITOS_AT, as the footer
// rules read it.
func delegated(cfg *config.Loaded, delegate, message string, r Reading, s Streams) (int, error) {
	o := shell.Options{Stdin: strings.NewReader(message), Stdout: s.Stdout, Stderr: s.Stderr}
	var stdout, stderr bytes.Buffer
	if s.JSON {
		o.Stdout, o.Stderr = &stdout, &stderr
	}
	if r.At != "" {
		o.Env = append(os.Environ(), "ITOS_AT="+r.At)
	}
	linted := shell.Run(cfg, delegate, o).OK()
	found, err := FooterProblems(cfg, message, r)
	if err != nil {
		return 0, err
	}
	ok := linted && len(found) == 0
	if s.JSON {
		problems, err := ParseReport(cfg, stdout.String()+"\n"+stderr.String(), !linted)
		if err != nil {
			return 0, err
		}
		for _, p := range found {
			problems = append(problems, Leveled{Rule: p.Rule, Message: p.Message, Fix: p.Fix, Level: "error"})
		}
		if err := out.Emit(s.Stdout, out.Field{Key: "ok", Value: ok}, out.Field{Key: "problems", Value: problems}); err != nil {
			return 0, err
		}
	} else {
		PrintFooters(s.Stderr, found)
	}
	if ok {
		return 0, nil
	}
	return 1, nil
}

// HookStreams are the commit-msg hook's streams, which its header lint
// delegate inherits.
type HookStreams struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// LintFile is the commit-msg hook's header lint (commit.ts's
// lintMessageFile): the built-in lint on the message file as commitlint
// --edit reads it (a line break added, git's comment lines left out), or the
// message file through commits.header_lint.hook, {file} filled in as one
// shell word, its report printed as it comes, then the footer rules, always,
// read where r says. A failing delegate's exit code is the hook's (1 when it
// did not exit by itself); header or footer problems alone exit 1. Without
// either, the footer rules alone.
func LintFile(cfg *config.Loaded, file, message string, r Reading, s HookStreams) (int, error) {
	if cfg.Commits.HeaderLint.Builtin() {
		header, err := HeaderProblems(cfg, message+"\n", CommentChar())
		if err != nil {
			return 0, err
		}
		return builtin(cfg, message, header, r, Streams{Stdout: s.Stdout, Stderr: s.Stderr})
	}
	delegate := cfg.Commits.HeaderLint.Hook
	if delegate == nil || *delegate == "" {
		return footersOnly(cfg, message, r, Streams{Stdout: s.Stdout, Stderr: s.Stderr})
	}
	command := strings.ReplaceAll(*delegate, "{file}", tests.ShellWord(file))
	run := shell.Run(cfg, command, shell.Options{Stdin: s.Stdin, Stdout: s.Stdout, Stderr: s.Stderr})
	found, err := FooterProblems(cfg, message, r)
	if err != nil {
		return 0, err
	}
	PrintFooters(s.Stderr, found)
	switch {
	case run.Code > 0:
		return run.Code, nil
	case run.Code < 0 || run.Err != nil:
		return 1, nil
	case len(found) > 0:
		return 1, nil
	}
	return 0, nil
}
