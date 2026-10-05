package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/donvargax/itos/v4/internal/config"
	"github.com/donvargax/itos/v4/internal/message"
	"github.com/donvargax/itos/v4/internal/out"
	"github.com/donvargax/itos/v4/internal/scope"
	"github.com/donvargax/itos/v4/internal/source"
)

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
		if text, err = source.Worktree.Read(file); err != nil {
			return 0, err
		}
	}
	if at == "" {
		at = os.Getenv("ITOS_AT")
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	return message.Check(cfg, text, message.Reading{At: at, Note: os.Getenv(message.FootersEnv), Warn: o.Stderr},
		message.Streams{JSON: o.JSON, Stdout: o.Stdout, Stderr: o.Stderr})
}

// checkPaths is `commit check-paths --type <t> <path>…` (commit-scope.ts's
// checkPaths): the path rules alone, for planning a split. 0 when they hold,
// 1 when not, the rejection on stderr or the problems under --json.
func checkPaths(typ string, files []string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
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
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	if f, ok := cfg.Commits.Footers.Get(name); !ok || !f.Text() {
		return 0, usage("commit footers needs a footer of free text: commits.footers.%s is not one (source: text)", name)
	}
	said, err := message.Gathered(from, to, name)
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
