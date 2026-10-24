package task

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
)

// The exit codes task gives itself (itos --help): a check failing, an ID
// not in the ledger or task add's refusal; and no task matching.
const (
	refused   = 1
	unmatched = 2
)

// checkJSON and taskJSON are a task's object under --json, the keys in
// cli.ts's order.
type checkJSON struct {
	Command  string              `json:"command"`
	Fails    bool                `json:"fails,omitempty"`
	Result   check.Result        `json:"result"`
	Evidence *check.EvidenceInfo `json:"evidence,omitempty"`
}

type taskJSON struct {
	ID     string      `json:"id"`
	Type   string      `json:"type"`
	Title  string      `json:"title"`
	Group  any         `json:"group"`
	Status taskStatus  `json:"status"`
	Checks []checkJSON `json:"checks"`
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

// Run is task over h: the status table after the checks' own output, the
// tasks listed, the next ID or the task added, on stdout or as --json's one
// object. An ID not in the ledger is a line on stderr each, exit 1; no task
// matching a usage line on stderr, exit 2; a check failing exit 1; task
// add's refusal its problems on stderr, or under --json on stdout, exit 1.
// An error is the command line's to report. The registry's lock task add
// holds is given back last.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, stdout, stderr)
	if a.Release != nil {
		defer a.Release()
	}
	switch {
	case err != nil:
		return a.Code, err
	case a.Code != 0:
		return a.Code, nil
	case len(a.Problems) > 0:
		return c.refuse(stdout, stderr, a.Problems)
	}
	switch c.Sub {
	case "list":
		return c.list(stdout, stderr, a)
	case "add":
		return c.added(stdout, a)
	case "next-id":
		if c.JSON {
			return 0, out.Emit(stdout, out.Field{Key: "id", Value: a.NextID})
		}
		fmt.Fprintln(stdout, a.NextID)
		return 0, nil
	}
	return c.table(stdout, stderr, a)
}

// table is the status table of the tasks run, after the unknown IDs or no
// task matching.
func (c CLI) table(stdout, stderr io.Writer, a Answer) (int, error) {
	if len(a.Unknown) > 0 {
		for _, id := range a.Unknown {
			fmt.Fprintf(stderr, "No task %s in %s/\n", id, a.Dir)
		}
		return refused, nil
	}
	if a.Unmatched {
		by := "--group"
		if own := "--" + a.Label; own != by {
			by += "|" + own
		}
		fmt.Fprintf(stderr, "No matching tasks. Usage: itos task <id>… | %s <g> [--skip <ids>] | --pending\n", by)
		return unmatched, nil
	}
	code := 0
	if a.Failing {
		code = refused
	}
	if c.JSON {
		fields := []out.Field{{Key: "tasks", Value: tasksJSON(a.Rows)}}
		if a.Evidence != nil {
			if unstaged, counted := a.Evidence.Unstaged(); counted {
				fields = append(fields, out.Field{Key: "unstaged", Value: unstaged})
			}
		}
		return code, out.Emit(stdout, fields...)
	}
	fmt.Fprintln(stdout)
	for _, r := range a.Rows {
		fmt.Fprintf(stdout, "%s %s  %s %s\n", value.PadEnd(string(r.status), 8), r.task.ID, value.PadEnd(r.task.Type, 8), r.task.Title)
	}
	return code, nil
}

// tasksJSON are the rows under --json, each check with its result:
// judgeTask gives a task with checks one result for each.
func tasksJSON(rows []taskRow) []taskJSON {
	list := make([]taskJSON, len(rows))
	for i, r := range rows {
		checks := make([]checkJSON, len(r.task.DoneWhen))
		for n, c := range r.task.DoneWhen {
			checks[n] = checkJSON{Command: c.Command(), Fails: c.Fails != nil, Result: r.results[n], Evidence: r.evidence[n]}
		}
		list[i] = taskJSON{r.task.ID, r.task.Type, r.task.Title, r.task.Group, r.status, checks}
	}
	return list
}

// list prints task list's tasks, each with its item's status or "no item",
// after a line on stderr when there is no registry.
func (c CLI) list(stdout, stderr io.Writer, a Answer) (int, error) {
	if a.NoRegistry {
		fmt.Fprintf(stderr, "No work registry at %s, so no task has an item.\n", a.Registry)
	}
	if c.JSON {
		list := make([]listJSON, len(a.Tasks))
		for i, t := range a.Tasks {
			status, _ := a.Statuses.Of(t.ID)
			list[i] = listJSON{t.ID, t.Type, t.Title, t.Group, len(t.DoneWhen), status}
		}
		return 0, out.Emit(stdout, out.Field{Key: "tasks", Value: list})
	}
	for _, t := range a.Tasks {
		status := "no item"
		if s, ok := a.Statuses.Of(t.ID); ok {
			status = value.String(s)
		}
		fmt.Fprintf(stdout, "%s %s  %s %s %s\n", value.PadEnd(status, 8), t.ID, value.PadEnd(t.GroupText(), 3), value.PadEnd(t.Type, 8), t.Title)
	}
	return 0, nil
}

// added reports the task task add wrote: the line, unless --quiet; under
// --json, ok, the task, its file, the item and the commit (null when none).
func (c CLI) added(stdout io.Writer, a Answer) (int, error) {
	t := a.Added
	if c.JSON {
		var commit any
		if t.Commit != "" {
			commit = t.Commit
		}
		return 0, out.Emit(stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "task", Value: t.Task},
			out.Field{Key: "file", Value: t.File}, out.Field{Key: "item", Value: t.Item}, out.Field{Key: "commit", Value: commit})
	}
	if c.Quiet {
		return 0, nil
	}
	result := "the ledger and the registry are the stealth config's, so nothing is committed"
	if t.Commit != "" {
		result = "committed " + t.Commit[:min(7, len(t.Commit))] + " " + t.Header
	}
	where := t.File
	if t.Created {
		where = "a new " + where
	}
	fmt.Fprintf(stdout, "%s is a new task in %s, and an item in %s: %s\n", t.ID, where, a.Registry, result)
	return 0, nil
}

// refuse prints task add's problems, exit 1: on stderr, or under --json as
// {"ok": false, "problems": […]} on stdout.
func (c CLI) refuse(stdout, stderr io.Writer, found []out.Problem) (int, error) {
	if c.JSON {
		return refused, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: found})
	}
	for _, p := range found {
		fmt.Fprintln(stderr, p.Message)
		if p.Fix != "" {
			fmt.Fprintf(stderr, "  fix: %s\n", p.Fix)
		}
	}
	return refused, nil
}
