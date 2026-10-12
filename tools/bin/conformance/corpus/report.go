package corpus

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Key is how a case is named outside its file: its file's name and its own, as a Changes: footer
// and the parity record name it.
func (r Result) Key() string { return r.Fixture.Base() + ": " + r.Case.Name }

// Failures writes each failed case, its command and what differs, as the runner prints them.
func Failures(w io.Writer, results []Result) {
	for _, r := range results {
		if len(r.Problems) == 0 {
			continue
		}
		fmt.Fprintf(w, "\nFAIL %s: %s\n", r.Fixture.Shown, r.Case.Name)
		fmt.Fprintf(w, "  $ itos %s\n", r.Case.shownArgv())
		for _, line := range r.Problems {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
}

// Summary is the line that says how many cases pass, of each file's, and how long they took.
func Summary(fixtures []*Fixture, results []Result, took time.Duration) string {
	failed := 0
	for _, r := range results {
		if len(r.Problems) > 0 {
			failed++
		}
	}
	counts := make([]string, len(fixtures))
	for i, f := range fixtures {
		counts[i] = fmt.Sprintf("%s %d", strings.TrimSuffix(f.Base(), ".yaml"), len(f.Cases))
	}
	return fmt.Sprintf("%d/%d conformance cases pass (%s) in %.1f s",
		len(results)-failed, len(results), strings.Join(counts, ", "), took.Seconds())
}

// Verdict is a case's verdict as --json writes it, a line each.
type Verdict struct {
	Name     string   `json:"name"`     // the case's Key
	File     string   `json:"file"`     // its fixture, as given
	Case     string   `json:"case"`     // its name in the fixture
	Verdict  string   `json:"verdict"`  // pass or fail
	Problems []string `json:"problems"` // what differs, for a case that fails
}

// Verdicts writes each case's verdict as a JSON line, in the fixtures' order.
func Verdicts(w io.Writer, results []Result) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, r := range results {
		v := Verdict{Name: r.Key(), File: r.Fixture.Path, Case: r.Case.Name, Verdict: "pass", Problems: r.Problems}
		if len(r.Problems) > 0 {
			v.Verdict = "fail"
		}
		if err := enc.Encode(v); err != nil {
			return err
		}
	}
	return nil
}
