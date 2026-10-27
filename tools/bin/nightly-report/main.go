// Command nightly-report prints the body of the Nightly red issue (T-185): what each red job of
// one attempt of a nightly run failed at, read from its log.
//
//	go run ./tools/bin/nightly-report -repo donvargax/itos -run <id> -attempt <n> -url <run's URL>
//
// nightly.yml's report job runs it, once every other job is done, and opens or comments on the
// issue with what it prints. A red job is one that concluded neither success nor skipped. Each
// one's log is read from the API (the run is still going, so not gh run view's logs) and stripped
// of its terminal escape sequences (CSI, colours among them, and OSC, such as hyperlinks), so none
// reaches the body, and of the carriage returns and timestamps GitHub adds. From it:
//
//   - the failing scenarios: go test's `--- FAIL: TestFeatures/…` lines, which the step scripts
//     GitHub echoes into the log never start with;
//   - the failing fuzz inputs: the `fuzz input: <path>` lines tools/bin/nightly-fuzz prints, each
//     before the input's bytes, which stay in the log (the nightly keeps no artifacts);
//   - each failed step of the plan, from itos ci run's "CI failed at" and "also failed at" lines,
//     with its exit code, and a command of several lines by its first; for one that runs itos-cc
//     mutation run, each site the --json report it printed names with rule mutation.survived,
//     mutation.uncovered or mutation.uncovered-statement (itos-cc 0.26.0's unexecuted block,
//     named by its line alone), and, when it names the rule interrupted, where its sampling says
//     it stopped (its keys alone, never a problem's message, which itos-cc's contract leaves out).
//     The report is read from the step's own output, the lines between its "$ " line and the next
//     step's, with any failure line itos interleaved into it left out. itos prints a step's "$ "
//     line, after a blank one, on stdout and a failure, after a blank one, on stderr, and the log
//     merges the two by line: a step's blank line may fall inside the failure before it, so a "$ "
//     line just after a failure starts a step too, and a failure's blank lines do not end it;
//   - when the runner shut down or the operation was cancelled, the last step that started
//     (the plan's, else the job's), that the steps after it did not run, and, for a mutation
//     step, the sites its report names, as it prints one when interrupted.
//
// Everything is read from the log, never from ci run --json: the report job needs no artifact.
// It asks GitHub through gh, with the token gh has (the job hands it github.token). A job list
// that cannot be read, or a log, is said in the body, never a failure of the program: the issue
// still names the run, and the error's first line, gh's stderr included, under that sentence; the
// whole error goes to stderr, into the report job's log.
//
// It imports nothing but the standard library, as the other gate programs do.
//
// Exit status: 0 the body printed, 2 a usage error.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// runner runs a program by name and returns its stdout; an error carries its stderr.
type runner func(name string, args ...string) (string, error)

func main() {
	os.Exit(run(os.Args[1:], execRun, os.Stdout, os.Stderr))
}

func run(args []string, cmd runner, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("nightly-report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", "", "the repository, owner/name")
	runID := flags.String("run", "", "the run's id")
	attempt := flags.String("attempt", "", "the run's attempt")
	url := flags.String("url", "", "the run's URL, which the body names")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 || *repo == "" || *runID == "" || *attempt == "" || *url == "" {
		fmt.Fprintln(stderr, "usage: nightly-report -repo <owner/name> -run <id> -attempt <n> -url <run's URL>")
		return 2
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Nightly run %s is red.\n", *url)
	jobs, err := redJobs(cmd, *repo, *runID, *attempt)
	if err != nil || len(jobs) == 0 {
		b.WriteString("\nIts jobs could not be read: see the run's log.\n")
	}
	if err != nil {
		fmt.Fprintf(stderr, "nightly-report: the jobs could not be read: %v\n", err)
		fmt.Fprintf(&b, "\n%s\n", code(firstLine(err), false))
	}
	for _, j := range jobs {
		// A newer gh (the nightly's 2.101.0) refuses to print a response holding escape
		// sequences without the flag, and a log holds the colours its steps print.
		text, err := cmd("gh", "api", "--allow-escape-sequences", fmt.Sprintf("repos/%s/actions/jobs/%d/logs", *repo, j.ID))
		fmt.Fprintf(&b, "\n### %s: %s\n\n", j.Name, j.Conclusion)
		if err != nil {
			fmt.Fprintf(stderr, "nightly-report: the log of job %s (%d) could not be read: %v\n", j.Name, j.ID, err)
			fmt.Fprintf(&b, "Its log could not be read: see the run.\n\n%s\n", code(firstLine(err), false))
			continue
		}
		b.WriteString(read(text).render())
	}
	fmt.Fprint(stdout, b.String())
	return 0
}

// firstLine is an error's text to its first line break, trimmed: gh's stderr, which execRun's
// error carries, may run to several lines.
func firstLine(err error) string {
	line, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")
	return strings.TrimSpace(line)
}

// job is a job of the attempt, as the jobs call lists it.
type job struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"` // empty while it runs
}

// redJobs are the attempt's jobs that concluded neither success nor skipped, in the API's order.
// gh --paginate prints one JSON object a page, back to back.
func redJobs(cmd runner, repo, runID, attempt string) ([]job, error) {
	out, err := cmd("gh", "api", "--paginate", fmt.Sprintf("repos/%s/actions/runs/%s/attempts/%s/jobs", repo, runID, attempt))
	if err != nil {
		return nil, err
	}
	var red []job
	dec := json.NewDecoder(strings.NewReader(out))
	for {
		var page struct {
			Jobs []job `json:"jobs"`
		}
		if err := dec.Decode(&page); errors.Is(err, io.EOF) {
			return red, nil
		} else if err != nil {
			return nil, err
		}
		for _, j := range page.Jobs {
			if j.Conclusion != "" && j.Conclusion != "success" && j.Conclusion != "skipped" {
				red = append(red, j)
			}
		}
	}
}

var (
	// An escape sequence: a CSI, an OSC or another string (DCS, SOS, PM, APC) to its BEL or ST
	// or the line's end, any other escape, or a lone ESC.
	escape    = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|[\]PX^_][^\x07\x1b]*(?:\x07|\x1b\\)?|[ -/]*[0-~])?`)
	timestamp = regexp.MustCompile(`^[0-9][0-9TZ:.-]* `)
	scenario  = regexp.MustCompile(`^\s*--- FAIL: TestFeatures/`)
	fuzzInput = regexp.MustCompile(`^fuzz input: (.+)$`)
	failedAt  = regexp.MustCompile(`^(CI failed at|also failed at)(.*)$`)
	exited    = regexp.MustCompile(`^(.*) \(it exited (\d+)\)$`)
	noCode    = regexp.MustCompile(`^(.*) \(it gave no exit code\)$`)
	jobStep   = regexp.MustCompile(`^##\[group\]Run (.*)$`)
)

// stops are the lines GitHub writes when a job is stopped from outside, and what each says.
var stops = []struct{ prefix, says string }{
	{"##[error]The runner has received a shutdown signal", "The runner received a shutdown signal"},
	{"##[error]The operation was canceled.", "The operation was cancelled"},
}

// clean is a log as the API gives it, one line an entry, without its escape sequences and
// GitHub's carriage returns and timestamps.
func clean(text string) []string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i, l := range lines {
		lines[i] = timestamp.ReplaceAllString(escape.ReplaceAllString(strings.TrimSuffix(l, "\r"), ""), "")
	}
	return lines
}

// failure is a failure itos ci run named: the text after its lead, the first line of the step's
// command and its exit code when the failure is a step's, and the lines it spans.
type failure struct {
	text    string // after the lead, for a failure that is not a step's
	command string // the step's command, its first line; empty when not a step's
	more    bool   // the command goes on past its first line
	code    string // "exited 1", "gave no exit code", or empty when not a step's or not in the log
	from    int    // its first line
	to      int    // its last line
	sites   *sites // a mutation step's report
}

// sites is what an itos-cc mutation run's report names: its survived and uncovered sites, and
// whether the run was interrupted, with where it stopped when its sampling says.
type sites struct {
	found       []site
	interrupted bool
	stop        string
	unread      bool // no report in the step's output, or one that is not JSON
}

// site is a problem of an itos-cc report.
type site struct {
	Rule        string `json:"rule"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
	Function    string `json:"function"`
}

// stepAt is a plan step's "$ " line: the first line of its command, and where it is.
type stepAt struct {
	command string
	line    int
}

// analysis is what one job's log says.
type analysis struct {
	scenarios []string
	fuzz      []string // the paths of the failing fuzz inputs
	failures  []*failure
	stopped   string // what stopped the job from outside, or empty
	last      string // the last step that started, when it was stopped
	plan      bool   // last is a plan step, not the job's
	lastSites *sites // the last step's report, when it is a mutation step no failure names
}

// read is what a job's log, as the API gives it, says.
func read(text string) analysis {
	lines := clean(text)
	var a analysis
	var steps []stepAt
	skip := map[int]bool{} // the failure lines, which may fall inside a step's output
	lastJobStep := ""
	failureEnd := -1 // the last line of the last failure
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch {
		case scenario.MatchString(l):
			a.scenarios = append(a.scenarios, l)
		case fuzzInput.MatchString(l):
			a.fuzz = append(a.fuzz, fuzzInput.FindStringSubmatch(l)[1])
		case strings.HasPrefix(l, "$ ") && (i == 0 || lines[i-1] == "" || i-1 == failureEnd):
			steps = append(steps, stepAt{command: l[2:], line: i})
		case failedAt.MatchString(l):
			f := failureAt(lines, i)
			for j := f.from; j <= f.to; j++ {
				skip[j] = true
			}
			a.failures = append(a.failures, f)
			i, failureEnd = f.to, f.to
		case jobStep.MatchString(l):
			lastJobStep = jobStep.FindStringSubmatch(l)[1]
		case a.stopped == "":
			for _, s := range stops {
				if strings.HasPrefix(l, s.prefix) {
					a.stopped = s.says
				}
			}
		}
	}
	for _, f := range a.failures {
		if mutationStep(f.command) {
			if s, ok := stepOf(steps, f.command, f.from); ok {
				f.sites = report(lines, skip, s, next(steps, s, len(lines)))
			} else {
				f.sites = &sites{unread: true}
			}
		}
	}
	if a.stopped == "" {
		return a
	}
	switch {
	case len(steps) > 0:
		s := steps[len(steps)-1]
		a.last, a.plan = s.command, true
		named := false
		for _, f := range a.failures {
			named = named || (f.command == s.command && f.from > s.line)
		}
		if mutationStep(s.command) && !named {
			a.lastSites = report(lines, skip, s, len(lines))
		}
	default:
		a.last = lastJobStep
	}
	return a
}

// failureAt is the failure whose first line is lines[i]. A step's failure ends with its exit code;
// a command of several lines carries it on its last, so a first line without one looks ahead for
// it, as far as the command could reach, past the blank lines the next step's stdout may put
// inside it. One whose exit code is not found is named by its first line alone.
func failureAt(lines []string, i int) *failure {
	m := failedAt.FindStringSubmatch(lines[i])
	f := &failure{text: strings.TrimPrefix(m[2], " "), from: i, to: i}
	rest, ok := strings.CutPrefix(m[2], ": ")
	if !ok {
		return f
	}
	if command, code, ok := exitOf(rest); ok {
		f.command, f.code = command, code
		return f
	}
	f.command, f.more = rest, true
	for j := i + 1; j < len(lines) && j <= i+50; j++ {
		if failedAt.MatchString(lines[j]) {
			break
		}
		if _, code, ok := exitOf(lines[j]); ok {
			f.code, f.to = code, j
			return f
		}
	}
	return f
}

// exitOf splits a failure line's end, "(it exited N)" or "(it gave no exit code)", from what is
// before it.
func exitOf(s string) (string, string, bool) {
	if m := exited.FindStringSubmatch(s); m != nil {
		return m[1], "exited " + m[2], true
	}
	if m := noCode.FindStringSubmatch(s); m != nil {
		return m[1], "gave no exit code", true
	}
	return "", "", false
}

// mutationStep is whether a step's command runs itos-cc's mutation check.
func mutationStep(command string) bool { return strings.Contains(command, "itos-cc mutation run") }

// stepOf is the last step started before line whose command's first line is command.
func stepOf(steps []stepAt, command string, line int) (stepAt, bool) {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].line < line && steps[i].command == command {
			return steps[i], true
		}
	}
	return stepAt{}, false
}

// next is the line the step after s starts at, or end.
func next(steps []stepAt, s stepAt, end int) int {
	for _, t := range steps {
		if t.line > s.line {
			return t.line
		}
	}
	return end
}

// report is the itos-cc report in the step's output, lines s.line to end: its first line "{" to
// the "}" that closes it, without the failure lines and GitHub's own ("##[…]") that fall inside.
func report(lines []string, skip map[int]bool, s stepAt, end int) *sites {
	start := -1
	var body []string
	for i := s.line + 1; i < end; i++ {
		if skip[i] || strings.HasPrefix(lines[i], "##[") {
			continue
		}
		if start < 0 && lines[i] == "{" {
			start = i
		}
		if start < 0 {
			continue
		}
		body = append(body, lines[i])
		if lines[i] == "}" {
			break
		}
	}
	var r struct {
		Problems []site `json:"problems"`
		Sampling struct {
			Stop string `json:"stop"`
		} `json:"sampling"`
	}
	if start < 0 || json.Unmarshal([]byte(strings.Join(body, "\n")), &r) != nil {
		return &sites{unread: true}
	}
	out := &sites{}
	for _, p := range r.Problems {
		switch p.Rule {
		case "mutation.survived", "mutation.uncovered", "mutation.uncovered-statement":
			out.found = append(out.found, p)
		case "interrupted":
			out.interrupted, out.stop = true, r.Sampling.Stop
		}
	}
	return out
}

// render is the job's part of the issue body, in Markdown.
func (a analysis) render() string {
	var b strings.Builder
	if len(a.failures) > 0 {
		b.WriteString("Failed at:\n\n")
		for _, f := range a.failures {
			switch {
			case f.command == "":
				fmt.Fprintf(&b, "- %s\n", f.text)
				continue
			case f.code == "":
				fmt.Fprintf(&b, "- %s: its exit code is not in the log.\n", code(f.command, f.more))
			default:
				fmt.Fprintf(&b, "- %s: it %s.\n", code(f.command, f.more), f.code)
			}
			f.sites.render(&b, "  ")
		}
		b.WriteString("\n")
	}
	if a.stopped != "" {
		where := "the job's step"
		if a.plan {
			where = "the plan's step"
		}
		if a.last == "" {
			fmt.Fprintf(&b, "%s before any step started.\n\n", a.stopped)
		} else {
			fmt.Fprintf(&b, "%s during %s %s: the steps after it did not run.\n", a.stopped, where, code(a.last, false))
			if a.lastSites != nil {
				b.WriteString("\nWhat it found before it stopped:\n\n")
				a.lastSites.render(&b, "")
			}
			b.WriteString("\n")
		}
	}
	if len(a.fuzz) > 0 {
		b.WriteString("Failing fuzz inputs, their bytes in the log; keep each as a seed at its path:\n\n")
		for _, path := range a.fuzz {
			fmt.Fprintf(&b, "- %s\n", code(path, false))
		}
		b.WriteString("\n")
	}
	switch {
	case len(a.scenarios) > 0:
		b.WriteString("Failing scenarios:\n\n```\n" + strings.Join(a.scenarios, "\n") + "\n```\n")
	case len(a.failures) > 0 || a.stopped != "":
		b.WriteString("No failing scenario in its log.\n")
	default:
		b.WriteString("No failing scenario in its log: it failed before the features ran, or outside them. See its log.\n")
	}
	return b.String()
}

// render lists a mutation step's sites, each line after indent, nothing for a step that is not one.
func (s *sites) render(b *strings.Builder, indent string) {
	switch {
	case s == nil:
		return
	case s.unread:
		b.WriteString(indent + "- Its itos-cc report could not be read from the log.\n")
		return
	}
	for _, p := range s.found {
		rule := strings.TrimPrefix(p.Rule, "mutation.")
		if p.Original == "" {
			// An unexecuted block: no mutant, so no column or change.
			fmt.Fprintf(b, "%s- %s %s:%d in %s\n", indent, rule, p.File, p.Line, code(short(p.Function), false))
			continue
		}
		replacement := code(p.Replacement, false)
		if p.Replacement == "" {
			replacement = "(deleted)"
		}
		fmt.Fprintf(b, "%s- %s %s:%d:%d %s → %s in %s\n", indent, rule,
			p.File, p.Line, p.Column, code(p.Original, false), replacement, code(short(p.Function), false))
	}
	switch {
	case s.interrupted && s.stop != "":
		fmt.Fprintf(b, "%s- interrupted: %s\n", indent, s.stop)
	case s.interrupted:
		b.WriteString(indent + "- interrupted\n")
	}
	if len(s.found) == 0 && !s.interrupted {
		b.WriteString(indent + "- Its itos-cc report names no survived or uncovered site.\n")
	}
}

// short is a function's name without its package's import path before the last slash:
// internal/cli's function, as the file beside it already says where it is.
func short(function string) string {
	return function[strings.LastIndex(function, "/")+1:]
}

// code is s as Markdown code, with " …" after it when more of it follows.
func code(s string, more bool) string {
	fence := "`"
	for strings.Contains(s, fence) {
		fence += "`"
	}
	pad := ""
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		pad = " "
	}
	out := fence + pad + s + pad + fence
	if more {
		out += " …"
	}
	return out
}

func execRun(name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	var stderr strings.Builder
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return string(out), fmt.Errorf("%s: %w: %s", name, err, msg)
		}
		return string(out), fmt.Errorf("%s: %w", name, err)
	}
	return string(out), nil
}
