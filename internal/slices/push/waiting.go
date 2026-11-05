package push

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/engine/reviewers"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/value"
)

// waiting is what --ready waits on and has said so far (slice 190): the head
// whose run is watched, the run's state when one is ("waiting", then
// "succeeded"), and each review_by login's, in the config's order. rebasing is
// whether GitHub's rebase of the pull request has not shown yet (slice 200).
type waiting struct {
	r          *pushRun
	repository string
	head       string
	run        string
	rebasing   bool
	logins     []string
	states     []string
	said       string
}

// newWaiting is the wait on the head and the logins, none of them read yet.
func newWaiting(r *pushRun, repository string, logins []string) *waiting {
	w := &waiting{r: r, repository: repository, head: r.sha, logins: logins, states: make([]string, len(logins))}
	for i := range w.states {
		w.states[i] = "not read"
	}
	return w
}

// parts are what is waited on, in the order the waiting line gives them:
// "rebase waiting" until GitHub's rebase shows, else the run's state when a run
// is watched, then each login's.
func (w *waiting) parts() string {
	if w.rebasing {
		return "rebase waiting"
	}
	var parts []string
	if w.run != "" {
		parts = append(parts, "run "+w.run)
	}
	for i, login := range w.logins {
		parts = append(parts, login+" "+w.states[i])
	}
	return strings.Join(parts, ", ")
}

// show says what is waited on, unless it said exactly that last: a look that
// changes nothing prints nothing.
func (w *waiting) show() {
	line := fmt.Sprintf("Waiting on %s at %s: %s.", w.r.pr.URL, short(w.head), w.parts())
	if line == w.said {
		return
	}
	w.said = line
	w.r.say(line)
}

// firstLook reads the logins' states while the run still goes (bug-166): the one
// question the wait asks before the run's end. It judges nothing: an answer it
// cannot get leaves them not read, for the next ask to judge the failure.
func (w *waiting) firstLook(ctx context.Context) {
	if len(w.logins) == 0 {
		return
	}
	got, err := providers.AskPullState(ctx, w.r.ports.Gh, w.repository, w.r.pr.Number)
	if err != nil {
		return
	}
	missing, _ := got.Verdicts(w.logins, w.head)
	w.read(pullState{pull: got}, missing, reviewers.NewReviewing(0))
}

// look records what the pull request says of each login; the run, when there is
// one, has succeeded by now.
func (w *waiting) look(state pullState, missing []string, rv *reviewers.Reviewing) {
	w.read(state, missing, rv)
	if w.run != "" {
		w.run = "succeeded"
	}
	w.show()
}

// read is each login's state, as stateOf reads it.
func (w *waiting) read(state pullState, missing []string, rv *reviewers.Reviewing) {
	for i, login := range w.logins {
		w.states[i] = w.stateOf(login, state, missing, rv)
	}
}

// stateOf is what the line says of the login, the first that holds (bug-166):
// completed with a verdict; running when its review-state comment reads a review
// under way on the head; asked; no review requested when it has neither a
// review-state comment nor a request, naming the command that would ask, for a
// login that command asks; else waiting.
func (w *waiting) stateOf(login string, state pullState, missing []string, rv *reviewers.Reviewing) string {
	kind, named := w.reviewer(login)
	switch {
	case !slices.Contains(missing, login):
		return "completed"
	case state.running(login, w.head):
		return "running"
	case w.asked(login, kind, state.pull.Comments, rv):
		return "asked"
	case named && !state.pull.StateOf(login).Stated:
		return "no review requested (itos review request " + w.r.branch + ")"
	}
	return "waiting"
}

// reviewer is the login's kind and whether push.pr.reviewers names it: one it
// does not is codex's, as the walk asks it.
func (w *waiting) reviewer(login string) (kind string, named bool) {
	for _, rv := range w.r.cfg.Push.PR.Reviewers {
		if providers.SameLogin(rv.Login, login) {
			return rv.Kind, true
		}
	}
	return "codex", false
}

// asked is whether a request of the login's kind names it and the head on the pull
// request, for either review, or this --ready posted one.
func (w *waiting) asked(login, kind string, comments []providers.Authored, rv *reviewers.Reviewing) bool {
	for _, line := range reviewers.Lines(kind) {
		if (reviewers.Request{Login: login, Kind: kind, Line: line}).OnHead(comments, w.head) {
			return true
		}
	}
	return rv.Posted(login)
}

// running is whether a review of the login's review-state comment is under way
// on the head; a status null of the head is not (slice 218).
func (s pullState) running(login, head string) bool {
	st := s.pull.StateOf(login)
	return st.Code.UnderWay(head) || st.Security.UnderWay(head)
}

// postRequest posts the request's comments for the head, in order, and keeps the
// address gh prints for the last, the one the request is read from.
func (r *pushRun) postRequest(ctx context.Context, repository string, rv *reviewers.Reviewing, req reviewers.Request, head string) (string, Answer, bool) {
	var address string
	for _, body := range req.Bodies(head) {
		printed, a, failed := r.gh(ctx, "pr", "comment", r.branch, "--repo", repository, "--body="+body)
		if failed {
			return "", a, true
		}
		address = strings.TrimSpace(printed)
	}
	rv.Record(req, address, now())
	return address, Answer{}, false
}

// walkReviews is one look's walk of the reviews (reviewers.Reviewing.Walk): --ready's
// poster, which asks by gh pr comment and says so, and, between the walk's two
// phases, the look's line and its limits, so ci.watch.timeout and
// push.pr.ready_wait come before the judging and after the requests a look makes.
// left is the time to the nearer limit; done, with the outcome reported, when gh pr
// comment fails, a limit has passed or a review stops the wait: 75, review-stalled or
// review-not-started, auto-merge not turned on.
func (r *pushRun) walkReviews(ctx context.Context, repository string, w *waiting, rv *reviewers.Reviewing, state pullState, missing []string, deadline time.Time) (left time.Duration, a Answer, done bool) {
	stop := rv.Walk(reviewers.Walk{
		Logins: w.logins,
		Head:   w.head,
		State:  state.pull,
		Now:    now,
		Post: func(p reviewers.Post) bool {
			address, failure, failed := r.postRequest(ctx, repository, rv, p.Request, w.head)
			if failed {
				a, done = failure, true
				return false
			}
			r.say(r.askedLine(p, address, w.head))
			return true
		},
		Between: func() bool {
			w.look(state, missing, rv)
			timedOut, bounded, nearer := r.limits(deadline)
			left = nearer
			switch {
			case timedOut:
				fmt.Fprintf(r.o.Stderr, "itos: no review by %s on %s after %s seconds (ci.watch.timeout), so auto-merge is not turned on; itos push --ready again turns it on if the reviews have come, else stops here at once, the wait counting from when the pull request was marked ready or its head pushed; a new head starts it afresh.\n",
					strings.Join(missing, ", "), r.pr.URL, value.Number(*r.cfg.CI.Watch.Timeout))
				a, done = r.report(temporary, "review-timeout"), true
			case bounded:
				a, done = r.stillWaiting(w), true
			}
			return !done
		},
	})
	if stop != nil {
		return left, r.reviewStopped(stop), true
	}
	return left, a, done
}

// askedLine is what --ready says of a request it posted at the address.
func (r *pushRun) askedLine(p reviewers.Post, address, head string) string {
	login, seconds := p.Request.Login, value.Number(r.cfg.Push.PR.ReviewStall)
	switch p.Why {
	case reviewers.ForHead:
		what := short(head)
		if p.Request.Review() == "security" {
			what += " for security"
		}
		return fmt.Sprintf("Asked %s to review %s on %s: its review state is for another commit.", login, what, r.pr.URL)
	case reviewers.NotStarted:
		return fmt.Sprintf("Asked %s again, at %s, for its %s review of %s: it had not started %s seconds (push.pr.review_stall) after the request.",
			login, address, p.Request.Review(), short(head), seconds)
	}
	return fmt.Sprintf("Asked %s to review %s again on %s: its review read running for %s seconds (push.pr.review_stall).",
		login, short(head), r.pr.URL, seconds)
}

// reviewStopped ends the wait on the review: exit 75, review-stalled for one under
// way, else review-not-started naming it (Answer.NotStarted), auto-merge not turned on.
func (r *pushRun) reviewStopped(stop *reviewers.Stop) Answer {
	seconds := value.Number(r.cfg.Push.PR.ReviewStall)
	if stop.Stalled {
		fmt.Fprintf(r.o.Stderr, "itos: the review by %s of %s on %s stalled: still running %s seconds (push.pr.review_stall) after it was asked again, so auto-merge is not turned on; look at the reviewer, then itos push --ready again.\n",
			stop.Login, short(stop.Head), r.pr.URL, seconds)
		return r.report(temporary, "review-stalled")
	}
	fmt.Fprintf(r.o.Stderr, "itos: the %s review by %s of %s on %s has not started %s seconds (push.pr.review_stall) after the request %s, so auto-merge is not turned on; look at the reviewer, then itos push --ready again.\n",
		stop.Review, stop.Login, stop.Head, r.pr.URL, seconds, stop.Request)
	a := r.report(temporary, "review-not-started")
	a.NotStarted = &NotStarted{Login: stop.Login, Review: stop.Review, Head: stop.Head, Request: stop.Request}
	return a
}
