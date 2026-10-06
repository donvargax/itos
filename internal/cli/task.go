package cli

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v5/internal/check"
	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/ledger"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/source"
	"github.com/donvargax/itos/v5/internal/value"
	"github.com/donvargax/itos/v5/internal/work"
)

// The task runner (cli.ts): `itos task <id>… | --group <g> [--skip <ids>] |
// --pending` (--phase and --<ledger.group.label> are --group too) and `itos
// task list`. One invocation runs each distinct check once (check.Runner):
// the first task that lists it runs it, and every later task reads that exit
// status by its own run: or fails:; an after: push check is pending before
// the push, as always. A known limit, accepted: a check that would need a
// fresh run because a check above it in its own task changed the tree reads
// the earlier result instead.

// taskStatus is a task's row in the status table.
type taskStatus string

const (
	statusDone    taskStatus = "done"
	statusPending taskStatus = "pending"
	statusFailing taskStatus = "failing"
	statusReview  taskStatus = "review"
)

// runTask runs a task's checks in written order, every one of them whatever
// the one before found: review when it has none, failing when one fails,
// pending when one waits for the push, else done.
func runTask(task ledger.Task, runner *check.Runner) (taskStatus, []check.Result) {
	if len(task.DoneWhen) == 0 {
		return statusReview, nil
	}
	if runner.Verbose {
		log := runner.Stdout
		if runner.ToStderr {
			log = runner.Stderr
		}
		fmt.Fprintf(log, "\n%s %s\n", task.ID, task.Title)
	}
	results := make([]check.Result, len(task.DoneWhen))
	for i, c := range task.DoneWhen {
		results[i] = runner.Run(c)
	}
	switch {
	case slices.Contains(results, check.Fail):
		return statusFailing, results
	case slices.Contains(results, check.Pending):
		return statusPending, results
	}
	return statusDone, results
}

// groupFlags are the flags that name a group: --group, --phase and the
// label's own (ledger.group.label), all the same.
func groupFlags(cfg *config.Loaded) []string {
	flags := []string{"--group", "--phase"}
	if own := "--" + cfg.Ledger.Group.Label; !slices.Contains(flags, own) {
		flags = append(flags, own)
	}
	return flags
}

// groupFlag is the group the first of the group flags names, in their order
// rather than the command line's, and whether one does.
func groupFlag(cfg *config.Loaded, args []string) (string, bool) {
	for _, f := range groupFlags(cfg) {
		if v, ok := flagValue(args, f); ok {
			return v, true
		}
	}
	return "", false
}

// inGroup is whether a task is in the group a group flag names, as the
// ledger's tasks hold it: Number() of the flag's value when the groups are
// numeric, so 01 is group 1 and nothing is group NaN.
func inGroup(cfg *config.Loaded, task ledger.Task, group string) bool {
	if cfg.Ledger.Group.Numeric {
		n, ok := task.Group.(float64)
		return ok && n == value.ToNumber(group)
	}
	return task.Group == group
}

// selectTasks are the tasks the arguments name: a group's, every task under
// --pending, else those whose IDs are given, in the ledger's order; less
// those --skip names.
func selectTasks(cfg *config.Loaded, args []string, tasks []ledger.Task) []ledger.Task {
	group, byGroup := groupFlag(cfg, args)
	skip := map[string]bool{}
	if ids, ok := flagValue(args, "--skip"); ok {
		for _, id := range strings.Split(ids, ",") {
			skip[id] = true
		}
	}
	var selected []ledger.Task
	for _, t := range tasks {
		var named bool
		switch {
		case byGroup:
			named = inGroup(cfg, t, group)
		case slices.Contains(args, "--pending"):
			named = true
		default:
			named = slices.Contains(args, t.ID)
		}
		if named && !skip[t.ID] {
			selected = append(selected, t)
		}
	}
	return selected
}

// unknownIDs are the IDs named on the command line that the ledger does not
// have: the arguments that look like a task's ID (ledger.id, or T-<n> when
// the config has none) and are not the value of a group flag or --skip.
func unknownIDs(cfg *config.Loaded, args []string, tasks []ledger.Task) []string {
	id := `T-\d+`
	if cfg.Ledger.ID != nil {
		id = *cfg.Ledger.ID
	}
	pattern := regexp.MustCompile("^(?:" + id + ")$")
	values := map[string]bool{}
	for _, f := range append(groupFlags(cfg), "--skip") {
		if v, ok := flagValue(args, f); ok {
			values[v] = true
		}
	}
	var unknown []string
	for _, a := range args {
		if !pattern.MatchString(a) || values[a] {
			continue
		}
		if !slices.ContainsFunc(tasks, func(t ledger.Task) bool { return t.ID == a }) {
			unknown = append(unknown, a)
		}
	}
	return unknown
}

// checkJSON and taskJSON are a task's object under --json, the keys in
// cli.ts's order.
type checkJSON struct {
	Command string       `json:"command"`
	Fails   bool         `json:"fails,omitempty"`
	Result  check.Result `json:"result"`
}

type taskJSON struct {
	ID     string      `json:"id"`
	Type   string      `json:"type"`
	Title  string      `json:"title"`
	Group  any         `json:"group"`
	Status taskStatus  `json:"status"`
	Checks []checkJSON `json:"checks"`
}

type taskRow struct {
	task    ledger.Task
	status  taskStatus
	results []check.Result
}

func task(args []string, o Out) (int, error) {
	switch sub, rest := split(args); sub {
	case "list":
		return listTasks(rest, o)
	case "add":
		return taskAdd(rest, o)
	case "next-id":
		return taskNextID(rest, o)
	}
	return runTasks(args, o)
}

// runTasks is `task <id>…`: runs the tasks' checks in written order, each
// distinct check once, verbose for one task, and prints the status table. 1
// when a check fails or an ID is unknown, 2 when nothing matches.
func runTasks(args []string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	tasks, err := ledger.Tasks(cfg)
	if err != nil {
		return 0, err
	}
	if unknown := unknownIDs(cfg, args, tasks); len(unknown) > 0 {
		layout, err := ledger.LayoutOf(cfg)
		if err != nil {
			return 0, err
		}
		for _, id := range unknown {
			fmt.Fprintf(o.Stderr, "No task %s in %s/\n", id, layout.Dir)
		}
		return ExitPolicy, nil
	}
	selected := selectTasks(cfg, args, tasks)
	if len(selected) == 0 {
		by := "--group"
		if own := "--" + cfg.Ledger.Group.Label; own != by {
			by += "|" + own
		}
		fmt.Fprintf(o.Stderr, "No matching tasks. Usage: itos task <id>… | %s <g> [--skip <ids>] | --pending\n", by)
		return ExitUsage, nil
	}
	runner := check.NewRunner(cfg, o.Stdout, o.Stderr)
	runner.Verbose, runner.ToStderr = len(selected) == 1, o.JSON
	for _, t := range selected {
		if t.Err != nil {
			return 0, t.Err
		}
	}
	rows := make([]taskRow, len(selected))
	code := 0
	for i, t := range selected {
		status, results := runTask(t, runner)
		rows[i] = taskRow{t, status, results}
		if status == statusFailing {
			code = ExitPolicy
		}
	}
	shown := rows
	if slices.Contains(args, "--pending") {
		shown = slices.DeleteFunc(slices.Clone(rows), func(r taskRow) bool { return r.status == statusDone })
	}
	if o.JSON {
		return code, out.Emit(o.Stdout, out.Field{Key: "tasks", Value: tasksJSON(shown)})
	}
	fmt.Fprintln(o.Stdout)
	for _, r := range shown {
		fmt.Fprintf(o.Stdout, "%s %s  %s %s\n", value.PadEnd(string(r.status), 8), r.task.ID, value.PadEnd(r.task.Type, 8), r.task.Title)
	}
	return code, nil
}

func tasksJSON(rows []taskRow) []taskJSON {
	list := make([]taskJSON, len(rows))
	for i, r := range rows {
		checks := make([]checkJSON, len(r.task.DoneWhen))
		for n, c := range r.task.DoneWhen {
			result := check.Result("review")
			if n < len(r.results) {
				result = r.results[n]
			}
			checks[n] = checkJSON{Command: c.Command(), Fails: c.Fails != nil, Result: result}
		}
		list[i] = taskJSON{r.task.ID, r.task.Type, r.task.Title, r.task.Group, r.status, checks}
	}
	return list
}

// listJSON is a task's object under task list --json.
type listJSON struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Group  any    `json:"group"`
	Checks int    `json:"checks"`
	Status any    `json:"status"`
}

// listTasks is `task list [--group <g>]`: the tasks, each with its work
// item's status in the registry (the item whose id is the task's), or "no
// item"; it runs nothing, and reads the registry alone, without the people.
// With no registry where itos looks, it says so, naming the path.
func listTasks(args []string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	all, err := ledger.Tasks(cfg)
	if err != nil {
		return 0, err
	}
	group, byGroup := groupFlag(cfg, args)
	tasks := slices.DeleteFunc(all, func(t ledger.Task) bool { return byGroup && !inGroup(cfg, t, group) })
	registry := cfg.Work.Registry
	if !source.Has(registry) {
		fmt.Fprintf(o.Stderr, "No work registry at %s, so no task has an item.\n", registry)
	}
	statuses := work.ItemStatuses(registry)
	if o.JSON {
		list := make([]listJSON, len(tasks))
		for i, t := range tasks {
			status, _ := statuses.Of(t.ID)
			list[i] = listJSON{t.ID, t.Type, t.Title, t.Group, len(t.DoneWhen), status}
		}
		return 0, out.Emit(o.Stdout, out.Field{Key: "tasks", Value: list})
	}
	for _, t := range tasks {
		status := "no item"
		if s, ok := statuses.Of(t.ID); ok {
			status = value.String(s)
		}
		fmt.Fprintf(o.Stdout, "%s %s  %s %s %s\n", value.PadEnd(status, 8), t.ID, value.PadEnd(t.GroupText(), 3), value.PadEnd(t.Type, 8), t.Title)
	}
	return 0, nil
}
