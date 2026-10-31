package review

import (
	"path/filepath"
	"regexp"
	"strings"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// The stages a review is of: the spec, by default, and the diff, the
// implementation's.
const (
	stageSpec = "spec"
	stageDiff = "diff"
)

// CLI is review's command line: the Query it asks, whether it prints
// --json's one object, and whether a success says nothing (the global -q).
type CLI struct {
	Query
	JSON, Quiet bool
}

// revisionForm is the shape of a revision itos next gives: "sha256:" and 64
// lowercase hex digits.
var revisionForm = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Parse reads the arguments internal/cli's flag spec hands the review
// command, with the global flags and origin, the folder the person stood in
// (relative to the top itos moved to, "" when it did not move), which the
// body file's path is read from. Every error is a usage error (out.Usage):
// review takes post or request and nothing else; post one item, --verdict
// and --at, and --stage spec or diff when given; request one item and
// --expect a revision when given. The verdict's word is Handle's to judge.
func Parse(args []string, origin string, json, quiet bool) (CLI, error) {
	sub, rest := argsengine.Split(args)
	switch sub {
	case "":
		return CLI{}, out.Usage("review needs post or request: itos review post <item> --verdict approve|changes --at <commit>, or itos review request <item>")
	case "post":
	case "request":
		q, err := parseRequest(rest)
		if err != nil {
			return CLI{}, err
		}
		return CLI{Query: q, JSON: json, Quiet: quiet}, nil
	default:
		return CLI{}, out.Usage("review has no subcommand %s: it takes post or request", sub)
	}
	pos, flags, _, err := argsengine.Sub("review", "post", rest, []string{"--verdict", "--at", "--stage", "--body-file"}, nil)
	if err != nil {
		return CLI{}, err
	}
	switch {
	case len(pos) == 0:
		return CLI{}, out.Usage("review post needs <item>, the item whose pull request the verdict goes on")
	case len(pos) > 1:
		return CLI{}, out.Usage("review post takes one <item>, not %s", strings.Join(pos, " "))
	}
	q := Query{Item: pos[0], Stage: stageSpec}
	var given bool
	if q.Verdict, given = flags["--verdict"]; !given {
		return CLI{}, out.Usage("review post needs --verdict approve|changes, the verdict")
	}
	if q.At, given = flags["--at"]; !given {
		return CLI{}, out.Usage("review post needs --at <commit>, the commit judged")
	}
	if stage, given := flags["--stage"]; given {
		if stage != stageSpec && stage != stageDiff {
			return CLI{}, out.Usage("review post --stage takes spec or diff, not %q", stage)
		}
		q.Stage = stage
	}
	q.BodyFile = typed(origin, flags["--body-file"])
	return CLI{Query: q, JSON: json, Quiet: quiet}, nil
}

// parseRequest is `review request <item> [--expect <revision>]`: one item,
// and a revision itos next gives when --expect is there.
func parseRequest(args []string) (Query, error) {
	pos, flags, _, err := argsengine.Sub("review", "request", args, []string{"--expect"}, nil)
	if err != nil {
		return Query{}, err
	}
	switch {
	case len(pos) == 0:
		return Query{}, out.Usage("review request needs <item>, the item whose pull request's head the reviewers are asked for")
	case len(pos) > 1:
		return Query{}, out.Usage("review request takes one <item>, not %s", strings.Join(pos, " "))
	}
	expect, given := flags["--expect"]
	if given && !revisionForm.MatchString(expect) {
		return Query{}, out.Usage("review request --expect takes the revision itos next gives, \"sha256:\" and 64 lowercase hex digits, not %s", expect)
	}
	return Query{Request: true, Item: pos[0], Expect: expect}, nil
}

// typed is a path the person typed, as itos reads it at the top it moved to
// from origin, the subfolder they stood in: relative to it, as git reads a
// path typed there. An absolute path, and every path of a run that did not
// move, stays as typed.
func typed(origin, p string) string {
	if origin == "" || p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(origin, p)
}
