package cli

// verify is CI's re-check of a pushed range (verify-commits.ts), so a commit
// made with the hooks bypassed still fails the build: each commit's
// message (against the footers' sources at that commit), its paths
// and the built-in moves rule against its parent, a merge commit's by its
// own changes alone (bug 30), then each kind's range commands once over the
// range. The commit commits.since names, and its
// ancestors, are left out, and the range checks start there: a history
// written before the rules (a template's squashed first commit, a project
// adopting itos) is not judged by them. A footer's own since does the same
// for that footer's required_for, so a footer added to the rules later does
// not fail the commits written before it. Under a stealth config a commit's
// footers are its itos note's (message/notes.go).

import (
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/message"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/scope"
	"github.com/donvargax/itos/v6/internal/shell"
	"github.com/donvargax/itos/v6/internal/tests"
)

// verified is one commit's result, as --json lists it.
type verified struct {
	SHA    string `json:"sha"`
	Header string `json:"header"`
	OK     bool   `json:"ok"`
}

// span is a range as --json names it.
type span struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// short is a commit's abbreviation as the TypeScript's slice(0, 7) writes it.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// verifier is one run of verify: the config, the moves rule with its trees
// read once, and where the logs and the delegate's report go (stderr under
// --json).
type verifier struct {
	cfg   *config.Loaded
	moves *tests.Moves
	log   io.Writer
	o     Out
}

// commit is one commit: its message at that commit, then its paths and its
// moves, one rejection. A commit whose message fails is not judged further,
// and a failing one is named under its report. A merge commit is judged by
// its own changes (git.OwnPaths, bug 30): one with none passes whatever its
// message says, as the commits that brought its changes were judged, and
// one with some is judged by them, as its first line's type.
func (v *verifier) commit(sha string) (verified, error) {
	text, err := git.Read("log", "-1", "--format=%B", sha)
	if err != nil {
		return verified{}, err
	}
	header, _, _ := strings.Cut(text, "\n")
	parents, err := git.Parents(sha)
	if err != nil {
		return verified{}, err
	}
	var files []string
	merge := len(parents) > 1
	if merge {
		files, err = git.OwnPaths(sha)
	} else {
		files, err = git.CommitPaths(sha)
	}
	if err != nil {
		return verified{}, err
	}
	if merge && len(files) == 0 {
		return verified{SHA: sha, Header: header, OK: true}, nil
	}
	reading := message.Reading{At: sha, Made: true, Warn: v.o.Stderr}
	if v.cfg.Stealth {
		if reading.Note, err = message.Note(sha); err != nil {
			return verified{}, err
		}
	}
	code, err := message.Check(v.cfg, text, reading, message.Streams{Stdout: v.log, Stderr: v.o.Stderr})
	if err != nil {
		return verified{}, err
	}
	ok := code == 0
	if ok {
		judged := judged{typ: message.Type(text), files: files, after: sha, merge: merge}
		if merge {
			judged.parent = parents[0]
		}
		if ok, err = v.holds(judged); err != nil {
			return verified{}, err
		}
	}
	if !ok {
		fmt.Fprintf(v.o.Stderr, "  ^ %s %s\n", short(sha), header)
	}
	return verified{SHA: sha, Header: header, OK: ok}, nil
}

// judged is a commit as its paths and moves are judged: its type, the paths
// it changes (a merge's own), the commit, and for a merge its first parent.
type judged struct {
	typ           string
	files         []string
	after, parent string
	merge         bool
}

// holds is a commit's paths against its type's rules and its feature files
// against its parent's by the moves rule, a merge's by its own changes, the
// problems of both one rejection, after a merge's with no type (untypedMerge).
func (v *verifier) holds(c judged) (bool, error) {
	rules, err := scope.Of(v.cfg)
	if err != nil {
		return false, err
	}
	found := untypedMerge(v.cfg, c)
	paths, err := rules.Issues(c.typ, c.files)
	if err != nil {
		return false, err
	}
	found = append(found, paths...)
	var moved []out.Problem
	if c.merge {
		moved, err = v.moves.Merge(c.typ, c.parent, c.after, c.files)
	} else {
		moved, err = v.moves.Commit(c.after, c.typ)
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

// untypedMerge is a merge's problem when its first line has no type of the
// commit types (bug 30): judged by its own changes, a merge with some is
// judged as its type, and with none the rules would not judge them at all.
// Nothing for any other commit, whose type is the header lint's to refuse.
func untypedMerge(cfg *config.Loaded, c judged) []out.Problem {
	if !c.merge || message.Typed(cfg, c.typ) {
		return nil
	}
	paths := strings.Join(c.files, ", ")
	if len(c.files) > 3 {
		paths = fmt.Sprintf("%s and %d more", strings.Join(c.files[:3], ", "), len(c.files)-3)
	}
	return []out.Problem{{
		Rule: "merge-type",
		Message: fmt.Sprintf("a merge commit with changes of its own (%s) needs one of the commit types in its first line",
			paths),
		Fix: "start the merge's first line with the type of its own changes, as in `chore: merge …`, or leave them to a commit of their own",
	}}
}

// verifyRange is `verify <from> <to>`: 0 when every commit of the
// range passes and its range checks hold, else 1; 2 when commits.since is no
// commit here. from may be empty or all zeros (a new branch): every commit up
// to to; or git.Unpushed, the commits of to on no remote branch, which
// `verify` with no range judges under a stealth config (slice 34), since
// others' commits in a repository that does not use itos follow no rules of
// the person's.
func verifyRange(from, to string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	if missing := cfg.SinceIssues(); len(missing) > 0 {
		if o.JSON {
			return ExitUsage, out.Emit(o.Stdout,
				out.Field{Key: "range", Value: span{from, to}},
				out.Field{Key: "problems", Value: missing})
		}
		printSinceIssues(missing, o)
		return ExitUsage, nil
	}
	v := newVerifier(cfg, o)
	run, err := v.over(from, to)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		fields := []out.Field{{Key: "range", Value: span{from, to}}}
		if run.since != "" {
			fields = append(fields, out.Field{Key: "since", Value: run.since})
		}
		fields = append(fields,
			out.Field{Key: "commits", Value: run.results},
			out.Field{Key: "passed", Value: run.passed},
			out.Field{Key: "total", Value: len(run.results)},
			out.Field{Key: "range_checks", Value: run.ranged})
		if err := out.Emit(o.Stdout, fields...); err != nil {
			return 0, err
		}
	}
	if !run.ok() {
		return ExitPolicy, nil
	}
	return 0, nil
}

// printSinceIssues says commits.since is no commit here, one FAIL line a
// problem.
func printSinceIssues(missing []out.Problem, o Out) {
	for _, p := range missing {
		fmt.Fprintf(o.Stderr, "FAIL %s\n", p.Message)
	}
}

// newVerifier is a run of verify writing to o: its logs on stdout, or on
// stderr under --json.
func newVerifier(cfg *config.Loaded, o Out) *verifier {
	v := &verifier{cfg: cfg, moves: tests.NewMoves(cfg), log: o.Stdout, o: o}
	if o.JSON {
		v.log = o.Stderr
	}
	return v
}

// verifyRun is what verify found over one range: commits.since when the
// config names one, each commit's result, how many passed, and whether the
// range checks held.
type verifyRun struct {
	since   string
	results []verified
	passed  int
	ranged  bool
}

// ok is whether every commit passed and the range checks held.
func (r verifyRun) ok() bool { return r.passed == len(r.results) && r.ranged }

// over judges the range's commits after commits.since, printing each
// failing one's report and the count, then runs each range command once,
// up to the first that fails.
func (v *verifier) over(from, to string) (verifyRun, error) {
	run := verifyRun{since: v.cfg.Since(), results: []verified{}, ranged: true}
	if run.since != "" {
		fmt.Fprintf(v.log, "Not checked: %s (commits.since) and its ancestors\n", short(run.since))
	}
	commits, err := git.Lines(append([]string{"rev-list", "--reverse"}, v.cfg.RangeArgs(from, to)...)...)
	if err != nil {
		return run, err
	}
	for _, sha := range commits {
		r, err := v.commit(sha)
		if err != nil {
			return run, err
		}
		run.results = append(run.results, r)
		if r.OK {
			run.passed++
		}
	}
	fmt.Fprintf(v.log, "%d/%d commits pass the commit rules\n", run.passed, len(commits))
	for _, command := range tests.RangeCommands(v.cfg, from, to) {
		if !shell.Run(v.cfg, command, shell.Options{Stdout: v.log, Stderr: v.o.Stderr}).OK() {
			run.ranged = false
			break
		}
	}
	return run, nil
}
