package cli

// The commands that write the work registry (slice 52, features/work.feature):
// work take here, work promote in workpromote.go. The registry is written by commands, never by
// hand, and each commits its own change (the user's calls, 2026-10-03), so a
// coordinator neither edits YAML nor words a registry commit. The edit is
// work's (internal/work/write.go); what every such command shares is here:
// a sound registry before anything is written, and writeRegistry, which
// writes the text and commits the registry alone, through the hooks as any
// commit, whatever else the person has staged (git commit --only), with the
// header and body the change gives. Under a stealth config the registry is
// in the git folder, tracked by nothing, so it is written and nothing is
// committed; and since every linked worktree writes the same files there,
// soundRegistry takes the registry's lock (internal/lock) before it reads,
// held until the command's write is done (bug 16).

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/lock"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// bodyWidth is the width a registry commit's body is wrapped at: the header
// lint's body-max-line-length (selfBody).
const bodyWidth = 100

// workArgs reads a registry command's arguments: one id, and the valued
// flags it takes, each given at most once, "--flag value" or "--flag=value".
// Anything else is a usage error.
func workArgs(sub string, args []string, flags ...string) (string, map[string]string, error) {
	return workArgsEmpty(sub, args, nil, flags...)
}

// workArgsEmpty is workArgs whose flags in empty may be given an empty
// value ("--depends-on ”", no dependencies).
func workArgsEmpty(sub string, args []string, empty []string, flags ...string) (string, map[string]string, error) {
	ids, values, err := workArgsParsed(sub, args, empty, flags...)
	if err != nil {
		return "", nil, err
	}
	if len(ids) != 1 {
		return "", nil, usage("work %s needs one <id>", sub)
	}
	return ids[0], values, nil
}

// workArgsOptionalID is workArgsEmpty for a writer whose item ID is minted
// from its kind: it leaves zero or one positional ID for the command to
// accept (ideas) or refuse (numbered kinds).
func workArgsOptionalID(sub string, args []string, flags ...string) (string, bool, map[string]string, error) {
	ids, values, err := workArgsParsed(sub, args, nil, flags...)
	if err != nil {
		return "", false, nil, err
	}
	if len(ids) > 1 {
		return "", false, nil, usage("work %s takes at most one <id>", sub)
	}
	if len(ids) == 0 {
		return "", false, values, nil
	}
	return ids[0], true, values, nil
}

func workArgsParsed(sub string, args []string, empty []string, flags ...string) ([]string, map[string]string, error) {
	values := map[string]string{}
	var ids []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			ids = append(ids, arg)
			continue
		}
		name, value, joined := strings.Cut(arg, "=")
		known := false
		for _, f := range flags {
			known = known || f == name
		}
		if !known {
			return nil, nil, usage("work %s does not take %s", sub, name)
		}
		if !joined {
			if i+1 >= len(args) {
				return nil, nil, usage("work %s %s needs a value", sub, name)
			}
			i++
			value = args[i]
		}
		if _, twice := values[name]; twice || (value == "" && !slices.Contains(empty, name)) {
			return nil, nil, usage("work %s takes one %s with a value", sub, name)
		}
		values[name] = value
	}
	return ids, values, nil
}

// refuseWork reports a registry command's problems and exits with code: on
// stderr, or under --json as {"ok": false, "problems": […]} on stdout.
func refuseWork(found []out.Problem, code int, o Out) (int, error) {
	if o.JSON {
		return code, out.Emit(o.Stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: found})
	}
	for _, p := range found {
		fmt.Fprintln(o.Stderr, p.Message)
		if p.Fix != "" {
			fmt.Fprintf(o.Stderr, "  fix: %s\n", p.Fix)
		}
	}
	return code, nil
}

// soundRegistry is the config, the registry and its text, when the registry
// is there and sound: a command writes only a registry work check passes.
// Otherwise its problems are reported, exit 1, and the config is nil.
//
// Under a stealth config it first takes the registry's lock, which release,
// never nil and deferred by the caller, gives back: every command that
// writes the registry or a ledger file reads them here, and writes the
// registry, so the lock held from here to the write keeps another itos, in
// another worktree, from reading the files before this one's change and
// writing them over it (bug 16). A project's registry needs none: its
// writes are commits, which git's own index lock takes in turn.
func soundRegistry(o Out) (cfg *config.Loaded, registry work.Registry, text string, release func(), code int, err error) {
	release = func() {}
	if cfg, err = loadConfig(); err != nil {
		return nil, work.Registry{}, "", release, 0, err
	}
	if cfg.Stealth {
		held, err := lock.Hold(cfg.Work.Registry, o.OS)
		if err != nil {
			return nil, work.Registry{}, "", release, 0, err
		}
		release = func() {
			if err := held.Release(); err != nil {
				fmt.Fprintf(o.Stderr, "itos: the lock cannot be given back: %s\n", err)
			}
		}
	}
	found, err := work.Problems(cfg)
	if err != nil {
		return nil, work.Registry{}, "", release, 0, err
	}
	if len(found) > 0 {
		code, err := refuseWork(found, ExitPolicy, o)
		return nil, work.Registry{}, "", release, code, err
	}
	file := cfg.Work.Registry
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, work.Registry{}, "", release, 0, err
	}
	registry, err = work.Load(cfg, file)
	if err != nil {
		return nil, work.Registry{}, "", release, 0, err
	}
	return cfg, registry, string(raw), release, 0, nil
}

// uncommitted is the problem of a file a command writes and commits that
// git does not hold as HEAD has it (the tracker's *Dirty), which the
// command's commit would sweep in beside its own change, a deletion among
// them (bug 13). The problem names the file, the registry, a ledger file,
// itos decision's questions, or the rule the command gives (a decision
// record's).
func uncommitted(cfg *config.Loaded, file, rule string) *out.Problem {
	switch {
	case rule != "":
	case file == cfg.Work.Registry:
		rule = "work-registry-uncommitted"
	case file == cfg.Work.Asks:
		rule = "asks-file-uncommitted"
	default:
		rule = "ledger-file-uncommitted"
	}
	return &out.Problem{
		Rule:    rule,
		Message: file + " has changes no commit holds, which itos's commit of it would sweep in",
		Fix:     "commit or drop the changes to " + file + " first",
	}
}

// writeRegistry writes the change to the registry and, in a project,
// commits the registry alone (writeCommitted).
func writeRegistry(cfg *config.Loaded, old string, change work.Change, o Out) (string, int, error) {
	return writeCommitted(cfg, []written{{path: cfg.Work.Registry, old: old, text: change.Text}}, change.Header, change.Body, o)
}

// written is one file a command writes: its path, its text before and
// after, whether it is new (not there before, so put back by removing it),
// and the rule that refuses it with changes no commit holds, when it is
// neither the registry, the questions nor a ledger file.
type written struct {
	path, old, text string
	created         bool
	rule            string
}

// writeCommitted writes the files and, in a project, commits them alone:
// git commit --only, with the header and body, through the hooks, what else
// is staged left staged (the tracker's Transact). Its result is the new
// commit's SHA, "" under a stealth config, which commits nothing. A file
// with changes no commit holds (uncommitted), a deletion among them, is
// refused before anything is written (exit 1). Whatever fails after the
// first write, a write, git add, a git that cannot start or a commit a hook
// refuses, puts every file back as it was, file and index, a new one
// removed; a refusal exits 1, the rest are errors. work's commands write
// the registry alone (writeRegistry), but for work done closing a task with
// checks, which writes the registry and the task's ledger file (slice 101),
// task add a ledger file and the registry (slice 55), work promote the
// registry, a task's ledger file and the questions (bug 105, slice 138),
// decision the questions alone (slice 62), decision record the questions, a decision
// record, the one it supersedes and their index (slice 69).
func writeCommitted(cfg *config.Loaded, files []written, header, body string, o Out) (string, int, error) {
	t := trackerport.Transaction{
		Changes:    make([]trackerport.Change, len(files)),
		Paragraphs: []string{header, selfBody(cfg, header, body)},
		Quiet:      o.Quiet,
		// git's words and the hooks' go to stderr: stdout is the command's
		// own result.
		Stdin: os.Stdin,
		Words: o.Stderr,
		Env:   append(withoutFooters(os.Environ()), AmendEnv+"=0"),
	}
	paths := make([]string, len(files))
	for i, f := range files {
		t.Changes[i] = trackerport.Change{Name: f.path, Old: f.old, Text: f.text, Created: f.created}
		paths[i] = f.path
	}
	sha, err := tracker(cfg, o.OS).Transact(context.Background(), t)
	var refused *trackerport.Refused
	switch {
	case errors.As(err, &refused) && refused.Restore != nil:
		return "", 0, fmt.Errorf("%s failed (git exited %d): %w", refused.Step, refused.Code, refused.Restore)
	case errors.As(err, &refused):
		as := "is as it was"
		if len(files) > 1 {
			as = "are as they were"
		}
		code, err := refuseWork([]out.Problem{{
			Rule:    "work-commit-failed",
			Message: fmt.Sprintf("%s failed (git exited %d), so %s %s", refused.Step, refused.Code, and(paths), as),
			Fix:     "fix what git or the hook above says, then run the command again",
		}}, ExitPolicy, o)
		return "", code, err
	}
	if p, err := dirty(cfg, files, err); err != nil {
		return "", 0, err
	} else if p != nil {
		code, err := refuseWork([]out.Problem{*p}, ExitPolicy, o)
		return "", code, err
	}
	return sha, 0, nil
}

// commitReady is what writeCommitted refuses before it writes anything, in
// a project (the tracker's Ready): itos's commit-msg hook not declared, an
// error, since the commit runs through the hooks as any commit and git
// would run none of itos's (slice 91); and the first file with changes no
// commit holds (uncommitted), a problem. A command that reserves an id asks
// it before it does (work promote, slice 138). Under a stealth config,
// which commits nothing, nothing is.
func commitReady(cfg *config.Loaded, files []written, o Out) (*out.Problem, error) {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.path
	}
	return dirty(cfg, files, tracker(cfg, o.OS).Ready(context.Background(), paths))
}

// dirty is the tracker's error as writeCommitted gives it: a *Dirty file
// the problem its rule names (uncommitted), anything else the error.
func dirty(cfg *config.Loaded, files []written, err error) (*out.Problem, error) {
	var d *trackerport.Dirty
	if !errors.As(err, &d) {
		return nil, err
	}
	rule := ""
	if at := slices.IndexFunc(files, func(f written) bool { return f.path == d.Name }); at >= 0 {
		rule = files[at].rule
	}
	return uncommitted(cfg, d.Name, rule), nil
}

// selfBody is the body of a commit itos makes of its own files, its words
// on one line, then wrapped at bodyWidth by the wrap itos commit uses
// (message.Wrap, bug 15): no line starts with a token the header lint reads
// as a footer, a breaking-change note, a configured footer's key and its
// colon, or git's comment char, a break there moving back a word. A plain
// wrap started lines with whatever word came next, so a question or a why
// holding "word:" drew the lint's footer-leading-blank warning on itos's own
// commit (bug 18).
func selfBody(cfg *config.Loaded, header, body string) string {
	parts := []string{header, strings.Join(strings.Fields(body), " ")}
	return message.Wrap(parts, bodyWidth, cfg.CommentChar(), cfg.Commits.Footers.Keys)[1]
}

// and is the names joined as a list is said: "a", "a and b", "a, b and c".
func and(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// committed is how a registry command's text output ends: the commit it
// made, or why there is none.
func committed(sha, header string) string {
	if sha == "" {
		return "the registry is the stealth config's, so nothing is committed"
	}
	return "committed " + sha[:min(7, len(sha))] + " " + header
}

// reportWork prints a registry command's result: the line, unless -q; under
// --json, ok, the item, the fields given and the commit (null when none).
func reportWork(line string, change work.Change, sha string, extra []out.Field, o Out) (int, error) {
	if o.JSON {
		var commit any
		if sha != "" {
			commit = sha
		}
		fields := append([]out.Field{{Key: "ok", Value: true}, {Key: "item", Value: change.Item}}, extra...)
		return 0, out.Emit(o.Stdout, append(fields, out.Field{Key: "commit", Value: commit})...)
	}
	if !o.Quiet {
		fmt.Fprintln(o.Stdout, line)
	}
	return 0, nil
}

// workTake is `work take <id> [--as <handle>]`: the item set in progress for
// the person and the registry committed (work.Take). The person is --as,
// else who the identity provider says, as for work; one that is nobody, or
// not among the people, exits 3. Under a stealth config, or with no one in
// the people (Registry.Nobody: a people file missing or listing nobody), with
// no --as nobody is asked: every item is the session's, and its owner is left
// as it is (bug 46).
func workTake(args []string, o Out) (int, error) {
	id, flags, err := workArgs("take", args, "--as")
	if err != nil {
		return 0, err
	}
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	person, every, problem := workPerson(cfg, registry, flags["--as"], providers.IdentityProvider(cfg))
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitMissing, o)
	}
	change, problem, err := work.Take(registry, text, id, person, every)
	if err != nil {
		return 0, uneditable(cfg.Work.Registry, err)
	}
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}
	forWhom := ""
	if owner, ok := change.Item.At("owner").(string); ok && owner != "" {
		forWhom = " for " + owner
	}
	if change.Unchanged {
		return reportWork(id+" is already in progress"+forWhom+"; nothing to change", change, "", nil, o)
	}
	sha, code, err := writeRegistry(cfg, text, change, o)
	if err != nil || code != 0 {
		return code, err
	}
	return reportWork(fmt.Sprintf("%s is in progress%s: %s", id, forWhom, committed(sha, change.Header)), change, sha, nil, o)
}

// workPerson is who a command that judges an item's owner acts for, work
// take and work give-back: --as (as), else who the identity provider says,
// as for work (identify, the config's work.identity provider). every is a session that owns every item, asked of nobody: a
// stealth config, or no one in the people (Registry.Nobody: a people file
// missing or listing nobody), with no --as (bug 46). The problem, exit 3, is
// a person who is nobody or not among the people.
func workPerson(cfg *config.Loaded, registry work.Registry, as string, identify providers.Identity) (person string, every bool, problem *out.Problem) {
	if as == "" && (cfg.Stealth || registry.Nobody()) {
		return "", true, nil
	}
	listedIn := cfg.Work.People.File
	who := work.Whoami(registry, listedIn, as, identify)
	switch {
	case who.Problem != "":
		return "", false, &out.Problem{Rule: "work-no-person", Message: who.Problem, Fix: "pass --as <handle>"}
	case !who.Listed:
		return "", false, &out.Problem{
			Rule:    "work-no-person",
			Message: fmt.Sprintf("%s is not in %s, so cannot own an item", who.Handle, listedIn),
			Fix:     "add " + who.Handle + " to " + listedIn + ", or pass --as <handle>",
		}
	}
	return who.Handle, false, nil
}

// promoteKinds are what work promote makes of an idea.
// workQueue is `work queue <id>… --top | --before <id> | --after <id> |
// --remove` (slice 66; --drop before v6.0.0): the item put first in the registry's queue, just
// before or after an item it holds, or taken out of it, the queue written
// whole and the registry committed, "docs: queue <id>" (work.Queue). One
// place, exactly, is given. Several ids go there together, in the order
// given (work.QueueEach, writeEach).
func workQueue(args []string, o Out) (int, error) {
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
	ids, flags, err := workIDs("queue", rest, nil, "--before", "--after")
	if err != nil {
		return 0, err
	}
	at := work.Place{Top: top, Remove: remove, Before: flags["--before"], After: flags["--after"]}
	given := 0
	for _, on := range []bool{at.Top, at.Remove, at.Before != "", at.After != ""} {
		if on {
			given++
		}
	}
	if given != 1 {
		return 0, usage("work queue needs one of --top, --before <id>, --after <id> or --remove")
	}
	where := "first in the queue"
	switch {
	case at.Remove:
		where = "out of the queue"
	case at.Before != "":
		where = "before " + at.Before + " in the queue"
	case at.After != "":
		where = "after " + at.After + " in the queue"
	}
	queued := func(list []string) []out.Field {
		queue := make([]any, len(list))
		for i, q := range list {
			queue[i] = q
		}
		return []out.Field{{Key: "queue", Value: queue}}
	}
	if len(ids) > 1 {
		return writeEach(o, "queue", func(_ *config.Loaded, r work.Registry, text string) (work.Several, []string, []out.Problem, error) {
			return work.QueueEach(r, text, ids, at)
		}, func(s work.Several) []string {
			placed := where
			if !at.Remove {
				placed += ", in that order"
			}
			if s.Unchanged {
				return []string{and(ids) + " are already " + placed}
			}
			return []string{and(ids) + " are " + placed}
		}, func(s work.Several) []out.Field { return queued(s.Queue) })
	}
	id := ids[0]
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	change, problem, err := work.Queue(registry, text, id, at)
	if err != nil {
		return 0, uneditable(cfg.Work.Registry, err)
	}
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}
	extra := queued(change.Queue)
	if change.Unchanged {
		return reportWork(id+" is already "+where+"; nothing to change", change, "", extra, o)
	}
	sha, code, err := writeRegistry(cfg, text, change, o)
	if err != nil || code != 0 {
		return code, err
	}
	return reportWork(fmt.Sprintf("%s is %s: %s", id, where, committed(sha, change.Header)), change, sha, extra, o)
}
