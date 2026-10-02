package cli

import (
	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/scope"
)

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
