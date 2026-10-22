package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/scope"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
)

// runCommit is a commit itself (gitCommit), every argument git commit's but
// itos's own flags, when there is no argument or the first is a flag or "--";
// a bare first word is itos's, one of commit's subcommands or a usage error
// (bug 5), so a mistyped subcommand never reaches git as a pathspec. Paths for
// git go after a flag or after "--".
func runCommit(args []string, o Out) (int, error) {
	sub, rest := split(args)
	if sub == "" || strings.HasPrefix(sub, "-") {
		return gitCommit(args, o)
	}
	switch sub {
	case "check-message":
		file, ok := first(positional(rest, "--at"))
		if !ok && !slices.Contains(rest, "-") {
			return 0, usage("commit check-message needs <file|->")
		}
		if !ok {
			file = "-"
		}
		at, _ := flagValue(rest, "--at")
		if err := commitRefs("commit check-message", at); err != nil {
			return 0, err
		}
		return checkMessage(typed(o.Origin, file), at, o)
	case "check-paths":
		typ, ok := flagValue(rest, "--type")
		if !ok {
			return 0, usage("commit check-paths needs --type <type>")
		}
		var files []string
		for _, f := range positional(rest, "--type") {
			files = append(files, filepath.ToSlash(typed(o.Origin, f)))
		}
		return checkPaths(typ, files, o)
	case "footers":
		footers := positional(rest)
		if len(footers) < 3 {
			return 0, usage("commit footers needs <name> <from> <to>")
		}
		if err := commitRefs("commit footers", footers[1], footers[2]); err != nil {
			return 0, err
		}
		return listFooters(footers[0], footers[1], footers[2], o)
	}
	return 0, usage("unknown command: commit %s", sub)
}

// checkMessage is `commit check-message <file|-> [--at <sha>]` (commit.ts's
// checkMessage): one message through the header lint's delegate and the
// footer rules, as the commit-msg hook reads it. 0 when it passes, 1 when
// not. --at reads the footers' IDs at that commit, as ITOS_AT does without
// it. Under a stealth config the footers are the ones ITOS_FOOTERS hands
// over, as itos commit hands them to the hook, never the message's.
func checkMessage(file, at string, o Out) (int, error) {
	var text string
	if file == "-" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return 0, err
		}
		text = string(raw)
	} else {
		var err error
		if text, err = trees.Worktree.Read(context.Background(), file); err != nil {
			return 0, err
		}
	}
	if at == "" {
		at = os.Getenv("ITOS_AT")
	}
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	return message.Check(cfg, text, message.Reading{At: at, Note: os.Getenv(message.FootersEnv), Warn: o.Stderr, Tests: o.Tests},
		message.Streams{JSON: o.JSON, Stdout: o.Stdout, Stderr: o.Stderr})
}

// checkPaths is `commit check-paths --type <t> <path>…` (commit-scope.ts's
// checkPaths): the path rules alone, for planning a split. 0 when they hold,
// 1 when not, the rejection on stderr or the problems under --json.
func checkPaths(typ string, files []string, o Out) (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	rules, err := scope.Of(cfg)
	if err != nil {
		return 0, err
	}
	found, err := rules.Issues(typ, files)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		if files == nil {
			files = []string{}
		}
		if err := out.Emit(o.Stdout,
			out.Field{Key: "type", Value: typ},
			out.Field{Key: "files", Value: files},
			out.Field{Key: "ok", Value: len(found) == 0},
			out.Field{Key: "problems", Value: found}); err != nil {
			return 0, err
		}
	} else if len(found) > 0 {
		rules.Reject(o.Stderr, found)
	}
	if len(found) > 0 {
		return ExitPolicy, nil
	}
	return 0, nil
}

// listFooters is `commit footers <name> <from> <to>`: the range's footers of
// commits.footers.<name>, a footer of free text, each with the commit it came
// from, leaving out those that say none, for a release's notes. One line
// each, the commit's short SHA and the text; under --json each with the full
// SHA and the subject. 2 when <name> is not a footer of free text.
func listFooters(name, from, to string, o Out) (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	if f, ok := cfg.Commits.Footers.Get(name); !ok || !f.Text() {
		return 0, usage("commit footers needs a footer of free text: commits.footers.%s is not one (source: text)", name)
	}
	said, err := message.Gathered(cfg, from, to, name)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout,
			out.Field{Key: "footer", Value: name},
			out.Field{Key: "range", Value: span{from, to}},
			out.Field{Key: "footers", Value: said})
	}
	for _, s := range said {
		fmt.Fprintf(o.Stdout, "%s %s\n", short(s.SHA), s.Text)
	}
	return 0, nil
}
