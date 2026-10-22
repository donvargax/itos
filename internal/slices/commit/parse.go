package commit

import (
	"path/filepath"
	"slices"
	"strings"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/tests"
)

// CLI is commit's command line: the Query it asks, whether it prints
// --json's one object, whether git is quiet (the global -q), and Tests,
// where the run's test listings warn.
type CLI struct {
	Query
	JSON, Quiet bool
	Tests       *tests.Warnings
}

// Parse reads the arguments internal/cli's flag spec hands the commit
// command, with the global --json and -q, origin, the folder the person
// stood in (relative to the top itos moved to, "" when it did not move),
// and the run's test warnings. A commit itself (Git) when there is no
// argument or the first is a flag or "--", every argument git commit's but
// itos's own flags; a bare first word is itos's, one of commit's
// subcommands or a usage error (bug 5), so a mistyped subcommand never
// reaches git as a pathspec. Paths for git go after a flag or after "--".
// A path a subcommand is given is read from origin, as git reads one typed
// there.
func Parse(args []string, json, quiet bool, origin string, warnings *tests.Warnings) (CLI, error) {
	sub, rest := argsengine.Split(args)
	if sub == "" || strings.HasPrefix(sub, "-") {
		return Git(args, json, quiet, origin, warnings), nil
	}
	c := CLI{Query: Query{Sub: sub, Origin: origin}, JSON: json, Quiet: quiet, Tests: warnings}
	switch sub {
	case "check-message":
		file, ok := argsengine.First(argsengine.Positional(rest, "--at"))
		if !ok && !slices.Contains(rest, "-") {
			return CLI{}, out.Usage("commit check-message needs <file|->")
		}
		if !ok {
			file = "-"
		}
		c.At, _ = argsengine.FlagValue(rest, "--at")
		c.File = typed(origin, file)
	case "check-paths":
		typ, ok := argsengine.FlagValue(rest, "--type")
		if !ok {
			return CLI{}, out.Usage("commit check-paths needs --type <type>")
		}
		c.Type = typ
		for _, f := range argsengine.Positional(rest, "--type") {
			c.Files = append(c.Files, filepath.ToSlash(typed(origin, f)))
		}
	case "footers":
		footers := argsengine.Positional(rest)
		if len(footers) < 3 {
			return CLI{}, out.Usage("commit footers needs <name> <from> <to>")
		}
		c.Footer, c.From, c.To = footers[0], footers[1], footers[2]
	default:
		return CLI{}, out.Usage("unknown command: commit %s", sub)
	}
	return c, nil
}

// Git is a commit itself, args every argument of it, whatever the first
// is: the git shim's git commit, whose arguments are git's.
func Git(args []string, json, quiet bool, origin string, warnings *tests.Warnings) CLI {
	return CLI{Query: Query{Args: args, Origin: origin}, JSON: json, Quiet: quiet, Tests: warnings}
}

// typed is a path the person typed, as itos reads it at the top it moved to
// from origin, the subfolder they stood in (Out.Origin): relative to it, as
// git reads a path typed there. An absolute path, "-" (stdin) and every path
// of a run that did not move stay as typed.
func typed(origin, p string) string {
	if origin == "" || p == "" || p == "-" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(origin, p)
}
