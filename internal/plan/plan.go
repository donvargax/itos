// Package plan is what a CI run runs, decided once (tools/itos/ci-plan.ts,
// ci-plan-json.ts and ci-scope.ts), so that `ci plan` prints it and `ci run`
// carries it out from the same value, and phase 3's shadow compares the two
// implementations on `ci plan --json`.
//
//   - A push whose range CI can read runs one run of the kind of named tests
//     its `tests:` step names, over the kind's smoke set, the tests its
//     footers name (`Scenarios:`) and the subsets in the done_when of the
//     tasks its ledger footers (`Task:`) name; a range it cannot read runs
//     every test. A task check the kind's recognize templates read as a run
//     of the kind is merged into that run, unless the plan has no such run
//     (nothing at all selected): then it runs as itself.
//   - A named task's check that is one of the steps this run runs, or that a
//     ci.covers rule gives to one of them, is covered and not run again. A
//     check in ci.nightly_only is left to the nightly.
//   - The run is in cost order: the static steps, the named tasks' static
//     checks, the other steps, then the other checks (internal/check's cost
//     classes and written order).
//   - The nightly runs its own steps in the order written; its `{ tasks:
//     done }` step stands for the checks of every task whose work item is
//     done, in cost order where it is written (only the static ones with
//     `cost: static`).
//   - A prose-only range runs the prose steps, then the named tasks' static
//     checks and those that say `prose: true`, and lists what it leaves out.
//
// The Plan is a value the driver walks (T-049): Order is every step and
// check in the order the run takes them, each step with its command and
// cost (and the kind, for the one run of named tests), each check with its
// task, its place in done_when, its cost and what the run does with it
// (Action: run, merged into the kind's run, covered by a step, or left to
// the nightly).
package plan

import (
	"regexp"

	"github.com/donvargax/itos/v3/internal/check"
	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/ledger"
	"github.com/donvargax/itos/v3/internal/tests"
	"github.com/donvargax/itos/v3/internal/value"
	"github.com/donvargax/itos/v3/internal/work"
)

// Action is what a run does with a task check.
type Action string

const (
	// Run runs the check as itself.
	Run Action = "run"
	// Merged is a check whose tests are in the run's one run of their kind.
	Merged Action = "merged"
	// Covered is a check one of the run's steps has done.
	Covered Action = "covered"
	// Nightly is a check a push leaves to the nightly (ci.nightly_only).
	Nightly Action = "nightly"
)

// Step is one step of a run: its command, its cost, and Tests, the kind when
// it is the run's one run of named tests (a selection's command in a push,
// the kind's whole run in the nightly).
type Step struct {
	check.Costed
	Command string
	Tests   string
	// own is the step's own cost:, "" when it gives none.
	own string
}

// Check is one task check of a run: the check with its task (whose title a
// failure names), its place in done_when and its cost, and what the run does
// with it: Kind is the kind it is merged into, CoveredBy the step that did
// it.
type Check struct {
	check.CostedCheck
	Action    Action
	Kind      string
	CoveredBy string
}

// Item is one item of a run's order: a step or a check, the other nil.
type Item struct {
	Step  *Step
	Check *Check
}

// Plan is what a run does.
type Plan struct {
	// Prose is whether only prose changed: the prose steps run, and the named
	// tasks' static and prose: true checks.
	Prose bool
	// Order is everything the run does, in the order it does it.
	Order []Item
	// Steps are the steps' commands, in the order they run.
	Steps []string
	// Tasks are the tasks whose checks the plan holds: the named ones that
	// are started, in ledger order; for the nightly, the done tasks with a
	// check in its tasks step.
	Tasks []string
	// Checks are the checks of Order, in their tasks' order.
	Checks []*Check
	// LeftOut are the named tasks' checks a prose-only range does not run.
	LeftOut []*Check
	// Unknown are the task IDs a footer names that no task file has, in the
	// footers' order.
	Unknown []string
	// NotStarted are the named tasks whose work item is in
	// ci.wait_on_status: their checks wait.
	NotStarted []string
}

// stepOf is a CI step as the run takes it (ci-scope.ts's stepOf): a command
// written as text, a command with its cost, or a kind's whole run, its tests
// the kind.
func stepOf(cfg *config.Loaded, s config.Step) Step {
	var step Step
	if s.Cost != nil && s.Text == nil {
		step.own = *s.Cost
	}
	switch {
	case s.Text != nil:
		step.Command = *s.Text
	case s.Tests != nil:
		step.Tests = *s.Tests
		if k, ok := cfg.Tests.Get(*s.Tests); ok && k.Run.Whole != nil {
			step.Command = *k.Run.Whole
		}
	case s.Run != nil:
		step.Command = *s.Run
	}
	return step.costed(cfg)
}

// costed is the step with its cost: its own cost:, else the patterns'.
func (s Step) costed(cfg *config.Loaded) Step {
	s.Costed = check.CostOf(cfg, s.Command, s.own)
	return s
}

// ciSteps are ci.steps as the run takes them; a config error without a ci
// section.
func ciSteps(cfg *config.Loaded) ([]Step, error) {
	if err := cfg.Section("ci"); err != nil {
		return nil, err
	}
	steps := make([]Step, len(cfg.CI.Steps))
	for i, s := range cfg.CI.Steps {
		steps[i] = stepOf(cfg, s)
	}
	return steps, nil
}

// TestsKind is the kind of named tests ci.steps' `tests:` step runs, which a
// push narrows to a selection; "" when no step runs named tests.
func TestsKind(cfg *config.Loaded) (string, error) {
	steps, err := ciSteps(cfg)
	if err != nil {
		return "", err
	}
	for _, s := range steps {
		if s.Tests != "" {
			return s.Tests, nil
		}
	}
	return "", nil
}

// Smoke is the smoke set of the kind CI's `tests:` step runs, without the
// tag prefix, in the working tree or with at at that commit; none when no
// step runs named tests, so a CI without them needs no kind and no smoke
// set.
func Smoke(cfg *config.Loaded, at string) ([]string, error) {
	kind, err := TestsKind(cfg)
	if err != nil || kind == "" {
		return []string{}, err
	}
	smoke, err := tests.LoadSmokeAt(cfg, kind, at)
	if err != nil {
		return nil, err
	}
	k, _ := tests.KindOf(cfg, kind)
	return tests.SmokeIDs(k, smoke), nil
}

// Input is what a plan is made from.
type Input struct {
	// Prose is whether only prose changed.
	Prose bool
	// Known is whether the range was read; false runs every test.
	Known bool
	// Scenarios are the kind's test IDs the range's footers name.
	Scenarios []string
	// Tasks are the tasks the range's ledger footers name; for the nightly,
	// the tasks whose work item is done.
	Tasks   []ledger.Task
	Nightly bool
	// Smoke is the kind's smoke set, without the tag prefix.
	Smoke []string
}

// checksOf are tasks' checks with their costs, in order; an error for a task
// whose checks cannot be read.
func checksOf(cfg *config.Loaded, tasks []ledger.Task) ([]*Check, error) {
	var checks []*Check
	for _, t := range tasks {
		if t.Err != nil {
			return nil, t.Err
		}
		for _, c := range check.CostedChecks(cfg, t) {
			checks = append(checks, &Check{CostedCheck: c, Action: Run})
		}
	}
	return checks, nil
}

// merging is a check the kind reads as one of its runs, with its selection.
type merging struct {
	check     *Check
	selection tests.Selection
}

// runsOfKind are the checks with a run: command that are runs of the kind,
// each with its selection.
func runsOfKind(cfg *config.Loaded, checks []*Check, kind string, smoke []string) ([]merging, error) {
	var found []merging
	for _, c := range checks {
		if c.Check.Run == nil {
			continue
		}
		selection, ok, err := tests.Recognize(cfg, kind, *c.Check.Run, smoke)
		if err != nil {
			return nil, err
		}
		if ok {
			found = append(found, merging{c, selection})
		}
	}
	return found, nil
}

// coveredBy is the step that has done what a command does: the same
// command, its whitespace collapsed, or the step of the first ci.covers rule
// that is one of the run's steps and whose pattern matches one of the
// command's readings, so one written for itos covers a check that calls
// hooks.bin.
func coveredBy(cfg *config.Loaded, command string, steps []string) (string, error) {
	c := config.Normal(command)
	if value.Includes(steps, c) {
		return c, nil
	}
	var forms []string
	for _, r := range cfg.Readings(command) {
		forms = append(forms, config.Normal(r))
	}
	for _, rule := range cfg.CI.Covers {
		if !value.Includes(steps, rule.By) {
			continue
		}
		matches, err := regexp.Compile(rule.Matches)
		if err != nil {
			return "", err
		}
		for _, form := range forms {
			if matches.MatchString(form) {
				return rule.By, nil
			}
		}
	}
	return "", nil
}

// leftToNightly is whether ci.nightly_only lists a command, an entry written
// for itos naming a check that calls hooks.bin too.
func leftToNightly(cfg *config.Loaded, command string) bool {
	for _, r := range cfg.Readings(command) {
		if value.Includes(cfg.CI.NightlyOnly, config.Normal(r)) {
			return true
		}
	}
	return false
}

// markDone gives each check with a run: command that is not merged the step
// that did it, or, in a push, the nightly when it is the nightly's.
func markDone(cfg *config.Loaded, checks []*Check, steps []string, nightly bool) error {
	for _, c := range checks {
		if c.Action == Merged || c.Check.Run == nil {
			continue
		}
		by, err := coveredBy(cfg, *c.Check.Run, steps)
		if err != nil {
			return err
		}
		switch {
		case by != "":
			c.Action, c.CoveredBy = Covered, by
		case !nightly && leftToNightly(cfg, *c.Check.Run):
			c.Action = Nightly
		}
	}
	return nil
}

// inCostOrder is the static steps, the static checks, the other steps (the
// unit tests, the build, the audit, the run of named tests), then the
// checks that may need any of them.
func inCostOrder(steps []Step, checks []*Check) []Item {
	order := []Item{}
	for _, static := range []bool{true, false} {
		for i := range steps {
			if (steps[i].Cost == check.Static) == static {
				order = append(order, Item{Step: &steps[i]})
			}
		}
		for _, c := range checks {
			if (c.Cost == check.Static) == static {
				order = append(order, Item{Check: c})
			}
		}
	}
	return order
}

// runsOnProse is whether a prose-only range still runs a check: one that
// takes seconds, or one whose result prose can change.
func runsOnProse(c *Check) bool { return c.Cost == check.Static || c.Check.Prose }

// commands are steps' commands.
func commands(steps []Step) []string {
	list := make([]string, len(steps))
	for i, s := range steps {
		list[i] = s.Command
	}
	return list
}

// Make is the plan for an input (ci-plan.ts's ciPlan).
func Make(cfg *config.Loaded, in Input) (*Plan, error) {
	if in.Nightly {
		return nightlyPlan(cfg, in.Tasks, in.Smoke)
	}
	named, err := checksOf(cfg, in.Tasks)
	if err != nil {
		return nil, err
	}
	p := &Plan{Prose: in.Prose, Tasks: []string{}, Checks: []*Check{}, LeftOut: []*Check{}, Unknown: []string{}, NotStarted: []string{}}
	for _, t := range in.Tasks {
		p.Tasks = append(p.Tasks, t.ID)
	}
	for _, c := range named {
		if !in.Prose || runsOnProse(c) {
			p.Checks = append(p.Checks, c)
		} else {
			p.LeftOut = append(p.LeftOut, c)
		}
	}
	kind, err := TestsKind(cfg)
	if err != nil {
		return nil, err
	}
	var merge []merging
	testsRun, haveRun := "", false
	if kind != "" {
		if merge, err = runsOfKind(cfg, p.Checks, kind, in.Smoke); err != nil {
			return nil, err
		}
		// The run's own selection: the smoke set and the named tests, every
		// test for a range it cannot read, none for prose.
		var selections []tests.Selection
		if !in.Prose {
			if in.Known {
				selections = append(selections, tests.IDs(append(append([]string{}, in.Smoke...), in.Scenarios...)...))
			} else {
				selections = append(selections, tests.Whole())
			}
		}
		for _, m := range merge {
			selections = append(selections, m.selection)
		}
		if testsRun, haveRun, err = tests.CommandFor(cfg, kind, selections); err != nil {
			return nil, err
		}
	}
	// A check is merged only into a run that happens: with nothing selected
	// (an empty smoke set, a range that names no test) it runs as itself.
	if haveRun {
		for _, m := range merge {
			m.check.Action, m.check.Kind = Merged, kind
		}
	}
	// The kind's run takes the merged selection's command, and keeps its
	// cost class as written.
	var configured []Step
	if in.Prose {
		if cfg.CI.Prose != nil {
			for _, command := range cfg.CI.Prose.Steps {
				configured = append(configured, Step{Command: command})
			}
		}
	} else if configured, err = ciSteps(cfg); err != nil {
		return nil, err
	}
	var runs []Step
	for _, s := range configured {
		if s.Tests == "" {
			runs = append(runs, s)
		} else if haveRun {
			s.Command = testsRun
			runs = append(runs, s)
		}
	}
	if in.Prose && haveRun {
		runs = append(runs, Step{Command: testsRun, Tests: kind})
	}
	for i := range runs {
		runs[i] = runs[i].costed(cfg)
	}
	p.Steps = commands(runs)
	if err := markDone(cfg, p.Checks, p.Steps, false); err != nil {
		return nil, err
	}
	p.Order = inCostOrder(runs, p.Checks)
	return p, nil
}

// nightlyPlan is the nightly's plan: its steps in written order, its tasks
// step standing for the done tasks' checks, static then late (only the
// static ones with cost: static). Each check is merged into the nightly's
// whole run of its kind (the first kind its steps run), covered by one of
// its steps, or run.
func nightlyPlan(cfg *config.Loaded, done []ledger.Task, smoke []string) (*Plan, error) {
	p := &Plan{Order: []Item{}, Steps: []string{}, Tasks: []string{}, Checks: []*Check{}, LeftOut: []*Check{}, Unknown: []string{}, NotStarted: []string{}}
	kind := ""
	var configured []config.Step
	if cfg.CI.Nightly != nil {
		configured = cfg.CI.Nightly.Steps
	}
	for _, s := range configured {
		if s.Text == nil && s.Tasks != nil {
			checks, err := checksOf(cfg, done)
			if err != nil {
				return nil, err
			}
			var planned []*Check
			for _, c := range checks {
				if s.Cost == nil || *s.Cost != "static" || c.Cost == check.Static {
					planned = append(planned, c)
				}
			}
			p.Checks = append(p.Checks, planned...)
			p.Order = append(p.Order, inCostOrder(nil, planned)...)
			continue
		}
		step := stepOf(cfg, s)
		if kind == "" {
			kind = step.Tests
		}
		p.Steps = append(p.Steps, step.Command)
		p.Order = append(p.Order, Item{Step: &step})
	}
	if kind != "" {
		merge, err := runsOfKind(cfg, p.Checks, kind, smoke)
		if err != nil {
			return nil, err
		}
		for _, m := range merge {
			m.check.Action, m.check.Kind = Merged, kind
		}
	}
	if err := markDone(cfg, p.Checks, p.Steps, true); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, c := range p.Checks {
		if !seen[c.Task.ID] {
			seen[c.Task.ID] = true
			p.Tasks = append(p.Tasks, c.Task.ID)
		}
	}
	return p, nil
}

// hasTasksStep is whether the nightly has a step that runs the done tasks'
// checks.
func hasTasksStep(cfg *config.Loaded) bool {
	if cfg.CI.Nightly == nil {
		return false
	}
	for _, s := range cfg.CI.Nightly.Steps {
		if s.Text == nil && s.Tasks != nil {
			return true
		}
	}
	return false
}

// DoneTasks are the tasks whose work item is done in the registry, for the
// nightly's tasks step; none, and the ledger unread, when the nightly has
// no such step.
func DoneTasks(cfg *config.Loaded) ([]ledger.Task, error) {
	if !hasTasksStep(cfg) {
		return nil, nil
	}
	statuses := work.ItemStatuses(cfg.Work.Registry)
	all, err := ledger.Tasks(cfg)
	if err != nil {
		return nil, err
	}
	var done []ledger.Task
	for _, t := range all {
		if status, ok := statuses.Of(t.ID); ok && status == "done" {
			done = append(done, t)
		}
	}
	return done, nil
}

// ForNightly is the nightly's plan, the done tasks read from the ledger and
// the registry, the smoke set from the working tree.
func ForNightly(cfg *config.Loaded) (*Plan, error) {
	done, err := DoneTasks(cfg)
	if err != nil {
		return nil, err
	}
	smoke, err := Smoke(cfg, "")
	if err != nil {
		return nil, err
	}
	return Make(cfg, Input{Nightly: true, Tasks: done, Smoke: smoke})
}
