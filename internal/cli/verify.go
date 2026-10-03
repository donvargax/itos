package cli

// verify is CI's re-check of a pushed range (verify-commits.ts), so a commit
// made with the hooks bypassed still fails the build: each non-merge
// commit's message (against the footers' sources at that commit), its paths
// and the built-in moves rule against its parent, then each kind's range
// commands once over the range. The commit commits.since names, and its
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

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/message"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/scope"
	"github.com/donvargax/itos/v2/internal/shell"
	"github.com/donvargax/itos/v2/internal/tests"
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
// and a failing one is named under its report.
func (v *verifier) commit(sha string) (verified, error) {
	text, err := git.Read("log", "-1", "--format=%B", sha)
	if err != nil {
		return verified{}, err
	}
	header, _, _ := strings.Cut(text, "\n")
	files, err := git.CommitPaths(sha)
	if err != nil {
		return verified{}, err
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
		if ok, err = v.holds(message.Type(text), files, sha); err != nil {
			return verified{}, err
		}
	}
	if !ok {
		fmt.Fprintf(v.o.Stderr, "  ^ %s %s\n", short(sha), header)
	}
	return verified{SHA: sha, Header: header, OK: ok}, nil
}

// holds is a commit's paths against its type's rules and its feature files
// against its parent's by the moves rule, the problems of both one
// rejection.
func (v *verifier) holds(typ string, files []string, sha string) (bool, error) {
	rules, err := scope.Of(v.cfg)
	if err != nil {
		return false, err
	}
	found, err := rules.Issues(typ, files)
	if err != nil {
		return false, err
	}
	moved, err := v.moves.Commit(sha, typ)
	if err != nil {
		return false, err
	}
	found = append(found, moved...)
	if len(found) > 0 {
		rules.Reject(v.o.Stderr, found)
	}
	return len(found) == 0, nil
}

// verifyRange is `verify <from> <to>`: 0 when every non-merge commit of the
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
	v := &verifier{cfg: cfg, moves: tests.NewMoves(cfg), log: o.Stdout, o: o}
	if o.JSON {
		v.log = o.Stderr
	}
	if missing := cfg.SinceIssues(); len(missing) > 0 {
		if o.JSON {
			return ExitUsage, out.Emit(o.Stdout,
				out.Field{Key: "range", Value: span{from, to}},
				out.Field{Key: "problems", Value: missing})
		}
		for _, p := range missing {
			fmt.Fprintf(o.Stderr, "FAIL %s\n", p.Message)
		}
		return ExitUsage, nil
	}
	start := cfg.Since()
	if start != "" {
		fmt.Fprintf(v.log, "Not checked: %s (commits.since) and its ancestors\n", short(start))
	}
	commits, err := git.Lines(append([]string{"rev-list", "--no-merges", "--reverse"}, cfg.RangeArgs(from, to)...)...)
	if err != nil {
		return 0, err
	}
	results := []verified{}
	passed := 0
	for _, sha := range commits {
		r, err := v.commit(sha)
		if err != nil {
			return 0, err
		}
		results = append(results, r)
		if r.OK {
			passed++
		}
	}
	fmt.Fprintf(v.log, "%d/%d commits pass the commit rules\n", passed, len(commits))
	ranged := true
	for _, command := range tests.RangeCommands(cfg, from, to) {
		if !shell.Run(cfg, command, shell.Options{Stdout: v.log, Stderr: o.Stderr}).OK() {
			ranged = false
			break
		}
	}
	if o.JSON {
		fields := []out.Field{{Key: "range", Value: span{from, to}}}
		if start != "" {
			fields = append(fields, out.Field{Key: "since", Value: start})
		}
		fields = append(fields,
			out.Field{Key: "commits", Value: results},
			out.Field{Key: "passed", Value: passed},
			out.Field{Key: "total", Value: len(commits)},
			out.Field{Key: "range_checks", Value: ranged})
		if err := out.Emit(o.Stdout, fields...); err != nil {
			return 0, err
		}
	}
	if passed < len(commits) || !ranged {
		return ExitPolicy, nil
	}
	return 0, nil
}
