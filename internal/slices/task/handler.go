package task

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/engine/ids"
	"github.com/donvargax/itos/v8/internal/engine/record"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/work"
)

// Query is what task asks: Sub, "" to run the tasks' checks, or list, add
// or next-id; Args, the arguments after it, every one for "", read by the
// group flags the config names; and ToStderr, whether a verbose check's
// command line and output go to stderr, as under --json, which keeps
// stdout for its one object.
type Query struct {
	Sub      string
	Args     []string
	ToStderr bool
}

// Ports are what task reaches the world through, which internal/cli binds.
// Config is the config, its data read from the working tree. task add
// writes as work's commands do, through their code: Sound is the work
// registry read sound, its lock held; ItemID the new task's id, the one a
// draft reserved for the command, else one minted past registryIDs;
// Tracker the tracker store the config's documents are in, which the
// ledger's file is read from; and Write writes the files and, in a
// project, commits them alone with header and body, giving the commit, ""
// under a stealth config. A registry that is not sound, a file with changes
// no commit holds and a commit refused are reported by Sound and Write
// themselves, the exit their Code.
type Ports interface {
	Config(ctx context.Context) (*config.Loaded, error)
	// Evidence is the engine that looks up the checks that opt in to evidence
	// records by the index tree, bound to the repository and its store; a
	// run takes it unless --working-tree asks for the working folder.
	Evidence(ctx context.Context, cfg *config.Loaded) *check.Evidence
	Sound(ctx context.Context) (Sound, error)
	ItemID(ctx context.Context, cfg *config.Loaded, registryIDs []string) (string, error)
	Tracker(ctx context.Context, cfg *config.Loaded) trackerport.Tracker
	Write(ctx context.Context, cfg *config.Loaded, files []record.File, header, body string) (sha string, code int, err error)
}

// Sound is the work registry as Ports.Sound read it: the config, the
// registry and its text, and Release, never nil, which gives back the lock
// held from the read to the write. A nil Cfg is a registry not sound, its
// problems reported, Code the exit.
type Sound struct {
	Cfg      *config.Loaded
	Registry work.Registry
	Text     string
	Release  func()
	Code     int
}

// Handler is task's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Answer is what Handle found or did. Running the checks: the Unknown IDs
// the ledger's folder Dir lacks, or Unmatched, no task selected, the
// config's group Label naming its flag; else the Rows shown and whether a
// task is Failing. task list: the Tasks, their Statuses in the Registry,
// and NoRegistry when it is not there. task next-id: NextID. task add:
// Release, nil until the registry's lock is held, then for the caller to
// defer; Code, a refusal Ports reported; the Problems found, nothing
// written; and Added, the task written.
type Answer struct {
	Unknown   []string
	Dir       string
	Unmatched bool
	Label     string
	Rows      []taskRow
	Failing   bool
	// Evidence is the engine the checks were looked up through, nil under
	// --working-tree; it says whether any opted-in check was and how many
	// changed paths the run left out.
	Evidence *check.Evidence

	Tasks      []ledger.Task
	Statuses   work.Statuses
	Registry   string
	NoRegistry bool

	NextID string

	Release  func()
	Code     int
	Problems []out.Problem
	Added    *Added
}

// Added is the task task add wrote: its ID, the Task as the ledger reads
// it, the ledger File, whether it was Created, the registry's Item, and the
// Commit, "" under a stealth config, with its Header.
type Added struct {
	ID      string
	Task    *value.Map
	File    string
	Created bool
	Item    *value.Map
	Commit  string
	Header  string
}

// Handle answers q. Running the checks, each check's command line and
// output go to stdout and stderr as it runs, verbose for one task.
func (h Handler) Handle(ctx context.Context, q Query, stdout, stderr io.Writer) (Answer, error) {
	switch q.Sub {
	case "list":
		return h.list(ctx, q.Args)
	case "add":
		return h.add(ctx, q.Args)
	case "next-id":
		return h.nextID(ctx)
	}
	return h.run(ctx, q, stdout, stderr)
}

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

// judgeTask runs a task's checks in written order, every one of them whatever
// the one before found: failing when one fails, pending when one waits for
// the push, else done. A task with none is done when its work item is
// (closed, work done having taken its checks out of the ledger, slice 101),
// review otherwise: no item, or one not done.
func judgeTask(task ledger.Task, runner *check.Runner, statuses work.Statuses) (taskStatus, []check.Result, []*check.EvidenceInfo) {
	if len(task.DoneWhen) == 0 {
		if status, _ := statuses.Of(task.ID); status == "done" {
			return statusDone, nil, nil
		}
		return statusReview, nil, nil
	}
	if runner.Verbose {
		log := runner.Stdout
		if runner.ToStderr {
			log = runner.Stderr
		}
		fmt.Fprintf(log, "\n%s %s\n", task.ID, task.Title)
	}
	results := make([]check.Result, len(task.DoneWhen))
	infos := make([]*check.EvidenceInfo, len(task.DoneWhen))
	for i, c := range task.DoneWhen {
		results[i], infos[i] = runner.RunInfo(c)
	}
	switch {
	case slices.Contains(results, check.Fail):
		return statusFailing, results, infos
	case slices.Contains(results, check.Pending):
		return statusPending, results, infos
	}
	return statusDone, results, infos
}

// groupFlag is the group the first of the group flags names, in their order
// rather than the command line's, and whether one does.
func groupFlag(cfg *config.Loaded, args []string) (string, bool) {
	for _, f := range ledger.GroupFlags(cfg) {
		if v, ok := argsengine.FlagValue(args, f); ok {
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
	if ids, ok := argsengine.FlagValue(args, "--skip"); ok {
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
// have: the arguments that are a task's ID (ledger.IDPattern: ledger.id, or,
// with none, every word that is no flag, slice 130) and are not the value of
// a group flag or --skip.
func unknownIDs(cfg *config.Loaded, args []string, tasks []ledger.Task) []string {
	pattern := ledger.IDPattern(cfg)
	values := map[string]bool{}
	for _, f := range append(ledger.GroupFlags(cfg), "--skip") {
		if v, ok := argsengine.FlagValue(args, f); ok {
			values[v] = true
		}
	}
	var unknown []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") || !pattern.MatchString(a) || values[a] {
			continue
		}
		if !slices.ContainsFunc(tasks, func(t ledger.Task) bool { return t.ID == a }) {
			unknown = append(unknown, a)
		}
	}
	return unknown
}

type taskRow struct {
	task    ledger.Task
	status  taskStatus
	results []check.Result
	// evidence is the evidence of each check that opted in, nil for one that
	// did not.
	evidence []*check.EvidenceInfo
}

// run is `task <id>…`: runs the tasks' checks in written order, each
// distinct check once, verbose for one task, reading the work registry's
// statuses (work.ItemStatuses) for the tasks with no checks.
func (h Handler) run(ctx context.Context, q Query, stdout, stderr io.Writer) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	// The ledger's layout, which reading its tasks reads first, giving the
	// same error.
	layout, err := ledger.LayoutOf(cfg)
	if err != nil {
		return Answer{}, err
	}
	tasks, err := ledger.Tasks(cfg)
	if err != nil {
		return Answer{}, err
	}
	if unknown := unknownIDs(cfg, q.Args, tasks); len(unknown) > 0 {
		return Answer{Unknown: unknown, Dir: layout.Dir}, nil
	}
	selected := selectTasks(cfg, q.Args, tasks)
	if len(selected) == 0 {
		return Answer{Unmatched: true, Label: cfg.Ledger.Group.Label}, nil
	}
	runner := check.NewRunner(cfg, stdout, stderr)
	runner.Verbose, runner.ToStderr = len(selected) == 1, q.ToStderr
	var a Answer
	if !slices.Contains(q.Args, "--working-tree") {
		runner.Evidence = h.ports.Evidence(ctx, cfg)
		a.Evidence = runner.Evidence
	}
	for _, t := range selected {
		if t.Err != nil {
			return Answer{}, t.Err
		}
	}
	statuses := work.ItemStatuses(cfg.Source, cfg.Work.Registry)
	rows := make([]taskRow, len(selected))
	for i, t := range selected {
		status, results, infos := judgeTask(t, runner, statuses)
		rows[i] = taskRow{t, status, results, infos}
		if status == statusFailing {
			a.Failing = true
		}
	}
	a.Rows = rows
	if slices.Contains(q.Args, "--pending") {
		a.Rows = slices.DeleteFunc(slices.Clone(rows), func(r taskRow) bool { return r.status == statusDone })
	}
	return a, nil
}

// list is `task list [--group <g>]`: the tasks, each with its work item's
// status in the registry (the item whose id is the task's); it runs
// nothing, and reads the registry alone, without the people, saying when
// it is not where itos looks.
func (h Handler) list(ctx context.Context, args []string) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	all, err := ledger.Tasks(cfg)
	if err != nil {
		return Answer{}, err
	}
	group, byGroup := groupFlag(cfg, args)
	tasks := slices.DeleteFunc(all, func(t ledger.Task) bool { return byGroup && !inGroup(cfg, t, group) })
	registry := cfg.Work.Registry
	return Answer{
		Tasks:      tasks,
		Registry:   registry,
		NoRegistry: !cfg.Source.Has(ctx, registry),
		Statuses:   work.ItemStatuses(cfg.Source, registry),
	}, nil
}

// nextID is `task next-id`: the ledger's next task ID (ledger.NextID),
// counting the registry's ids that ledger.id matches too (ids.Registry).
func (h Handler) nextID(ctx context.Context) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	others, err := ids.Registry(ctx, cfg)
	if err != nil {
		return Answer{}, err
	}
	next, err := ledger.NextID(cfg, others)
	return Answer{NextID: next}, err
}

// add is `task add --group <g> --type <type> --title <title> --why <why>
// --check <command> [--timeout <seconds>] [--check …]` (slice 55,
// features/task.feature): a task is added by a command, as registry items
// are. It writes the task at the end of its group's ledger file
// (ledger.Add) and its item, of kind task, todo and nobody's, with no why
// (the ledger entry holds it), at the end of the registry (work.Add), and
// commits the two files alone, "docs: add <id>", through Write, which puts
// both back if the commit is refused.
func (h Handler) add(ctx context.Context, args []string) (a Answer, err error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return a, err
	}
	n, err := AddArgs(cfg, args)
	if err != nil {
		return a, err
	}
	s, err := h.ports.Sound(ctx)
	a.Release = s.Release
	if s.Cfg == nil {
		a.Code = s.Code
		return a, err
	}
	cfg = s.Cfg
	n.ID, err = h.ports.ItemID(ctx, cfg, ids.Items(s.Registry))
	if err != nil {
		return a, err
	}
	added, found, err := ledger.Add(cfg, n, h.ports.Tracker(ctx, cfg).Load)
	if err != nil || len(found) > 0 {
		a.Problems = found
		return a, err
	}
	// The why is the ledger entry's alone, the registry being an index
	// (slice 76, bug-107).
	item := work.New{ID: n.ID, Title: n.Title, Kind: "task", Phase: n.Group, DependsOn: []string{}}
	change, found, err := work.Add(cfg, s.Registry, s.Text, item, ledger.IDPattern(cfg))
	if err != nil {
		return a, kind.Wrap(kind.Usage, fmt.Errorf("%s cannot be edited in place: %w", cfg.Work.Registry, err))
	}
	if len(found) > 0 {
		a.Problems = found
		return a, nil
	}
	body := fmt.Sprintf("Add the task %s (%s) to %s, and its item to %s, with itos task add.",
		n.ID, value.JSON(n.Title), added.Path, cfg.Work.Registry)
	files := []record.File{
		{Path: added.Path, Old: added.Old, Text: added.Text, Created: added.Created},
		{Path: cfg.Work.Registry, Old: s.Text, Text: change.Text},
	}
	sha, code, err := h.ports.Write(ctx, cfg, files, change.Header, body)
	if err != nil || code != 0 {
		a.Code = code
		return a, err
	}
	a.Registry = cfg.Work.Registry
	a.Added = &Added{ID: n.ID, Task: added.Task, File: added.Path, Created: added.Created, Item: change.Item, Commit: sha, Header: change.Header}
	return a, nil
}
