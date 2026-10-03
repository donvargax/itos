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
// they are trailers in the message; in the stealth mode (slice 32,
// stealth.feature) the same lines become a git note on the new commit
// instead, handed to the hook rather than written into the message.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/message"
	"github.com/donvargax/itos/v2/internal/out"
)

// commitSubcommands are the words that, first after commit, name one of its
// subcommands; any other first argument is git commit's.
var commitSubcommands = []string{"check-message", "check-paths", "footers"}

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

// trailers are the footers as git commit's own --trailer arguments.
func trailers(lines []string) []string {
	var args []string
	for _, l := range lines {
		args = append(args, "--trailer", l)
	}
	return args
}

// gitCommit is `itos commit [--task <ids>] [--scenarios <ids>] [<git commit
// args>…]`: git commit with the footers as trailers, git's streams the
// terminal's (its stdout on stderr under --json, which prints the outcome),
// -q passed on as git's --quiet, and git's exit code handed back.
func gitCommit(args []string, o Out) (int, error) {
	flags, err := readCommitFlags(args)
	if err != nil {
		return 0, err
	}
	lines, err := flags.footerLines()
	if err != nil {
		return 0, err
	}
	argv := append([]string{"commit"}, trailers(lines)...)
	if o.Quiet {
		argv = append(argv, "--quiet")
	}
	argv = append(argv, flags.git...)
	stdout := o.Stdout
	if o.JSON {
		stdout = o.Stderr
	}
	code, err := runGit(argv, stdout, o)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: cannot run git: %s\n", err)
		return ExitMissing, nil
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
func runGit(argv []string, stdout io.Writer, o Out) (int, error) {
	cmd := exec.Command("git", argv...)
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
