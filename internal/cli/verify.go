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
// footers are its itos note's (message/notes.go). The walk is
// internal/engine/judge's (judge.Over), bound here to the config's VCS.

import (
	"context"
	"fmt"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/out"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

func runVerify(args []string, o Out) (int, error) {
	if len(args) == 0 && config.IsStealth(vcs.Git{}, configPath()) {
		return verifyRange(vcsport.Unpushed, "HEAD", o)
	}
	if len(args) < 2 {
		return 0, usage("verify needs <from> <to>")
	}
	if err := commitRefs("verify", args[0], args[1]); err != nil {
		return 0, err
	}
	return verifyRange(args[0], args[1], o)
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

// verifyRange is `verify <from> <to>`: 0 when every commit of the
// range passes and its range checks hold, else 1; 2 when commits.since is no
// commit here. from may be empty or all zeros (a new branch): every commit up
// to to; or vcsport.Unpushed, the commits of to on no remote branch, which
// `verify` with no range judges under a stealth config (slice 34), since
// others' commits in a repository that does not use itos follow no rules of
// the person's.
func verifyRange(from, to string, o Out) (int, error) {
	cfg, err := loadConfig()
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
	run, err := judge.Over(context.Background(), cfg, cfg.VCS, from, to, judgeOut(o))
	if err != nil {
		return 0, err
	}
	if o.JSON {
		fields := []out.Field{{Key: "range", Value: span{from, to}}}
		if run.Since != "" {
			fields = append(fields, out.Field{Key: "since", Value: run.Since})
		}
		fields = append(fields,
			out.Field{Key: "commits", Value: run.Results},
			out.Field{Key: "passed", Value: run.Passed},
			out.Field{Key: "total", Value: len(run.Results)},
			out.Field{Key: "range_checks", Value: run.Ranged})
		if err := out.Emit(o.Stdout, fields...); err != nil {
			return 0, err
		}
	}
	if !run.OK() {
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

// judgeOut is o as the judge's writers and flags.
func judgeOut(o Out) judge.Out {
	return judge.Out{Stdout: o.Stdout, Stderr: o.Stderr, JSON: o.JSON, Tests: o.Tests}
}
