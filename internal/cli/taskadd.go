package cli

// task add (slice 55, features/task.feature): a task is added by a command,
// as registry items are (workedit.go). It writes the task at the end of its
// group's ledger file (ledger.Add) and its item, of kind task, todo and
// nobody's, with no why (the ledger entry holds it), at the end of the registry (work.Add), and commits the two files
// alone, "docs: add <id>", through writeCommitted, which puts both back if
// the commit is refused.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/work"
)

// taskAddUsage is what task add needs, as a usage error says it.
const taskAddUsage = "task add needs --group <g>, --type <type>, --title <title>, --why <why> and a --check <command>"

// taskAddArgs reads task add's arguments: --group (or --phase, or
// --<ledger.group.label>), --type, --title and --why, each once; --check,
// once per check, in order, and --timeout after a --check, that check's
// seconds. "--flag value" or "--flag=value". Anything else is a usage
// error.
func taskAddArgs(cfg *config.Loaded, args []string) (ledger.NewTask, error) {
	var n ledger.NewTask
	var ids []string
	once := map[string]*string{"--type": &n.Type, "--title": &n.Title, "--why": &n.Why}
	for _, f := range groupFlags(cfg) {
		once[f] = &n.Group
	}
	seen := map[*string]bool{}
	timed := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			ids = append(ids, arg)
			continue
		}
		name, v, joined := strings.Cut(arg, "=")
		field, single := once[name]
		if !single && name != "--check" && name != "--timeout" {
			return n, usage("task add does not take %s", name)
		}
		if !joined {
			if i+1 >= len(args) {
				return n, usage("task add %s needs a value", name)
			}
			i++
			v = args[i]
		}
		switch {
		case single:
			if seen[field] || strings.TrimSpace(v) == "" {
				return n, usage("task add takes one %s with a value", name)
			}
			seen[field], *field = true, v
		default:
			if err := checkArg("task add", &n.Checks, &timed, name, v); err != nil {
				return n, err
			}
		}
	}
	if len(ids) > 0 {
		return n, usage("task add mints the task id; do not pass one")
	}
	if n.Group == "" || n.Type == "" || n.Title == "" || n.Why == "" || len(n.Checks) == 0 {
		return n, usage(taskAddUsage)
	}
	return n, nil
}

// checkArg reads one --check or --timeout of a command that writes a
// task's checks, task add or work promote (cmd, as its usage errors name
// it): a --check's command, never blank, is the next check; a --timeout,
// once after a --check, is that check's seconds, a number above 0. timed is
// whether the last check has its timeout.
func checkArg(cmd string, checks *[]ledger.NewCheck, timed *bool, name, v string) error {
	if name == "--check" {
		if strings.TrimSpace(v) == "" {
			return usage("%s --check needs a command", cmd)
		}
		*checks, *timed = append(*checks, ledger.NewCheck{Run: v}), false
		return nil
	}
	seconds, err := strconv.ParseFloat(v, 64)
	if len(*checks) == 0 || *timed || err != nil || seconds <= 0 {
		return usage("%s takes one --timeout <seconds> after a --check, a number above 0, for that check", cmd)
	}
	(*checks)[len(*checks)-1].Timeout, *timed = &seconds, true
	return nil
}

// taskAdd is `task add --group <g> --type <type> --title <title> --why
// <why> --check <command> [--timeout <seconds>] [--check …]`: the task at the
// end of its group's ledger file, its item in the registry, committed.
func taskAdd(args []string, o Out) (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	n, err := taskAddArgs(cfg, args)
	if err != nil {
		return 0, err
	}
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	n.ID, err = newItemID(cfg, "task", itemIDs(registry), o.reserved, o.OS)
	if err != nil {
		return 0, err
	}
	added, found, err := ledger.Add(cfg, n)
	if err != nil {
		return 0, err
	}
	if len(found) > 0 {
		return refuseWork(found, ExitPolicy, o)
	}
	// The why is the ledger entry's alone, the registry being an index
	// (slice 76, bug-107).
	item := work.New{ID: n.ID, Title: n.Title, Kind: "task", Phase: n.Group, DependsOn: []string{}}
	change, found, err := work.Add(cfg, registry, text, item, ledger.IDPattern(cfg))
	if err != nil {
		return 0, uneditable(cfg.Work.Registry, err)
	}
	if len(found) > 0 {
		return refuseWork(found, ExitPolicy, o)
	}
	body := fmt.Sprintf("Add the task %s (%s) to %s, and its item to %s, with itos task add.",
		n.ID, value.JSON(n.Title), added.Path, cfg.Work.Registry)
	files := []written{
		{path: added.Path, old: added.Old, text: added.Text, created: added.Created},
		{path: cfg.Work.Registry, old: text, text: change.Text},
	}
	sha, code, err := writeCommitted(cfg, files, change.Header, body, o)
	if err != nil || code != 0 {
		return code, err
	}
	if o.JSON {
		var commit any
		if sha != "" {
			commit = sha
		}
		return 0, out.Emit(o.Stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "task", Value: added.Task},
			out.Field{Key: "file", Value: added.Path}, out.Field{Key: "item", Value: change.Item}, out.Field{Key: "commit", Value: commit})
	}
	if o.Quiet {
		return 0, nil
	}
	result := "the ledger and the registry are the stealth config's, so nothing is committed"
	if sha != "" {
		result = committed(sha, change.Header)
	}
	where := added.Path
	if added.Created {
		where = "a new " + where
	}
	fmt.Fprintf(o.Stdout, "%s is a new task in %s, and an item in %s: %s\n", n.ID, where, cfg.Work.Registry, result)
	return 0, nil
}
