package commit

// itos commit with no subcommand (slice 31, features/commit-command.feature):
// git commit with the footers itos writes. itos reads --task, --item (slice
// 63) and --scenarios and hands every other argument to git commit as it is, so the message comes
// from -m, -F or the editor as git takes it. Each footer the message does not
// already give (bug 138) is passed as git's own --trailer, which git adds to whatever message it ends up with before
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
//
// What would commit past the gates is refused before anything else, exit 2
// (slice 128, refuseCommitArgs): --no-verify and -n, which skip the hooks, a
// core.hooksPath set by git -c, which moves them, and -a and --all, which
// stage every change to a tracked file, someone else's among them. git's own
// reading decides what is an option: a value (-m -n), a cluster (-am) and
// anything after "--" are read as git reads them.

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/gitrun"
	"github.com/donvargax/itos/v8/internal/engine/hooks"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
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
				return f, out.Usage("commit %s needs %s", name, what)
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
			return f, out.Usage("commit %s needs %s", name, what)
		}
		*into = append(*into, ids...)
	}
	return f, nil
}

// footers are the footers the flags write: the links, the ledger's first,
// then the work registry's, then the named tests', the config's footer for
// each source; and the content, the footers of free text in the config's
// order, then BREAKING-CHANGE, the form git reads as a trailer. A link flag
// whose footer the config does not have is a usage error, and one given with
// no config is the error loading it gave (missing).
func (f commitFlags) footers(cfg *config.Loaded, missing error) (links, content []message.Written, err error) {
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
				return nil, nil, out.Usage("commit %s needs a footer whose source is %s in commits.footers", flag.name, flag.source)
			}
			links = append(links, message.Written{Key: key, Flag: flag.name, Values: flag.ids})
		}
	}
	if cfg != nil {
		for _, key := range cfg.Commits.Footers.Keys {
			var texts []string
			for _, t := range f.texts {
				if t.key == key {
					texts = append(texts, t.text)
				}
			}
			if len(texts) > 0 {
				content = append(content, message.Written{Key: key, Flag: message.TextFlag(key), Text: true, Values: texts})
			}
		}
	}
	if len(f.breaking) > 0 {
		content = append(content, message.Written{Key: "BREAKING-CHANGE", Flag: "--breaking", Text: true, Values: f.breaking})
	}
	return links, content, nil
}

// lines are the lines of the footers.
func lines(footers []message.Written) []string {
	var all []string
	for _, f := range footers {
		all = append(all, f.Lines()...)
	}
	return all
}

// settle compares each footer a flag would write with the message's (Written
// .Against): the footers the message does not give, to be written, and the
// problems of those it gives with other values. A footer it gives with the
// same values is dropped. Under a stealth config the links are never in the
// message, so they are all kept.
func settle(cfg *config.Loaded, text string, stealth bool, links, content []message.Written) (keptLinks, keptContent []message.Written, differing []out.Problem) {
	keep := func(footers []message.Written) []message.Written {
		var kept []message.Written
		for _, f := range footers {
			switch standing, p := f.Against(cfg, text); standing {
			case message.Absent:
				kept = append(kept, f)
			case message.Differs:
				differing = append(differing, p)
			}
		}
		return kept
	}
	keptLinks = links
	if !stealth {
		keptLinks = keep(links)
	}
	return keptLinks, keep(content), differing
}

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
// --no-amend winning), whether --no-edit keeps the message it starts with,
// and the first option itos commit refuses (slice 128), "" when none:
// --no-verify or -n, which skip the hooks, or -a or --all, which stage every
// change to a tracked file.
type gitArgs struct {
	messages      []string
	messagesAt    []valueAt
	file          string
	fileAt        valueAt
	amend, noEdit bool
	refused       string
	// origin is where -F's file is read from: the query's Origin, from the
	// folder itos runs in.
	origin string
}

// refuse keeps the option as the first refused, when none is yet.
func (g *gitArgs) refuse(option string) {
	if g.refused == "" {
		g.refused = option
	}
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
		case gitrun.NoVerify(arg), arg == "--all":
			g.refuse(arg)
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
		case c == 'a' || c == 'n':
			g.refuse("-" + string(c))
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
// runs: the -m ones joined as git joins them, the -F file, or, for an amend
// with --no-edit and no -F, HEAD's in v; false when it comes from the editor
// or stdin (-F -, whose text replaces HEAD's).
func (g gitArgs) message(ctx context.Context, v vcsport.VCS) (string, bool) {
	switch {
	case len(g.messages) > 0:
		return strings.Join(g.messages, "\n\n"), true
	case g.file == "-":
		return "", false
	case g.file != "":
		text, err := os.ReadFile(typed(g.origin, g.file))
		return string(text), err == nil
	case g.amend && g.noEdit:
		head, err := v.Show(ctx, "HEAD")
		return head.Message, err == nil
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
func lacking(ctx context.Context, cfg *config.Loaded, text string, ok bool, g gitArgs, links, content []string, stealth bool) []out.Problem {
	typ := message.Type(text)
	if !ok || typ == "" {
		return nil
	}
	linked := message.Trailed(text, slices.Concat(links, content), cfg.CommentChar())
	if stealth {
		linked = strings.Join(links, "\n")
		if len(links) == 0 && g.amend {
			head, _ := cfg.VCS.Show(ctx, "HEAD")
			linked = head.Note
		}
	}
	var found []out.Problem
	for _, key := range message.Missing(cfg, typ, linked, message.Trailed(text, content, cfg.CommentChar())) {
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

// wrapBody is git's arguments with the message they give wrapped to the
// header lint's body line limit (message.Wrap, slice 58), no line starting
// with one of keys, the configured footers', or comment, the repository's
// comment character, and the file it
// wrote the message to, for the caller to remove, "" when none: each -m in
// its place, or the -F file's text, stdin's for -F -, in a file of its own,
// named where the file was. The arguments as given when the limit is 0, the
// message comes from the editor or HEAD, git will refuse them (-m with -F),
// or the -F file is already wrapped or cannot be read, which git reports.
func wrapBody(args []string, g gitArgs, limit int, keys []string, comment string) ([]string, string, error) {
	if limit <= 0 || len(g.messages) > 0 && g.file != "" {
		return args, "", nil
	}
	args = slices.Clone(args)
	if len(g.messages) > 0 {
		for i, text := range message.Wrap(g.messages, limit, comment, keys) {
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
		raw, err = os.ReadFile(typed(g.origin, g.file))
	}
	if err != nil {
		return args, "", nil
	}
	text := message.Wrap([]string{string(raw)}, limit, comment, keys)[0]
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

// refuseCommitArgs refuses, before anything else, what would commit past the
// gates (slice 128): an option that skips the hooks (--no-verify, -n), a
// core.hooksPath set by git -c (hooks.PathSet), which moves the hooks git
// runs, and an option that stages every change to a tracked file (-a,
// --all), someone else's work in progress among them. A usage error, exit 2,
// naming what is refused; nothing is committed.
func refuseCommitArgs(g gitArgs) error {
	switch {
	case g.refused == "-a" || g.refused == "--all":
		return out.Usage("commit refuses %s: it stages every change to a tracked file, someone else's work in progress "+
			"among them; stage your own files by path (git add <path>), then commit without it", g.refused)
	case g.refused != "":
		return out.Usage("commit refuses %s: it skips git's hooks, and the checks they run; commit without it, and fix "+
			"what the hooks report", g.refused)
	case hooks.PathSet():
		return out.Usage("commit refuses core.hooksPath set by git -c: it moves the hooks git runs, skipping the " +
			"repository's own; commit without it")
	}
	return nil
}

// trailers are the footers as git commit's own --trailer arguments.
func trailers(lines []string) []string {
	var args []string
	for _, l := range lines {
		args = append(args, "--trailer", l)
	}
	return args
}

// commit is `itos commit [--task <ids>] [--item <ids>] [--scenarios
// <ids>] [--<footer> <text>] [--breaking <text>] [<git commit args>…]`: a commit missing a
// footer its type requires, or whose message gives a footer with other
// values than its flag's (settle), refused before git runs, else git commit
// with the footers the message does not already give as trailers, or under a stealth config the links as the new
// commit's note and the content as trailers, git's streams the terminal's
// (its stdout on stderr under --json), -q passed on as git's --quiet, and
// git's exit code handed back, with HEAD under --json once it made one.
// settle drops a footer the message gives with the flag's values, so an amend
// keeps one; trailer.ifExists is addIfDifferent for the commit, which
// still keeps a line out of a message itos does not read (the editor's, -F -).
func (h Handler) commit(ctx context.Context, q Query, o Out) (Answer, error) {
	cfg, absent, err := h.ports.CommitConfig(ctx)
	if err != nil {
		return Answer{}, err
	}
	flags, err := readCommitFlags(q.Args, message.TextFlags(cfg))
	if err != nil {
		return Answer{}, err
	}
	g := readGitArgs(flags.git)
	g.origin = vcsport.Within(ctx, h.ports, q.Origin)
	if err := refuseCommitArgs(g); err != nil {
		return Answer{}, err
	}
	linked, texted, err := flags.footers(cfg, absent)
	if err != nil {
		return Answer{}, err
	}
	stealth := h.ports.Stealth(ctx)
	if cfg != nil {
		text, ok := g.message(ctx, h.ports)
		found := lacking(ctx, cfg, text, ok, g, lines(linked), lines(texted), stealth)
		if ok {
			var differing []out.Problem
			linked, texted, differing = settle(cfg, text, stealth, linked, texted)
			found = append(differing, found...)
		}
		if len(found) > 0 {
			return Answer{Code: refused, Reject: cfg.Commits.RejectMessage, Problems: found}, nil
		}
		// Where itos is set up, a commit git would make without itos's
		// commit-msg hook is refused before git runs (slice 91).
		if err := hooks.Ready(ctx, h.ports, "commit-msg"); err != nil {
			return Answer{}, err
		}
	}
	links, content := lines(linked), lines(texted)
	v := h.ports
	env := message.WithoutFooters(os.Environ())
	amend := "0"
	if g.amend {
		amend = "1"
	}
	env = append(env, message.AmendEnv+"="+amend)
	before := ""
	var trailed []string
	if stealth {
		if err := gitrun.RewriteNotes(ctx, v); err != nil {
			return Answer{Code: missing, Said: fmt.Sprintf("itos: cannot set notes.rewriteRef: %s\n", err)}, nil
		}
		if len(links) > 0 {
			env = append(env, message.FootersEnv+"="+strings.Join(links, "\n"))
		}
		before, _ = v.Resolve(ctx, "HEAD")
		trailed = trailers(content)
	} else {
		trailed = trailers(slices.Concat(links, content))
	}
	var keys []string
	var comment string
	if cfg != nil {
		keys, comment = cfg.Commits.Footers.Keys, cfg.CommentChar()
	} else {
		comment = v.CommentChar(ctx)
	}
	given, written, err := wrapBody(flags.git, g, message.BodyLimit(cfg), keys, comment)
	if err != nil {
		return Answer{}, err
	}
	if written != "" {
		defer os.Remove(written)
	}
	stdout := o.Stdout
	if o.JSON {
		stdout = o.Stderr
	}
	code, err := gitrun.ExitOf(v.Commit(ctx, vcsport.Commit{Args: append(trailed, given...), Quiet: o.Quiet},
		gitrun.Talking(q.Origin, env, stdout, o.Stderr)))
	if err != nil {
		return Answer{Code: missing, Said: fmt.Sprintf("itos: cannot run git: %s\n", err)}, nil
	}
	if code == 0 && stealth && len(links) > 0 {
		if err := gitrun.WriteNote(ctx, v, before, links); err != nil {
			return Answer{Code: refused, Said: fmt.Sprintf("itos: the commit is made, but its note is not: %s\n", err)}, nil
		}
	}
	a := Answer{Code: code}
	if o.JSON && code == 0 {
		if sha, err := v.Resolve(ctx, "HEAD"); err == nil {
			a.Commit = sha
		}
	}
	return a, nil
}
