package review

import (
	"context"
	"fmt"
	"strconv"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/reviewers"
	"github.com/donvargax/itos/v8/internal/providers"
)

// The outcomes of review request: one request or more posted, and every
// request of every reviewer on the head already, nothing posted.
const requested = "requested"

// request asks each reviewer push.pr.reviewers names for the head of the
// item's pull request, each request not on it posted once, or refuses
// (doc.go says the order).
func (h Handler) request(ctx context.Context, q Query) (Answer, error) {
	cfg, _, refused, err := h.known(ctx, q.Item, "a request", "ask the reviewers yourself")
	switch {
	case err != nil:
		return Answer{}, err
	case refused != nil:
		return *refused, nil
	case len(cfg.Push.PR.Reviewers) == 0:
		return refuse(policy, "review-no-reviewers",
			"itos review: push.pr.reviewers names no reviewer to ask; nothing was posted.",
			"list the reviewers to ask, each {login, kind}, in push.pr.reviewers"), nil
	}
	p, refused := h.pull(ctx, cfg, "request", q.Item)
	if refused != nil {
		return *refused, nil
	}
	head := p.state.Head
	a := Answer{Outcome: already, Item: q.Item, Head: head, Address: p.address, Posted: []reviewers.Request{}}
	asks := reviewers.Missing(cfg.Push.PR.Reviewers, p.state, head)
	if len(asks) == 0 {
		return a, nil
	}
	stale, err := h.stale(ctx, cfg, q)
	switch {
	case err != nil:
		return Answer{}, err
	case stale != nil:
		return *stale, nil
	}
	for _, r := range asks {
		if _, err := h.ports.Gh(ctx, "pr", "comment", strconv.Itoa(p.number), "--repo", p.repository, "--body="+r.Body(head)); err != nil {
			return refuse(missing, "review-pull-request-failed",
				fmt.Sprintf("itos review: gh pr comment failed: %s; the requests before it stay posted, and none after it was tried.", providers.GhReason(err)),
				"fix that, then itos review request "+q.Item+" again, which asks only for what is still missing"), nil
		}
		a.Posted = append(a.Posted, r)
	}
	a.Outcome = requested
	return a, nil
}

// stale is the review-stale refusal of --expect's revision when it is not
// the item's current one, as itos next gives it (Ports.Revision, slice 208's
// rule): what the caller knew when it chose the command no longer holds.
// None without --expect, or for an item next gives no revision for, which
// the command has refused before; what next cannot read is the error.
func (h Handler) stale(ctx context.Context, cfg *config.Loaded, q Query) (*Answer, error) {
	if q.Expect == "" {
		return nil, nil
	}
	current, refused, err := h.ports.Revision(ctx, cfg, q.Item)
	if err != nil || refused != nil || current == q.Expect {
		return nil, err
	}
	a := refuse(policy, "review-stale",
		fmt.Sprintf("%s's revision is %s, not %s: what was known of it when this command was chosen no longer holds", q.Item, current, q.Expect),
		"run itos next "+q.Item+" again, and act on what it says now")
	return &a, nil
}
