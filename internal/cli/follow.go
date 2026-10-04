package cli

// itos follow (slice 61, features/follow.feature): the person's private
// threads with people, in follow-ups.yaml of itos's folder under the git
// common dir (internal/follow). Every subcommand reads the file, and add,
// note, close and doc write it back, holding its lock from before the read
// to after the write (heldThreads, bug 16); none reads a config or commits,
// so any git repository will do. The clock is read once a run, by followNow.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/follow"
	"github.com/donvargax/itos/v3/internal/git"
	"github.com/donvargax/itos/v3/internal/lock"
	"github.com/donvargax/itos/v3/internal/out"
)

// followNow is the time a change is dated with: the one place follow reads
// the clock.
func followNow() time.Time { return time.Now() }

// followTakes is what follow takes, as a usage error names it.
const followTakes = "it takes add, note, close, show or doc, else --all"

// followCommand is `follow [--all]`, `follow add`, `follow note`, `follow close`,
// `follow show` and `follow doc`.
func followCommand(args []string, o Out) (int, error) {
	sub, rest := split(args)
	switch sub {
	case "add":
		return followAdd(rest, o)
	case "note":
		return followNote(rest, o)
	case "close":
		return followClose(rest, o)
	case "show":
		return followShow(rest, o)
	case "doc":
		return followDoc(rest, o)
	}
	pos, _, set, err := followArgs("", args, nil, []string{"--all"})
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usage("follow has no subcommand %s: %s", pos[0], followTakes)
	}
	return followList(set["--all"], o)
}

// followArgs reads a follow subcommand's arguments: its positional ones, the
// valued flags' values and the switches given. A flag it does not take, a
// valued flag with no value or an empty one, and a flag given twice are usage
// errors; after "--" every argument is positional.
func followArgs(sub string, args []string, valued, switches []string) ([]string, map[string]string, map[string]bool, error) {
	return subArgs("follow", sub, args, valued, switches)
}

// subArgs is followArgs for a subcommand of command: itos ask's read so too.
func subArgs(command, sub string, args []string, valued, switches []string) ([]string, map[string]string, map[string]bool, error) {
	name := strings.TrimSpace(command + " " + sub)
	var pos []string
	values := map[string]string{}
	set := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			pos = append(pos, arg)
			continue
		}
		flag, value, joined := strings.Cut(arg, "=")
		switch {
		case slices.Contains(switches, flag) && !joined:
			if set[flag] {
				return nil, nil, nil, usage("%s takes one %s", name, flag)
			}
			set[flag] = true
		case slices.Contains(valued, flag):
			if !joined {
				if i+1 >= len(args) {
					return nil, nil, nil, usage("%s %s needs a value", name, flag)
				}
				i++
				value = args[i]
			}
			if _, twice := values[flag]; twice || strings.TrimSpace(value) == "" {
				return nil, nil, nil, usage("%s takes one %s with a value", name, flag)
			}
			values[flag] = value
		default:
			return nil, nil, nil, usage("%s does not take %s", name, flag)
		}
	}
	return pos, values, set, nil
}

// threadID is the one id a subcommand names first, checked; the arguments
// after it are the rest.
func threadID(sub string, pos []string, what string) (string, []string, error) {
	if len(pos) == 0 {
		return "", nil, usage("follow %s needs %s", sub, what)
	}
	if !follow.ValidID(pos[0]) {
		return "", nil, usage("follow %s: %q is no thread id (letters, digits, '.', '_' and '-', a letter or digit first)", sub, pos[0])
	}
	return pos[0], pos[1:], nil
}

// threadsFile is where the threads are: follow-ups.yaml in itos's folder of
// the git common dir, "" outside a git repository.
func threadsFile() string {
	common, err := git.Output("rev-parse", "--path-format=absolute", "--git-common-dir")
	if common = strings.TrimSpace(common); err != nil || common == "" {
		return ""
	}
	return filepath.Join(common, config.StealthFolder, follow.FileName)
}

// loadThreads is the threads' file and its threads; outside a git repository
// it reports so and gives exit 3, the file "".
func loadThreads(o Out) (string, follow.File, int, error) {
	file := threadsFile()
	if file == "" {
		fmt.Fprintln(o.Stderr, "itos: follow keeps its threads in the git folder, and this is no git repository")
		return "", follow.File{}, ExitMissing, nil
	}
	threads, err := follow.Load(file)
	return file, threads, 0, err
}

// heldThreads is loadThreads for a subcommand that changes the threads: the
// file's lock (internal/lock) taken before they are read, given back by
// release, which the caller defers, so another itos changing them at once
// waits rather than one change being lost (bug 16). release is never nil;
// a lock not had is an error, the file "".
func heldThreads(o Out) (file string, threads follow.File, release func(), code int, err error) {
	release = func() {}
	file = threadsFile()
	if file == "" {
		file, threads, code, err = loadThreads(o)
		return file, threads, release, code, err
	}
	held, err := lock.Hold(file)
	if err != nil {
		return "", follow.File{}, release, 0, err
	}
	release = func() {
		if err := held.Release(); err != nil {
			fmt.Fprintf(o.Stderr, "itos: the lock cannot be given back: %s\n", err)
		}
	}
	threads, err = follow.Load(file)
	return file, threads, release, 0, err
}

// noThread refuses an id no thread has, exit 1.
func noThread(id string, o Out) (int, error) {
	return refuseWork([]out.Problem{{
		Rule:    "follow-no-thread",
		Message: fmt.Sprintf("there is no thread %s", id),
		Fix:     "itos follow --all lists the threads; itos follow add " + id + " opens one",
	}}, ExitPolicy, o)
}

// reportThread prints a change's line, or under --json the thread as it is
// now, with the extra fields after it.
func reportThread(line string, t follow.Thread, o Out, extra ...out.Field) (int, error) {
	if o.JSON {
		fields := append([]out.Field{{Key: "ok", Value: true}, {Key: "thread", Value: t}}, extra...)
		return 0, out.Emit(o.Stdout, fields...)
	}
	if !o.Quiet {
		fmt.Fprintln(o.Stdout, line)
	}
	return 0, nil
}

// followAdd is `follow add <id> --with <who> --title <title> --note <text>`:
// a new thread, open, its first note dated now.
func followAdd(args []string, o Out) (int, error) {
	pos, flags, _, err := followArgs("add", args, []string{"--with", "--title", "--note"}, nil)
	if err != nil {
		return 0, err
	}
	id, rest, err := threadID("add", pos, "<id> --with <who> --title <title> --note <text>")
	if err != nil {
		return 0, err
	}
	if len(rest) > 0 {
		return 0, usage("follow add takes one <id>, not %s", strings.Join(rest, " "))
	}
	if flags["--with"] == "" || flags["--title"] == "" || flags["--note"] == "" {
		return 0, usage("follow add needs --with <who>, --title <title> and --note <text>")
	}
	file, threads, release, code, err := heldThreads(o)
	defer release()
	if file == "" || err != nil {
		return code, err
	}
	if t := threads.Find(id); t != nil {
		return refuseWork([]out.Problem{{
			Rule:    "follow-id-taken",
			Message: fmt.Sprintf("%s is already a thread, with %s: %s", id, t.With, t.Title),
			Fix:     "itos follow note " + id + " adds to it; another id opens a new one",
		}}, ExitPolicy, o)
	}
	t := threads.Add(id, flags["--with"], flags["--title"], flags["--note"], followNow())
	if err := follow.Save(file, threads); err != nil {
		return 0, err
	}
	return reportThread(fmt.Sprintf("%s opened, with %s: %s", id, t.With, t.Title), *t, o)
}

// followNote is `follow note <id> <text>…`: a dated note appended, its words
// joined by spaces.
func followNote(args []string, o Out) (int, error) {
	pos, _, _, err := followArgs("note", args, nil, nil)
	if err != nil {
		return 0, err
	}
	id, rest, err := threadID("note", pos, "<id> <text>")
	if err != nil {
		return 0, err
	}
	text := strings.Join(rest, " ")
	if strings.TrimSpace(text) == "" {
		return 0, usage("follow note needs <id> <text>")
	}
	file, threads, release, code, err := heldThreads(o)
	defer release()
	if file == "" || err != nil {
		return code, err
	}
	t := threads.Find(id)
	if t == nil {
		return noThread(id, o)
	}
	t.Note(text, followNow())
	if err := follow.Save(file, threads); err != nil {
		return 0, err
	}
	return reportThread(fmt.Sprintf("%s: note %d added", id, len(t.Notes)), *t, o)
}

// followClose is `follow close <id> [--note <text>]`: the thread closed, with
// a last note when given.
func followClose(args []string, o Out) (int, error) {
	pos, flags, _, err := followArgs("close", args, []string{"--note"}, nil)
	if err != nil {
		return 0, err
	}
	id, rest, err := threadID("close", pos, "<id>")
	if err != nil {
		return 0, err
	}
	if len(rest) > 0 {
		return 0, usage("follow close takes one <id>, not %s (a last note goes in --note)", strings.Join(rest, " "))
	}
	file, threads, release, code, err := heldThreads(o)
	defer release()
	if file == "" || err != nil {
		return code, err
	}
	t := threads.Find(id)
	if t == nil {
		return noThread(id, o)
	}
	if t.Status == follow.Closed {
		return refuseWork([]out.Problem{{
			Rule:    "follow-closed",
			Message: fmt.Sprintf("%s is already closed, since %s", id, follow.Show(t.Closed)),
			Fix:     "itos follow note " + id + " adds to it",
		}}, ExitPolicy, o)
	}
	t.Close(flags["--note"], followNow())
	if err := follow.Save(file, threads); err != nil {
		return 0, err
	}
	return reportThread(fmt.Sprintf("%s closed: %s", id, t.Title), *t, o)
}

// followShow is `follow show <id>`: the whole thread, every note in order.
func followShow(args []string, o Out) (int, error) {
	pos, _, _, err := followArgs("show", args, nil, nil)
	if err != nil {
		return 0, err
	}
	id, rest, err := threadID("show", pos, "<id>")
	if err != nil {
		return 0, err
	}
	if len(rest) > 0 {
		return 0, usage("follow show takes one <id>, not %s", strings.Join(rest, " "))
	}
	file, threads, code, err := loadThreads(o)
	if file == "" || err != nil {
		return code, err
	}
	t := threads.Find(id)
	if t == nil {
		return noThread(id, o)
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "thread", Value: *t})
	}
	state := follow.Open
	if t.Status == follow.Closed {
		state = "closed " + follow.Show(t.Closed)
	}
	fmt.Fprintf(o.Stdout, "%s: %s\nwith %s, %s\n\n", t.ID, t.Title, t.With, state)
	for _, n := range t.Notes {
		at := follow.Show(n.At)
		indent := strings.Repeat(" ", len(at)+2)
		text := strings.ReplaceAll(strings.TrimRight(n.Text, "\n"), "\n", "\n"+indent)
		fmt.Fprintf(o.Stdout, "%s  %s\n", at, text)
	}
	if len(t.Docs) > 0 {
		fmt.Fprintf(o.Stdout, "\nWritten out to %s\n", strings.Join(t.Docs, ", "))
	}
	return 0, nil
}

// followDoc is `follow doc <id> <path> [--force]`: the whole thread as
// Markdown at path, readable by the person alone (0600, the folders it makes
// 0700: the thread is private), the thread recording where. A file already
// there is refused unless --force. A path in the work tree that git does not
// ignore is written with a warning on stderr, since one git add -A commits
// it (bug 16). <path> "-" is stdout: the Markdown printed, no file written
// and nothing recorded.
func followDoc(args []string, o Out) (int, error) {
	pos, _, set, err := followArgs("doc", args, nil, []string{"--force"})
	if err != nil {
		return 0, err
	}
	id, rest, err := threadID("doc", pos, "<id> <path>")
	if err != nil {
		return 0, err
	}
	if len(rest) != 1 || rest[0] == "" {
		return 0, usage("follow doc needs <id> <path>")
	}
	path := rest[0]
	if path == "-" {
		return followDocOut(id, o)
	}
	file, threads, release, code, err := heldThreads(o)
	defer release()
	if file == "" || err != nil {
		return code, err
	}
	t := threads.Find(id)
	if t == nil {
		return noThread(id, o)
	}
	target, err := filepath.Abs(typed(path))
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(target)
	switch {
	case err == nil && info.IsDir():
		return refuseWork([]out.Problem{{
			Rule:    "follow-doc-exists",
			Message: fmt.Sprintf("%s is a folder", path),
			Fix:     "name the file to write, " + filepath.Join(path, id+".md") + " say",
		}}, ExitPolicy, o)
	case err == nil && !set["--force"]:
		return refuseWork([]out.Problem{{
			Rule:    "follow-doc-exists",
			Message: fmt.Sprintf("%s is already there; nothing written", path),
			Fix:     "--force writes over it",
		}}, ExitPolicy, o)
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return 0, err
	}
	if err := os.WriteFile(target, []byte(follow.Markdown(*t)), 0o600); err != nil {
		return 0, err
	}
	// A file --force wrote over keeps its mode through WriteFile; the thread
	// is no less private in it.
	if err := os.Chmod(target, 0o600); err != nil {
		return 0, err
	}
	if notIgnored(typed(path)) {
		fmt.Fprintf(o.Stderr, "itos: warning: %s is in the work tree and not ignored by git, so a git add can commit this private thread; "+
			"list it in .git/info/exclude or .gitignore, or write it outside the work tree\n", path)
	}
	t.Wrote(target)
	if err := follow.Save(file, threads); err != nil {
		return 0, err
	}
	return reportThread(fmt.Sprintf("%s written to %s", id, path), *t, o, out.Field{Key: "path", Value: target})
}

// followDocOut is `follow doc <id> -`: the thread's Markdown on stdout, or
// under --json in "markdown" beside the thread; nothing written.
func followDocOut(id string, o Out) (int, error) {
	file, threads, code, err := loadThreads(o)
	if file == "" || err != nil {
		return code, err
	}
	t := threads.Find(id)
	if t == nil {
		return noThread(id, o)
	}
	text := follow.Markdown(*t)
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "thread", Value: *t}, out.Field{Key: "markdown", Value: text})
	}
	_, err = fmt.Fprint(o.Stdout, text)
	return 0, err
}

// notIgnored is whether path, as the person typed it, is in the work tree
// and git does not ignore it: git check-ignore exits 1. Outside the work
// tree, or in the git folder, it exits 128, and an ignored path 0.
func notIgnored(path string) bool {
	err := exec.Command(git.Bin(), "check-ignore", "-q", "--", path).Run()
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}

// followEntry is a thread as follow --json lists it.
type followEntry struct {
	ID     string `json:"id"`
	With   string `json:"with"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Opened string `json:"opened"`
	Last   string `json:"last"`
	Notes  int    `json:"notes"`
}

// followList is `follow [--all]`: the open threads, or with --all every
// thread, the closed ones after them.
func followList(all bool, o Out) (int, error) {
	file, threads, code, err := loadThreads(o)
	if file == "" || err != nil {
		return code, err
	}
	var open, closed []follow.Thread
	for _, t := range threads.Threads {
		if t.Status == follow.Open {
			open = append(open, t)
		} else if all {
			closed = append(closed, t)
		}
	}
	if o.JSON {
		entries := []followEntry{}
		for _, t := range append(open, closed...) {
			entries = append(entries, followEntry{t.ID, t.With, t.Title, t.Status, t.Opened(), t.Last(), len(t.Notes)})
		}
		return 0, out.Emit(o.Stdout, out.Field{Key: "threads", Value: entries})
	}
	if len(open) == 0 && len(closed) == 0 {
		if all {
			fmt.Fprintln(o.Stdout, "No threads.")
		} else {
			fmt.Fprintln(o.Stdout, "No open threads.")
		}
		return 0, nil
	}
	width := 0
	for _, t := range append(open, closed...) {
		width = max(width, len(t.ID))
	}
	list := func(heading string, ts []follow.Thread) {
		fmt.Fprintln(o.Stdout, heading)
		for _, t := range ts {
			fmt.Fprintf(o.Stdout, "  %-*s  %s  (%s, %s)\n", width, t.ID, t.Title, t.With, follow.Show(t.Last()))
		}
	}
	if len(open) > 0 {
		list("Open threads:", open)
	} else {
		fmt.Fprintln(o.Stdout, "No open threads.")
	}
	if len(closed) > 0 {
		fmt.Fprintln(o.Stdout)
		list("Closed threads:", closed)
	}
	return 0, nil
}
