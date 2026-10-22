package cli

// work promote (slices 52, 65 and 138, features/work.feature): an idea made
// a slice or a task under a minted id. A task's promotion writes its ledger
// entry too, so the id works in a Task footer from the commit that makes it
// (#54): the idea's group, its title and why unless the line gives others,
// and the line's type and checks, read as task add reads them. Everything
// the command can refuse is judged before the id is reserved, since a
// reserved id is never given back (ADR-0045): the line, the registry, the
// idea, the questions, the task's ledger entry and the files the commit
// would sweep in. The ledger, the registry and any question renamed are then
// committed together (writeCommitted), or put back when the commit fails.

import (
	"fmt"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/work"
)

var promoteKinds = []string{"slice", "task"}

// promotion is work promote's line: the idea, the kind it becomes, the new
// title ("" to keep the idea's) and, for a task, its why ("" for the
// idea's), its type and its checks in order.
type promotion struct {
	id, kind, title, why, taskType string
	checks                         []ledger.NewCheck
}

// promoteTaskOnly are the flags only a task's promotion takes.
var promoteTaskOnly = []string{"--why", "--type", "--check", "--timeout"}

// workPromote is `work promote <idea> --kind slice [--title <title>]` and
// `work promote <idea> --kind task --type <type> --check <command>
// [--timeout <seconds>]… [--title <title>] [--why <why>]`: the idea renamed,
// given a minted ID and the kind (and the title, when given), every
// depends_on and question (work.asks, bug 105) naming it renamed too, and,
// for a task, its ledger entry written (slice 138); then the registry, the
// ledger file and the questions that changed committed together.
func workPromote(args []string, o Out) (int, error) {
	p, err := workPromoteLine(args)
	if err != nil {
		return 0, err
	}
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	idea, problem := work.Promotable(registry, p.id)
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}
	asks, old, problem, err := promotionAsks(cfg, p.id)
	if err != nil {
		return 0, err
	}
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}
	files := []written{{path: cfg.Work.Registry, old: text}}
	if named := asks.Rename(p.id, p.id); len(named) > 0 {
		files = append(files, written{path: cfg.Work.Asks, old: old})
	}
	var task ledger.NewTask
	if p.kind == "task" {
		var added ledger.Added
		var found []out.Problem
		if task, added, found, err = promotedTask(cfg, registry, idea, p); err != nil || len(found) > 0 {
			return stopped(found, err, o)
		}
		files = append([]written{{path: added.Path, old: added.Old, created: added.Created}}, files...)
	}
	if problem, err := commitReady(cfg, files); err != nil {
		return 0, err
	} else if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}

	newID, err := newItemID(cfg, p.kind, itemIDs(registry), o.reserved, o.OS)
	if err != nil {
		return 0, err
	}
	change, problem, err := work.Promote(registry, text, p.id, newID, p.kind, p.title, ledger.IDPattern(cfg))
	if err != nil {
		return 0, uneditable(cfg.Work.Registry, err)
	}
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}
	body := strings.TrimSuffix(change.Body, ".")
	var added ledger.Added
	extra := []out.Field{{Key: "was", Value: p.id}, {Key: "rewritten", Value: anys(change.Rewritten)}}
	if p.kind == "task" {
		task.ID = newID
		var found []out.Problem
		if added, found, err = ledger.Add(cfg, task); err != nil || len(found) > 0 {
			return stopped(found, err, o)
		}
		files[0].text = added.Text
		body += ", and write its ledger entry, with its why and checks, in " + added.Path
		extra = append(extra, out.Field{Key: "task", Value: added.Task}, out.Field{Key: "file", Value: added.Path})
	}
	registryAt := slices.IndexFunc(files, func(f written) bool { return f.path == cfg.Work.Registry })
	files[registryAt].text = change.Text
	renamed := asks.Rename(p.id, newID)
	if len(renamed) > 0 {
		files[len(files)-1].text = ask.Text(asks)
		body += ", and rename it in " + and(renamed) + " of " + cfg.Work.Asks
	}
	sha, code, err := writeCommitted(cfg, files, change.Header, body+".", o)
	if err != nil || code != 0 {
		return code, err
	}
	line := fmt.Sprintf("%s is the %s %s", p.id, p.kind, newID)
	if p.kind == "task" {
		line += " of " + added.Path
	}
	if namedBy := append(slices.Clone(change.Rewritten), renamed...); len(namedBy) > 0 {
		line += ", named now by " + strings.Join(namedBy, ", ")
	}
	// Only when a question named the idea, so a promote without one reports
	// as it did.
	if len(renamed) > 0 {
		extra = append(extra, out.Field{Key: "questions", Value: anys(renamed)})
	}
	result := committed(sha, change.Header)
	if sha == "" && p.kind == "task" {
		result = "the ledger and the registry are the stealth config's, so nothing is committed"
	}
	return reportWork(line+": "+result, change, sha, extra, o)
}

// stopped is how a step that gives an error or problems ends work promote:
// the error as it is, else the problems refused (exit 1). The task's ledger
// entry, judged before the id is reserved and written after it, ends both
// ways through it (T-189).
func stopped(found []out.Problem, err error, o Out) (int, error) {
	if err != nil {
		return 0, err
	}
	return refuseWork(found, ExitPolicy, o)
}

// anys is the strings as a list out.Emit writes.
func anys(names []string) []any {
	list := make([]any, len(names))
	for i, n := range names {
		list[i] = n
	}
	return list
}

// promotionAsks is the questions' file, work.asks, and its text, read before
// an id is minted; an empty file when it is not there. The problem, which
// refuses the promote, is a file that cannot be read as itos decision writes
// it.
func promotionAsks(cfg *config.Loaded, id string) (ask.File, string, *out.Problem, error) {
	f, old, there, err := loadAsks(cfg)
	if err != nil && there {
		return ask.File{}, "", &out.Problem{
			Rule:    "asks-invalid",
			Message: err.Error(),
			Fix:     "correct " + cfg.Work.Asks + " to what itos decision writes, then promote " + id + " again",
		}, nil
	}
	return f, old, nil, err
}

// promotedTask is the ledger entry a task's promotion writes, judged by
// ledger.Add under the id the ledger would give next (the counter not yet
// asked), so what it refuses is refused before an id is reserved: a type
// commits.types does not list, a ledger with problems, a group ledger.files
// cannot name. The task is the idea's group, its title and why unless the
// line gives others, and the line's type and checks; an idea with no group
// or no why, and no --why, is refused too.
func promotedTask(cfg *config.Loaded, registry work.Registry, idea *value.Map, p promotion) (ledger.NewTask, ledger.Added, []out.Problem, error) {
	refuse := func(rule, message, fix string) (ledger.NewTask, ledger.Added, []out.Problem, error) {
		return ledger.NewTask{}, ledger.Added{}, []out.Problem{{Rule: rule, Message: message, Fix: fix}}, nil
	}
	n := ledger.NewTask{Type: p.taskType, Title: p.title, Why: p.why, Checks: p.checks}
	switch phase := idea.At("phase").(type) {
	case string:
		n.Group = phase
	case float64:
		n.Group = value.Number(phase)
	default:
		label := cfg.Ledger.Group.Label
		return refuse("work-promote-no-group", fmt.Sprintf("%s names no %s, so its task has no ledger file", p.id, label),
			"give "+p.id+" a "+label+" in "+cfg.Work.Registry+", then promote it again")
	}
	if n.Title == "" {
		n.Title = value.String(idea.At("title"))
	}
	if n.Why == "" {
		why, _ := idea.At("why").(string)
		n.Why = value.Trim(why)
	}
	if n.Why == "" {
		return refuse("work-promote-no-why", p.id+" has no why for its task's ledger entry",
			"pass --why <why>")
	}
	var err error
	if n.ID, err = ledger.NextID(cfg, itemIDs(registry)); err != nil {
		return ledger.NewTask{}, ledger.Added{}, nil, err
	}
	added, found, err := ledger.Add(cfg, n)
	return n, added, found, err
}

// workPromoteLine reads work promote's arguments: the idea, the kind it
// becomes and its flags, each once but --check, given once per check in
// order, and --timeout, once after a --check, that check's seconds
// (checkArg, as task add reads them). A line the command refuses is a usage
// error: no kind, an --id (the id is minted), a flag only a task's
// promotion takes given for a slice, and a task with no type or no check.
func workPromoteLine(args []string) (promotion, error) {
	type flag struct{ name, value string }
	var flags []flag
	var ids []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			ids = append(ids, arg)
			continue
		}
		name, v, joined := strings.Cut(arg, "=")
		if name != "--id" && name != "--kind" && name != "--title" && !slices.Contains(promoteTaskOnly, name) {
			return promotion{}, usage("work promote does not take %s", name)
		}
		if !joined {
			if i+1 >= len(args) {
				return promotion{}, usage("work promote %s needs a value", name)
			}
			i++
			v = args[i]
		}
		flags = append(flags, flag{name, v})
	}
	if len(ids) != 1 {
		return promotion{}, usage("work promote needs one <id>")
	}
	p := promotion{id: ids[0]}
	var id string
	once := map[string]*string{"--id": &id, "--kind": &p.kind, "--title": &p.title, "--why": &p.why, "--type": &p.taskType}
	seen := map[string]bool{}
	for _, f := range flags {
		if field, single := once[f.name]; single {
			if seen[f.name] || strings.TrimSpace(f.value) == "" {
				return promotion{}, usage("work promote takes one %s with a value", f.name)
			}
			seen[f.name], *field = true, f.value
		}
	}
	if !slices.Contains(promoteKinds, p.kind) {
		return promotion{}, usage("work promote needs --kind %s", strings.Join(promoteKinds, "|"))
	}
	if id != "" {
		return promotion{}, usage("work promote mints the %s id; do not pass --id", p.kind)
	}
	timed := false
	for _, f := range flags {
		switch {
		case p.kind == "slice" && slices.Contains(promoteTaskOnly, f.name):
			return promotion{}, usage("work promote --kind slice does not take %s, which only a task's promotion takes", f.name)
		case f.name == "--check" || f.name == "--timeout":
			if err := checkArg("work promote", &p.checks, &timed, f.name, f.value); err != nil {
				return promotion{}, err
			}
		}
	}
	if p.kind == "task" && (p.taskType == "" || len(p.checks) == 0) {
		return promotion{}, usage("work promote --kind task needs --type <type> and a --check <command>")
	}
	return p, nil
}
