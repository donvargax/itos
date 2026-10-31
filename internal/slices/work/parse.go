package work

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// CLI is work's command line: the Query it asks, and how it answers,
// --json's one object or, under --quiet, no line for a change.
type CLI struct {
	Query
	JSON, Quiet bool
}

// Query is what work asks. Sub is "" for the proposal, else the subcommand.
// As is the handle --as names (the proposal, take, give-back). All and Tag
// are work list's --all and --tag. File is the file work check judges, as
// typed from where itos was started, Named whether one was given. ID is the
// one item show, take, done and add (an idea's) name, Patch show's --patch,
// and JSON whether show answers as --json's object, which reads each
// commit's patch on its own. IDs are the items edit, queue, drop, defer,
// resume and give-back name, in the order given; Why their reason; Place
// where queue puts them; Edits what edit changes. New is the item add
// makes, and Promotion the idea promote makes a slice or a task. Expect is
// take's and done's --expect, the revision itos next gave the caller, ""
// for none.
type Query struct {
	Sub       string
	As        string
	All       bool
	Tag       string
	File      string
	Named     bool
	ID        string
	Expect    string
	Patch     bool
	JSON      bool
	IDs       []string
	Why       string
	Place     domain.Place
	Edits     domain.Edits
	New       domain.New
	Promotion Promotion
}

// Promotion is work promote's line: the idea, the Kind it becomes, the new
// Title ("" to keep the idea's) and, for a task, its Why ("" for the
// idea's), its Type and its Checks in order.
type Promotion struct {
	ID, Kind, Title, Why, Type string
	Checks                     []ledger.NewCheck
}

// takes is what work takes, as a usage error names it.
const takes = "it takes list, show, take, promote, done, add, edit, queue, drop, defer, resume, give-back or check, else --as <handle>"

// addKinds are the kinds work add makes.
var addKinds = []string{"idea", "slice", "task", "bug"}

// promoteKinds are what work promote makes of an idea.
var promoteKinds = []string{"slice", "task"}

// promoteTaskOnly are the flags only a task's promotion takes.
var promoteTaskOnly = []string{"--why", "--type", "--check", "--timeout"}

// Parse reads the arguments internal/cli's flag spec hands the work
// command, with where itos was started (origin, which work check's file is
// read from) and the global --json and --quiet. Every error is a usage
// error (out.Usage), given before anything is read; an argument work does
// not know is one, never ignored (bug 11).
func Parse(args []string, origin string, json, quiet bool) (CLI, error) {
	sub, rest := argsengine.Split(args)
	var q Query
	var err error
	switch sub {
	case "list":
		q, err = parseList(rest)
	case "check":
		if len(rest) > 1 {
			return CLI{}, out.Usage("work check takes at most one file, not %s", rest[1])
		}
		file, named := argsengine.First(rest)
		q = Query{File: typed(origin, file), Named: named}
	case "show":
		q, err = parseShow(rest)
	case "take":
		q.ID, q.As, q.Expect, err = parseTake(rest)
	case "done":
		q.ID, q.Expect, err = parseDone(rest)
	case "promote":
		q.Promotion, err = PromoteLine(rest)
	case "add":
		q.New, err = AddLine(rest)
	case "edit":
		q, err = parseEdit(rest)
	case "queue":
		q, err = parseQueue(rest)
	case "drop", "defer", "give-back":
		q, err = parseWhy(sub, rest)
	case "resume":
		q.IDs, _, err = someIDs("resume", rest, nil)
	default:
		sub = ""
		q.As, err = parseProposal(args)
	}
	if err != nil {
		return CLI{}, err
	}
	q.Sub, q.JSON = sub, json
	return CLI{Query: q, JSON: json, Quiet: quiet}, nil
}

// parseProposal is `work [--as <handle>]`, which takes nothing else.
func parseProposal(args []string) (string, error) {
	as := ""
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--as":
			if i+1 >= len(args) || args[i+1] == "" {
				return "", out.Usage("work --as needs a handle")
			}
			i++
			as = args[i]
		case strings.HasPrefix(arg, "--as="):
			if as = strings.TrimPrefix(arg, "--as="); as == "" {
				return "", out.Usage("work --as needs a handle")
			}
		case strings.HasPrefix(arg, "-"):
			return "", out.Usage("work does not take %s: %s", arg, takes)
		default:
			return "", out.Usage("work has no subcommand %s: %s", arg, takes)
		}
	}
	return as, nil
}

// parseList is `work list [--all] [--tag <tag>]`, which takes nothing else.
func parseList(args []string) (Query, error) {
	others, values, set, err := argsengine.Sub("work", "list", args, []string{"--tag"}, []string{"--all"})
	if err != nil {
		return Query{}, err
	}
	if len(others) > 0 {
		return Query{}, out.Usage("work list takes only --all and --tag <tag>: %s", strings.Join(others, " "))
	}
	return Query{All: set["--all"], Tag: values["--tag"]}, nil
}

// parseShow is `work show <id> [--patch]`.
func parseShow(args []string) (Query, error) {
	var ids []string
	patch := false
	for _, arg := range args {
		switch {
		case arg == "--patch":
			patch = true
		case strings.HasPrefix(arg, "-"):
			return Query{}, out.Usage("work show does not take %s", arg)
		default:
			ids = append(ids, arg)
		}
	}
	if len(ids) != 1 {
		return Query{}, out.Usage("work show needs one <id>")
	}
	return Query{ID: ids[0], Patch: patch}, nil
}

// parseTake is `work take <id> [--as <handle>] [--expect <revision>]`: the
// id, the handle and the revision.
func parseTake(args []string) (id, as, expect string, err error) {
	id, flags, err := oneID("take", args, nil, "--as", "--expect")
	if err != nil {
		return "", "", "", err
	}
	if expect, err = expected("take", flags["--expect"]); err != nil {
		return "", "", "", err
	}
	return id, flags["--as"], expect, nil
}

// parseDone is `work done <id> [--expect <revision>]`: the id and the
// revision.
func parseDone(args []string) (id, expect string, err error) {
	id, flags, err := oneID("done", args, nil, "--expect")
	if err != nil {
		return "", "", err
	}
	if expect, err = expected("done", flags["--expect"]); err != nil {
		return "", "", err
	}
	return id, expect, nil
}

// revisionForm is the shape of a revision itos next gives (next.Revision):
// "sha256:" and the digest's 64 lowercase hex digits.
var revisionForm = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// expected is --expect's revision, "" for none: one not of revisionForm is
// a usage error naming the form (slice 208), before anything is read.
func expected(sub, revision string) (string, error) {
	if revision != "" && !revisionForm.MatchString(revision) {
		return "", out.Usage("work %s --expect takes the revision itos next gives, \"sha256:\" and 64 lowercase hex digits, not %s", sub, revision)
	}
	return revision, nil
}

// parseEdit is `work edit <id>… [--title <title>] [--depends-on <ids>]
// [--refs <refs>] [--tags <tags>] [--note <text>]`: at least one of them,
// and --title for one id alone. An empty --depends-on, --refs or --tags
// empties the list.
func parseEdit(args []string) (Query, error) {
	ids, flags, err := someIDs("edit", args, []string{"--depends-on", "--refs", "--tags"}, "--title", "--depends-on", "--refs", "--tags", "--note")
	if err != nil {
		return Query{}, err
	}
	if len(flags) == 0 {
		return Query{}, out.Usage("work edit needs --title, --depends-on, --refs, --tags or --note")
	}
	if _, ok := flags["--title"]; ok && len(ids) > 1 {
		return Query{}, out.Usage("work edit --title is one item's own: give it one <id>, not %d", len(ids))
	}
	var e domain.Edits
	if title, ok := flags["--title"]; ok {
		e.Title = &title
	}
	if deps, ok := flags["--depends-on"]; ok {
		list := idList(deps)
		e.DependsOn = &list
	}
	if refs, ok := flags["--refs"]; ok {
		list := idList(refs)
		e.Refs = &list
	}
	if tags, ok := flags["--tags"]; ok {
		list := idList(tags)
		e.Tags = &list
	}
	e.Note = flags["--note"]
	return Query{IDs: ids, Edits: e}, nil
}

// parseQueue is `work queue <id>… --top | --before <id> | --after <id> |
// --remove`: exactly one place.
func parseQueue(args []string) (Query, error) {
	var rest []string
	top, remove := false, false
	for _, arg := range args {
		switch arg {
		case "--top":
			top = true
		case "--remove":
			remove = true
		default:
			rest = append(rest, arg)
		}
	}
	ids, flags, err := someIDs("queue", rest, nil, "--before", "--after")
	if err != nil {
		return Query{}, err
	}
	at := domain.Place{Top: top, Remove: remove, Before: flags["--before"], After: flags["--after"]}
	given := 0
	for _, on := range []bool{at.Top, at.Remove, at.Before != "", at.After != ""} {
		if on {
			given++
		}
	}
	if given != 1 {
		return Query{}, out.Usage("work queue needs one of --top, --before <id>, --after <id> or --remove")
	}
	return Query{IDs: ids, Place: at}, nil
}

// whyNeeded is what drop, defer and give-back say of a --why not given.
var whyNeeded = map[string]string{
	"drop":      "the commit's record of why the item is dropped",
	"defer":     "why the item is put off, which its deferred key holds",
	"give-back": "the commit's record of why the item is given back",
}

// parseWhy is `work drop <id>… --why <reason>`, `work defer <id>… --why
// <reason>` and `work give-back <id>… --why <reason> [--as <handle>]`: a
// reason is required.
func parseWhy(sub string, args []string) (Query, error) {
	flags := []string{"--why"}
	if sub == "give-back" {
		flags = append(flags, "--as")
	}
	ids, values, err := someIDs(sub, args, nil, flags...)
	if err != nil {
		return Query{}, err
	}
	if strings.TrimSpace(values["--why"]) == "" {
		return Query{}, out.Usage("work %s needs --why <reason>, %s", sub, whyNeeded[sub])
	}
	return Query{IDs: ids, Why: values["--why"], As: values["--as"]}, nil
}

// AddLine reads work add's arguments, `[<idea-id>] --title <t> --why <w>
// [--kind idea|slice|task|bug] [--phase <p>] [--owner <handle>]
// [--depends-on <ids>] [--refs <refs>] [--tags <tags>]`: the item, its id
// "" for a kind whose id is minted, idea when no --kind is given. A line
// the command refuses is a usage error, a caller's id for a minted kind
// among them.
func AddLine(args []string) (domain.New, error) {
	ids, flags, err := parsed("add", args, nil, "--title", "--why", "--kind", "--phase", "--owner", "--depends-on", "--refs", "--tags")
	if err != nil {
		return domain.New{}, err
	}
	if len(ids) > 1 {
		return domain.New{}, out.Usage("work add takes at most one <id>")
	}
	// Ideas keep their author's id; a numbered kind's is minted.
	hasID := len(ids) == 1
	var id string
	if hasID {
		id = ids[0]
	}
	if flags["--title"] == "" || strings.TrimSpace(flags["--why"]) == "" {
		return domain.New{}, out.Usage("work add needs --title <title> and --why <why>")
	}
	kind := flags["--kind"]
	if kind == "" {
		kind = "idea"
	}
	switch {
	case !slices.Contains(addKinds, kind):
		return domain.New{}, out.Usage("work add takes --kind %s", strings.Join(addKinds, "|"))
	case kind == "idea" && !hasID:
		return domain.New{}, out.Usage("work add needs <idea-id> when --kind is idea")
	case kind != "idea" && hasID:
		return domain.New{}, out.Usage("work add mints the %s id; do not pass one", kind)
	}
	return domain.New{
		ID: id, Title: flags["--title"], Why: flags["--why"], Kind: kind, Phase: flags["--phase"], Owner: flags["--owner"],
		DependsOn: idList(flags["--depends-on"]), Refs: idList(flags["--refs"]), Tags: idList(flags["--tags"]),
	}, nil
}

// PromoteLine reads work promote's arguments: the idea, the kind it becomes
// and its flags, each once but --check, given once per check in order, and
// --timeout, once after a --check, that check's seconds (ledger.CheckArg,
// as task add reads them). A line the command refuses is a usage error: no
// kind, an --id (the id is minted), a flag only a task's promotion takes
// given for a slice, and a task with no type or no check.
func PromoteLine(args []string) (Promotion, error) {
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
			return Promotion{}, out.Usage("work promote does not take %s", name)
		}
		if !joined {
			if i+1 >= len(args) {
				return Promotion{}, out.Usage("work promote %s needs a value", name)
			}
			i++
			v = args[i]
		}
		flags = append(flags, flag{name, v})
	}
	if len(ids) != 1 {
		return Promotion{}, out.Usage("work promote needs one <id>")
	}
	p := Promotion{ID: ids[0]}
	var id string
	once := map[string]*string{"--id": &id, "--kind": &p.Kind, "--title": &p.Title, "--why": &p.Why, "--type": &p.Type}
	seen := map[string]bool{}
	for _, f := range flags {
		if field, single := once[f.name]; single {
			if seen[f.name] || strings.TrimSpace(f.value) == "" {
				return Promotion{}, out.Usage("work promote takes one %s with a value", f.name)
			}
			seen[f.name], *field = true, f.value
		}
	}
	if !slices.Contains(promoteKinds, p.Kind) {
		return Promotion{}, out.Usage("work promote needs --kind %s", strings.Join(promoteKinds, "|"))
	}
	if id != "" {
		return Promotion{}, out.Usage("work promote mints the %s id; do not pass --id", p.Kind)
	}
	// ledger.CheckArg reads timed only after a --check, which sets it.
	var timed bool
	for _, f := range flags {
		switch {
		case p.Kind == "slice" && slices.Contains(promoteTaskOnly, f.name):
			return Promotion{}, out.Usage("work promote --kind slice does not take %s, which only a task's promotion takes", f.name)
		case f.name == "--check" || f.name == "--timeout":
			if err := ledger.CheckArg("work promote", &p.Checks, &timed, f.name, f.value); err != nil {
				return Promotion{}, err
			}
		}
	}
	if p.Kind == "task" && (p.Type == "" || len(p.Checks) == 0) {
		return Promotion{}, out.Usage("work promote --kind task needs --type <type> and a --check <command>")
	}
	return p, nil
}

// oneID reads a registry command's arguments: one id, and the valued flags
// it takes, each given at most once, "--flag value" or "--flag=value"; those
// in empty may be given an empty value. Anything else is a usage error.
func oneID(sub string, args, empty []string, flags ...string) (string, map[string]string, error) {
	ids, values, err := parsed(sub, args, empty, flags...)
	if err != nil {
		return "", nil, err
	}
	if len(ids) != 1 {
		return "", nil, out.Usage("work %s needs one <id>", sub)
	}
	return ids[0], values, nil
}

// someIDs reads the arguments of a registry command that takes one or more
// ids: the ids, in the order given, each once, and the flags as oneID reads
// them.
func someIDs(sub string, args, empty []string, flags ...string) ([]string, map[string]string, error) {
	ids, values, err := parsed(sub, args, empty, flags...)
	if err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return nil, nil, out.Usage("work %s needs one <id> or more", sub)
	}
	for i, id := range ids {
		if slices.Contains(ids[:i], id) {
			return nil, nil, out.Usage("work %s names %s twice", sub, id)
		}
	}
	return ids, values, nil
}

// parsed are a registry command's positional arguments and its valued
// flags, each once, with a value unless empty names it.
func parsed(sub string, args, empty []string, flags ...string) ([]string, map[string]string, error) {
	values := map[string]string{}
	var ids []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			ids = append(ids, arg)
			continue
		}
		name, value, joined := strings.Cut(arg, "=")
		if !slices.Contains(flags, name) {
			return nil, nil, out.Usage("work %s does not take %s", sub, name)
		}
		if !joined {
			if i+1 >= len(args) {
				return nil, nil, out.Usage("work %s %s needs a value", sub, name)
			}
			i++
			value = args[i]
		}
		if _, twice := values[name]; twice || (value == "" && !slices.Contains(empty, name)) {
			return nil, nil, out.Usage("work %s takes one %s with a value", sub, name)
		}
		values[name] = value
	}
	return ids, values, nil
}

// idList is a flag's comma-separated ids, each trimmed, the empty ones left
// out: "" is none.
func idList(s string) []string {
	list := []string{}
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			list = append(list, id)
		}
	}
	return list
}

// typed is a path the person typed, as itos reads it at the top it moved to
// from origin, the subfolder they stood in: relative to it, as git reads a
// path typed there. An absolute path, "-" (stdin) and every path of a run
// that did not move stay as typed.
func typed(origin, p string) string {
	if origin == "" || p == "" || p == "-" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(origin, p)
}

// itos-cc mutation: 111 killed, 0 survived, 0 uncovered
// end itos-cc mutation
