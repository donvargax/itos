package ciwatch

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/donvargax/itos/v8/internal/providers"
)

// What a failed job's log may name (slice 172, features/watch-failures.feature):
// the lines a failure keeps, the failures a job keeps, and the lines of a tail.
const (
	maxLines    = 10
	maxFailures = 20
	tailLines   = 20
)

var (
	// An escape sequence: a CSI, an OSC or another string (DCS, SOS, PM, APC) to its BEL or
	// ST or the line's end, any other escape, or a lone ESC; as tools/bin/nightly-report
	// strips them.
	escape = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|[\]PX^_][^\x07\x1b]*(?:\x07|\x1b\\)?|[ -/]*[0-~])?`)
	// The time GitHub stamps a line with, and the space after it.
	timestamp = regexp.MustCompile(`^[0-9][0-9TZ:.-]* `)
	// go test's line for a failed test or subtest, its indentation and its name.
	goTestFail = regexp.MustCompile(`^([ \t]*)--- FAIL: (\S+)`)
	// go test's line for a failed package, its name.
	packageFail = regexp.MustCompile(`^FAIL\t(\S+)`)
	// itos ci run's line for a step that failed, what follows its lead.
	ciRunFail = regexp.MustCompile(`^(?:CI failed at|also failed at)(.*)$`)
	// godog's line for a failed scenario in its summary, the scenario's name.
	scenarioFail = regexp.MustCompile(`^\s*Scenario: (.*) # \S+:\d+$`)
	// The suffix go test gives a subtest whose name another has.
	duplicateSuffix = regexp.MustCompile(`#\d+$`)
)

// cleanLog is a job's log as the API gives it, one line an entry, without the
// carriage return at the end of a Windows runner's lines, GitHub's timestamps
// and every terminal escape sequence, as tools/bin/nightly-report reads the
// nightly's logs (T-185): none for an empty log.
func cleanLog(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i, l := range lines {
		lines[i] = timestamp.ReplaceAllString(escape.ReplaceAllString(strings.TrimSuffix(l, "\r"), ""), "")
	}
	return lines
}

// readLog is what a failed job's log names: its failures in the log's order,
// the job's at most maxFailures with the rest counted, each with at most
// maxLines lines and the rest counted. When no failure has lines of its own,
// a tail of the log comes last, so a failed job's log never says nothing.
func readLog(text string) (failures []providers.Failure, more int) {
	lines := cleanLog(text)
	failures = mergeScenarios(failuresIn(lines))
	if !slices.ContainsFunc(failures, func(f providers.Failure) bool { return len(f.Lines) > 0 }) {
		if tail, ok := tailOf(lines); ok {
			failures = append(failures, tail)
		}
	}
	if len(failures) > maxFailures {
		more, failures = len(failures)-maxFailures, failures[:maxFailures]
	}
	for i, f := range failures {
		if f.Kind != "tail" && len(f.Lines) > maxLines {
			failures[i].MoreLines, failures[i].Lines = len(f.Lines)-maxLines, f.Lines[:maxLines]
		}
	}
	return failures, more
}

// failuresIn are the failures the lines name, in order, their lines whole.
func failuresIn(lines []string) []providers.Failure {
	var failures []providers.Failure
	for i := 0; i < len(lines); i++ {
		switch {
		case lines[i] == "--- Failed steps:":
			var found []providers.Failure
			found, i = failedScenarios(lines, i+1)
			failures = append(failures, found...)
			i--
		case ciRunFail.MatchString(lines[i]):
			var f providers.Failure
			f, i = ciRunAt(lines, i)
			failures = append(failures, f)
			i--
		case goTestFail.MatchString(lines[i]):
			end := endOfTest(lines, i)
			if leaf(lines, i, end) {
				failures = append(failures, goTestAt(lines, i, end))
				i = end - 1
			}
		}
	}
	return failures
}

// failedScenarios are the entries of godog's summary of failed steps that
// start at lines[from], and the line after them. An entry is the line of the
// scenario, its failing step and its error, up to the blank line that ends
// it: the step and the error less godog's indentation, the error's further
// lines as it printed them.
func failedScenarios(lines []string, from int) ([]providers.Failure, int) {
	var failures []providers.Failure
	i := from
	for i < len(lines) {
		if lines[i] == "" {
			i++
			continue
		}
		m := scenarioFail.FindStringSubmatch(lines[i])
		if m == nil {
			break
		}
		f := providers.Failure{Kind: "scenario", Name: m[1], Lines: []string{}}
		for i++; i < len(lines) && lines[i] != ""; i++ {
			l := lines[i]
			if len(f.Lines) < 2 {
				l = strings.TrimLeft(l, " \t")
			}
			f.Lines = append(f.Lines, l)
		}
		failures = append(failures, f)
	}
	return failures, i
}

// ciRunAt is the failure itos ci run's line lines[i] names, and the line
// after it: its name is the text after the lead less the colon or space that
// opens it, its lines the ones after it up to a blank line, GitHub's next
// "##[" line or the next failure line.
func ciRunAt(lines []string, i int) (providers.Failure, int) {
	rest := ciRunFail.FindStringSubmatch(lines[i])[1]
	f := providers.Failure{Kind: "ci-run", Name: strings.TrimPrefix(strings.TrimPrefix(rest, ":"), " "), Lines: []string{}}
	for i++; i < len(lines); i++ {
		l := lines[i]
		if l == "" || strings.HasPrefix(l, "##[") || ciRunFail.MatchString(l) || goTestFail.MatchString(l) {
			break
		}
		f.Lines = append(f.Lines, l)
	}
	f.Lines = dedent(f.Lines)
	return f, i
}

// indent is how many spaces and tabs a line starts with.
func indent(line string) int { return len(line) - len(strings.TrimLeft(line, " \t")) }

// endOfTest is the line after the lines indented deeper than the "--- FAIL:"
// line lines[i].
func endOfTest(lines []string, i int) int {
	depth := indent(lines[i])
	end := i + 1
	for end < len(lines) && indent(lines[end]) > depth {
		end++
	}
	return end
}

// leaf is whether the failed test lines[i] has no failed subtest among the
// lines of the test, up to end.
func leaf(lines []string, i, end int) bool {
	return !slices.ContainsFunc(lines[i+1:end], goTestFail.MatchString)
}

// goTestAt is the failure of the test whose "--- FAIL:" line is lines[i]: its
// lines are the ones indented deeper, up to end, and its package is the one
// of the first "FAIL", tab, package line after them.
func goTestAt(lines []string, i, end int) providers.Failure {
	f := providers.Failure{Kind: "go-test", Name: goTestFail.FindStringSubmatch(lines[i])[2], Lines: dedent(slices.Clone(lines[i+1 : end]))}
	for _, l := range lines[end:] {
		if m := packageFail.FindStringSubmatch(l); m != nil {
			f.Package = m[1]
			break
		}
	}
	return f
}

// dedent are the lines less the indentation they all share.
func dedent(lines []string) []string {
	shared := -1
	for _, l := range lines {
		if strings.TrimSpace(l) != "" && (shared < 0 || indent(l) < shared) {
			shared = indent(l)
		}
	}
	for i, l := range lines {
		lines[i] = l[min(max(shared, 0), len(l)):]
	}
	return lines
}

// mergeScenarios drops the go test failure of a godog scenario that godog's
// summary names, the same failure: TestFeatures/<name>, the name rewritten as
// go test rewrites a subtest's.
func mergeScenarios(failures []providers.Failure) []providers.Failure {
	named := map[string]bool{}
	for _, f := range failures {
		if f.Kind == "scenario" {
			named["TestFeatures/"+rewrite(f.Name)] = true
		}
	}
	return slices.DeleteFunc(failures, func(f providers.Failure) bool {
		return f.Kind == "go-test" && named[duplicateSuffix.ReplaceAllString(f.Name, "")]
	})
}

// rewrite is a subtest's name as go test prints it: spaces as underscores
// and a character that does not print escaped.
func rewrite(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsSpace(r):
			b.WriteByte('_')
		case !strconv.IsPrint(r):
			q := strconv.QuoteRune(r)
			b.WriteString(q[1 : len(q)-1])
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// tailOf is the failure of no name that holds the last tailLines lines of the
// log up to and including GitHub's first "##[error]" line, else the log's
// last ones; the lines before them are counted. Not ok for an empty log.
func tailOf(lines []string) (providers.Failure, bool) {
	end := len(lines)
	for i, l := range lines {
		if strings.HasPrefix(l, "##[error]") {
			end = i + 1
			break
		}
	}
	if end == 0 {
		return providers.Failure{}, false
	}
	start := max(end-tailLines, 0)
	return providers.Failure{Kind: "tail", Lines: slices.Clone(lines[start:end]), MoreLines: start}, true
}
