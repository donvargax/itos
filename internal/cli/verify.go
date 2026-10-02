package cli

// verify is CI's re-check of a pushed range (verify-commits.ts), so a commit
// made with the hooks bypassed still fails the build: each non-merge
// commit's message (against the footers' sources at that commit), its paths
// and the built-in moves rule against its parent, then each kind's range
// commands once over the range. The commit commits.since names, and its
// ancestors, are left out, and the range checks start there: a history
// written before the rules (a template's squashed first commit, a project
// adopting itos) is not judged by them.

import (
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/git"
	"github.com/donvargax/itos/internal/message"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/scope"
	"github.com/donvargax/itos/internal/shell"
	"github.com/donvargax/itos/internal/tests"
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
	code, err := message.Check(v.cfg, text, message.Reading{At: sha, Warn: v.o.Stderr},
		message.Streams{Stdout: v.log, Stderr: v.o.Stderr})
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
// to to.
func verifyRange(from, to string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	v := &verifier{cfg: cfg, moves: tests.NewMoves(cfg), log: o.Stdout, o: o}
	if o.JSON {
		v.log = o.Stderr
	}
	if missing := cfg.SinceIssue(); missing != nil {
		if o.JSON {
			return ExitUsage, out.Emit(o.Stdout,
				out.Field{Key: "range", Value: span{from, to}},
				out.Field{Key: "problems", Value: []out.Problem{*missing}})
		}
		fmt.Fprintf(o.Stderr, "FAIL %s\n", missing.Message)
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
