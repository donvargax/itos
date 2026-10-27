package hook

import (
	"path/filepath"
	"slices"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/tests"
)

// CLI is hook's command line: the Query it asks, whether hook install
// prints --json's one object, and Tests, where the run's test listings
// warn.
type CLI struct {
	Query
	JSON  bool
	Tests *tests.Warnings
}

// Parse reads the arguments internal/cli's flag spec hands the hook
// command, with the global --json, origin, the folder the person stood in
// (relative to the top itos moved to, "" when it did not move), and the
// run's test warnings. commit-msg's file is read from origin, as git hands
// a path from there.
func Parse(args []string, json bool, origin string, warnings *tests.Warnings) (CLI, error) {
	sub, rest := argsengine.Split(args)
	c := CLI{Query: Query{Sub: sub}, JSON: json, Tests: warnings}
	switch sub {
	case "commit-msg":
		if len(rest) == 0 {
			return CLI{}, out.Usage("hook commit-msg needs <file>")
		}
		c.File = typed(origin, rest[0])
	case "pre-push":
		if len(rest) > 0 {
			c.Remote = rest[0]
		}
	case "install":
		c.Print, c.Force = slices.Contains(rest, "--print"), slices.Contains(rest, "--force")
	default:
		return CLI{}, out.Usage("unknown command: hook %s", sub)
	}
	return c, nil
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
