package push

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/value"
)

// requestMarker opens the hidden line of the comment that asks a login for a
// review again; the line is "<!-- itos-review-request:v1 <login> <sha> -->".
const requestMarker = "<!-- itos-review-request:v1 "

// waiting is what --ready waits on and has said so far (slice 190): the head
// whose run is watched, the run's state when one is ("waiting", then
// "succeeded"), and each review_by login's, in the config's order.
type waiting struct {
	r      *pushRun
	head   string
	run    string
	logins []string
	states []string
	said   string
}

// newWaiting is the wait on the head and the logins, none of them asked yet.
func newWaiting(r *pushRun, logins []string) *waiting {
	w := &waiting{r: r, head: r.sha, logins: logins, states: make([]string, len(logins))}
	for i := range w.states {
		w.states[i] = "not asked"
	}
	return w
}

// show says what is waited on, unless it said exactly that last: a look that
// changes nothing prints nothing.
func (w *waiting) show() {
	var parts []string
	if w.run != "" {
		parts = append(parts, "run "+w.run)
	}
	for i, login := range w.logins {
		parts = append(parts, login+" "+w.states[i])
	}
	line := fmt.Sprintf("Waiting on %s at %s: %s.", w.r.pr.URL, short(w.head), strings.Join(parts, ", "))
	if line == w.said {
		return
	}
	w.said = line
	w.r.say(line)
}

// look records what the pull request says of each login: completed with a
// verdict, else running when its review-state comment reads a review not
// completed, else waiting; the run, when there is one, has succeeded by now.
func (w *waiting) look(state pullState, missing []string) {
	for i, login := range w.logins {
		switch {
		case !slices.Contains(missing, login):
			w.states[i] = "completed"
		case state.running(login):
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

// running is whether the login wrote a review-state comment whose reviews are
// not both completed.
func (s pullState) running(login string) bool {
	completed, stated := s.stateOf(login)
	return stated && !completed
}

// requested is whether some comment, by anyone, asks the login to review the
// head again: a line "<!-- itos-review-request:v1 <login> <head> -->", the
// login matched as review_by's are.
func (s pullState) requested(login, head string) bool {
	for _, c := range s.comments {
		for _, line := range strings.Split(c.body, "\n") {
			text, ok := strings.CutPrefix(strings.TrimSpace(line), requestMarker)
			text, ok2 := strings.CutSuffix(text, " -->")
			if !ok || !ok2 {
				continue
			}
			if who, sha, found := strings.Cut(text, " "); found && sha == head && sameLogin(who, login) {
				return true
			}
		}
	}
	return false
}

// stalls watches the logins whose reviews read running for push.pr.review_stall
// seconds: since is the first look of this --ready that read each running, and
// asked when this --ready asked for it again.
type stalls struct {
	seconds float64
	since   map[string]time.Time
	asked   map[string]time.Time
}

// stalled judges a look at reviews still running (slice 190), each login in
// the config's order: one that has read running for review_stall seconds and
// was not asked for this head is asked again (gh pr comment), and it says so;
// one that was asked, by this --ready or an earlier one, and still reads
// running review_stall seconds later (counted from the request this --ready
// posted, else from its first look that read it so) ends the wait: done, 75,
// review-stalled, auto-merge not turned on. Also done, with the outcome
// reported, when gh pr comment fails.
func (r *pushRun) stalled(ctx context.Context, repository string, w *waiting, st *stalls, state pullState, missing []string) (Answer, bool) {
	at := now()
	for _, login := range w.logins {
		if !slices.Contains(missing, login) || !state.running(login) {
			delete(st.since, login)
			continue
		}
		if _, ok := st.since[login]; !ok {
			st.since[login] = at
		}
		from, requested := st.asked[login]
		if !requested && state.requested(login, w.head) {
			from, requested = st.since[login], true
		}
		seconds := value.Number(st.seconds)
		if requested {
			if at.Sub(from).Seconds() >= st.seconds {
				fmt.Fprintf(r.o.Stderr, "itos: the review by %s of %s on %s stalled: still running %s seconds (push.pr.review_stall) after it was asked again, so auto-merge is not turned on; look at the reviewer, then itos push --ready again.\n",
					login, short(w.head), r.pr.URL, seconds)
				return r.report(temporary, "review-stalled"), true
			}
			continue
		}
		if at.Sub(st.since[login]).Seconds() < st.seconds {
			continue
		}
		body := "@codex review\n\n" + requestMarker + login + " " + w.head + " -->"
		if _, a, failed := r.gh(ctx, "pr", "comment", r.branch, "--repo", repository, "--body="+body); failed {
			return a, true
		}
		st.asked[login] = at
		r.say(fmt.Sprintf("Asked %s to review %s again on %s: its review read running for %s seconds (push.pr.review_stall).",
			login, short(w.head), r.pr.URL, seconds))
	}
	return Answer{}, false
}
