package cli

// itos commit with no subcommand (slice 31, features/commit-command.feature):
// git commit with the footers itos writes. itos reads --task, --item (slice
// 63) and --scenarios and hands every other argument to git commit as it is, so the message comes
// from -m, -F or the editor as git takes it. Each footer is passed as git's
// own --trailer, which git adds to whatever message it ends up with before
// the commit-msg hook runs, so the hook judges a footer itos writes as it
// judges a typed one, and a commit it refuses is never made. itos hands back
// git's exit code.
//
// How the footers are written is the one step a mode changes: in a project
// they are trailers in the message; under a stealth config (slice 32,
// stealth.feature) the same lines are handed to the hook in ITOS_FOOTERS and
// written as a git note on the new commit, in refs/notes/itos, never into the
// message, and itos makes sure notes.rewriteRef names that ref, so an amend
// or a rebase carries the note to the commit it makes.
//
// git tells the commit-msg hook nothing of an amend, so itos commit tells it
// (slice 37): AmendEnv says whether --amend is among the arguments it hands
// git, always, so the hook never guesses for a commit made through itos.
//
// A message from -m or -F (-F - included) has its long body lines wrapped to
// the built-in header lint's limit before git sees it (slice 58, wrapBody),
// so a commit is not refused for a line itos can break; the editor's
// message is the hook's to judge.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"slices"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/message"
	"github.com/donvargax/itos/v6/internal/out"
)

// commitFlags are what itos commit reads of its arguments: the IDs --task,
// --item and --scenarios give, each flag as often as wanted, written --task
// <ids> or --task=<ids>, its value one ID or several separated by commas or
// spaces;
// the texts the flags of the footers of free text give (--upgrading <text>,
// one for each such footer the config declares) and --breaking <text>, each
// as often as wanted, its value the next argument whatever it is, as git
// takes an option's value; and the rest, git commit's, in order. A "--" ends
// what itos reads, as it ends git's options.
type commitFlags struct {
	tasks, items, scenarios []string
	texts                   []said
	breaking                []string
	git                     []string
}

// said is one footer of free text a flag gives: its key and its text.
type said struct{ key, text string }

// readCommitFlags reads itos commit's arguments, texts the flags of the
// config's footers of free text (message.TextFlags); a flag without its
// value, or with an empty text, is a usage error.
func readCommitFlags(args []string, texts map[string]string) (commitFlags, error) {
	var f commitFlags
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			f.git = append(f.git, args[i:]...)
			break
		}
		name, value, given := strings.Cut(arg, "=")
		key, isText := texts[name]
		var into *[]string
		switch {
		case name == "--task":
			into = &f.tasks
		case name == "--item":
			into = &f.items
		case name == "--scenarios":
			into = &f.scenarios
		case name == "--breaking" || isText:
			if !given && i+1 < len(args) {
				i++
				value, given = args[i], true
			}
			text := strings.TrimSpace(value)
			if text == "" {
				what := "<text|none>"
				if name == "--breaking" {
					what = "<text>"
				}
				return f, usage("commit %s needs %s", name, what)
			}
			if isText {
				f.texts = append(f.texts, said{key, text})
			} else {
				f.breaking = append(f.breaking, text)
			}
			continue
		default:
			f.git = append(f.git, arg)
			continue
		}
		if !given && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			value, given = args[i], true
		}
		ids := message.SplitIDs(value)
		if !given || len(ids) == 0 {
			what := "<id>"
			if name == "--scenarios" {
				what = "<ids>"
			}
			return f, usage("commit %s needs %s", name, what)
		}
		*into = append(*into, ids...)
	}
	return f, nil
}

// footerLines are the footers the flags write: the links, each "<key>:
// <ids>", the ledger's first, then the work registry's, then the named
// tests', the config's footer for each source; and the
// content, each "<key>: <text>", the footers of free text in the config's
// order, then BREAKING-CHANGE, the form git reads as a trailer. A link flag
// whose footer the config does not have is a usage error, and one given with
// no config is the error loading it gave (missing).
func (f commitFlags) footerLines(cfg *config.Loaded, missing error) (links, content []string, err error) {
	if len(f.tasks) > 0 || len(f.items) > 0 || len(f.scenarios) > 0 {
		if cfg == nil {
			return nil, nil, missing
		}
		for _, flag := range []struct {
			name, source string
			ids          []string
			key          func(*config.Loaded) (string, bool)
		}{
			{"--task", "the ledger", f.tasks, message.LedgerFooter},
			{"--item", "the work registry", f.items, message.RegistryFooter},
			{"--scenarios", "a kind of named tests", f.scenarios, message.TestsFooter},
		} {
			if len(flag.ids) == 0 {
				continue
			}
			key, ok := flag.key(cfg)
			if !ok {
				return nil, nil, usage("commit %s needs a footer whose source is %s in commits.footers", flag.name, flag.source)
			}
			links = append(links, message.FooterLines(key, flag.ids)...)
		}
	}
	if cfg != nil {
		for _, key := range cfg.Commits.Footers.Keys {
			for _, t := range f.texts {
				if t.key == key {
					content = append(content, key+": "+t.text)
				}
			}
		}
	}
	for _, text := range f.breaking {
		content = append(content, "BREAKING-CHANGE: "+text)
	}
	return links, content, nil
}

// commitConfig is the config itos commit reads its flags by: none, with the
// error loading it gave, when there is no config file, so that itos commit
// still runs git where itos is not set up; an error when there is one that
// does not load.
func commitConfig() (cfg *config.Loaded, missing, err error) {
	file := config.Path()
	cfg, err = config.Load(file)
	if err == nil {
		return cfg, nil, nil
	}
	if _, statErr := os.Stat(file); statErr == nil {
		return nil, nil, err
	}
	return nil, err, nil
}

// AmendEnv is the variable itos commit tells the commit-msg hook in whether
// the commit amends HEAD: "1" when it does, "0" when it does not.
const AmendEnv = "ITOS_AMEND"

// gitValued are git commit's long options that take their value as the next
// argument when it is not joined by "=", and gitShortValued its short ones
// that do when nothing follows them in their cluster: the value is never an
// option, so `-m --amend` is a message, not an amend.
var (
	gitValued = []string{"--message", "--file", "--reuse-message", "--reedit-message", "--fixup",
		"--squash", "--author", "--date", "--template", "--cleanup", "--trailer", "--pathspec-from-file"}
	gitShortValued = "mFCct"
)

// gitArgs are what itos reads of git commit's arguments, before any "--":
// the messages -m gives and the file -F gives, each with where it sits among
// the arguments, whether the commit amends HEAD (the last of --amend and
// --no-amend winning) and whether --no-edit keeps the message it starts with.
type gitArgs struct {
	messages      []string
	messagesAt    []valueAt
	file          string
	fileAt        valueAt
	amend, noEdit bool
}

// valueAt is where an option's value sits among git's arguments: the
// argument's index, -1 when there is none, and what comes before the value
// in it ("--message=", "-am", or "" when the value is an argument of its own).
type valueAt struct {
	index  int
	prefix string
}

// readGitArgs reads git commit's arguments as git reads them, a valued
// option's value never taken for an option.
func readGitArgs(args []string) gitArgs {
	var g gitArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		next := func() (string, valueAt) {
			if i+1 < len(args) {
				i++
				return args[i], valueAt{index: i}
			}
			return "", valueAt{index: -1}
		}
		name, value, given := strings.Cut(arg, "=")
		switch {
		case arg == "--":
			return g
		case arg == "--amend", arg == "--no-amend":
			g.amend = arg == "--amend"
		case arg == "--no-edit", arg == "--edit":
			g.noEdit = arg == "--no-edit"
		case strings.HasPrefix(arg, "--") && slices.Contains(gitValued, name):
			at := valueAt{index: i, prefix: name + "="}
			if !given {
				value, at = next()
			}
			switch name {
			case "--message":
				g.messages, g.messagesAt = append(g.messages, value), append(g.messagesAt, at)
			case "--file":
				g.file, g.fileAt = value, at
			}
		case strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--"):
			g.readCluster(arg[1:], i, next)
		}
	}
	return g
}

// readCluster reads one cluster of short options (-am), the argument at
// index, the first valued one taking the rest of it as its value, or the
// next argument.
func (g *gitArgs) readCluster(cluster string, index int, next func() (string, valueAt)) {
	for j := 0; j < len(cluster); j++ {
		c := cluster[j]
		switch {
		case c == 'e':
			g.noEdit = false
		case c == 'u' || c == 'S':
			return
		case strings.IndexByte(gitShortValued, c) >= 0:
			value := cluster[j+1:]
			at := valueAt{index: index, prefix: "-" + cluster[:j+1]}
			if value == "" {
				value, at = next()
			}
			switch c {
			case 'm':
				g.messages, g.messagesAt = append(g.messages, value), append(g.messagesAt, at)
			case 'F':
				g.file, g.fileAt = value, at
			}
			return
		}
	}
}

// message is the commit's message as far as itos can read it before git
// runs: the -m ones joined as git joins them, the -F file (not stdin), or,
// for an amend with --no-edit, HEAD's; false when it comes from the editor.
func (g gitArgs) message() (string, bool) {
	switch {
	case len(g.messages) > 0:
		return strings.Join(g.messages, "\n\n"), true
	case g.file != "" && g.file != "-":
		text, err := os.ReadFile(typed(g.file))
		return string(text), err == nil
	case g.amend && g.noEdit:
		text, err := git.Output("log", "-1", "--format=%B", "HEAD")
		return text, err == nil
	}
	return "", false
}

// lacking are the footers the commit's type requires that neither the flags
// nor the message give, each naming the flag that writes it, as the hook
// would refuse them, so a commit missing one is refused before git runs: in
// a project the footers are the message's and the flags'; under a stealth
// config the links are the flags' or, for an amend that gives none, HEAD's
// note, which the amend carries, and the content the message's and the
// flags'. Nothing when itos cannot read the message (the editor's, or -F -),
// which the hook judges; a footer no flag writes is the hook's too.
func lacking(cfg *config.Loaded, g gitArgs, links, content []string, stealth bool) []out.Problem {
	text, ok := g.message()
	typ := message.Type(text)
	if !ok || typ == "" {
		return nil
	}
	written := strings.Join(content, "\n")
	linked := text + "\n" + strings.Join(links, "\n") + "\n" + written
	if stealth {
		linked = strings.Join(links, "\n")
		if len(links) == 0 && g.amend {
			linked, _ = message.Note("HEAD")
		}
	}
	var found []out.Problem
	for _, key := range message.Missing(cfg, typ, linked, text+"\n"+written) {
		flag, what := message.Flag(cfg, key)
		if flag == "" {
			continue
		}
		found = append(found, out.Problem{
			Rule:    strings.ToLower(key) + "-footer",
			Message: fmt.Sprintf("%s commits need %s %s", typ, flag, what),
			Fix:     fmt.Sprintf("commit with itos commit %s %s", flag, what),
		})
	}
	return found
}

// refuseCommit reports the footers a commit lacks, as the hook reports a
// rejection, and exits 1: no commit is made.
func refuseCommit(cfg *config.Loaded, found []out.Problem, o Out) (int, error) {
	if o.JSON {
		return ExitPolicy, out.Emit(o.Stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: found})
	}
	fmt.Fprintln(o.Stderr, cfg.Commits.RejectMessage)
	for _, p := range found {
		fmt.Fprintf(o.Stderr, "  - %s\n", p.Message)
	}
	return ExitPolicy, nil
}

// wrapBody is git's arguments with the message they give wrapped to the
// header lint's body line limit (message.Wrap, slice 58), no line starting
// with one of keys, the configured footers', and the file it
// wrote the message to, for the caller to remove, "" when none: each -m in
// its place, or the -F file's text, stdin's for -F -, in a file of its own,
// named where the file was. The arguments as given when the limit is 0, the
// message comes from the editor or HEAD, git will refuse them (-m with -F),
// or the -F file is already wrapped or cannot be read, which git reports.
func wrapBody(args []string, g gitArgs, limit int, keys []string) ([]string, string, error) {
	if limit <= 0 || len(g.messages) > 0 && g.file != "" {
		return args, "", nil
	}
	args = slices.Clone(args)
	if len(g.messages) > 0 {
		for i, text := range message.Wrap(g.messages, limit, message.CommentChar(), keys) {
			if at := g.messagesAt[i]; at.index >= 0 {
				args[at.index] = at.prefix + text
			}
		}
		return args, "", nil
	}
	if g.file == "" || g.fileAt.index < 0 {
		return args, "", nil
	}
	var raw []byte
	var err error
	if g.file == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(typed(g.file))
	}
	if err != nil {
		return args, "", nil
	}
	text := message.Wrap([]string{string(raw)}, limit, message.CommentChar(), keys)[0]
	if text == string(raw) && g.file != "-" {
		return args, "", nil
	}
	f, err := os.CreateTemp("", "itos-message-*")
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		os.Remove(f.Name())
		return nil, "", err
	}
	args[g.fileAt.index] = g.fileAt.prefix + f.Name()
	return args, f.Name(), nil
}

// trailers are the footers as git commit's own --trailer arguments.
func trailers(lines []string) []string {
	var args []string
	for _, l := range lines {
		args = append(args, "--trailer", l)
	}
	return args
}

// gitCommit is `itos commit [--task <ids>] [--item <ids>] [--scenarios
// <ids>] [--<footer> <text>] [--breaking <text>] [<git commit args>…]`: a commit missing a
// footer its type requires refused before git runs, else git commit with the
// footers as trailers, or under a stealth config the links as the new
// commit's note and the content as trailers, git's streams the terminal's
// (its stdout on stderr under --json, which prints the outcome), -q passed on
// as git's --quiet, and git's exit code handed back. trailer.ifExists is
// addIfDifferent for the commit, so a footer the message already has, as an
// amend's does, is not written twice.
func gitCommit(args []string, o Out) (int, error) {
	cfg, missing, err := commitConfig()
	if err != nil {
		return 0, err
	}
	flags, err := readCommitFlags(args, message.TextFlags(cfg))
	if err != nil {
		return 0, err
	}
	links, content, err := flags.footerLines(cfg, missing)
	if err != nil {
		return 0, err
	}
	stealth := config.IsStealth(config.Path())
	g := readGitArgs(flags.git)
	if cfg != nil {
		if found := lacking(cfg, g, links, content, stealth); len(found) > 0 {
			return refuseCommit(cfg, found, o)
		}
		// Where itos is set up, a commit git would make without itos's
		// commit-msg hook is refused before git runs (slice 91).
		if err := hooksReady("commit-msg"); err != nil {
			return 0, err
		}
	}
	argv := []string{"-c", "trailer.ifExists=addIfDifferent", "commit"}
	env := withoutFooters(os.Environ())
	amend := "0"
	if g.amend {
		amend = "1"
	}
	env = append(env, AmendEnv+"="+amend)
	before := ""
	if stealth {
		if err := rewriteNotes(); err != nil {
			fmt.Fprintf(o.Stderr, "itos: cannot set notes.rewriteRef: %s\n", err)
			return ExitMissing, nil
		}
		if len(links) > 0 {
			env = append(env, message.FootersEnv+"="+strings.Join(links, "\n"))
		}
		before, _ = git.Output("rev-parse", "--verify", "--quiet", "HEAD")
		argv = append(argv, trailers(content)...)
	} else {
		argv = append(argv, trailers(slices.Concat(links, content))...)
	}
	if o.Quiet {
		argv = append(argv, "--quiet")
	}
	var keys []string
	if cfg != nil {
		keys = cfg.Commits.Footers.Keys
	}
	given, written, err := wrapBody(flags.git, g, message.BodyLimit(cfg), keys)
	if err != nil {
		return 0, err
	}
	if written != "" {
		defer os.Remove(written)
	}
	argv = append(argv, given...)
	stdout := o.Stdout
	if o.JSON {
		stdout = o.Stderr
	}
	code, err := runGit(argv, env, stdout, o)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: cannot run git: %s\n", err)
		return ExitMissing, nil
	}
	if code == 0 && stealth && len(links) > 0 {
		if err := writeNote(before, links); err != nil {
			fmt.Fprintf(o.Stderr, "itos: the commit is made, but its note is not: %s\n", err)
			return ExitPolicy, nil
		}
	}
	if o.JSON {
		fields := []out.Field{{Key: "ok", Value: code == 0}}
		if code == 0 {
			if sha, err := git.Output("rev-parse", "HEAD"); err == nil {
				fields = append(fields, out.Field{Key: "commit", Value: strings.TrimSpace(sha)})
			}
		}
		if err := out.Emit(o.Stdout, fields...); err != nil {
			return 0, err
		}
	}
	return code, nil
}

// runGit runs git with the terminal's stdin, so an editor can open, and gives
// its exit code; the error is why it could not start. An interrupt is git's
// and the editor's to act on while it runs, not a reason for itos to leave
// first. It runs in the folder the person stood in, so the paths and the
// files they gave git mean what they mean there, though itos moved to the
// repository's top to read its config (slice 40).
func runGit(argv, env []string, stdout io.Writer, o Out) (int, error) {
	cmd := exec.Command(git.Bin(), argv...)
	cmd.Dir = origin
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, o.Stderr
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if code := exit.ExitCode(); code > 0 {
			return code, nil
		}
		return ExitPolicy, nil
	}
	return 0, err
}

// withoutFooters is an environment less ITOS_FOOTERS and ITOS_AMEND, so only
// the footers this itos commit writes, and its own word on an amend, reach
// the hook.
func withoutFooters(env []string) []string {
	var kept []string
	for _, kv := range env {
		if !strings.HasPrefix(kv, message.FootersEnv+"=") && !strings.HasPrefix(kv, AmendEnv+"=") {
			kept = append(kept, kv)
		}
	}
	return kept
}

// rewriteNotes adds refs/notes/itos to notes.rewriteRef in the repository's
// own config unless a value already names it, so that git commit --amend and
// git rebase, which copy the notes of the refs it names to the commits they
// make, carry a commit's footers along.
func rewriteNotes() error {
	set, _ := git.Output("config", "--get-all", "notes.rewriteRef")
	for _, ref := range strings.Split(set, "\n") {
		if ok, _ := path.Match(strings.TrimSpace(ref), message.NotesRef); ok {
			return nil
		}
	}
	return exec.Command(git.Bin(), "config", "--local", "--add", "notes.rewriteRef", message.NotesRef).Run()
}

// writeNote writes the footers as the itos note of the commit git just made,
// replacing the one an amend carried over; nothing when HEAD is still the
// commit it was before (a dry run made none).
func writeNote(before string, lines []string) error {
	head, err := git.Output("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(head) == strings.TrimSpace(before) {
		return nil
	}
	note := strings.Join(lines, "\n")
	if out, err := exec.Command(git.Bin(), "notes", "--ref="+message.NotesRef, "add", "-f", "-m", note, "HEAD").CombinedOutput(); err != nil {
		return fmt.Errorf("git notes add: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
