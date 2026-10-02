package ledger

import (
	"fmt"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/source"
	"github.com/donvargax/itos/internal/value"
)

// Check is one check of a task's done_when, as the tools read it (repo.ts's
// Check): its command under run or fails, after: push, its own timeout in
// seconds (nil for the ledger's), prose and its own cost class ("" for the
// patterns').
type Check struct {
	Run, Fails *string
	After      string
	Timeout    *float64
	Prose      bool
	Cost       string
}

// Command is the check's command: run's, else fails', else "" for a check
// without one (its task's Err says so).
func (c Check) Command() string {
	switch {
	case c.Run != nil:
		return *c.Run
	case c.Fails != nil:
		return *c.Fails
	}
	return ""
}

// MustFail is whether the check must fail: it has a fails: command that is
// not empty, as JavaScript reads `check.fails` as true.
func (c Check) MustFail() bool { return c.Fails != nil && *c.Fails != "" }

// Pushed is whether the check waits for the push (after: push).
func (c Check) Pushed() bool { return c.After == "push" }

// Task is one task of the ledger (repo.ts's Task): its fields, its checks in
// written order, none for a task left to review, and its group, a number
// when ledger.group.numeric holds (Number() of the file's group) and the
// group's text otherwise. Err is why its checks cannot run (a done_when that
// is not a list, a check without a command): an error only for a command
// that runs this task's checks, as the TypeScript fails only there, so one
// malformed task does not keep the others from running.
type Task struct {
	ID, Type, Title string
	DoneWhen        []Check
	Group           any
	Err             error
}

// GroupText is the group as a template literal writes it.
func (t Task) GroupText() string { return value.String(t.Group) }

// Tasks are the ledger's tasks (repo.ts's loadTasks): every ledger file's,
// the files in the order Files gives, each file's in written order. It reads
// them as they are and checks nothing (config check holds the ledger to its
// schema); a file that cannot be read or is not a list is an error, and a
// task whose checks cannot run says so in its Err.
func Tasks(cfg *config.Loaded) ([]Task, error) {
	files, err := Files(cfg)
	if err != nil {
		return nil, err
	}
	var tasks []Task
	for _, f := range files {
		text, err := source.Read(f.Path)
		if err != nil {
			return nil, err
		}
		raw, err := value.Parse(text)
		if err != nil {
			return nil, err
		}
		if raw == nil {
			continue
		}
		list, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("%s is not a list of tasks", f.Path)
		}
		var group any = f.Group
		if cfg.Ledger.Group.Numeric {
			group = value.ToNumber(f.Group)
		}
		for _, raw := range list {
			task := taskOf(raw, group)
			if task.Err != nil {
				task.Err = fmt.Errorf("%s: %w", f.Path, task.Err)
			}
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func taskOf(raw, group any) Task {
	task := Task{
		ID:    value.String(value.Prop(raw, "id")),
		Type:  value.String(value.Prop(raw, "type")),
		Title: value.String(value.Prop(raw, "title")),
		Group: group,
	}
	doneWhen := value.Prop(raw, "done_when")
	if doneWhen == nil || doneWhen == value.Undefined {
		return task
	}
	checks, ok := doneWhen.([]any)
	if !ok {
		task.Err = fmt.Errorf("%s: done_when is not a list", task.ID)
		return task
	}
	task.DoneWhen = make([]Check, len(checks))
	for n, c := range checks {
		check, ok := checkOf(c)
		if !ok && task.Err == nil {
			task.Err = fmt.Errorf("%s: check %d has no command", task.ID, n)
		}
		task.DoneWhen[n] = check
	}
	return task
}

// checkOf is a check as written, and whether it has a command to run.
func checkOf(raw any) (Check, bool) {
	text := func(key string) *string {
		if s, ok := value.Prop(raw, key).(string); ok {
			return &s
		}
		return nil
	}
	check := Check{Run: text("run"), Fails: text("fails")}
	if after := text("after"); after != nil {
		check.After = *after
	}
	if cost := text("cost"); cost != nil {
		check.Cost = *cost
	}
	if timeout, ok := value.Prop(raw, "timeout").(float64); ok {
		check.Timeout = &timeout
	}
	check.Prose = value.Prop(raw, "prose") == true
	return check, check.Run != nil || check.Fails != nil
}
