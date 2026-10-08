package cli

// work done's code proof (slice 100, features/work.feature): with the
// config's proof.code, once the landing has passed, CI's run included, the
// item's commits (belonging, work show's rule) are asked whether they touch
// proof.code.paths; when they do, proof.code.check runs with {base}, the
// parent of the item's first commit, and the item closes only when it exits
// 0 (internal/proof reads its answer). Exit 1 refuses, exit 1, naming each
// problem its --json lists by its rule and message; any other answer, a
// check that cannot start included, refuses with exit 3. Nothing writes in
// between, and nothing closes past it.

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/message"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/proof"
	"github.com/donvargax/itos/v7/internal/tests"
)

// codeProof runs the item's code proof when it needs one: the sentence the
// close commit's body gains when it passed ("" when none ran), else the
// refusal, reported, its exit code 1 or 3.
func codeProof(cfg *config.Loaded, id string, o Out) (string, int, error) {
	if cfg.Proof == nil || cfg.Proof.Code == nil {
		return "", 0, nil
	}
	pc := cfg.Proof.Code
	commits, err := itemCommits(cfg, id)
	if err != nil || len(commits) == 0 {
		return "", 0, err
	}
	touched := false
	for _, c := range commits {
		files, err := git.CommitPaths(c.SHA)
		if err != nil {
			return "", 0, err
		}
		if touched, err = proof.Touches(pc.Paths, files); err != nil {
			return "", 0, err
		}
		if touched {
			break
		}
	}
	if !touched {
		return "", 0, nil
	}
	parent, err := git.Output("rev-parse", "--verify", "--quiet", commits[0].SHA+"^")
	if err != nil {
		code, err := refuseWork([]out.Problem{{
			Rule:    "work-done-proof-error",
			Message: fmt.Sprintf("%s's first commit, %s, has no parent, so its code proof has no base to judge from", id, commits[0].Short),
			Fix:     "ask the person: the code proof judges an item's commits from the parent of its first",
		}}, ExitMissing, o)
		return "", code, err
	}
	base := strings.TrimSpace(parent)
	command := proof.Command(pc.Check, base)
	fmt.Fprintf(o.Stderr, "itos: %s's commits touch proof.code.paths, so its code proof runs: %s\n", id, command)
	v := proof.Run(cfg, command, o.Stderr)
	switch v.Outcome {
	case proof.Passed:
		return fmt.Sprintf(" Its code proof passed, judged from %s.", short(base)), 0, nil
	case proof.Refused:
		code, err := refuseWork(proofProblems(id, v.Problems, "work-done-proof"), ExitPolicy, o)
		return "", code, err
	}
	found := append([]out.Problem{{
		Rule:    "work-done-proof-error",
		Message: fmt.Sprintf("%s's code proof could not run, so %s is not done: %s %s", id, id, command, v.Why),
		Fix:     "make proof.code.check run (its command, its tools), then run itos work done " + id + " again",
	}}, proofProblems(id, v.Problems, "work-done-proof-error")...)
	code, err := refuseWork(found, ExitMissing, o)
	return "", code, err
}

// proofProblems are the check's problems as work done's, under the rule,
// each naming the check's own rule and message, its fix the check's.
func proofProblems(id string, problems []proof.Problem, rule string) []out.Problem {
	fix := "make the code proof pass, then run itos work done " + id + " again; an equivalent mutant is excepted only " +
		"with its reason, which the person reviews, so ask them"
	found := []out.Problem{}
	for _, p := range problems {
		f := p.Fix
		if f == "" {
			f = fix
		}
		found = append(found, out.Problem{Rule: rule, Message: fmt.Sprintf("%s's code proof: %s: %s", id, p.Rule, p.Message), Fix: f})
	}
	if len(found) == 0 && rule == "work-done-proof" {
		found = append(found, out.Problem{Rule: rule, Message: id + "'s code proof failed and named no problem", Fix: fix})
	}
	return found
}

// itemCommits are the commits of HEAD's history that belong to the item,
// oldest first, as work show lists them (belonging).
func itemCommits(cfg *config.Loaded, id string) ([]message.Logged, error) {
	scenarios, err := tests.Tagged(cfg, "@"+id, "HEAD")
	if err != nil {
		return nil, err
	}
	history, err := message.History(cfg, "HEAD")
	if err != nil {
		return nil, err
	}
	return belonging(cfg, id, scenarios, history), nil
}
