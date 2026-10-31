package review

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/work"
)

// statusContext is the commit status's context, the name branch protection
// requires.
const statusContext = "itos/reviews"

// descriptionLimit is the longest description GitHub takes; it refuses (422)
// a longer one rather than cutting it.
const descriptionLimit = 140

// The states of the status, and the outcome --json names for each.
const (
	stateSuccess = "success"
	statePending = "pending"
	stateFailure = "failure"
)

var outcomes = map[string]string{
	stateSuccess: "reviewed",
	statePending: "review-pending",
	stateFailure: "review-refused",
}

// StatusAnswer is what Status did: Code, its exit; Problems, what refused
// it, nothing posted; else the Item, the State posted and its Description,
// the Head it went on (the full SHA), the pull request's Address and Number,
// the review_by logins (Logins, as the config writes them), those Missing a
// verdict for the head and those that Refused, each with its last refusal.
type StatusAnswer struct {
	Code        int
	Problems    []out.Problem
	Item        string
	State       string
	Description string
	Head        string
	Address     string
	Number      int
	Logins      []string
	Missing     []string
	Refused     []providers.Refusal
}

// Status judges the reviews of item's pull request's head as --ready judges
// them (providers.PullState.Verdicts) and posts the itos/reviews commit
// status on that head, or refuses (doc.go says the order).
func (h Handler) Status(ctx context.Context, item string) (StatusAnswer, error) {
	refuse := func(code int, rule, message, fix string) StatusAnswer {
		return StatusAnswer{Code: code, Problems: []out.Problem{{Rule: rule, Message: message, Fix: fix}}}
	}
	from := func(a Answer) StatusAnswer { return StatusAnswer{Code: a.Code, Problems: a.Problems} }
	cfg, pr := h.prMode(ctx)
	if !pr {
		return refuse(policy, "review-without-pr-mode",
			"itos review: the reviews' status goes on an item's pull request, and this config does not set push.mode: pr; nothing was posted.",
			"set push.mode: pr in the config"), nil
	}
	logins := cfg.Push.PR.ReviewBy
	if len(logins) == 0 {
		return refuse(policy, "review-without-review-by",
			"itos review: push.pr.review_by names no login whose review to judge; nothing was posted.",
			"list the logins whose reviews the merge waits on in push.pr.review_by"), nil
	}
	registry := cfg.Work.Registry
	if !cfg.Source.Has(ctx, registry) {
		return StatusAnswer{Code: policy, Problems: []out.Problem{work.Missing(registry, false)}}, nil
	}
	r, err := work.Load(cfg, registry)
	if err != nil {
		return StatusAnswer{}, err
	}
	if _, problem := work.Show(r, item); problem != nil {
		return StatusAnswer{Code: policy, Problems: []out.Problem{*problem}}, nil
	}
	remote := h.ports.Remote(ctx)
	repository := h.ports.GitHubRepository(ctx, cfg, remote)
	if repository == "" {
		return refuse(missing, "no-github-repository",
			fmt.Sprintf("itos review: %s names no GitHub repository to find %s's pull request in; nothing was posted.", remote, item),
			"set ci.range.github.repository_env to a variable naming it, or give the remote a GitHub address"), nil
	}
	number, address, a, failed := h.openPull(ctx, subStatus, repository, item)
	if failed {
		return from(a), nil
	}
	state, err := providers.AskPullState(ctx, h.ports.Gh, repository, number)
	switch {
	case errors.Is(err, providers.ErrNoPullState):
		return from(ghFailed(subStatus, "gh api graphql", err.Error())), nil
	case err != nil:
		return from(ghFailed(subStatus, "gh api graphql", providers.GhReason(err))), nil
	case state.Head == "":
		return from(ghFailed(subStatus, "gh api graphql", "its answer names no head for the pull request")), nil
	}
	s := StatusAnswer{Item: item, Head: state.Head, Address: address, Number: number, Logins: logins}
	s.Missing, s.Refused = state.Verdicts(logins, state.Head)
	s.State, s.Description = judged(s)
	path := "repos/" + repository + "/statuses/" + state.Head
	if _, err := h.ports.Gh(ctx, "api", path, "-f", "state="+s.State, "-f", "context="+statusContext, "-f", "description="+s.Description); err != nil {
		return from(ghFailed(subStatus, "gh api "+path, providers.GhReason(err))), nil
	}
	return s, nil
}

// judged is the status's state and description: failure when a login
// refused, whatever the others'; else pending when a login has no verdict;
// else success; the description naming the logins of its state, cut to
// GitHub's limit (described).
func judged(s StatusAnswer) (state, description string) {
	switch {
	case len(s.Refused) > 0:
		return stateFailure, described("Refused to review: ", refusedLogins(s.Refused))
	case len(s.Missing) > 0:
		return statePending, described("Waiting on ", s.Missing)
	}
	return stateSuccess, described("Reviewed by ", s.Logins)
}

// refusedLogins are the logins that refused, in the config's order.
func refusedLogins(refused []providers.Refusal) []string {
	logins := []string{}
	for _, f := range refused {
		logins = append(logins, f.Login)
	}
	return logins
}

// described is the beginning, the logins joined by ", " and a full stop,
// when that is within descriptionLimit; else the first logins, as many as
// fit, and " and <n> more." for the n left out; else, when not even the first
// fits, the beginning and "<n> logins.".
func described(beginning string, logins []string) string {
	fits := func(text string) bool { return utf8.RuneCountInString(text) <= descriptionLimit }
	if whole := beginning + strings.Join(logins, ", ") + "."; fits(whole) {
		return whole
	}
	for kept := len(logins) - 1; kept > 0; kept-- {
		cut := beginning + strings.Join(logins[:kept], ", ") + " and " + strconv.Itoa(len(logins)-kept) + " more."
		if fits(cut) {
			return cut
		}
	}
	return beginning + strconv.Itoa(len(logins)) + " logins."
}
