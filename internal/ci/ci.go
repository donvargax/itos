package ci

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/plan"
	"github.com/donvargax/itos/v8/internal/proof"
	"github.com/donvargax/itos/v8/internal/shell"
)

// Failure is where a run failed, as --json's failed_at gives the first and
// failures each in run order: a step (Code its own exit code), a task check,
// the code proof (Problems, what its check's --json named), or the unknown
// tasks named.
type Failure struct {
	Step     *string       `json:"step,omitempty"`
	Task     string        `json:"task,omitempty"`
	Title    string        `json:"title,omitempty"`
	Command  string        `json:"command,omitempty"`
	Problems []out.Problem `json:"problems,omitempty"`
	Unknown  []string      `json:"unknown,omitempty"`
	Code     int           `json:"code"`
}

// Options are how a run reports: under JSON its log goes to Stderr and
// Stdout holds the verdict. Nightly is the nightly's run, which goes on past
// a failing step whatever ci.stop_at_first_failure says (slice 133).
// Evidence looks up the task checks and the steps that opt in to evidence
// records by the index tree (internal/engine/evidence), applying ci.env to the key
// as the run applies it to the commands; nil runs every one as it always
// did.
type Options struct {
	JSON           bool
	Nightly        bool
	Stdout, Stderr io.Writer
	Evidence       *evidence.Engine
	// Annotate prints one GitHub Actions error command on Stderr for each
	// failure the run reports, up to 10 (slice 198).
	Annotate bool
}

// driver is one run: the config, its streams, its check runner and the
// evidence engine the steps that opt in go through.
type driver struct {
	cfg      *config.Loaded
	log      io.Writer
	stderr   io.Writer
	runner   *check.Runner
	evidence *evidence.Engine
	ends     plan.Ends
	proofRun *ProofRun
	// ann prints the failures as workflow commands: nil unless the run
	// annotates.
	ann *annotator
}

// ProofRun is the code proof of a run that was executed, as --json's proof
// gives it: the command run, the range's resolved ends, how the provider
// answered (passed, refused or cannot-run) and the summary it reported, null
// when it reported none or was not read for one.
type ProofRun struct {
	Command string         `json:"command"`
	From    string         `json:"from"`
	To      string         `json:"to"`
	Outcome string         `json:"outcome"`
	Summary *proof.Summary `json:"summary"`
}

// Run carries out a plan and gives the run's exit code: 0, or 1 for a
// failing step, task check or the unknown tasks.
func Run(cfg *config.Loaded, p *plan.Plan, o Options) (int, error) {
	for key, value := range cfg.CI.Env {
		if err := os.Setenv(key, value); err != nil {
			return 0, err
		}
	}
	for key, value := range map[string]string{"ITOS_FROM": p.Ends.From, "ITOS_TO": p.Ends.To} {
		if err := os.Setenv(key, value); err != nil {
			return 0, err
		}
	}
	d := &driver{cfg: cfg, log: o.Stdout, stderr: o.Stderr, evidence: o.Evidence, ends: p.Ends}
	if o.Annotate {
		d.ann = newAnnotator(cfg, o.Stderr)
	}
	if o.JSON {
		d.log = o.Stderr
	}
	d.runner = check.NewRunner(cfg, o.Stdout, o.Stderr)
	d.runner.Verbose, d.runner.ToStderr = true, o.JSON
	d.runner.Runs = nil
	d.runner.Evidence = o.Evidence
	failed, err := d.preamble(p)
	if err != nil {
		return 0, err
	}
	var failures []*Failure
	if failed != nil {
		failures = append(failures, failed)
	}
	stop := cfg.CI.StopAtFirstFailure && !o.Nightly
	for _, item := range p.Order {
		if failed != nil && (stop || failed.Unknown != nil) {
			break
		}
		lead := "CI failed at"
		if failed != nil {
			lead = "also failed at"
		}
		var failure *Failure
		switch {
		case item.Step == nil:
			failure = d.check(item.Check, lead)
		case item.Step.Proof:
			failure = d.proof(item.Step, lead)
		default:
			failure = d.step(item.Step, lead)
		}
		if failure == nil {
			continue
		}
		failures = append(failures, failure)
		if failed == nil {
			failed = failure
		}
	}
	if failed == nil {
		fmt.Fprintln(d.log, "\nCI passed")
	}
	if o.JSON {
		fields := []out.Field{{Key: "ok", Value: failed == nil}}
		if failed != nil {
			fields = append(fields, out.Field{Key: "failed_at", Value: failed}, out.Field{Key: "failures", Value: failures})
		}
		if d.proofRun != nil {
			fields = append(fields, out.Field{Key: "proof", Value: d.proofRun})
		}
		if d.evidence != nil && d.evidence.Used() {
			fields = append(fields, out.Field{Key: "evidence", Value: d.evidence.Entries()})
			if unstaged, counted := d.evidence.Unstaged(); counted {
				fields = append(fields, out.Field{Key: "unstaged", Value: unstaged})
			}
		}
		if err := out.Emit(o.Stdout, fields...); err != nil {
			return 0, err
		}
	}
	if failed == nil {
		return 0, nil
	}
	return 1, nil
}

// preamble is what the run says before its first item: the unknown tasks,
// named against the ledger's folder, which end it; the tasks not started;
// and what a prose-only range runs and leaves out.
func (d *driver) preamble(p *plan.Plan) (*Failure, error) {
	if len(p.Unknown) > 0 {
		layout, err := ledger.LayoutOf(d.cfg)
		if err != nil {
			return nil, err
		}
		for _, id := range p.Unknown {
			fmt.Fprintf(d.stderr, "No task %s in %s/, though a footer names it\n", id, layout.Dir)
		}
		fmt.Fprintf(d.stderr, "\nCI failed at the tasks named: %s\n", strings.Join(p.Unknown, ", "))
		if d.ann != nil {
			d.ann.emit("", 0, fmt.Sprintf("the tasks named failed: no task %s in %s/", strings.Join(p.Unknown, ", "), layout.Dir))
		}
		return &Failure{Unknown: p.Unknown, Code: 1}, nil
	}
	for _, id := range p.NotStarted {
		fmt.Fprintf(d.log, "%s is named but not started (todo in %s): its checks wait.\n", id, d.cfg.Work.Registry)
	}
	if p.Prose {
		if p.Trusted != nil {
			fmt.Fprintln(d.log, p.Trusted.Line)
		} else {
			fmt.Fprintln(d.log, d.proseLine())
		}
		for _, c := range p.LeftOut {
			fmt.Fprintf(d.log, "  - %s: %s   (reads no prose; left out)\n", c.Task.ID, c.Check.Command())
		}
	}
	if p.Note != "" {
		fmt.Fprintln(d.log, p.Note)
	}
	if p.ProofOmitted != "" {
		fmt.Fprintln(d.log, p.ProofOmitted)
	}
	return nil, nil
}

// proseLine says what a prose-only range runs: the prose steps as
// ci.prose.steps gives them, then the named tasks' checks that read prose.
func (d *driver) proseLine() string {
	var steps strings.Builder
	if d.cfg.CI.Prose != nil {
		for _, step := range d.cfg.CI.Prose.Steps {
			fmt.Fprintf(&steps, "`%s`, ", step)
		}
	}
	and := ""
	if steps.Len() > 0 {
		and = "and "
	}
	return "Only prose changed: " + steps.String() + and + "the named tasks' static and `prose: true` checks."
}

// step runs one step through the config's shell, its output in the log; a
// failure carries the step's exit code, named on stderr after lead ("CI
// failed at" for the run's first, "also failed at" for a later one), 1 when
// it has none (stopped by a signal, or never started).
func (d *driver) step(s *plan.Step, lead string) *Failure {
	command := s.Command
	fmt.Fprintf(d.log, "\n$ %s\n", command)
	var ok bool
	var code int
	stdout, stderr := d.log, d.stderr
	if d.ann != nil {
		stdout, stderr = d.ann.watch(stdout, stderr)
	}
	if s.Evidence != nil && d.evidence != nil {
		o := d.evidence.Run(context.Background(), evidence.Request{Command: command, Decl: *s.Evidence, Verbose: true, Log: stdout})
		ok, code = o.OK, o.Code
	} else {
		r := shell.Run(d.cfg, command, shell.Options{Stdin: os.Stdin, Stdout: stdout, Stderr: stderr})
		ok, code = r.OK(), r.Code
	}
	if ok {
		return nil
	}
	if code <= 0 {
		fmt.Fprintf(d.stderr, "\n%s: %s (it gave no exit code)\n", lead, command)
		d.annotate(command, "it gave no exit code")
		return &Failure{Step: &command, Code: 1}
	}
	fmt.Fprintf(d.stderr, "\n%s: %s (it exited %d)\n", lead, command, code)
	d.annotate(command, fmt.Sprintf("it exited %d", code))
	return &Failure{Step: &command, Code: code}
}

// proof runs the range's code proof as a step of the run (slice 106, issue
// #28): the config's proof.code.check with {base} the range's start, run
// through internal/proof, so its answer is read as a provider's --json
// rather than as a step's exit code. A survivor, an uncovered or a missing
// result fails the run, naming each problem's rule and message, so the
// release that needs the run never runs; a check that cannot run fails it
// too, as any step does (@ID-CI-19), never a pass. The run's exit code is 1
// either way, the code of a step passed through (decision 35).
//
// A check that exits 0 may report, in the itos_proof envelope of its stdout,
// what it judged (slice 164, issue #25): the run logs one line of it, and
// fails, without the generic ci-code-proof problem, a check whose envelope
// breaks the contract (ci-code-proof-summary) or says it judged no work
// (ci-code-proof-empty). The executed proof, however it ended, is --json's
// proof.
func (d *driver) proof(s *plan.Step, lead string) *Failure {
	fmt.Fprintf(d.log, "\nCode proof\n$ %s\n", s.Command)
	v := proof.Run(d.cfg, s.Command, d.stderr)
	d.proofRun = &ProofRun{Command: s.Command, From: d.ends.From, To: d.ends.To, Outcome: v.Outcome.String(), Summary: v.Summary}
	problems, places := proofProblems(v.Problems)
	switch {
	case v.Outcome == proof.Passed:
		fmt.Fprintln(d.log, v.Summary.Line())
		return nil
	case v.Invalid != nil:
		problems, places = []out.Problem{{
			Rule:    "ci-code-proof-summary",
			Message: "the code proof's summary breaks its contract: " + v.Invalid.Message(),
			Fix:     "make proof.code.check print a valid itos_proof envelope (README.md, Proof of done), or none, and push the range again",
		}}, nil
	case v.NoWork():
		fmt.Fprintln(d.log, v.Summary.Line())
		problems, places = []out.Problem{{
			Rule:    "ci-code-proof-empty",
			Message: "the code proof reported that it judged no work (judged 0), so it proves nothing for this range",
			Fix:     "make proof.code.check judge the changed code: widen its scope or correct the base it is given, and push the range again",
		}}, nil
	case v.Outcome == proof.CannotRun:
		problems = append([]out.Problem{{
			Rule: "ci-code-proof",
			Message: fmt.Sprintf("the code proof could not run, so the run fails rather than passes it: %s %s",
				s.Command, v.Why),
			Fix: "make proof.code.check run (its command, its tools) and push the range again; nothing is proved until it answers",
		}}, problems...)
		places = append([]place{{}}, places...)
	}
	fmt.Fprintf(d.stderr, "\n%s the code proof: %s\n", lead, s.Command)
	if len(problems) == 0 {
		problems = []out.Problem{{
			Rule:    "ci-code-proof",
			Message: "the code proof refused and named no problem",
			Fix:     "run proof.code.check by hand to see what it found",
		}}
	}
	for i, p := range problems {
		fmt.Fprintf(d.stderr, "%s: %s\n", p.Rule, p.Message)
		if p.Fix != "" {
			fmt.Fprintf(d.stderr, "  fix: %s\n", p.Fix)
		}
		if d.ann != nil {
			var at place
			if i < len(places) {
				at = places[i]
			}
			d.ann.emit(at.file, at.line, "the code proof failed: "+p.Rule+": "+p.Message)
		}
	}
	return &Failure{Step: &s.Command, Problems: problems, Code: 1}
}

// place is where a problem's subject is: a file and line, or neither.
type place struct {
	file string
	line int
}

// proofProblems are a provider's problems as the run's, each naming the
// check's own rule and message and keeping its fix: what it found is the
// provider's to say, and an agent fixes it from the rule. The places are the
// files and lines their subjects name, one for each problem.
func proofProblems(found []proof.Problem) ([]out.Problem, []place) {
	problems := make([]out.Problem, 0, len(found))
	places := make([]place, 0, len(found))
	for _, p := range found {
		problems = append(problems, out.Problem{Rule: p.Rule, Message: p.Message, Fix: p.Fix})
		places = append(places, place{file: p.File, line: p.Line})
	}
	return problems, places
}

// check is one task check: logged as merged, covered, left out or pending,
// else run; a failure names the task and its title after lead.
func (d *driver) check(c *plan.Check, lead string) *Failure {
	command := c.Check.Command()
	fmt.Fprintf(d.log, "\n%s\n", c.Task.ID)
	switch c.Action {
	case plan.Merged:
		fmt.Fprintf(d.log, "  = %s   (in the %s run above)\n", command, c.Kind)
	case plan.Covered:
		fmt.Fprintf(d.log, "  = %s   (ran above as `%s`)\n", command, c.CoveredBy)
	case plan.Nightly:
		fmt.Fprintf(d.log, "  = %s   (left out of push CI by ci.nightly_only)\n", command)
	case plan.Pending:
		fmt.Fprintf(d.log, "  - %s   (pending: runs after the push)\n", command)
	default:
		if d.ann != nil {
			d.runner.Stdout, d.runner.Stderr = d.ann.watch(d.log, d.stderr)
			d.runner.ToStderr = false
		}
		if d.runner.Run(c.Check) != check.Fail {
			return nil
		}
		title := ""
		if c.Task.Title != "" {
			title = " (" + c.Task.Title + ")"
		}
		fmt.Fprintf(d.stderr, "\n%s %s%s, its check: %s\n", lead, c.Task.ID, title, command)
		d.annotate(c.Task.ID+title, "its check "+command)
		return &Failure{Task: c.Task.ID, Title: c.Task.Title, Command: command, Code: 1}
	}
	return nil
}

// annotate prints the failure of the step or check just run, when the run
// annotates.
func (d *driver) annotate(what, reason string) {
	if d.ann != nil {
		d.ann.run(what, reason)
	}
}
