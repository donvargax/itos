package push

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/value"
)

// requestMarker opens the hidden line of the comment that asks a login for a
// review again; the line is "<!-- itos-review-request:v1 <login> <sha> -->".
const requestMarker = "<!-- itos-review-request:v1 "

// waiting is what --ready waits on and has said so far (slice 190): the head
// whose run is watched, the run's state when one is ("waiting", then
// "succeeded"), and each review_by login's, in the config's order. rebasing is
// whether GitHub's rebase of the pull request has not shown yet (slice 200).
type waiting struct {
	r        *pushRun
	head     string
	run      string
	rebasing bool
	logins   []string
	states   []string
	said     string
}

// newWaiting is the wait on the head and the logins, none of them asked yet.
func newWaiting(r *pushRun, logins []string) *waiting {
	w := &waiting{r: r, head: r.sha, logins: logins, states: make([]string, len(logins))}
	for i := range w.states {
		w.states[i] = "not asked"
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

// look records what the pull request says of each login: completed with a
// verdict, else running when its review-state comment reads a review of the
// head not completed, else waiting; the run, when there is one, has succeeded
// by now.
func (w *waiting) look(state pullState, missing []string) {
	for i, login := range w.logins {
		switch {
		case !slices.Contains(missing, login):
			w.states[i] = "completed"
		case state.running(login, w.head):
			w.states[i] = "running"
		default:
			w.states[i] = "waiting"
		}
	}
	if w.run != "" {
		w.run = "succeeded"
	}
	w.show()
}

// running is whether the login wrote a review-state comment for the head whose
// reviews are not both completed.
func (s pullState) running(login, head string) bool {
	st := s.pull.StateOf(login)
	return st.Stated && st.ForHead(head) && !st.Completed()
}

// elsewhere is whether the login wrote a review-state comment for another
// commit than the head, and none of its reviews that name the head is under
// way: the review of the head has not been asked for, or not begun.
func (s pullState) elsewhere(login, head string) bool {
	st := s.pull.StateOf(login)
	return st.Stated && !st.ForHead(head) && !underway(st, head)
}

// requested is whether some comment, by anyone, asks the login to review the
// head again: a line "<!-- itos-review-request:v1 <login> <head> -->", the
// login matched as review_by's are; at is when the latest such comment was
// made, zero when GitHub does not say.
func (s pullState) requested(login, head string) (found bool, at time.Time) {
	for _, c := range s.pull.Comments {
		for _, line := range strings.Split(c.Body, "\n") {
			text, ok := strings.CutPrefix(strings.TrimSpace(line), requestMarker)
			text, ok2 := strings.CutSuffix(text, " -->")
			if !ok || !ok2 {
				continue
			}
			if who, sha, with := strings.Cut(text, " "); with && sha == head && providers.SameLogin(who, login) {
				found = true
				if c.Created.After(at) {
					at = c.Created
				}
			}
		}
	}
	return found, at
}

// reviewing is what a --ready keeps of the logins' reviews between its looks:
// stall, push.pr.review_stall; since, the first look of this --ready that read
// each login's review running for the head; and asked, when this --ready asked
// the login for the head.
type reviewing struct {
	stall float64
	since map[string]time.Time
	asked map[string]time.Time
}

// postRequest posts the request for the login to review the head: "@codex review", an
// empty line and the marker line, as one gh pr comment.
func (r *pushRun) postRequest(ctx context.Context, repository, login, head string) (Answer, bool) {
	body := "@codex review\n\n" + requestMarker + login + " " + head + " -->"
	_, a, failed := r.gh(ctx, "pr", "comment", r.branch, "--repo", repository, "--body="+body)
	return a, failed
}

// askForHead asks, once per head, a login whose review-state comment is for
// another commit to review the head (slice 200): Codex here reviews on
// ready_for_review alone, so after a fix is pushed nothing else would. It does
// so at the look that reads the state, whether or not push.pr.review_stall is
// set, unless a comment with the login's request for the head is on the pull
// request, or this --ready posted one. done, with the outcome reported, when
// gh pr comment fails.
func (r *pushRun) askForHead(ctx context.Context, repository string, w *waiting, rv *reviewing, state pullState, missing []string) (Answer, bool) {
	for _, login := range w.logins {
		if !slices.Contains(missing, login) || !state.elsewhere(login, w.head) {
			continue
		}
		if _, posted := rv.asked[login]; posted {
			continue
		}
		if found, _ := state.requested(login, w.head); found {
			continue
		}
		if a, failed := r.postRequest(ctx, repository, login, w.head); failed {
			return a, true
		}
		rv.asked[login] = now()
		r.say(fmt.Sprintf("Asked %s to review %s on %s: its review state is for another commit.", login, short(w.head), r.pr.URL))
	}
	return Answer{}, false
}

// stalled judges a look at reviews still running (slice 190), each login in
// the config's order: one that has read running for review_stall seconds and
// was not asked for this head is asked again (gh pr comment), and it says so;
// one that was asked, by this --ready or an earlier one, and still reads
// running review_stall seconds later ends the wait: done, 75, review-stalled,
// auto-merge not turned on. Also done, with the outcome reported, when gh pr
// comment fails. The clocks are GitHub's where it gives them (slice 200): before
// a request, the review-state comment's last edit; after one, the request
// comment's time, a request this --ready posted counting from when it posted it
// until GitHub lists it; a time it does not give counts from the first look of
// this --ready that read the review running.
func (r *pushRun) stalled(ctx context.Context, repository string, w *waiting, rv *reviewing, state pullState, missing []string) (Answer, bool) {
	at := now()
	for _, login := range w.logins {
		if !slices.Contains(missing, login) || !state.running(login, w.head) {
			delete(rv.since, login)
			continue
		}
		if _, ok := rv.since[login]; !ok {
			rv.since[login] = at
		}
		from := rv.since[login]
		if edited := state.pull.StateOf(login).Updated; !edited.IsZero() {
			from = edited
		}
		posted, wasPosted := rv.asked[login]
		listed, madeAt := state.requested(login, w.head)
		requested := wasPosted || listed
		switch {
		case !madeAt.IsZero():
			from = madeAt
		case wasPosted:
			from = posted
		case listed:
			from = rv.since[login]
		}
		seconds := value.Number(rv.stall)
		if requested {
			if at.Sub(from).Seconds() >= rv.stall {
				fmt.Fprintf(r.o.Stderr, "itos: the review by %s of %s on %s stalled: still running %s seconds (push.pr.review_stall) after it was asked again, so auto-merge is not turned on; look at the reviewer, then itos push --ready again.\n",
					login, short(w.head), r.pr.URL, seconds)
				return r.report(temporary, "review-stalled"), true
			}
			continue
		}
		if at.Sub(from).Seconds() < rv.stall {
			continue
		}
		if a, failed := r.postRequest(ctx, repository, login, w.head); failed {
			return a, true
		}
		rv.asked[login] = at
		r.say(fmt.Sprintf("Asked %s to review %s again on %s: its review read running for %s seconds (push.pr.review_stall).",
			login, short(w.head), r.pr.URL, seconds))
	}
	return Answer{}, false
}
