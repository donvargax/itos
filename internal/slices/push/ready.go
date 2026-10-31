package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/value"
)

// reviewPoll is the least pause between two looks at the pull request's
// reviews: a ci.watch.interval of 0 would otherwise ask GitHub as fast as gh
// answers.
const reviewPoll = 50 * time.Millisecond

// reviewQuery is the question the reviews, the comments, the ready time and
// the review threads are read with: providers.PullStateQuery, which review
// post reads too (slice-206), so the two commands read one answer.
var reviewQuery = providers.PullStateQuery

// pullState is what GitHub says of the pull request: the answer itself, whose
// reviews and comments the verdicts are read from (providers.PullState's
// Verdicts, which itos review status reads too), when it was last marked
// ready (zero when it does not say), the node ids of all its review threads
// and the threads still open.
type pullState struct {
	pull       providers.PullState
	readyAt    time.Time
	ids        []string
	unresolved []openThread
}

// openThread is a review thread that is not resolved: its node id, where it is
// (line 0 for an outdated one, which GitHub gives none), the login of its first
// comment's author and the first line of that comment.
type openThread struct {
	id     string
	path   string
	line   int
	author string
	first  string
}

// finish is --ready's end under push.pr.draft, once the pull request is ready
// and, when main moved, asked to be rebased: the run of its head is watched
// when ci.watch has a provider, then the reviews of push.pr.review_by are
// waited for, then the review threads must all be resolved, and only then is
// the merge asked for (merge), so GitHub never merges before the checks and the
// reviews. The head judged is the one GitHub made when it rebased the pull
// request: its run is watched, a review state counts for it alone and the merge
// is pinned to it. Whichever stops it exits without merging, naming what is
// left, and push.pr.ready_wait ends the call with ready-still-waiting (slice
// 200). outcome is pushed or nothing-to-push, line what the push says of it.
func (r *pushRun) finish(ctx context.Context, repository, outcome, line string) Answer {
	cfg := r.cfg
	r.announce(line)
	if seconds := cfg.Push.PR.ReadyWait; seconds > 0 {
		r.bound = r.began.Add(time.Duration(seconds * float64(time.Second)))
	}
	w := newWaiting(r, cfg.Push.PR.ReviewBy)
	wr, ok, err := r.ports.Watcher(ctx, cfg, r.remote)
	switch {
	case ok || err != nil:
		if ok && err == nil {
			w.run, r.waiting = "waiting", w
		}
		if a, failed := r.watchRun(ctx, cfg, wr, err, r.remote, r.sha); failed {
			return a
		}
		if r.watched.Outcome == "bound" {
			r.watched = nil
			return r.stillWaiting(w)
		}
		if r.watched.Code != 0 {
			fmt.Fprintln(r.o.Stderr, "itos: auto-merge is not turned on: the run did not succeed. Fix what failed and push, or wait for the run, then itos push --ready again.")
			return r.report(r.watched.Code, outcome)
		}
		if w.run != "" {
			w.run = "succeeded"
			if len(w.logins) == 0 {
				w.show()
			}
		}
	case r.rebased != nil:
		head, a, done := r.unwatchedHead(ctx)
		if done {
			return a
		}
		w.head = head
	}
	var state pullState
	var a Answer
	var done bool
	if logins := cfg.Push.PR.ReviewBy; len(logins) > 0 {
		state, a, done = r.awaitReviews(ctx, repository, w)
	} else {
		state, a, done = r.ask(ctx, repository, r.pr.Number)
	}
	if done {
		return a
	}
	if len(state.unresolved) > 0 {
		fmt.Fprintf(r.o.Stderr, "itos push: auto-merge is not turned on: %s has unresolved review threads:\n", r.pr.URL)
		for _, t := range state.unresolved {
			where := t.path
			if t.line > 0 {
				where += ":" + strconv.Itoa(t.line)
			}
			fmt.Fprintf(r.o.Stderr, "  %s\n", strings.TrimSpace(where+" "+t.first))
		}
		fmt.Fprintln(r.o.Stderr, "Address each (fix it and push, or reply on the pull request), resolve it, then itos push --ready again.")
		fmt.Fprintln(r.o.Stderr, "itos push threads lists them with their ids, and itos push thread <id> --reply <text> --resolve answers one.")
		return r.report(policy, "review-threads-unresolved")
	}
	return r.merge(ctx, repository, w.head, outcome)
}

// readyBound is the watch's share of push.pr.ready_wait: how long the call has left
// of it, when it has a bound.
func (r *pushRun) readyBound() ciwatch.Ready {
	if r.bound.IsZero() {
		return ciwatch.Ready{}
	}
	return ciwatch.Ready{Left: func() time.Duration { return r.bound.Sub(now()) }}
}

// limits judges the two limits of a wait at a look: timedOut, whether the
// deadline of ci.watch.timeout has passed, bounded, whether push.pr.ready_wait
// has, only the one that passed first when both have, and left, the time to
// the nearer of them, which the wait may sleep.
func (r *pushRun) limits(deadline time.Time) (timedOut, bounded bool, left time.Duration) {
	at := now()
	left = deadline.Sub(at)
	if r.bound.IsZero() {
		return left <= 0, false, left
	}
	bound := r.bound.Sub(at)
	timedOut = left <= 0 && left <= bound
	return timedOut, bound <= 0 && !timedOut, min(left, bound)
}

// stillWaiting ends --ready at push.pr.ready_wait: exit 75, outcome
// ready-still-waiting, auto-merge not turned on, saying what it waits on and
// that itos push --ready again goes on.
func (r *pushRun) stillWaiting(w *waiting) Answer {
	fmt.Fprintf(r.o.Stderr, "itos: still waiting after %s seconds (push.pr.ready_wait) on %s at %s: %s; auto-merge is not turned on. itos push --ready again goes on waiting.\n",
		value.Number(r.cfg.Push.PR.ReadyWait), r.pr.URL, w.head, w.parts())
	return r.report(temporary, "ready-still-waiting")
}

// awaitReviews asks GitHub for the pull request at once and each
// ci.watch.interval until each login has a verdict for the head, or
// ci.watch.timeout has passed, counted from when the pull request was marked
// ready or its head pushed, whichever is later, else from the first ask: done,
// with the exit reported (75, outcome review-timeout, naming the logins that
// have none; or at once 75, review-refused, for a login whose reviews are all
// refusals; or 75, review-stalled, see stalled; or 75, ready-still-waiting, at
// push.pr.ready_wait), or the failure of gh api graphql. The last answer is the
// state. w says what is waited on, at the start and at each look that changes
// it.
func (r *pushRun) awaitReviews(ctx context.Context, repository string, w *waiting) (pullState, Answer, bool) {
	logins := w.logins
	watch := r.cfg.CI.Watch
	interval := time.Duration(*watch.Interval * float64(time.Second))
	timeout := time.Duration(*watch.Timeout * float64(time.Second))
	deadline := now().Add(timeout)
	r.say(fmt.Sprintf("Waiting for reviews by %s on %s.", strings.Join(logins, ", "), r.pr.URL))
	if w.run == "" {
		w.show()
	}
	rv := &reviewing{stall: r.cfg.Push.PR.ReviewStall, since: map[string]time.Time{}, asked: map[string]time.Time{}}
	for first := true; ; first = false {
		state, a, failed := r.ask(ctx, repository, r.pr.Number)
		if failed {
			return pullState{}, a, true
		}
		if first {
			if from, ok := r.reviewsFrom(ctx, repository, state, w.head); ok {
				deadline = from.Add(timeout)
			}
		}
		missing, refused := state.pull.Verdicts(logins, w.head)
		w.look(state, missing)
		if len(missing) == 0 {
			return state, Answer{}, false
		}
		if len(refused) > 0 {
			for _, f := range refused {
				fmt.Fprintf(r.o.Stderr, "itos: %s gave no verdict on %s: \"%s\"; auto-merge is not turned on: request its review again, then itos push --ready again.\n",
					f.Login, r.pr.URL, f.Line)
			}
			return state, r.report(temporary, "review-refused"), true
		}
		if a, done := r.askForHead(ctx, repository, w, rv, state, missing); done {
			return state, a, true
		}
		timedOut, bounded, left := r.limits(deadline)
		if timedOut {
			fmt.Fprintf(r.o.Stderr, "itos: no review by %s on %s after %s seconds (ci.watch.timeout), so auto-merge is not turned on; itos push --ready again waits again.\n",
				strings.Join(missing, ", "), r.pr.URL, value.Number(*watch.Timeout))
			return state, r.report(temporary, "review-timeout"), true
		}
		if bounded {
			return state, r.stillWaiting(w), true
		}
		if rv.stall > 0 {
			if a, done := r.stalled(ctx, repository, w, rv, state, missing); done {
				return state, a, true
			}
		}
		sleep(min(max(interval, reviewPoll), left))
	}
}

// underway is whether a review of the head, in the state, reads anything but
// completed.
func underway(s providers.ReviewState, head string) bool {
	return s.Code.SHA == head && s.Code.Status != "completed" || s.Security.SHA == head && s.Security.Status != "completed"
}

// reviewsFrom is when the clock of ci.watch.timeout for the reviews starts:
// when the pull request was last marked ready or its head was pushed,
// whichever is later, as GitHub says (the activity of the branch, asked once);
// ok is false when it says neither, and the clock counts from the first ask.
func (r *pushRun) reviewsFrom(ctx context.Context, repository string, state pullState, head string) (time.Time, bool) {
	from := state.readyAt
	if pushed := r.pushedAt(ctx, repository, head); pushed.After(from) {
		from = pushed
	}
	return from, !from.IsZero()
}

// pushedAt is when the head was pushed, from the first entry of the branch's
// activity (gh api repos/<owner>/<name>/activity), when that entry's after is
// the head: the time of the last push, which this is. An answer that fails,
// cannot be read or is not the head's is the zero time, and no failure: it only
// sets a clock.
func (r *pushRun) pushedAt(ctx context.Context, repository, head string) time.Time {
	answered, err := r.ports.Gh(ctx, "api", "repos/"+repository+"/activity?ref=refs/heads/"+r.branch+"&per_page=1")
	if err != nil {
		return time.Time{}
	}
	var activity []struct {
		After     string `json:"after"`
		Timestamp string `json:"timestamp"`
	}
	if json.Unmarshal([]byte(answered), &activity) != nil || len(activity) == 0 || activity[0].After != head {
		return time.Time{}
	}
	at, err := time.Parse(time.RFC3339, activity[0].Timestamp)
	if err != nil {
		return time.Time{}
	}
	return at
}

// ask puts reviewQuery to GitHub about the pull request
// (providers.AskPullState, the comments paged up to providers.CommentPages):
// failed, with pull-request-failed reported, when gh does or its answer has
// not the reviews and the review threads.
func (r *pushRun) ask(ctx context.Context, repository string, number int) (pullState, Answer, bool) {
	got, err := providers.AskPullState(ctx, r.ports.Gh, repository, number)
	switch {
	case errors.Is(err, providers.ErrNoPullState):
		return pullState{}, r.ghFailed("gh api graphql", err.Error()), true
	case err != nil:
		return pullState{}, r.ghError([]string{"api", "graphql"}, err), true
	}
	return fromPullState(got), Answer{}, false
}

// fromPullState is the state as --ready reads it: the reviews and comments,
// the ready time, every thread's id and the threads still open, each with the
// first line of its first comment.
func fromPullState(got providers.PullState) pullState {
	state := pullState{pull: got, readyAt: got.ReadyAt}
	for _, t := range got.Threads {
		state.ids = append(state.ids, t.ID)
		if t.Resolved {
			continue
		}
		first, _, _ := strings.Cut(t.Body, "\n")
		state.unresolved = append(state.unresolved, openThread{id: t.ID, path: t.Path, line: t.Line, author: t.Author, first: strings.TrimSpace(first)})
	}
	return state
}
