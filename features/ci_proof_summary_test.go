// The steps of the optional proof summary (ci-proof-summary.feature, slice
// 164): the scratch root's files written as the scenario says, a provider
// that is a shell script printing the file proof.stdout and exiting with the
// number in proof.exit, tags on the scratch repository's commits, and the
// reading of ci run's logs and --json report. A scenario's state is its
// files: the provider and the recording script append to proof-runs and
// ci-runs, so what ran is what the files say, on every platform that has the
// sh the scratch config names. A report is read as JSON, never by its text,
// except for the lines of the logs, which are compared whole.
package features

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// What the steps keep of the scenario: whether itos is given the config by
// its flag.
type proofSummaryFixture struct {
	configFlag bool
}

func initializeProofSummarySteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the scratch file "([^"]*)" contains exactly:$`, func(path string, text *godog.DocString) error {
		return w.write(path, text.Content)
	})
	sc.Step(`^the scratch file "([^"]*)" contains "([^"]*)"$`, w.write)
	sc.Step(`^the scratch files "([^"]*)" and "([^"]*)" each contain "([^"]*)"$`, func(a, b, text string) error {
		return errors.Join(w.write(a, text), w.write(b, text))
	})
	sc.Step(`^the scratch root's "([^"]*)" is replaced with:$`, func(path string, text *godog.DocString) error {
		return w.write(path, text.Content)
	})
	sc.Step(`^the scratch fixture files are committed with message "([^"]*)" and tagged "([^"]*)"$`, func(message, tag string) error {
		if err := w.commit(message); err != nil {
			return err
		}
		return w.git("tag", tag)
	})
	sc.Step(`^only "([^"]*)" is committed with message "([^"]*)" and tagged "([^"]*)"$`, func(path, message, tag string) error {
		if err := w.git("add", "--", path); err != nil {
			return err
		}
		if err := w.git("commit", "-q", "--no-verify", "-m", message, "--", path); err != nil {
			return err
		}
		return w.git("tag", tag)
	})
	sc.Step(`^the commit "([^"]*)" touching only "([^"]*)" on top of it is tagged "([^"]*)"$`, func(message, path, tag string) error {
		if err := w.commitTouchingOnly(message, path); err != nil {
			return err
		}
		return w.git("tag", tag)
	})
	sc.Step(`^every itos invocation uses the binary under test from the scratch root with "--config itos\.yaml", no pin and no inherited ITOS_CONFIG$`, func() error {
		if w.config.pin != nil {
			return errors.New("the scratch config is pinned")
		}
		w.proofSummary = &proofSummaryFixture{configFlag: true}
		return nil
	})
	sc.Step(`^I invoke itos with "([^"]*)"$`, func(command string) error {
		args := strings.Fields(command)
		if w.proofSummary != nil && w.proofSummary.configFlag {
			args = append([]string{"--config", "itos.yaml"}, args...)
		}
		return w.itos(args...)
	})

	sc.Step(`^"([^"]*)" has exactly (\d+) lines?$`, func(path string, n int) error {
		lines, err := w.recordedLines(path)
		if err != nil {
			return err
		}
		if len(lines) != n {
			return fmt.Errorf("%s has %d lines, not %d: %q\n%s", path, len(lines), n, lines, w.report())
		}
		return nil
	})
	sc.Step(`^"([^"]*)" contains only "([^"]*)"$`, func(path, line string) error {
		lines, err := w.recordedLines(path)
		if err != nil {
			return err
		}
		if !slices.Equal(lines, []string{line}) {
			return fmt.Errorf("%s holds %q, not only %q\n%s", path, lines, line, w.report())
		}
		return nil
	})
	sc.Step(`^"([^"]*)" contains exactly one line with "--since " followed by the full SHA of "([^"]*)" and " --json"$`, func(path, tag string) error {
		return w.providerLineIs(path, tag)
	})
	sc.Step(`^"([^"]*)" still has exactly the executed control's single line$`, func(path string) error {
		return w.providerLineIs(path, "summary-base")
	})
	sc.Step(`^the recorded file "([^"]*)" equals exactly:$`, func(path string, text *godog.DocString) error {
		got, err := w.recordedText(path)
		if err != nil {
			return err
		}
		if strings.TrimSuffix(got, "\n") != text.Content {
			return fmt.Errorf("%s holds %q, not %q", path, got, text.Content)
		}
		return nil
	})
	sc.Step(`^the latest "([^"]*)" entry is exactly "([^"]*)"$`, func(path, line string) error {
		lines, err := w.recordedLines(path)
		if err != nil {
			return err
		}
		if len(lines) == 0 || lines[len(lines)-1] != line {
			return fmt.Errorf("the latest entry of %s is not %q: %q", path, line, lines)
		}
		return nil
	})

	sc.Step(`^(stdout|stderr) contains the complete line "([^"]*)" exactly once$`, func(stream, line string) error {
		return w.lineOnce(stream, line)
	})
	sc.Step(`^that summary line follows the proof command and precedes "CI passed"$`, w.summaryBetweenCommandAndPassed)
	sc.Step(`^the summary line contains neither "([^"]*)" nor "([^"]*)" nor "([^"]*)" nor "([^"]*)"$`, func(a, b, c, d string) error {
		line, err := w.summaryLine("stdout")
		if err != nil {
			return err
		}
		for _, text := range []string{a, b, c, d} {
			if strings.Contains(line, text) {
				return fmt.Errorf("the summary line %q contains %q", line, text)
			}
		}
		return nil
	})
	sc.Step(`^neither stream contains "([^"]*)"(?: or "([^"]*)")?(?: or "([^"]*)")?$`, func(a, b, c string) error {
		for _, text := range []string{a, b, c} {
			if text != "" && (strings.Contains(w.stdout, text) || strings.Contains(w.stderr, text)) {
				return fmt.Errorf("a stream contains %q\n%s", text, w.report())
			}
		}
		return nil
	})
	sc.Step(`^stdout contains neither "([^"]*)" nor "([^"]*)"$`, func(a, b string) error {
		for _, text := range []string{a, b} {
			if strings.Contains(w.stdout, text) {
				return fmt.Errorf("stdout contains %q\n%s", text, w.report())
			}
		}
		return nil
	})
	sc.Step(`^stderr contains the complete line "CI failed at the code proof: " followed by the resolved proof command,( after that summary line and)? before the line naming rule "([^"]*)"$`,
		w.failureLineBeforeRule)
	sc.Step(`^stderr names rule "([^"]*)" and the field "([^"]*)" as a whole token, not followed by "\."$`, func(rule, field string) error {
		for _, line := range logLines(w.stderr) {
			if strings.HasPrefix(line, rule+": ") && hasToken(line, field) {
				return nil
			}
		}
		return fmt.Errorf("no stderr line names %s and the field %s\n%s", rule, field, w.report())
	})

	sc.Step(`^stdout is one JSON report with schema 1 and ok (true|false)( and no proof member| and no failed_at or failures members)?$`, w.reportIs)
	sc.Step(`^stdout has no top-level proof member$`, func() error { return w.reportLacks("proof") })
	sc.Step(`^the JSON proof has outcome "([^"]*)" and exactly this normalized summary:$`, func(outcome string, summary *godog.DocString) error {
		p, err := w.proofMember()
		if err != nil {
			return err
		}
		if err := jsonStringIs(p["outcome"], "outcome", outcome); err != nil {
			return err
		}
		return jsonEquals(p["summary"], summary.Content)
	})
	sc.Step(`^the JSON proof has outcome "([^"]*)" and summary null$`, func(outcome string) error {
		p, err := w.proofMember()
		if err != nil {
			return err
		}
		if err := jsonStringIs(p["outcome"], "outcome", outcome); err != nil {
			return err
		}
		return jsonEquals(p["summary"], "null")
	})
	sc.Step(`^the JSON proof names the resolved proof command and full SHAs of "([^"]*)" and "([^"]*)"$`, func(from, to string) error {
		command, err := w.resolvedCommand()
		if err != nil {
			return err
		}
		return w.proofNames(command, from, to)
	})
	sc.Step(`^the JSON proof names command "([^"]*)", from "([^"]*)" and to "([^"]*)" using the refs' full SHAs$`, func(command, from, to string) error {
		for _, tag := range []string{"summary-base", "summary-head"} {
			sha, err := w.tagSHA(tag)
			if err != nil {
				return err
			}
			for _, text := range []*string{&command, &from, &to} {
				*text = strings.ReplaceAll(*text, "{"+tag+"-sha}", sha)
			}
		}
		return w.proofNames(command, from, to)
	})
	sc.Step(`^the JSON proof has exactly keys command, from, to, outcome and summary$`, func() error {
		p, err := w.proofMember()
		if err != nil {
			return err
		}
		keys := make([]string, 0, len(p))
		for key := range p {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		if want := []string{"command", "from", "outcome", "summary", "to"}; !slices.Equal(keys, want) {
			return fmt.Errorf("the JSON proof has the keys %q, not %q", keys, want)
		}
		return nil
	})
	sc.Step(`^the JSON plan has the resolved proof command once in order with proof true$`, w.planHasProofOnce)

	sc.Step(`^failed_at equals the single failures entry with code 1 and the resolved proof command as step$`, func() error {
		f, err := w.soleFailure()
		if err != nil {
			return err
		}
		command, err := w.resolvedCommand()
		if err != nil {
			return err
		}
		if f.Code != 1 || f.Step == nil || *f.Step != command {
			return fmt.Errorf("the failure is %+v, not code 1 at %q", f, command)
		}
		return nil
	})
	sc.Step(`^failed_at names step "([^"]*)" with code (\d+)$`, func(step string, code int) error {
		r, err := w.report1()
		if err != nil {
			return err
		}
		var f proofFailure
		if err := json.Unmarshal(r["failed_at"], &f); err != nil {
			return fmt.Errorf("failed_at: %v\n%s", err, w.report())
		}
		if f.Step == nil || *f.Step != step || f.Code != code {
			return fmt.Errorf("failed_at is %+v, not code %d at %q", f, code, step)
		}
		return nil
	})
	sc.Step(`^that failure has exactly the problem rules "([^"]*)" in order$`, func(rules string) error {
		f, err := w.soleFailure()
		if err != nil {
			return err
		}
		var got []string
		for _, p := range f.Problems {
			got = append(got, p.Rule)
		}
		if want := strings.Split(rules, ","); !slices.Equal(got, want) {
			return fmt.Errorf("the failure's problem rules are %q, not %q\n%s", got, want, w.report())
		}
		return nil
	})
	sc.Step(`^that rule's fix names "([^"]*)", "([^"]*)" and "([^"]*)"$`, func(a, b, c string) error {
		f, err := w.soleFailure()
		if err != nil {
			return err
		}
		if len(f.Problems) != 1 {
			return fmt.Errorf("the failure has %d problems, not one", len(f.Problems))
		}
		for _, text := range []string{a, b, c} {
			if !strings.Contains(f.Problems[0].Fix, text) {
				return fmt.Errorf("the fix %q does not name %q", f.Problems[0].Fix, text)
			}
		}
		return nil
	})
	sc.Step(`^its "([^"]*)" problem message names the field "([^"]*)" as a whole token, not followed by "\."$`, func(rule, field string) error {
		f, err := w.soleFailure()
		if err != nil {
			return err
		}
		for _, p := range f.Problems {
			if p.Rule == rule && hasToken(p.Message, field) {
				return nil
			}
		}
		return fmt.Errorf("no %s problem message names the field %s: %+v", rule, field, f.Problems)
	})
	sc.Step(`^its "([^"]*)" problem keeps message "([^"]*)" and fix "([^"]*)"$`, func(rule, message, fix string) error {
		f, err := w.soleFailure()
		if err != nil {
			return err
		}
		for _, p := range f.Problems {
			if p.Rule == rule && p.Message == message && p.Fix == fix {
				return nil
			}
		}
		return fmt.Errorf("no %s problem keeps that message and fix: %+v", rule, f.Problems)
	})
}

// A failure of ci run's JSON, as far as these steps read it.
type proofFailure struct {
	Step     *string `json:"step"`
	Code     int     `json:"code"`
	Problems []struct {
		Rule    string `json:"rule"`
		Message string `json:"message"`
		Fix     string `json:"fix"`
	} `json:"problems"`
}

// The text of a file of the scratch root, a missing file an error.
func (w *world) recordedText(path string) (string, error) {
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return "", err
	}
	return string(text), nil
}

// The lines of a file of the scratch root; none for a file that was never
// written, since the scripts create theirs by appending.
func (w *world) recordedLines(path string) ([]string, error) {
	text, err := w.recordedText(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return logLines(strings.TrimSuffix(text, "\n")), nil
}

// A stream's lines, a carriage return before a line break dropped.
func logLines(text string) []string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// The full SHA of the commit a tag names.
func (w *world) tagSHA(tag string) (string, error) {
	sha, err := w.gitOutput("rev-parse", tag+"^{commit}")
	return strings.TrimSpace(sha), err
}

// The proof command ci run resolves: the config's check with {base} the
// range's start, the tag summary-base.
func (w *world) resolvedCommand() (string, error) {
	base, err := w.tagSHA("summary-base")
	return "sh proof-provider.sh --since " + base + " --json", err
}

// The provider's one recorded run, over the commit the tag names.
func (w *world) providerLineIs(path, tag string) error {
	sha, err := w.tagSHA(tag)
	if err != nil {
		return err
	}
	lines, err := w.recordedLines(path)
	if err != nil {
		return err
	}
	if want := []string{"--since " + sha + " --json"}; !slices.Equal(lines, want) {
		return fmt.Errorf("%s holds %q, not %q\n%s", path, lines, want, w.report())
	}
	return nil
}

func (w *world) streamLines(stream string) []string {
	if stream == "stdout" {
		return logLines(w.stdout)
	}
	return logLines(w.stderr)
}

// The stream has the line, whole, once.
func (w *world) lineOnce(stream, line string) error {
	n := 0
	for _, l := range w.streamLines(stream) {
		if l == line {
			n++
		}
	}
	if n != 1 {
		return fmt.Errorf("%s has the line %q %d times, not once\n%s", stream, line, n, w.report())
	}
	return nil
}

// The index of the first line of the stream that the test accepts, -1 for
// none.
func indexOfLine(lines []string, accept func(string) bool) int {
	return slices.IndexFunc(lines, accept)
}

// The stream's first line of the code proof's summary, available or not.
func (w *world) summaryLine(stream string) (string, error) {
	lines := w.streamLines(stream)
	i := indexOfLine(lines, func(l string) bool { return strings.HasPrefix(l, "Code proof summary") })
	if i < 0 {
		return "", fmt.Errorf("%s has no summary line\n%s", stream, w.report())
	}
	return lines[i], nil
}

func (w *world) summaryBetweenCommandAndPassed() error {
	command, err := w.resolvedCommand()
	if err != nil {
		return err
	}
	lines := w.streamLines("stdout")
	at := indexOfLine(lines, func(l string) bool { return l == "$ "+command })
	summary := indexOfLine(lines, func(l string) bool { return strings.HasPrefix(l, "Code proof summary") })
	passed := indexOfLine(lines, func(l string) bool { return l == "CI passed" })
	if at < 0 || summary <= at || passed <= summary {
		return fmt.Errorf("the lines of the proof command (%d), its summary (%d) and CI passed (%d) are not in that order\n%s",
			at, summary, passed, w.report())
	}
	return nil
}

// ci run's failure line comes before the line naming the rule and, when
// asked, after the summary line, on stderr.
func (w *world) failureLineBeforeRule(afterSummary, rule string) error {
	command, err := w.resolvedCommand()
	if err != nil {
		return err
	}
	lines := w.streamLines("stderr")
	failed := indexOfLine(lines, func(l string) bool { return l == "CI failed at the code proof: "+command })
	named := indexOfLine(lines, func(l string) bool { return strings.HasPrefix(l, rule+": ") })
	if failed < 0 || named <= failed {
		return fmt.Errorf("the failure line (%d) is not before the line naming %s (%d)\n%s", failed, rule, named, w.report())
	}
	if afterSummary != "" {
		summary := indexOfLine(lines, func(l string) bool { return strings.HasPrefix(l, "Code proof summary") })
		if summary < 0 || summary >= failed {
			return fmt.Errorf("the summary line (%d) is not before the failure line (%d)\n%s", summary, failed, w.report())
		}
	}
	return nil
}

// Whether the text names the field as a whole token: no token character
// before it, and neither a token character nor a full stop after it.
func hasToken(text, field string) bool {
	return regexp.MustCompile(`(?:^|[^A-Za-z0-9_.])` + regexp.QuoteMeta(field) + `(?:[^A-Za-z0-9_.]|$)`).MatchString(text)
}

// Standard output as one JSON object and nothing else.
func (w *world) report1() (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(strings.NewReader(w.stdout))
	var r map[string]json.RawMessage
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("stdout is not a JSON object: %v\n%s", err, w.report())
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("stdout holds more than one JSON value\n%s", w.report())
	}
	return r, nil
}

func (w *world) reportIs(ok, lacks string) error {
	r, err := w.report1()
	if err != nil {
		return err
	}
	if err := jsonEquals(r["schema"], "1"); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	if err := jsonEquals(r["ok"], ok); err != nil {
		return fmt.Errorf("ok: %w", err)
	}
	switch lacks {
	case " and no proof member":
		return w.reportLacks("proof")
	case " and no failed_at or failures members":
		return errors.Join(w.reportLacks("failed_at"), w.reportLacks("failures"))
	}
	return nil
}

func (w *world) reportLacks(key string) error {
	r, err := w.report1()
	if err != nil {
		return err
	}
	if _, has := r[key]; has {
		return fmt.Errorf("the report has the member %s\n%s", key, w.report())
	}
	return nil
}

// The report's proof member as an object.
func (w *world) proofMember() (map[string]json.RawMessage, error) {
	r, err := w.report1()
	if err != nil {
		return nil, err
	}
	var p map[string]json.RawMessage
	if err := json.Unmarshal(r["proof"], &p); err != nil || p == nil {
		return nil, fmt.Errorf("the report has no proof object: %v\n%s", err, w.report())
	}
	return p, nil
}

func (w *world) proofNames(command, from, to string) error {
	p, err := w.proofMember()
	if err != nil {
		return err
	}
	want := map[string]string{"command": command}
	for key, ref := range map[string]string{"from": from, "to": to} {
		if want[key], err = w.tagSHA(ref); err != nil {
			return err
		}
	}
	for key, text := range want {
		if err := jsonStringIs(p[key], key, text); err != nil {
			return err
		}
	}
	return nil
}

func jsonStringIs(raw json.RawMessage, key, want string) error {
	var got string
	if err := json.Unmarshal(raw, &got); err != nil || got != want {
		return fmt.Errorf("the JSON proof's %s is %s, not %q", key, raw, want)
	}
	return nil
}

// The raw JSON is the text, spaces aside: member order, nulls and number
// tokens as written.
func jsonEquals(raw json.RawMessage, text string) error {
	var got, want bytes.Buffer
	if err := json.Compact(&got, raw); err != nil {
		return fmt.Errorf("not JSON: %q", raw)
	}
	if err := json.Compact(&want, []byte(text)); err != nil {
		return err
	}
	if got.String() != want.String() {
		return fmt.Errorf("the JSON is %s, not %s", got.String(), want.String())
	}
	return nil
}

// The report's one failure, which is also failed_at.
func (w *world) soleFailure() (proofFailure, error) {
	var f proofFailure
	r, err := w.report1()
	if err != nil {
		return f, err
	}
	var failures []json.RawMessage
	if err := json.Unmarshal(r["failures"], &failures); err != nil || len(failures) != 1 {
		return f, fmt.Errorf("the report does not have exactly one failure: %s\n%s", r["failures"], w.report())
	}
	if err := jsonEquals(r["failed_at"], string(failures[0])); err != nil {
		return f, fmt.Errorf("failed_at is not the failures entry: %w", err)
	}
	return f, json.Unmarshal(failures[0], &f)
}

// The plan lists the proof command once, flagged, after ci.steps' first step
// and before the run of named tests.
func (w *world) planHasProofOnce() error {
	command, err := w.resolvedCommand()
	if err != nil {
		return err
	}
	r, err := w.report1()
	if err != nil {
		return err
	}
	var order []struct {
		Step  *string `json:"step"`
		Proof bool    `json:"proof"`
	}
	if err := json.Unmarshal(r["order"], &order); err != nil {
		return fmt.Errorf("order: %v", err)
	}
	proof, before, tests := -1, -1, -1
	for i, item := range order {
		switch {
		case item.Step == nil:
		case *item.Step == command:
			if proof >= 0 || !item.Proof {
				return fmt.Errorf("the proof command is listed twice or without proof true: %s", r["order"])
			}
			proof = i
		case *item.Step == "sh record-ci.sh before":
			before = i
		case strings.HasPrefix(*item.Step, "sh record-ci.sh tests "):
			tests = i
		}
	}
	if before < 0 || proof <= before || tests <= proof {
		return fmt.Errorf("the steps before (%d), the proof (%d) and the tests (%d) are not in that order: %s", before, proof, tests, r["order"])
	}
	return nil
}
