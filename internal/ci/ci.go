// Package ci is CI's driver (tools/itos/ci.ts's ciRun): the plan
// internal/plan made, carried out item by item in the order it gives, so
// that `ci run` runs what `ci plan` prints and never plans again.
//
//   - Before the first item comes the preamble: the tasks a footer names that
//     no task file has (which end the run, whatever the settings), the tasks
//     named but not started, and, for a prose-only range, what it leaves out.
//   - A step runs through the config's shell with itos's streams; a failing
//     step's exit code is the run's.
//   - A task check the plan runs goes through a check.Runner, verbose, so its
//     command line and output keep their place in the log; a failure exits 1,
//     naming the task and its title. A check merged into the run of named
//     tests, covered by a step, left out by ci.nightly_only or pending until
//     after the push (an after: push check) is only logged, and never fails
//     the run. The
//     nightly shares one Runner's runs across its done tasks, so a check they
//     share runs once; a push runs each named task's checks as its own.
//   - The first failure ends the run, unless ci.stop_at_first_failure is
//     false: then everything runs, each failure says where, and the first is
//     the run's.
//
// Every step and check sees ci.env. The log goes to stdout, or to stderr
// under --json, which keeps stdout for its one object.
package ci

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/v5/internal/check"
	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/ledger"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/plan"
	"github.com/donvargax/itos/v5/internal/shell"
)

// Failure is where a run stopped, as --json's failed_at gives it: a step
// (whose exit code is the run's), a task check, or the unknown tasks named.
type Failure struct {
	Step    *string  `json:"step,omitempty"`
	Task    string   `json:"task,omitempty"`
	Title   string   `json:"title,omitempty"`
	Command string   `json:"command,omitempty"`
	Unknown []string `json:"unknown,omitempty"`
	Code    int      `json:"code"`
}

// Options are how a run reports: under JSON its log goes to Stderr and
// Stdout holds the verdict. Nightly shares the task checks' runs.
type Options struct {
	JSON, Nightly  bool
	Stdout, Stderr io.Writer
}

// driver is one run: the config, its streams and its check runner.
type driver struct {
	cfg    *config.Loaded
	log    io.Writer
	stderr io.Writer
	runner *check.Runner
}

// Run carries out a plan and gives the run's exit code: 0, a failing step's
// own code, or 1 for a task check or the unknown tasks.
func Run(cfg *config.Loaded, p *plan.Plan, o Options) (int, error) {
	for key, value := range cfg.CI.Env {
		if err := os.Setenv(key, value); err != nil {
			return 0, err
		}
	}
	d := driver{cfg: cfg, log: o.Stdout, stderr: o.Stderr}
	if o.JSON {
		d.log = o.Stderr
	}
	d.runner = check.NewRunner(cfg, o.Stdout, o.Stderr)
	d.runner.Verbose, d.runner.ToStderr = true, o.JSON
	if !o.Nightly {
		d.runner.Runs = nil
	}
	failed, err := d.preamble(p)
	if err != nil {
		return 0, err
	}
	for _, item := range p.Order {
		if failed != nil && (cfg.CI.StopAtFirstFailure || failed.Unknown != nil) {
			break
		}
		var failure *Failure
		if item.Step != nil {
			failure = d.step(item.Step.Command)
		} else {
			failure = d.check(item.Check)
		}
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
			fields = append(fields, out.Field{Key: "failed_at", Value: failed})
		}
		if err := out.Emit(o.Stdout, fields...); err != nil {
			return 0, err
		}
	}
	if failed == nil {
		return 0, nil
	}
	return failed.Code, nil
}

// preamble is what the run says before its first item: the unknown tasks,
// named against the ledger's folder, which end it; the tasks not started;
// and what a prose-only range runs and leaves out.
func (d driver) preamble(p *plan.Plan) (*Failure, error) {
	if len(p.Unknown) > 0 {
		layout, err := ledger.LayoutOf(d.cfg)
		if err != nil {
			return nil, err
		}
		for _, id := range p.Unknown {
			fmt.Fprintf(d.stderr, "No task %s in %s/, though a footer names it\n", id, layout.Dir)
		}
		fmt.Fprintf(d.stderr, "\nCI failed at the tasks named: %s\n", strings.Join(p.Unknown, ", "))
		return &Failure{Unknown: p.Unknown, Code: 1}, nil
	}
	for _, id := range p.NotStarted {
		fmt.Fprintf(d.log, "%s is named but not started (todo in %s): its checks wait.\n", id, d.cfg.Work.Registry)
	}
	if p.Prose {
		fmt.Fprintln(d.log, d.proseLine())
		for _, c := range p.LeftOut {
			fmt.Fprintf(d.log, "  - %s: %s   (reads no prose; left out)\n", c.Task.ID, c.Check.Command())
		}
	}
	return nil, nil
}

// proseLine says what a prose-only range runs: the prose steps as
// ci.prose.steps gives them, then the named tasks' checks that read prose.
func (d driver) proseLine() string {
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
// failure carries its exit code, 1 when it has none (stopped by a signal, or
// never started).
func (d driver) step(command string) *Failure {
	fmt.Fprintf(d.log, "\n$ %s\n", command)
	r := shell.Run(d.cfg, command, shell.Options{Stdin: os.Stdin, Stdout: d.log, Stderr: d.stderr})
	if r.OK() {
		return nil
	}
	fmt.Fprintf(d.stderr, "\nCI failed at: %s\n", command)
	code := r.Code
	if code <= 0 {
		code = 1
	}
	return &Failure{Step: &command, Code: code}
}

// check is one task check: logged as merged, covered, left out or pending,
// else run; a failure names the task and its title.
func (d driver) check(c *plan.Check) *Failure {
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
		if d.runner.Run(c.Check) != check.Fail {
			return nil
		}
		title := ""
		if c.Task.Title != "" {
			title = " (" + c.Task.Title + ")"
		}
		fmt.Fprintf(d.stderr, "\nCI failed at %s%s, its check: %s\n", c.Task.ID, title, command)
		return &Failure{Task: c.Task.ID, Title: c.Task.Title, Command: command, Code: 1}
	}
	return nil
}
