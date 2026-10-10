package cli

// The commands that write the work registry (slice 52, features/work.feature):
// work take and work promote. The registry is written by commands, never by
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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/ledger"
	"github.com/donvargax/itos/v7/internal/lock"
	"github.com/donvargax/itos/v7/internal/message"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/providers"
	"github.com/donvargax/itos/v7/internal/work"
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
	if cfg, err = config.Load(config.Path()); err != nil {
		return nil, work.Registry{}, "", release, 0, err
	}
	if cfg.Stealth {
		held, err := lock.Hold(cfg.Work.Registry)
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
// git does not hold as HEAD has it, which the command's commit would sweep in
// beside its own change; nil when it is clean. A file in the working tree is
// clean when git tracks it and reports no change to it; a file not there,
// which the command makes, when git knows nothing of it either: one deleted
// and the deletion not committed, staged or not, is a change no commit holds
// (bug 13), which a commit of the file written afresh would undo. The
// problem names the file, the registry, a ledger file, itos decision's
// questions, or the rule the command gives (a decision record's).
func uncommitted(cfg *config.Loaded, file, rule string) *out.Problem {
	_, err := os.Lstat(file)
	there := !errors.Is(err, fs.ErrNotExist)
	tracked := git.Succeeds("ls-files", "--error-unmatch", "--", file)
	status, err := git.Output("status", "--porcelain", "--", file)
	if err == nil && strings.TrimSpace(status) == "" && tracked == there {
		return nil
	}
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
// is staged left staged. Its result is the new commit's SHA, "" under a
// stealth config, which commits nothing. A file with changes no commit holds
// (uncommitted), a deletion among them, is refused before anything is
// written (exit 1). Whatever fails after the first write, a write, git add,
// a git that cannot start or a commit a hook refuses, puts every file back
// as it was, file and index, a new one removed (restore); a refusal exits 1,
// the rest are errors. work's commands write the registry alone
// (writeRegistry), but for work done closing a task with checks, which
// writes the registry and the task's ledger file (slice 101), task add a
// ledger file and the registry (slice 55),
// decision the questions alone (slice 62), decision record the questions, a decision
// record, the one it supersedes and their index (slice 69).
func writeCommitted(cfg *config.Loaded, files []written, header, body string, o Out) (string, int, error) {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.path
	}
	if !cfg.Stealth {
		// The commit runs through the hooks as any commit, so it is refused,
		// before anything is written, where git would run none of itos's
		// (slice 91).
		if err := hooksReady("commit-msg"); err != nil {
			return "", 0, err
		}
		for _, f := range files {
			if p := uncommitted(cfg, f.path, f.rule); p != nil {
				code, err := refuseWork([]out.Problem{*p}, ExitPolicy, o)
				return "", code, err
			}
		}
	}
	named := and(paths)
	modes := make([]fs.FileMode, len(files))
	for i, f := range files {
		modes[i] = fs.FileMode(0o644)
		if info, err := os.Stat(f.path); err == nil {
			modes[i] = info.Mode().Perm()
		}
		if touched, err := put(f.path, f.text, modes[i]); err != nil {
			if !touched {
				i--
			}
			return "", 0, restored(cfg, files[:i+1], modes, fmt.Errorf("%s cannot be written: %w", f.path, err))
		}
	}
	if cfg.Stealth {
		return "", 0, nil
	}
	failed := func(step string, exit *exec.ExitError) (string, int, error) {
		if err := restored(cfg, files, modes, nil); err != nil {
			return "", 0, fmt.Errorf("%s failed (git exited %d): %w", step, exit.ExitCode(), err)
		}
		as := "is as it was"
		if len(files) > 1 {
			as = "are as they were"
		}
		code, err := refuseWork([]out.Problem{{
			Rule:    "work-commit-failed",
			Message: fmt.Sprintf("%s failed (git exited %d), so %s %s", step, exit.ExitCode(), named, as),
			Fix:     "fix what git or the hook above says, then run the command again",
		}}, ExitPolicy, o)
		return "", code, err
	}
	var exit *exec.ExitError
	for _, f := range files {
		// A path git does not know is no pathspec for --only until it is
		// added; what git add says goes to stderr, as the commit's words do.
		if !f.created {
			continue
		}
		if err := gitTo(o.Stderr, nil, "add", "--", f.path); errors.As(err, &exit) {
			return failed("git add of "+f.path, exit)
		} else if err != nil {
			return "", 0, restored(cfg, files, modes, fmt.Errorf("cannot run git: %w", err))
		}
	}
	argv := []string{"commit", "--only", "-m", header, "-m", selfBody(cfg, header, body)}
	if o.Quiet {
		argv = append(argv, "--quiet")
	}
	argv = append(argv, "--")
	argv = append(argv, paths...)
	if err := gitTo(o.Stderr, append(withoutFooters(os.Environ()), AmendEnv+"=0"), argv...); errors.As(err, &exit) {
		return failed("the commit of "+named, exit)
	} else if err != nil {
		return "", 0, restored(cfg, files, modes, fmt.Errorf("cannot run git: %w", err))
	}
	// git commit --only stages the files in the real index before the hooks
	// run on a temporary one, so a hook that rewrites them, a formatter,
	// puts its version in the commit and the working tree while the real
	// index keeps itos's (bug 17). Their entries are set to the commit's:
	// they had no change of the person's, which uncommitted refused above.
	if err := gitErr(append([]string{"reset", "-q", "--"}, paths...)...); err != nil {
		return "", 0, fmt.Errorf("the commit of %s was made, but git's index of them cannot be set to it: %w", named, err)
	}
	sha, err := git.Output("rev-parse", "HEAD")
	if err != nil {
		return "", 0, err
	}
	return strings.TrimSpace(sha), 0, nil
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
	return message.Wrap(parts, bodyWidth, message.CommentChar(), cfg.Commits.Footers.Keys)[1]
}

// put writes the text to the file in the mode, as os.WriteFile does, and
// says whether it touched the file: false when the file could not be opened,
// so it is as it was.
func put(path, text string, mode fs.FileMode) (bool, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return false, err
	}
	_, err = f.WriteString(text)
	return true, errors.Join(err, f.Close())
}

// gitTo runs git with its stdout and stderr to w, and stdin the command's,
// under env (nil: the command's own).
func gitTo(w io.Writer, env []string, args ...string) error {
	cmd := exec.Command(git.Bin(), args...)
	cmd.Env = env
	// stdout is the command's own result; git's and the hooks' words go to
	// stderr.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, w, w
	return cmd.Run()
}

// restore puts the files back as they were before the command wrote them,
// and git's index for them as HEAD has it (they were clean before): a file
// that was there gets its old text back, in its mode, where it differs; a
// new one is unstaged and removed. Each file is put back whatever failed
// with another, and what failed is the error, naming the file.
func restore(cfg *config.Loaded, files []written, modes []fs.FileMode) error {
	var failed error
	for i, f := range files {
		var err error
		if f.created {
			if !cfg.Stealth {
				err = gitErr("rm", "-q", "--cached", "--ignore-unmatch", "--", f.path)
			}
			if rm := os.Remove(f.path); rm != nil && !errors.Is(rm, fs.ErrNotExist) {
				err = errors.Join(err, rm)
			}
		} else {
			if now, read := os.ReadFile(f.path); read != nil || string(now) != f.old {
				err = os.WriteFile(f.path, []byte(f.old), modes[i])
			}
			if !cfg.Stealth {
				err = errors.Join(err, gitErr("reset", "-q", "--", f.path))
			}
		}
		if err != nil {
			failed = errors.Join(failed, fmt.Errorf("%s cannot be put back: %w", f.path, err))
		}
	}
	return failed
}

// restored is cause, after restore has put the files back; when that fails
// too, an error that says both.
func restored(cfg *config.Loaded, files []written, modes []fs.FileMode, cause error) error {
	err := restore(cfg, files, modes)
	switch {
	case err == nil:
		return cause
	case cause == nil:
		return err
	}
	return fmt.Errorf("%w, and %w", cause, err)
}

// gitErr runs git, its output dropped; the error says what git said on
// stderr when it fails.
func gitErr(args ...string) error {
	var stderr strings.Builder
	cmd := exec.Command(git.Bin(), args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
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
	as := flags["--as"]
	every := as == "" && (cfg.Stealth || registry.Nobody())
	person := ""
	if !every {
		listedIn := cfg.Work.People.File
		who := work.Whoami(registry, listedIn, as, providers.IdentityProvider(cfg))
		switch {
		case who.Problem != "":
			return refuseWork([]out.Problem{{Rule: "work-no-person", Message: who.Problem, Fix: "pass --as <handle>"}}, ExitMissing, o)
		case !who.Listed:
			return refuseWork([]out.Problem{{
				Rule:    "work-no-person",
				Message: fmt.Sprintf("%s is not in %s, so cannot own an item", who.Handle, listedIn),
				Fix:     "add " + who.Handle + " to " + listedIn + ", or pass --as <handle>",
			}}, ExitMissing, o)
		}
		person = who.Handle
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

// promoteKinds are what work promote makes of an idea.
var promoteKinds = []string{"slice", "task"}

// workPromote is `work promote <idea> --kind slice|task [--title <title>]`:
// the idea renamed, given a minted ID and the kind (and the title, when given),
// "Was <idea>." before its why, every depends_on naming it renamed too, and
// the registry committed (work.Promote). A task's id must match the ledger's
// ledger.id.
func workPromote(args []string, o Out) (int, error) {
	id, kind, flags, err := workPromoteLine(args)
	if err != nil {
		return 0, err
	}
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	newID, err := newItemID(cfg, kind, itemIDs(registry))
	if err != nil {
		return 0, err
	}
	change, problem, err := work.Promote(registry, text, id, newID, kind, flags["--title"], ledger.IDPattern(cfg))
	if err != nil {
		return 0, uneditable(cfg.Work.Registry, err)
	}
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}
	sha, code, err := writeRegistry(cfg, text, change, o)
	if err != nil || code != 0 {
		return code, err
	}
	line := fmt.Sprintf("%s is the %s %s", id, kind, newID)
	if len(change.Rewritten) > 0 {
		line += ", named now by " + strings.Join(change.Rewritten, ", ")
	}
	rewritten := make([]any, len(change.Rewritten))
	for i, r := range change.Rewritten {
		rewritten[i] = r
	}
	extra := []out.Field{{Key: "was", Value: id}, {Key: "rewritten", Value: rewritten}}
	return reportWork(line+": "+committed(sha, change.Header), change, sha, extra, o)
}

// workPromoteLine reads work promote's arguments: the idea, the kind it
// becomes and the flags. A line the command refuses is a usage error, an
// --id among them, since the id is minted.
func workPromoteLine(args []string) (id, kind string, flags map[string]string, err error) {
	id, flags, err = workArgs("promote", args, "--id", "--kind", "--title")
	if err != nil {
		return "", "", nil, err
	}
	kind = flags["--kind"]
	if kind != "slice" && kind != "task" {
		return "", "", nil, usage("work promote needs --kind %s", strings.Join(promoteKinds, "|"))
	}
	if flags["--id"] != "" {
		return "", "", nil, usage("work promote mints the %s id; do not pass --id", kind)
	}
	return id, kind, flags, nil
}

// workQueue is `work queue <id> --top | --before <id> | --after <id> |
// --remove` (slice 66; --drop before v6.0.0): the item put first in the registry's queue, just
// before or after an item it holds, or taken out of it, the queue written
// whole and the registry committed, "docs: queue <id>" (work.Queue). One
// place, exactly, is given.
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
	id, flags, err := workArgs("queue", rest, "--before", "--after")
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
	where := "first in the queue"
	switch {
	case at.Remove:
		where = "out of the queue"
	case at.Before != "":
		where = "before " + at.Before + " in the queue"
	case at.After != "":
		where = "after " + at.After + " in the queue"
	}
	queue := make([]any, len(change.Queue))
	for i, q := range change.Queue {
		queue[i] = q
	}
	extra := []out.Field{{Key: "queue", Value: queue}}
	if change.Unchanged {
		return reportWork(id+" is already "+where+"; nothing to change", change, "", extra, o)
	}
	sha, code, err := writeRegistry(cfg, text, change, o)
	if err != nil || code != 0 {
		return code, err
	}
	return reportWork(fmt.Sprintf("%s is %s: %s", id, where, committed(sha, change.Header)), change, sha, extra, o)
}
