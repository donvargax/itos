package proof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/glob"
	"github.com/donvargax/itos/v8/internal/shell"
)

// Outcome is what a check's answer comes to.
type Outcome int

const (
	// Passed is a check that exited 0.
	Passed Outcome = iota
	// Refused is a check that exited 1 with its problems in its --json.
	Refused
	// CannotRun is every other answer: never a pass.
	CannotRun
)

// Problem is one problem of the check's --json: its stable rule, and the
// sentences it gives, which are for people.
//
// File and Line are the subject keys of the same names (itos-cc's file and
// line), when the problem has both, a string and a positive whole number;
// otherwise both are zero.
type Problem struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Fix     string `json:"fix"`
	File    string `json:"-"`
	Line    int    `json:"-"`
}

// UnmarshalJSON reads the problem as the struct's tags say, and its file and
// line when both are as the contract has them: a subject key of another
// shape leaves the problem without a place, never unreadable.
func (p *Problem) UnmarshalJSON(data []byte) error {
	var a struct {
		Rule    string          `json:"rule"`
		Message string          `json:"message"`
		Fix     string          `json:"fix"`
		File    json.RawMessage `json:"file"`
		Line    json.RawMessage `json:"line"`
	}
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*p = Problem{Rule: a.Rule, Message: a.Message, Fix: a.Fix}
	var file string
	var line int
	if json.Unmarshal(a.File, &file) == nil && json.Unmarshal(a.Line, &line) == nil && file != "" && line > 0 {
		p.File, p.Line = file, line
	}
	return nil
}

// String is the outcome as ci run --json gives it.
func (o Outcome) String() string {
	return [...]string{"passed", "refused", "cannot-run"}[o]
}

// Verdict is how a check answered: its outcome, the problems its --json
// listed (with Refused, and with CannotRun when it printed them), and with
// CannotRun why, as the end of a sentence ("exited 3"). A check that exited
// 0 gives the Summary its itos_proof envelope normalized, nil when it opted
// in to none; one whose envelope breaks the contract cannot run, Invalid
// naming the field; one whose envelope says it judged nothing is Refused,
// with that Summary and no problems (NoWork).
type Verdict struct {
	Outcome  Outcome
	Problems []Problem
	Why      string
	Summary  *Summary
	Invalid  *FieldError
}

// NoWork is whether the check exited 0 saying it judged no work at all.
func (v Verdict) NoWork() bool {
	return v.Outcome == Refused && v.Summary != nil
}

// Touches is whether one of the files matches one of the globs.
func Touches(globs, files []string) (bool, error) {
	for _, f := range files {
		ok, err := glob.MatchesAny(f, globs)
		if ok || err != nil {
			return ok, err
		}
	}
	return false, nil
}

// Command is the check with {base}, every one of it, standing for the base.
func Command(check, base string) string {
	return strings.ReplaceAll(check, "{base}", base)
}

// Run runs the command through the config's shell, in the folder itos runs
// in, its stderr to stderr, and reads its answer.
func Run(cfg *config.Loaded, command string, stderr io.Writer) Verdict {
	var stdout bytes.Buffer
	r := shell.Run(cfg, command, shell.Options{Stdout: &stdout, Stderr: stderr})
	switch {
	case r.Err != nil:
		return Verdict{Outcome: CannotRun, Why: "did not start: " + r.Err.Error()}
	case r.Code < 0:
		return Verdict{Outcome: CannotRun, Why: "was stopped by " + r.Status()}
	}
	return Read(r.Code, stdout.Bytes())
}

// answer is the --json object of itos-cc's contract, as far as itos reads it.
type answer struct {
	Schema   *int      `json:"schema"`
	OK       *bool     `json:"ok"`
	Problems []Problem `json:"problems"`
}

// Read is the verdict of a check that exited with the code and printed
// stdout.
func Read(code int, stdout []byte) Verdict {
	if code == 0 {
		return readPassed(stdout)
	}
	a, readable := parse(stdout)
	if code == 1 && readable && !*a.OK {
		return Verdict{Outcome: Refused, Problems: a.Problems}
	}
	v := Verdict{Outcome: CannotRun, Why: fmt.Sprintf("exited %d", code)}
	if code == 1 {
		v.Why = "exited 1 without the --json object that names its problems"
	}
	if readable {
		v.Problems = a.Problems
	}
	return v
}

// readPassed is the verdict of a check that exited 0: it passes whatever it
// prints, unless it opted in to the summary envelope (ReadSummary) and
// broke its contract, or reported that it judged nothing.
func readPassed(stdout []byte) Verdict {
	summary, invalid := ReadSummary(stdout)
	switch {
	case invalid != nil:
		return Verdict{Outcome: CannotRun, Why: invalid.Message(), Invalid: invalid}
	case summary != nil && summary.Judged != nil && *summary.Judged == 0:
		return Verdict{Outcome: Refused, Summary: summary}
	}
	return Verdict{Outcome: Passed, Summary: summary}
}

// parse is the object stdout holds, and whether it is one of the contract's:
// schema 1, ok given.
func parse(stdout []byte) (answer, bool) {
	var a answer
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &a); err != nil {
		return answer{}, false
	}
	return a, a.Schema != nil && *a.Schema == 1 && a.OK != nil
}
