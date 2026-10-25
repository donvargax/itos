package push

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/value"
)

// reviewPoll is the least pause between two looks at the pull request's
// reviews: a ci.watch.interval of 0 would otherwise ask GitHub as fast as gh
// answers.
const reviewPoll = 50 * time.Millisecond

// reviewQuery is the one question the reviews and the review threads are read
// with (gh api graphql, since gh pr view has no threads), on a single line.
var reviewQuery = strings.Join(strings.Fields(`
	query($owner: String!, $name: String!, $number: Int!) {
	repository(owner: $owner, name: $name) { pullRequest(number: $number) {
	reviews(first: 100) { nodes { author { login } } }
	reviewThreads(first: 100) { nodes { isResolved path line
	comments(first: 1) { nodes { body } } } } } } }`), " ")

// pullState is what GitHub says of the pull request's reviews and threads: the
// logins that reviewed it, as GraphQL gives them, and the threads still open.
type pullState struct {
	reviewers  []string
	unresolved []openThread
}

// openThread is a review thread that is not resolved: where it is (line 0 for
// an outdated one, which GitHub gives none) and the first line of its first
// comment.
type openThread struct {
	path  string
	line  int
	first string
}

// finish is --ready's end under push.pr.draft, once the pull request is ready
// and, when main moved, asked to be rebased: the run of its head is watched
// when ci.watch has a provider, then the reviews of push.pr.review_by are
// waited for, then the review threads must all be resolved, and only then is
// auto-merge turned on, so GitHub never merges before the checks and the
// reviews. Whichever stops it exits without turning auto-merge on, naming
// what is left. outcome is pushed or nothing-to-push, line what the push says
// of it.
func (r *pushRun) finish(ctx context.Context, repository, outcome, line string) Answer {
	cfg := r.cfg
	r.announce(line)
	wr, ok, err := r.ports.Watcher(ctx, cfg, r.remote)
	if ok || err != nil {
		if a, failed := r.watchRun(ctx, cfg, wr, err, r.remote, r.sha); failed {
			return a
		}
		if r.watched.Code != 0 {
			fmt.Fprintln(r.o.Stderr, "itos: auto-merge is not turned on: the run did not succeed. Fix what failed and push, or wait for the run, then itos push --ready again.")
			return r.report(r.watched.Code, outcome)
		}
	}
	var state pullState
	var a Answer
	var done bool
	if logins := cfg.Push.PR.ReviewBy; len(logins) > 0 {
		state, a, done = r.awaitReviews(ctx, repository, logins)
	} else {
		state, a, done = r.ask(ctx, repository)
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
		return r.report(policy, "review-threads-unresolved")
	}
	if _, a, failed := r.gh(ctx, "pr", "merge", r.branch, "--repo", repository, "--auto", "--rebase"); failed {
		return a
	}
	r.pr.AutoMerge = true
	return r.report(0, outcome, fmt.Sprintf("Auto-merge is on: GitHub merges %s by rebase once its required checks pass.", r.pr.URL))
}

// awaitReviews asks GitHub for the pull request's reviews at once and each
// ci.watch.interval until each login has one, or ci.watch.timeout, counted
// from the first ask, has passed: done, with the exit reported (75, outcome
// review-timeout, naming the logins that have none), or the failure of gh api
// graphql. The last answer is the state.
func (r *pushRun) awaitReviews(ctx context.Context, repository string, logins []string) (pullState, Answer, bool) {
	watch := r.cfg.CI.Watch
	interval := time.Duration(*watch.Interval * float64(time.Second))
	deadline := now().Add(time.Duration(*watch.Timeout * float64(time.Second)))
	r.say(fmt.Sprintf("Waiting for reviews by %s on %s.", strings.Join(logins, ", "), r.pr.URL))
	for {
		state, a, failed := r.ask(ctx, repository)
		if failed {
			return pullState{}, a, true
		}
		missing := state.without(logins)
		if len(missing) == 0 {
			return state, Answer{}, false
		}
		left := deadline.Sub(now())
		if left <= 0 {
			fmt.Fprintf(r.o.Stderr, "itos: no review by %s on %s after %s seconds (ci.watch.timeout), so auto-merge is not turned on; itos push --ready again waits again.\n",
				strings.Join(missing, ", "), r.pr.URL, value.Number(*watch.Timeout))
			return state, r.report(temporary, "review-timeout"), true
		}
		sleep(min(max(interval, reviewPoll), left))
	}
}

// without are the logins that no review is by, as the config wrote them. A
// login matches an author when the two are equal once a trailing "[bot]" is
// dropped from each, GraphQL naming a bot without it and the REST API with it.
func (s pullState) without(logins []string) []string {
	var missing []string
	for _, login := range logins {
		had := false
		for _, who := range s.reviewers {
			had = had || strings.TrimSuffix(who, "[bot]") == strings.TrimSuffix(login, "[bot]")
		}
		if !had {
			missing = append(missing, login)
		}
	}
	return missing
}

// ask puts reviewQuery to GitHub (gh api graphql, which takes no --repo, so
// the repository is its owner and name) about the pull request: failed, with
// pull-request-failed reported, when gh does or its answer has not the
// reviews and the review threads.
func (r *pushRun) ask(ctx context.Context, repository string) (pullState, Answer, bool) {
	owner, name, _ := strings.Cut(repository, "/")
	answered, a, failed := r.gh(ctx, "api", "graphql", "-f", "query="+reviewQuery, "-f", "owner="+owner, "-f", "name="+name,
		"-F", "number="+strconv.Itoa(r.pr.Number))
	if failed {
		return pullState{}, a, true
	}
	var got struct {
		Data struct {
			Repository struct {
				PullRequest *struct {
					Reviews *struct {
						Nodes []struct {
							Author *struct {
								Login string `json:"login"`
							} `json:"author"`
						} `json:"nodes"`
					} `json:"reviews"`
					ReviewThreads *struct {
						Nodes []struct {
							IsResolved bool   `json:"isResolved"`
							Path       string `json:"path"`
							Line       *int   `json:"line"`
							Comments   struct {
								Nodes []struct {
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
		return pullState{}, r.ghFailed("gh api graphql", "its answer is not a pull request's reviews and review threads"), true
	}
	var state pullState
	for _, n := range pull.Reviews.Nodes {
		if n.Author != nil {
			state.reviewers = append(state.reviewers, n.Author.Login)
		}
	}
	for _, t := range pull.ReviewThreads.Nodes {
		if t.IsResolved {
			continue
		}
		open := openThread{path: t.Path}
		if t.Line != nil {
			open.line = *t.Line
		}
		if len(t.Comments.Nodes) > 0 {
			first, _, _ := strings.Cut(t.Comments.Nodes[0].Body, "\n")
			open.first = strings.TrimSpace(first)
		}
		state.unresolved = append(state.unresolved, open)
	}
	return state, Answer{}, false
}
