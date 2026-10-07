package ci

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/v7/internal/check"
	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/ledger"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/plan"
	"github.com/donvargax/itos/v7/internal/shell"
)

// Failure is where a run stopped, as --json's failed_at gives it: a step
// (Code its own exit code), a task check, or the unknown tasks named.
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
	return 1, nil
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
// failure carries the step's exit code, named on stderr, 1 when it has none
// (stopped by a signal, or never started).
func (d driver) step(command string) *Failure {
	fmt.Fprintf(d.log, "\n$ %s\n", command)
	r := shell.Run(d.cfg, command, shell.Options{Stdin: os.Stdin, Stdout: d.log, Stderr: d.stderr})
	if r.OK() {
		return nil
	}
	if r.Code <= 0 {
		fmt.Fprintf(d.stderr, "\nCI failed at: %s (it gave no exit code)\n", command)
		return &Failure{Step: &command, Code: 1}
	}
	fmt.Fprintf(d.stderr, "\nCI failed at: %s (it exited %d)\n", command, r.Code)
	return &Failure{Step: &command, Code: r.Code}
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
