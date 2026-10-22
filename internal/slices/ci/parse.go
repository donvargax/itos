package ci

import (
	"slices"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is the ci command's line: the Query it asks, and how it answers,
// --json's one object or --quiet's silence on a watch's progress.
type CLI struct {
	Query
	JSON, Quiet bool
}

// Parse reads the arguments internal/cli's flag spec hands the ci command:
// the subcommand, a range's ends where it takes one, its switches and
// values. ci plan with no range and neither --nightly nor --whole is Bare,
// which only a stealth config takes (Handle asks). Every error is a usage
// error (out.Usage).
func Parse(args []string, json, quiet bool) (CLI, error) {
	c := CLI{JSON: json, Quiet: quiet}
	sub, rest := argsengine.Split(args)
	ends := argsengine.Positional(rest, "--data-at", "--head", "--base")
	q := Query{Sub: sub, Nightly: slices.Contains(rest, "--nightly")}
	if len(ends) > 0 {
		q.From = ends[0]
	}
	if len(ends) > 1 {
		q.To = ends[1]
	}
	switch sub {
	case "plan":
		q.Whole = slices.Contains(rest, "--whole")
		q.Bare = !q.Nightly && !q.Whole && len(ends) == 0
		if !q.Nightly && !q.Whole && !q.Bare && q.To == "" {
			return c, needsRange()
		}
		q.DataAt, _ = argsengine.FlagValue(rest, "--data-at")
	case "run":
		q.Tests = slices.Contains(rest, "--tests")
	case "scope":
		if q.To == "" {
			return c, out.Usage("ci scope needs <from> <to>")
		}
	case "watch":
		words := argsengine.Positional(rest)
		if len(words) > 1 {
			return c, out.Usage("ci watch takes at most one commit (%s)", words[1])
		}
		q.Rev = "HEAD"
		if len(words) == 1 {
			q.Rev = words[0]
		}
	case "range":
		head, ok := argsengine.FlagValue(rest, "--head")
		if !ok {
			return c, out.Usage("ci range needs --head <sha>")
		}
		q.Head = head
		q.Base, _ = argsengine.FlagValue(rest, "--base")
	default:
		return c, out.Usage("unknown command: ci %s", sub)
	}
	c.Query = q
	return c, nil
}

// needsRange is the usage error of ci plan given no range, neither
// --nightly nor --whole.
func needsRange() error {
	return out.Usage("ci plan needs <from> <to>, --nightly or --whole")
}
