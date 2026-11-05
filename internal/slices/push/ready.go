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
	"github.com/donvargax/itos/v8/internal/engine/reviewers"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/value"
)

// reviewPoll is the least pause between two looks at the pull request's
// reviews: a ci.watch.interval of 0 would otherwise ask GitHub as fast as gh
// answers.
const reviewPoll = 50 * time.Millisecond

// noRunWindow is how long --ready, having pushed the head, looks for a run of it
// before it takes it that none is coming (bug-160).
const noRunWindow = 60 * time.Second

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
// comment's author and that comment, whole and as its summary (commentSummary).
type openThread struct {
	id      string
	path    string
	line    int
	author  string
	body    string
	summary string
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
// 200). outcome is pushed or nothing-to-push.
func (r *pushRun) finish(ctx context.Context, repository, outcome string) Answer {
	cfg := r.cfg
	w := newWaiting(r, repository, cfg.Push.PR.ReviewBy)
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
			return r.runNotPassed(outcome)
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
			fmt.Fprintf(r.o.Stderr, "  %s %s\n", where, t.summary)
		}
		fmt.Fprintln(r.o.Stderr, "Address each (fix it and push, or reply on the pull request), resolve it, then itos push --ready again.")
		fmt.Fprintln(r.o.Stderr, "itos push threads lists them with their ids, and itos push thread <id> --reply <text> --resolve answers one.")
		return r.report(policy, "review-threads-unresolved")
	}
	return r.merge(ctx, repository, w.head, outcome)
}

// runNotPassed ends --ready at a watch that did not end in success, r.watched:
// a run cancelled with none to follow is its own end, outcome run-cancelled,
// naming the rerun that would give the head a result; any other leaves outcome,
// pushed or nothing-to-push. A failure on a run still going is the watch's stop
// at a failed job (slice-225), which cannot say the run did not succeed: a job
// allowed to fail reads as failed, and its run may yet.
func (r *pushRun) runNotPassed(outcome string) Answer {
	if r.watched.Outcome == "cancelled" {
		run := r.watched.Run
		fmt.Fprintf(r.o.Stderr, "itos: the run %s of %s on %s was cancelled and no other run checked that head, so auto-merge is not turned on: rerun it with gh run rerun %d, then itos push --ready again.\n",
			run.URL, short(run.HeadSHA), r.pr.URL, run.ID)
		return r.report(r.watched.Code, "run-cancelled")
	}
	if run := r.watched.Run; r.watched.Outcome == "failure" && !run.Done() {
		fmt.Fprintf(r.o.Stderr, "itos: auto-merge is not turned on: a job failed and the run goes on. Fix what failed and push, or wait for the run's end with itos ci watch %s, then itos push --ready again.\n", run.HeadSHA)
		return r.report(r.watched.Code, outcome)
	}
	fmt.Fprintln(r.o.Stderr, "itos: auto-merge is not turned on: the run did not succeed. Fix what failed and push, or wait for the run, then itos push --ready again.")
	return r.report(r.watched.Code, outcome)
}

// pushStarted is whether a push starts runs of the event: GitHub's run names no
// activity type, a synchronize and a ready_for_review alike being pull_request,
// and none is needed before a draft is marked, when nothing has marked its head
// ready yet. A provider that names no event counts as naming the push's.
func pushStarted(event string) bool {
	return event == "pull_request" || event == "push" || event == ""
}

// awaitPushRuns is what --ready does before it marks a draft ready with
// ci.watch's provider (bug-160): every GitHub operation that starts a run
// waits for it before the next, for the draft's run of a push, which skips, and
// the ready run share a concurrency group, and the later cancels the earlier.
// It looks at the head's runs at once and each ci.watch.interval, and comes back
// not done once none its push started is going, what those concluded being the
// ready run's to judge. Having pushed the head it looks for one for noRunWindow
// from the first look, or ci.watch.timeout when that is shorter, a push path
// filters start no run for; with nothing pushed one look is enough. A run going
// counts toward ci.watch.timeout from its start, as WatchReady's does, and
// push.pr.ready_wait bounds the whole, nothing marked. A failed look follows the
// watch's rule (ciwatch.Looks), and a provider that cannot be built stops here.
// Done, the answer is the call's end. outcome is pushed or nothing-to-push.
func (r *pushRun) awaitPushRuns(ctx context.Context, repository, outcome string, pushed bool) (Answer, bool) {
	wr, ok, err := r.ports.Watcher(ctx, r.cfg, r.remote)
	if err != nil {
		r.watcherFailed(err, r.sha)
		return r.runNotPassed(outcome), true
	}
	if !ok {
		return Answer{}, false
	}
	interval := time.Duration(*r.cfg.CI.Watch.Interval * float64(time.Second))
	timeout := time.Duration(*r.cfg.CI.Watch.Timeout * float64(time.Second))
	first := now()
	deadline := first.Add(timeout)
	window := min(noRunWindow, timeout)
	var last *providers.Run
	var looks ciwatch.Looks
	said := false
	for {
		looks.Pause = 0
		var windowLeft time.Duration // what is left of the window, after a look that found no run
		runs, err := wr.HeadRuns(r.sha)
		if err != nil {
			if code, ended := looks.Failing(err, r.sha, r.o.Stderr); ended {
				r.watched = &ciwatch.Watched{Code: code, Outcome: "error", Run: last}
				return r.runNotPassed(outcome), true
			}
		} else {
			looks.Failed, looks.Last = 0, nil
			going, shown := goingOf(runs)
			switch {
			case going != nil:
				last = going
				if started, ok := ciwatch.StartOf(*going); ok {
					deadline = started.Add(timeout)
				}
			case shown || !pushed:
				return Answer{}, false
			default:
				if windowLeft = first.Add(window).Sub(now()); windowLeft <= 0 {
					r.say(fmt.Sprintf("No run of %s appeared within %s seconds, so none is coming: marking %s ready.",
						short(r.sha), value.Number(window.Seconds()), r.pr.URL))
					return Answer{}, false
				}
			}
		}
		timedOut, bounded, left := r.limits(deadline)
		if timedOut {
			watched := ciwatch.TimedOut(r.cfg, r.sha, last, looks.Last, r.o.Stderr)
			r.watched = &watched
			return r.runNotPassed(outcome), true
		}
		if bounded {
			w := newWaiting(r, repository, r.cfg.Push.PR.ReviewBy)
			w.run = "waiting"
			w.firstLook(ctx)
			return r.stillWaiting(w), true
		}
		if !said {
			said = true
			r.say(fmt.Sprintf("Waiting for the run of %s to finish before marking %s ready.", short(r.sha), r.pr.URL))
		}
		wait := min(max(interval, looks.Pause, reviewPoll), left)
		if windowLeft > 0 {
			wait = min(wait, windowLeft)
		}
		if looks.Pause > interval {
			r.say("GitHub's rate limit: looking again in " + wait.Round(time.Second).String())
		}
		sleep(wait)
	}
}

// goingOf reads the head's runs, newest first, for those a push started: going,
// the newest not done, nil for none; shown, whether there is one at all.
func goingOf(runs []providers.Run) (going *providers.Run, shown bool) {
	for i := range runs {
		if !pushStarted(runs[i].Event) {
			continue
		}
		shown = true
		if !runs[i].Done() {
			return &runs[i], true
		}
	}
	return nil, shown
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
// refusals; or 75, review-stalled or review-not-started, see walkReviews; or 75,
// ready-still-waiting, at push.pr.ready_wait), or the failure of gh api graphql. The last answer is the
// state. w says what is waited on, at the start and at each look that changes
// it.
func (r *pushRun) awaitReviews(ctx context.Context, repository string, w *waiting) (pullState, Answer, bool) {
	logins := w.logins
	watch := r.cfg.CI.Watch
	interval := time.Duration(*watch.Interval * float64(time.Second))
	timeout := time.Duration(*watch.Timeout * float64(time.Second))
	deadline := now().Add(timeout)
	r.say(fmt.Sprintf("Waiting for reviews by %s on %s.", strings.Join(logins, ", "), r.pr.URL))
	rv := reviewers.NewReviewing(r.cfg.Push.PR.ReviewStall)
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
		w.look(state, missing, rv)
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
		left, a, done := r.walkReviews(ctx, repository, w, rv, state, missing, deadline)
		if done {
			return state, a, true
		}
		sleep(min(max(interval, reviewPoll), left))
	}
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
// the ready time, every thread's id and the threads still open, each with its
// first comment.
func fromPullState(got providers.PullState) pullState {
	state := pullState{pull: got, readyAt: got.ReadyAt}
	for _, t := range got.Threads {
		state.ids = append(state.ids, t.ID)
		if t.Resolved {
			continue
		}
		state.unresolved = append(state.unresolved, openThread{id: t.ID, path: t.Path, line: t.Line, author: t.Author, body: t.Body, summary: commentSummary(t.Body)})
	}
	return state
}
