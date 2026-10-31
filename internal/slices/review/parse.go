package review

import (
	"path/filepath"
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

// review's words: post records a verdict, status posts the reviews' status.
const (
	subPost   = "post"
	subStatus = "status"
)

// CLI is review's command line: the Query it asks, whether it is status's
// (only the Query's Item then), whether it prints --json's one object, and
// whether a success says nothing (the global -q).
type CLI struct {
	Query
	Status      bool
	JSON, Quiet bool
}

// Parse reads the arguments internal/cli's flag spec hands the review
// command, with the global flags and origin, the folder the person stood in
// (relative to the top itos moved to, "" when it did not move), which the
// body file's path is read from. Every error is a usage error (out.Usage):
// review takes post or status and nothing else; post one item, --verdict and
// --at, and --stage spec or diff when given; status one item and no flag. The
// verdict's word is Handle's to judge.
func Parse(args []string, origin string, json, quiet bool) (CLI, error) {
	sub, rest := argsengine.Split(args)
	switch sub {
	case "":
		return CLI{}, out.Usage("review needs post or status: itos review post <item> --verdict approve|changes --at <commit>, or itos review status <item>")
	case subPost:
	case subStatus:
		return parseStatus(rest, json, quiet)
	default:
		return CLI{}, out.Usage("review has no subcommand %s: it takes post or status", sub)
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

// parseStatus reads review status's arguments: one item, and no flag.
func parseStatus(rest []string, json, quiet bool) (CLI, error) {
	pos, _, _, err := argsengine.Sub("review", subStatus, rest, nil, nil)
	if err != nil {
		return CLI{}, err
	}
	switch {
	case len(pos) == 0:
		return CLI{}, out.Usage("review status needs <item>, the item whose pull request's reviews are judged")
	case len(pos) > 1:
		return CLI{}, out.Usage("review status takes one <item>, not %s", strings.Join(pos, " "))
	}
	return CLI{Query: Query{Item: pos[0]}, Status: true, JSON: json, Quiet: quiet}, nil
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
