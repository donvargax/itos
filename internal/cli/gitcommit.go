package cli

// itos commit with no subcommand (slice 31, features/commit-command.feature):
// git commit with the footers itos writes. itos reads --task and --scenarios
// and hands every other argument to git commit as it is, so the message comes
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

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/message"
	"github.com/donvargax/itos/v2/internal/out"
)

// commitFlags are what itos commit reads of its arguments: the IDs --task
// and --scenarios give, each flag as often as wanted, written --task <ids> or
// --task=<ids>, its value one ID or several separated by commas or spaces;
// and the rest, git commit's, in order. A "--" ends what itos reads, as it
// ends git's options.
type commitFlags struct {
	tasks, scenarios []string
	git              []string
}

// readCommitFlags reads itos commit's arguments; a flag without its value is
// a usage error.
func readCommitFlags(args []string) (commitFlags, error) {
	var f commitFlags
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			f.git = append(f.git, args[i:]...)
			break
		}
		var into *[]string
		name, value, given := strings.Cut(arg, "=")
		switch name {
		case "--task":
			into = &f.tasks
		case "--scenarios":
			into = &f.scenarios
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

// footerLines are the footers the flags write, each "<key>: <ids>", the
// ledger's first: the config's footer for each source, read only when a flag
// is given. A flag whose footer the config does not have is a usage error.
func (f commitFlags) footerLines() ([]string, error) {
	if len(f.tasks) == 0 && len(f.scenarios) == 0 {
		return nil, nil
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, flag := range []struct {
		name, source string
		ids          []string
		key          func(*config.Loaded) (string, bool)
	}{
		{"--task", "the ledger", f.tasks, message.LedgerFooter},
		{"--scenarios", "a kind of named tests", f.scenarios, message.TestsFooter},
	} {
		if len(flag.ids) == 0 {
			continue
		}
		key, ok := flag.key(cfg)
		if !ok {
			return nil, usage("commit %s needs a footer whose source is %s in commits.footers", flag.name, flag.source)
		}
		lines = append(lines, message.FooterLines(key, flag.ids)...)
	}
	return lines, nil
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

// amends is whether git commit's arguments amend HEAD: --amend among its
// options, before any "--", the last of --amend and --no-amend winning.
func amends(args []string) bool {
	amend := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return amend
		case arg == "--amend":
			amend = true
		case arg == "--no-amend":
			amend = false
		case strings.HasPrefix(arg, "--"):
			if slices.Contains(gitValued, arg) {
				i++
			}
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			if j := strings.IndexAny(arg[1:], gitShortValued+"uS"); j >= 0 {
				if c := arg[1+j]; strings.IndexByte(gitShortValued, c) >= 0 && j+2 == len(arg) {
					i++
				}
			}
		}
	}
	return amend
}

// trailers are the footers as git commit's own --trailer arguments.
func trailers(lines []string) []string {
	var args []string
	for _, l := range lines {
		args = append(args, "--trailer", l)
	}
	return args
}

// gitCommit is `itos commit [--task <ids>] [--scenarios <ids>] [<git commit
// args>…]`: git commit with the footers as trailers, or under a stealth
// config as the new commit's note, git's streams the terminal's (its stdout
// on stderr under --json, which prints the outcome), -q passed on as git's
// --quiet, and git's exit code handed back.
func gitCommit(args []string, o Out) (int, error) {
	flags, err := readCommitFlags(args)
	if err != nil {
		return 0, err
	}
	lines, err := flags.footerLines()
	if err != nil {
		return 0, err
	}
	stealth := config.IsStealth(config.Path())
	argv := []string{"commit"}
	env := withoutFooters(os.Environ())
	amend := "0"
	if amends(flags.git) {
		amend = "1"
	}
	env = append(env, AmendEnv+"="+amend)
	before := ""
	if stealth {
		if err := rewriteNotes(); err != nil {
			fmt.Fprintf(o.Stderr, "itos: cannot set notes.rewriteRef: %s\n", err)
			return ExitMissing, nil
		}
		if len(lines) > 0 {
			env = append(env, message.FootersEnv+"="+strings.Join(lines, "\n"))
		}
		before, _ = git.Output("rev-parse", "--verify", "--quiet", "HEAD")
	} else {
		argv = append(argv, trailers(lines)...)
	}
	if o.Quiet {
		argv = append(argv, "--quiet")
	}
	argv = append(argv, flags.git...)
	stdout := o.Stdout
	if o.JSON {
		stdout = o.Stderr
	}
	code, err := runGit(argv, env, stdout, o)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: cannot run git: %s\n", err)
		return ExitMissing, nil
	}
	if code == 0 && stealth && len(lines) > 0 {
		if err := writeNote(before, lines); err != nil {
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
// first.
func runGit(argv, env []string, stdout io.Writer, o Out) (int, error) {
	cmd := exec.Command("git", argv...)
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
	return exec.Command("git", "config", "--local", "--add", "notes.rewriteRef", message.NotesRef).Run()
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
	if out, err := exec.Command("git", "notes", "--ref="+message.NotesRef, "add", "-f", "-m", note, "HEAD").CombinedOutput(); err != nil {
		return fmt.Errorf("git notes add: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
