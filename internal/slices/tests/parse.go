package tests

import (
	"path/filepath"
	"slices"
	"strconv"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is the tests command's line: the Query it asks, and how it answers,
// --json's one object or --quiet's silence on a rule that holds.
type CLI struct {
	Query
	JSON, Quiet bool
}

// Parse reads the arguments internal/cli's flag spec hands the tests
// command, origin being the folder the person stood in when the run moved
// to the top (cli.Out's Origin), which smoke check's --features is read
// from. Every error is a usage error (out.Usage).
func Parse(args []string, origin string, json, quiet bool) (CLI, error) {
	c := CLI{JSON: json, Quiet: quiet}
	sub, rest := argsengine.Split(args)
	var err error
	switch sub {
	case "list":
		name, ok := argsengine.First(argsengine.Positional(rest, "--at"))
		if !ok {
			return c, out.Usage("tests list needs <kind>")
		}
		at, _ := argsengine.FlagValue(rest, "--at")
		c.Query = Query{Sub: sub, Kind: name, At: at}
	case "moves":
		name, ok := argsengine.First(argsengine.Positional(rest))
		if !ok {
			return c, out.Usage("tests moves needs <kind>")
		}
		c.Query = Query{Sub: sub, Kind: name}
	case "next-id":
		c.Query, err = parseNextID(rest)
	case "smoke":
		c.Query, err = parseSmoke(rest, origin)
	default:
		return c, out.Usage("unknown command: tests %s", sub)
	}
	return c, err
}

// parseNextID reads `tests next-id <kind> <stem> [--count <n>]`: one count
// when --count is not given, else a number from 1 up.
func parseNextID(args []string) (Query, error) {
	words := argsengine.Positional(args, "--count")
	if len(words) != 2 {
		return Query{}, out.Usage("tests next-id needs <kind> <stem>")
	}
	text, counted := argsengine.FlagValue(args, "--count")
	count, err := strconv.Atoi(text)
	if !counted {
		count, err = 1, nil
	}
	if err != nil || count < 1 {
		return Query{}, out.Usage("tests next-id --count takes how many ids to claim, a number from 1 up, not %q", text)
	}
	return Query{Sub: "next-id", Kind: words[0], Stem: words[1], Count: count, Counted: counted}, nil
}

// parseSmoke reads `tests smoke check|ids|run <kind>`: check's --features
// as typed from origin, run's arguments after the kind.
func parseSmoke(args []string, origin string) (Query, error) {
	action, more := argsengine.Split(args)
	name, _ := argsengine.First(argsengine.Positional(more, "--features"))
	if name == "" {
		if len(args) == 0 {
			action = "check|ids|run"
		}
		return Query{}, out.Usage("tests smoke %s needs <kind>", action)
	}
	q := Query{Sub: "smoke", Action: action, Kind: name}
	switch action {
	case "check":
		if features, ok := argsengine.FlagValue(more, "--features"); ok {
			features = typed(origin, features)
			q.Features = &features
		}
	case "ids":
	case "run":
		// Everything after the kind is the runner's, as given; a -- right
		// after it is taken, so what looks like an itos flag can follow it,
		// and is not passed on (bug 8: they were read only after a --, and
		// --workers=1 without one was dropped without a word).
		q.Runner = more[slices.Index(more, name)+1:]
		if len(q.Runner) > 0 && q.Runner[0] == "--" {
			q.Runner = q.Runner[1:]
		}
	default:
		return Query{}, out.Usage("unknown command: tests smoke %s", action)
	}
	return q, nil
}

// typed is a path the person typed, as itos reads it at the top it moved to
// from origin, the subfolder they stood in: relative to it, as git reads a
// path typed there. An absolute path, "-" (stdin) and every path of a run
// that did not move stay as typed.
func typed(origin, p string) string {
	if origin == "" || p == "" || p == "-" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(origin, p)
}
