package push

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
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
	w.read(pullState{pull: got}, missing, newReviewing(0))
}

// look records what the pull request says of each login; the run, when there is
// one, has succeeded by now.
func (w *waiting) look(state pullState, missing []string, rv *reviewing) {
	w.read(state, missing, rv)
	if w.run != "" {
		w.run = "succeeded"
	}
	w.show()
}

// read is each login's state, as stateOf reads it.
func (w *waiting) read(state pullState, missing []string, rv *reviewing) {
	for i, login := range w.logins {
		w.states[i] = w.stateOf(login, state, missing, rv)
	}
}

// stateOf is what the line says of the login, the first that holds (bug-166):
// completed with a verdict; running when its review-state comment reads a review
// under way on the head; asked; no review requested when it has neither a
// review-state comment nor a request, naming the command that would ask, for a
// login that command asks; else waiting.
func (w *waiting) stateOf(login string, state pullState, missing []string, rv *reviewing) string {
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
// does not is codex's, as askForHead asks it.
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
func (w *waiting) asked(login, kind string, comments []providers.Authored, rv *reviewing) bool {
	for _, line := range reviewers.Lines(kind) {
		if (reviewers.Request{Login: login, Kind: kind, Line: line}).OnHead(comments, w.head) {
			return true
		}
	}
	for req := range rv.asked {
		if req.Login == login {
			return true
		}
	}
	return false
}

// running is whether a review of the login's review-state comment is under way
// on the head; a status null of the head is not (slice 218).
func (s pullState) running(login, head string) bool {
	st := s.pull.StateOf(login)
	return st.Code.UnderWay(head) || st.Security.UnderWay(head)
}

// reviewing is what a --ready keeps of the logins' reviews between its looks:
// stall, push.pr.review_stall; since, the first look of this --ready that read
// each login's review running for the head; unstarted, the first that read each
// review not started; and asked, each request this --ready posted for the head,
// since GitHub may not list one yet.
type reviewing struct {
	stall     float64
	since     map[string]time.Time
	unstarted map[reviewers.Request]time.Time
	asked     map[reviewers.Request][]requestMade
}

// newReviewing is a --ready's first look at the reviews.
func newReviewing(stall float64) *reviewing {
	return &reviewing{stall: stall, since: map[string]time.Time{}, unstarted: map[reviewers.Request]time.Time{}, asked: map[reviewers.Request][]requestMade{}}
}

// requestMade is one comment making a request: its address and when it was made,
// each empty where GitHub or gh does not say.
type requestMade struct {
	address string
	at      time.Time
}

// requests are the comments making the request for the head: those GitHub lists,
// then those this --ready posted that it does not list yet. A request posted and
// listed counts once, known by its address.
func (rv *reviewing) requests(req reviewers.Request, comments []providers.Authored, head string) []requestMade {
	var made []requestMade
	for _, c := range req.Made(comments, head) {
		made = append(made, requestMade{c.URL, c.Created})
	}
	listed := len(made)
	for _, posted := range rv.asked[req] {
		if posted.address == "" || !slices.ContainsFunc(made[:listed], func(m requestMade) bool { return m.address == posted.address }) {
			made = append(made, posted)
		}
	}
	return made
}

// latest is the request made last by its time, the one given last of those with
// the same time or with none, GitHub's times being whole seconds.
func latest(made []requestMade) requestMade {
	var last requestMade
	for _, m := range made {
		if !m.at.Before(last.at) {
			last = m
		}
	}
	return last
}

// postRequest posts the request's comments for the head, in order, and keeps the
// address gh prints for the last, the one the request is read from.
func (r *pushRun) postRequest(ctx context.Context, repository string, rv *reviewing, req reviewers.Request, head string) (string, Answer, bool) {
	var address string
	for _, body := range req.Bodies(head) {
		printed, a, failed := r.gh(ctx, "pr", "comment", r.branch, "--repo", repository, "--body="+body)
		if failed {
			return "", a, true
		}
		address = strings.TrimSpace(printed)
	}
	rv.asked[req] = append(rv.asked[req], requestMade{address, now()})
	return address, Answer{}, false
}

// askForHead asks for each review, in a login's review-state comment, whose
// commit is not the head, one with no commit included (slices 200, 217): Codex
// here reviews on ready_for_review alone, so after a fix is pushed nothing else
// would, and each review is judged alone, whatever the other's commit or status.
// It does so at the look that reads the state, whether or not
// push.pr.review_stall is set, unless a comment with that review's request for
// the head is on the pull request (reviewers.Missing), or this --ready posted it.
// A login with no review-state comment is asked nothing. done, with the outcome
// reported, when gh pr comment fails.
func (r *pushRun) askForHead(ctx context.Context, repository string, w *waiting, rv *reviewing, state pullState) (Answer, bool) {
	for _, login := range w.logins {
		if !state.pull.StateOf(login).Stated {
			continue
		}
		for _, req := range reviewers.Missing([]config.Reviewer{{Login: login, Kind: "codex"}}, state.pull, w.head) {
			if _, posted := rv.asked[req]; posted {
				continue
			}
			if _, a, failed := r.postRequest(ctx, repository, rv, req, w.head); failed {
				return a, true
			}
			what := short(w.head)
			if req != reviewers.CodexReview(login) {
				what += " for security"
			}
			r.say(fmt.Sprintf("Asked %s to review %s on %s: its review state is for another commit.", login, what, r.pr.URL))
		}
	}
	return Answer{}, false
}

// judgeReviews judges a look under push.pr.review_stall, each login in the
// config's order that wrote a review-state comment, each of its reviews alone:
// its code review not started, then a review under way (stalled), then its
// security review not started (notStarted). done, with the outcome reported, when
// one of them ends the wait.
func (r *pushRun) judgeReviews(ctx context.Context, repository string, w *waiting, rv *reviewing, state pullState) (Answer, bool) {
	at := now()
	for _, login := range w.logins {
		st := state.pull.StateOf(login)
		if !st.Stated {
			delete(rv.since, login)
			continue
		}
		if a, done := r.notStarted(ctx, repository, w, rv, state, at, "code", reviewers.CodexReview(login), st.Code); done {
			return a, true
		}
		if a, done := r.stalled(ctx, repository, w, rv, state, at, login); done {
			return a, true
		}
		if a, done := r.notStarted(ctx, repository, w, rv, state, at, "security", reviewers.CodexSecurity(login), st.Security); done {
			return a, true
		}
	}
	return Answer{}, false
}

// notStarted judges one review, by name, of a login's review-state comment
// (slice 218). One with a request for it on the head (rv.requests) that reads not
// started review_stall seconds after the latest such request is asked once more,
// when there is one request, and ends the wait when there are two: 75,
// review-not-started, auto-merge not turned on. Two requests for a review and a
// head, ever: they are counted from the pull request, across --ready runs, since
// a driver rerunning a --ready stopped here would otherwise post at each
// review_stall. The clock is GitHub's, as stalled's: a request's createdAt, or
// when this --ready posted it until GitHub lists it; a time it does not give
// counts from the first look of this --ready that read the review not started.
// Also done, with the outcome reported, when gh pr comment fails.
func (r *pushRun) notStarted(ctx context.Context, repository string, w *waiting, rv *reviewing, state pullState, at time.Time, name string, req reviewers.Request, review providers.StateReview) (Answer, bool) {
	made := rv.requests(req, state.pull.Comments, w.head)
	if !review.NotStarted(w.head) || len(made) == 0 {
		delete(rv.unstarted, req)
		return Answer{}, false
	}
	if _, ok := rv.unstarted[req]; !ok {
		rv.unstarted[req] = at
	}
	last := latest(made)
	from := last.at
	if from.IsZero() {
		from = rv.unstarted[req]
	}
	if at.Sub(from).Seconds() < rv.stall {
		return Answer{}, false
	}
	seconds := value.Number(rv.stall)
	if len(made) > 1 {
		fmt.Fprintf(r.o.Stderr, "itos: the %s review by %s of %s on %s has not started %s seconds (push.pr.review_stall) after the request %s, so auto-merge is not turned on; look at the reviewer, then itos push --ready again.\n",
			name, req.Login, w.head, r.pr.URL, seconds, last.address)
		a := r.report(temporary, "review-not-started")
		a.NotStarted = &NotStarted{Login: req.Login, Review: name, Head: w.head, Request: last.address}
		return a, true
	}
	address, a, failed := r.postRequest(ctx, repository, rv, req, w.head)
	if failed {
		return a, true
	}
	r.say(fmt.Sprintf("Asked %s again, at %s, for its %s review of %s: it had not started %s seconds (push.pr.review_stall) after the request.",
		req.Login, address, name, short(w.head), seconds))
	return Answer{}, false
}

// stalled judges a login's reviews under way (slice 190): one that has read
// running for review_stall seconds by stallClock and does not count as asked is
// asked again (gh pr comment), and it says so; one that counts as asked ends the
// wait: done, 75, review-stalled, auto-merge not turned on. Also done, with the
// outcome reported, when gh pr comment fails. A security request counts for
// nothing here: it neither takes the place of "@codex review" nor starts a clock.
func (r *pushRun) stalled(ctx context.Context, repository string, w *waiting, rv *reviewing, state pullState, at time.Time, login string) (Answer, bool) {
	if !state.running(login, w.head) {
		delete(rv.since, login)
		return Answer{}, false
	}
	if _, ok := rv.since[login]; !ok {
		rv.since[login] = at
	}
	st := state.pull.StateOf(login)
	code := reviewers.CodexReview(login)
	from, asked := stallClock(rv.since[login], st.Updated, rv.requests(code, state.pull.Comments, w.head), st.Code.UnderWay(w.head))
	if at.Sub(from).Seconds() < rv.stall {
		return Answer{}, false
	}
	seconds := value.Number(rv.stall)
	if asked {
		fmt.Fprintf(r.o.Stderr, "itos: the review by %s of %s on %s stalled: still running %s seconds (push.pr.review_stall) after it was asked again, so auto-merge is not turned on; look at the reviewer, then itos push --ready again.\n",
			login, short(w.head), r.pr.URL, seconds)
		return r.report(temporary, "review-stalled"), true
	}
	if _, a, failed := r.postRequest(ctx, repository, rv, code, w.head); failed {
		return a, true
	}
	r.say(fmt.Sprintf("Asked %s to review %s again on %s: its review read running for %s seconds (push.pr.review_stall).",
		login, short(w.head), r.pr.URL, seconds))
	return Answer{}, false
}

// stallClock is what a review under way counts from and whether its code review
// counts as asked. The clocks are GitHub's where it gives them (slice 200, bug-164):
// the review-state comment's last edit, when the review started or later; a request
// made at or after it instead, GitHub's times being whole seconds, and one this
// --ready posted counting from when it posted it until GitHub lists it. A request
// older than that edit asked for what is running, not for it again, so one of them
// alone leaves the review unasked while its code review is under way; with the
// code review not under way "@codex review" would ask for nothing that runs, so it
// counts as asked, as two do. A time GitHub does not give counts from first, the
// first look of this --ready that read the review running.
func stallClock(first, edited time.Time, made []requestMade, codeUnderWay bool) (time.Time, bool) {
	if len(made) == 0 {
		if edited.IsZero() {
			return first, false
		}
		return edited, false
	}
	last := latest(made).at
	switch {
	case last.IsZero():
		return first, true
	case edited.IsZero() || !last.Before(edited):
		return last, true
	}
	return edited, len(made) > 1 || !codeUnderWay
}
