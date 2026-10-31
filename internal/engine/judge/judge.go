package judge

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/scope"
	"github.com/donvargax/itos/v8/internal/shell"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Out is where a run writes: its logs on Stdout, or on Stderr under JSON,
// its rejections and the delegate's report on Stderr, and the warnings the
// tests' configuration gives into Tests.
type Out struct {
	Stdout, Stderr io.Writer
	JSON           bool
	Tests          *tests.Warnings
}

// Verified is one commit's result, as --json lists it.
type Verified struct {
	SHA    string `json:"sha"`
	Header string `json:"header"`
	OK     bool   `json:"ok"`
}

// Run is what verify found over one range: commits.since when the config
// names one, each commit's result, how many passed, and whether the range
// checks held.
type Run struct {
	Since   string
	Results []Verified
	Passed  int
	Ranged  bool
}

// OK is whether every commit passed and the range checks held.
func (r Run) OK() bool { return r.Passed == len(r.Results) && r.Ranged }

// short is a commit's abbreviation as the TypeScript's slice(0, 7) writes it.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// verifier is one run of verify: the config, the repository, the moves rule
// with its trees read once, and where the logs and the delegate's report go
// (stderr under --json).
type verifier struct {
	cfg   *config.Loaded
	vcs   vcsport.VCS
	moves *tests.Moves
	log   io.Writer
	o     Out
}

// Over judges the range's commits after commits.since in repo, printing each
// failing one's report and the count, then runs each range command once, up
// to the first that fails.
func Over(ctx context.Context, cfg *config.Loaded, repo vcsport.VCS, from, to string, o Out) (Run, error) {
	return newVerifier(cfg, repo, o).over(ctx, from, to, func() ([]string, error) {
		return repo.Commits(ctx, cfg.Range(from, to))
	})
}

// OverUnpushed is Over judging, of the range's commits, those no remote branch
// has (slice-213): the pre-push hook's for a ref the remote has, so the
// commits a merge of main brings in, judged when they reached the remote, are
// not judged again. commits.since and its ancestors are left out apart from
// from's, since the two may be on histories HEAD merges; the range commands
// still take from as their start.
func OverUnpushed(ctx context.Context, cfg *config.Loaded, repo vcsport.VCS, from, to string, o Out) (Run, error) {
	return newVerifier(cfg, repo, o).over(ctx, from, to, func() ([]string, error) {
		commits, err := repo.Commits(ctx, vcsport.Range{From: vcsport.Unpushed, To: to, Since: from})
		if since := cfg.Since(); since != "" {
			commits = slices.DeleteFunc(commits, func(c string) bool { return repo.IsAncestor(ctx, c, since) })
		}
		return commits, err
	})
}

// newVerifier is the run of verify over repo that writes where o says.
func newVerifier(cfg *config.Loaded, repo vcsport.VCS, o Out) *verifier {
	v := &verifier{cfg: cfg, vcs: repo, moves: tests.NewMoves(cfg, o.Tests), log: o.Stdout, o: o}
	if o.JSON {
		v.log = o.Stderr
	}
	return v
}

// over is the walk of Over and OverUnpushed: the commits judged lists, then
// the range commands from from to to.
func (v *verifier) over(ctx context.Context, from, to string, judged func() ([]string, error)) (Run, error) {
	run := Run{Since: v.cfg.Since(), Results: []Verified{}, Ranged: true}
	if run.Since != "" {
		fmt.Fprintf(v.log, "Not checked: %s (commits.since) and its ancestors\n", short(run.Since))
	}
	commits, err := judged()
	if err != nil {
		return run, err
	}
	for _, sha := range commits {
		r, err := v.commit(ctx, sha)
		if err != nil {
			return run, err
		}
		run.Results = append(run.Results, r)
		if r.OK {
			run.Passed++
		}
	}
	fmt.Fprintf(v.log, "%d/%d commits pass the commit rules\n", run.Passed, len(commits))
	for _, command := range tests.RangeCommands(v.cfg, from, to) {
		if !shell.Run(v.cfg, command, shell.Options{Stdout: v.log, Stderr: v.o.Stderr}).OK() {
			run.Ranged = false
			break
		}
	}
	return run, nil
}

// commit is one commit: its message at that commit, then its paths and its
// moves, one rejection. A commit whose message fails is not judged further,
// and a failing one is named under its report. A merge commit is judged by
// its own changes (VCS.OwnPaths, bug 30): one with none passes whatever its
// message says, as the commits that brought its changes were judged, and
// one with some is judged by them, as its first line's type.
func (v *verifier) commit(ctx context.Context, sha string) (Verified, error) {
	shown, err := v.vcs.Show(ctx, sha)
	if err != nil {
		return Verified{}, err
	}
	text := shown.Message
	header, _, _ := strings.Cut(text, "\n")
	parents, err := v.vcs.Parents(ctx, sha)
	if err != nil {
		return Verified{}, err
	}
	var files []string
	merge := len(parents) > 1
	if merge {
		files, err = v.vcs.OwnPaths(ctx, sha)
	} else {
		files, err = v.vcs.CommitPaths(ctx, sha)
	}
	if err != nil {
		return Verified{}, err
	}
	if merge && len(files) == 0 {
		return Verified{SHA: sha, Header: header, OK: true}, nil
	}
	reading := message.Reading{At: sha, Made: true, Warn: v.o.Stderr, Tests: v.o.Tests}
	if v.cfg.Stealth {
		reading.Note = shown.Note
	}
	code, err := message.Check(v.cfg, text, reading, message.Streams{Stdout: v.log, Stderr: v.o.Stderr})
	if err != nil {
		return Verified{}, err
	}
	ok := code == 0
	if ok {
		footers := text
		if v.cfg.Stealth {
			footers = reading.Note
		}
		judged := Judged{Type: message.Type(text), Files: files, After: sha, Merge: merge, Footers: footers}
		if merge {
			judged.Parent = parents[0]
		}
		if ok, err = v.holds(judged); err != nil {
			return Verified{}, err
		}
	}
	if !ok {
		fmt.Fprintf(v.o.Stderr, "  ^ %s %s\n", short(sha), header)
	}
	return Verified{SHA: sha, Header: header, OK: ok}, nil
}

// Judged is a commit as its paths and moves are judged: its type, the paths
// it changes (a merge's own), the commit ("index" for one being made), for a
// merge its first parent, and the footers it carries, its message's or under
// a stealth config its note's, whose named tests may bring a test live
// (slice 134).
type Judged struct {
	Type          string
	Files         []string
	After, Parent string
	Merge         bool
	Footers       string
}

// holds is a commit's paths against its type's rules and its feature files
// against its parent's by the moves rule, a merge's by its own changes, the
// problems of both one rejection, after a merge's with no type (UntypedMerge).
func (v *verifier) holds(c Judged) (bool, error) {
	rules, err := scope.Of(v.cfg)
	if err != nil {
		return false, err
	}
	found := UntypedMerge(v.cfg, c)
	paths, err := rules.Issues(c.Type, c.Files)
	if err != nil {
		return false, err
	}
	found = append(found, paths...)
	var moved []out.Problem
	change := MovesChange(v.cfg, c.Footers, c.Files)
	if c.Merge {
		moved, err = v.moves.Merge(c.Type, c.Parent, c.After, c.Files, change)
	} else {
		moved, err = v.moves.Commit(c.After, c.Type, change)
	}
	if err != nil {
		return false, err
	}
	found = append(found, moved...)
	if len(found) > 0 {
		rules.Reject(v.o.Stderr, found)
	}
	return len(found) == 0, nil
}

// UntypedMerge is a merge's problem when its first line has no type of the
// commit types (bug 30): judged by its own changes, a merge with some is
// judged as its type, and with none the rules would not judge them at all.
// Nothing for any other commit, whose type is the header lint's to refuse.
func UntypedMerge(cfg *config.Loaded, c Judged) []out.Problem {
	if !c.Merge || message.Typed(cfg, c.Type) {
		return nil
	}
	paths := strings.Join(c.Files, ", ")
	if len(c.Files) > 3 {
		paths = fmt.Sprintf("%s and %d more", strings.Join(c.Files[:3], ", "), len(c.Files)-3)
	}
	return []out.Problem{{
		Rule: "merge-type",
		Message: fmt.Sprintf("a merge commit with changes of its own (%s) needs one of the commit types in its first line",
			paths),
		Fix: "start the merge's first line with the type of its own changes, as in `chore: merge …`, or leave them to a commit of their own",
	}}
}

// MovesChange is a commit as the moves rule reads it beside its trees: the
// IDs its footers (a message, or a note) name, by kind of named tests, and
// the paths it changes.
func MovesChange(cfg *config.Loaded, footers string, paths []string) tests.Change {
	change := tests.Change{Named: map[string][]string{}, Paths: paths}
	for _, l := range message.Links(cfg, footers) {
		if l.Tests != "" {
			change.Named[l.Tests] = append(change.Named[l.Tests], l.ID)
		}
	}
	return change
}
