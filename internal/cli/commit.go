package cli

import (
	"io"
	"os"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/message"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/scope"
	"github.com/donvargax/itos/internal/source"
)

// checkMessage is `commit check-message <file|-> [--at <sha>]` (commit.ts's
// checkMessage): one message through the header lint's delegate and the
// footer rules, as the commit-msg hook reads it. 0 when it passes, 1 when
// not. --at reads the footers' IDs at that commit, as ITOS_AT does without
// it.
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
	return message.Check(cfg, text, message.Reading{At: at, Warn: o.Stderr},
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
