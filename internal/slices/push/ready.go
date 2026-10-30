package push

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/value"
)

// reviewPoll is the least pause between two looks at the pull request's
// reviews: a ci.watch.interval of 0 would otherwise ask GitHub as fast as gh
// answers.
const reviewPoll = 50 * time.Millisecond

// reviewQuery is the one question the reviews, the comments (with the times
// they were made and last edited, a page of them after the cursor), the time
// the pull request was last marked ready and the review threads are read with
// (gh api graphql, since gh pr view has no threads), on a single line.
var reviewQuery = strings.Join(strings.Fields(`
	query($owner: String!, $name: String!, $number: Int!, $cursor: String) {
	repository(owner: $owner, name: $name) { pullRequest(number: $number) {
	timelineItems(last: 1, itemTypes: [READY_FOR_REVIEW_EVENT]) { nodes { ... on ReadyForReviewEvent { createdAt } } }
	reviews(first: 100) { nodes { author { login } body } }
	comments(first: 100, after: $cursor) { pageInfo { hasNextPage endCursor } nodes { author { login } body createdAt updatedAt } }
	reviewThreads(first: 100) { nodes { id isResolved path line
	comments(first: 1) { nodes { author { login } body } } } } } } }`), " ")

// pullState is what GitHub says of the pull request: its reviews and comments,
// by the logins GraphQL gives, when it was last marked ready (zero when it does
// not say), the node ids of all its review threads and the threads still open.
type pullState struct {
	reviews    []authored
	comments   []authored
	readyAt    time.Time
	ids        []string
	unresolved []openThread
}

// authored is a review or a comment: who wrote it, its body and, for a
// comment, when it was made and last edited (zero when GitHub does not say).
type authored struct {
	login, body      string
	created, updated time.Time
}

// refusal is a review_by login with no verdict whose reviews are all refusals:
// the first line of the last.
type refusal struct {
	login, line string
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
		missing, refused := state.verdicts(logins, w.head)
		w.look(state, missing)
		if len(missing) == 0 {
			return state, Answer{}, false
		}
		if len(refused) > 0 {
			for _, f := range refused {
				fmt.Fprintf(r.o.Stderr, "itos: %s gave no verdict on %s: \"%s\"; auto-merge is not turned on: request its review again, then itos push --ready again.\n",
					f.login, r.pr.URL, f.line)
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

// sameLogin is whether two logins are equal once a trailing "[bot]" is dropped
// from each, GraphQL naming a bot without it and the REST API with it.
func sameLogin(a, b string) bool {
	return strings.TrimSuffix(a, "[bot]") == strings.TrimSuffix(b, "[bot]")
}

// verdicts splits the logins, as the config wrote them, that have no verdict
// for the head (missing) from those among them whose reviews are all refusals
// (refused, in the config's order). A login that wrote a review-state comment
// has the verdict that comment gives and no other: both its reviews completed,
// both of the head (slice 200); else any review of its that is not a refusal is
// one.
func (s pullState) verdicts(logins []string, head string) (missing []string, refused []refusal) {
	for _, login := range logins {
		if st := s.stateOf(login); st.stated {
			if !st.completed() || !st.forHead(head) {
				missing = append(missing, login)
			}
			continue
		}
		gave, last := false, ""
		for _, rv := range s.reviews {
			if !sameLogin(rv.login, login) {
				continue
			}
			if line, refuses := refusalLine(rv.body); refuses {
				last = line
			} else {
				gave = true
			}
		}
		if gave {
			continue
		}
		missing = append(missing, login)
		if last != "" {
			refused = append(refused, refusal{login, last})
		}
	}
	return missing, refused
}

// refusalLine is the first line of a review body and whether it is Copilot's
// refusal to review: it begins "Copilot" and says it was unable to review the
// pull request.
func refusalLine(body string) (string, bool) {
	first, _, _ := strings.Cut(body, "\n")
	first = strings.TrimSpace(first)
	return first, strings.HasPrefix(first, "Copilot") && strings.Contains(first, "was unable to review this pull request")
}

// reviewState is a login's review-state comment: whether it wrote one, each of
// its two reviews and when the comment was last edited (zero when GitHub does
// not say).
type reviewState struct {
	stated         bool
	code, security stateReview
	updated        time.Time
}

// stateReview is one review of a review-state comment: its status and the
// commit it is of.
type stateReview struct {
	status, sha string
}

// completed is whether both reviews are.
func (s reviewState) completed() bool {
	return s.code.status == "completed" && s.security.status == "completed"
}

// forHead is whether both reviews are of the head.
func (s reviewState) forHead(head string) bool {
	return s.code.sha == head && s.security.sha == head
}

// underway is whether a review of the head, in the state, reads anything but
// completed.
func (s reviewState) underway(head string) bool {
	return s.code.sha == head && s.code.status != "completed" || s.security.sha == head && s.security.status != "completed"
}

// stateOf reads the login's review-state comment, the last it wrote with a
// "<!-- codex-review-state:v1 {json} -->" line; one that cannot be read has
// reviews that are none and no commit.
func (s pullState) stateOf(login string) (state reviewState) {
	for _, c := range s.comments {
		if !sameLogin(c.login, login) {
			continue
		}
		for _, line := range strings.Split(c.body, "\n") {
			text, ok := strings.CutPrefix(strings.TrimSpace(line), "<!-- codex-review-state:v1 ")
			text, ok2 := strings.CutSuffix(text, " -->")
			if !ok || !ok2 {
				continue
			}
			var parsed struct {
				CodeReview struct {
					Status string `json:"status"`
					SHA    string `json:"commit_sha"`
				} `json:"code_review"`
				SecurityReview struct {
					Status string `json:"status"`
					SHA    string `json:"commit_sha"`
				} `json:"security_review"`
			}
			state = reviewState{stated: true, updated: c.updated}
			if json.Unmarshal([]byte(text), &parsed) == nil {
				state.code = stateReview{parsed.CodeReview.Status, parsed.CodeReview.SHA}
				state.security = stateReview{parsed.SecurityReview.Status, parsed.SecurityReview.SHA}
			}
		}
	}
	return state
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

// commentPages is how many pages of a hundred comments ask reads at most.
const commentPages = 20

// ask puts reviewQuery to GitHub (gh api graphql, which takes no --repo, so
// the repository is its owner and name) about the pull request: failed, with
// pull-request-failed reported, when gh does or its answer has not the
// reviews and the review threads (comments, which an answer from before slice
// 187 lacks, are none then, and the times from before slice 200 are none). The
// reviews and the threads are those of the first answer; the comments are all
// the pages of them, a hundred each, up to commentPages, since a request or a
// review state that sits past the first hundred is read as none (slice 200).
func (r *pushRun) ask(ctx context.Context, repository string, number int) (pullState, Answer, bool) {
	owner, name, _ := strings.Cut(repository, "/")
	var state pullState
	cursor := ""
	for page := 0; ; page++ {
		args := []string{"api", "graphql", "-f", "query=" + reviewQuery, "-f", "owner=" + owner, "-f", "name=" + name,
			"-F", "number=" + strconv.Itoa(number)}
		if cursor != "" {
			args = append(args, "-f", "cursor="+cursor)
		}
		answered, a, failed := r.gh(ctx, args...)
		if failed {
			return pullState{}, a, true
		}
		got, next, ok := askedPage(answered)
		if !ok {
			return pullState{}, r.ghFailed("gh api graphql", "its answer is not a pull request's reviews and review threads"), true
		}
		if page == 0 {
			state = got
		} else {
			state.comments = append(state.comments, got.comments...)
		}
		if next == "" || page+1 >= commentPages {
			return state, Answer{}, false
		}
		cursor = next
	}
}

// askedPage reads one answer to reviewQuery: the state it gives, the cursor of
// the next page of comments ("" for none), and whether it has the reviews and
// the review threads.
func askedPage(answered string) (state pullState, next string, ok bool) {
	var got struct {
		Data struct {
			Repository struct {
				PullRequest *struct {
					TimelineItems *struct {
						Nodes []struct {
							CreatedAt string `json:"createdAt"`
						} `json:"nodes"`
					} `json:"timelineItems"`
					Reviews       *authoredNodes `json:"reviews"`
					Comments      *authoredNodes `json:"comments"`
					ReviewThreads *struct {
						Nodes []struct {
							ID         string `json:"id"`
							IsResolved bool   `json:"isResolved"`
							Path       string `json:"path"`
							Line       *int   `json:"line"`
							Comments   struct {
								Nodes []struct {
									Author *struct {
										Login string `json:"login"`
									} `json:"author"`
									Body string `json:"body"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	err := json.Unmarshal([]byte(answered), &got)
	pull := got.Data.Repository.PullRequest
	if err != nil || pull == nil || pull.Reviews == nil || pull.ReviewThreads == nil {
		return pullState{}, "", false
	}
	state.reviews, state.comments = pull.Reviews.authored(), pull.Comments.authored()
	if pull.TimelineItems != nil && len(pull.TimelineItems.Nodes) > 0 {
		state.readyAt = githubTime(pull.TimelineItems.Nodes[0].CreatedAt)
	}
	for _, t := range pull.ReviewThreads.Nodes {
		state.ids = append(state.ids, t.ID)
		if t.IsResolved {
			continue
		}
		open := openThread{id: t.ID, path: t.Path}
		if t.Line != nil {
			open.line = *t.Line
		}
		if len(t.Comments.Nodes) > 0 {
			c := t.Comments.Nodes[0]
			first, _, _ := strings.Cut(c.Body, "\n")
			open.first = strings.TrimSpace(first)
			if c.Author != nil {
				open.author = c.Author.Login
			}
		}
		state.unresolved = append(state.unresolved, open)
	}
	return state, pull.Comments.next(), true
}

// githubTime is a time as GitHub writes it; zero for none or one that cannot be
// read.
func githubTime(text string) time.Time {
	at, _ := time.Parse(time.RFC3339, text)
	return at
}

// authoredNodes are GraphQL's reviews or comments; a node whose author is gone
// is no one's.
type authoredNodes struct {
	PageInfo *struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
	Nodes []struct {
		Author *struct {
			Login string `json:"login"`
		} `json:"author"`
		Body      string `json:"body"`
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	} `json:"nodes"`
}

// next is the cursor of the page of comments after this one, "" when this is
// the last.
func (a *authoredNodes) next() string {
	if a == nil || a.PageInfo == nil || !a.PageInfo.HasNextPage {
		return ""
	}
	return a.PageInfo.EndCursor
}

func (a *authoredNodes) authored() []authored {
	var out []authored
	if a == nil {
		return out
	}
	for _, n := range a.Nodes {
		if n.Author != nil {
			out = append(out, authored{n.Author.Login, n.Body, githubTime(n.CreatedAt), githubTime(n.UpdatedAt)})
		}
	}
	return out
}
