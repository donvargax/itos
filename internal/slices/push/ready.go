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

// reviewQuery is the one question the reviews, the comments and the review
// threads are read with (gh api graphql, since gh pr view has no threads), on a
// single line.
var reviewQuery = strings.Join(strings.Fields(`
	query($owner: String!, $name: String!, $number: Int!) {
	repository(owner: $owner, name: $name) { pullRequest(number: $number) {
	reviews(first: 100) { nodes { author { login } body } }
	comments(first: 100) { nodes { author { login } body } }
	reviewThreads(first: 100) { nodes { id isResolved path line
	comments(first: 1) { nodes { author { login } body } } } } } } }`), " ")

// pullState is what GitHub says of the pull request: its reviews and comments,
// by the logins GraphQL gives, the node ids of all its review threads and the
// threads still open.
type pullState struct {
	reviews    []authored
	comments   []authored
	ids        []string
	unresolved []openThread
}

// authored is a review or a comment: who wrote it and its body.
type authored struct {
	login, body string
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
// auto-merge turned on, so GitHub never merges before the checks and the
// reviews. Whichever stops it exits without turning auto-merge on, naming
// what is left. outcome is pushed or nothing-to-push, line what the push says
// of it.
func (r *pushRun) finish(ctx context.Context, repository, outcome, line string) Answer {
	cfg := r.cfg
	r.announce(line)
	w := newWaiting(r, cfg.Push.PR.ReviewBy)
	wr, ok, err := r.ports.Watcher(ctx, cfg, r.remote)
	if ok || err != nil {
		if ok && err == nil {
			w.run, r.waiting = "waiting", w
		}
		if a, failed := r.watchRun(ctx, cfg, wr, err, r.remote, r.sha); failed {
			return a
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
	if _, a, failed := r.gh(ctx, "pr", "merge", r.branch, "--repo", repository, "--auto", "--rebase"); failed {
		return a
	}
	r.pr.AutoMerge = true
	return r.report(0, outcome, fmt.Sprintf("Auto-merge is on: GitHub merges %s by rebase once its required checks pass.", r.pr.URL))
}

// awaitReviews asks GitHub for the pull request at once and each
// ci.watch.interval until each login has a verdict, or ci.watch.timeout,
// counted from the first ask, has passed: done, with the exit reported (75,
// outcome review-timeout, naming the logins that have none; or at once 75,
// review-refused, for a login whose reviews are all refusals; or 75,
// review-stalled, see stalled), or the failure of gh api graphql. The last
// answer is the state. w says what is waited on, at the start and at each
// look that changes it.
func (r *pushRun) awaitReviews(ctx context.Context, repository string, w *waiting) (pullState, Answer, bool) {
	logins := w.logins
	watch := r.cfg.CI.Watch
	interval := time.Duration(*watch.Interval * float64(time.Second))
	deadline := now().Add(time.Duration(*watch.Timeout * float64(time.Second)))
	r.say(fmt.Sprintf("Waiting for reviews by %s on %s.", strings.Join(logins, ", "), r.pr.URL))
	if w.run == "" {
		w.show()
	}
	var st *stalls
	if seconds := r.cfg.Push.PR.ReviewStall; seconds > 0 {
		st = &stalls{seconds: seconds, since: map[string]time.Time{}, asked: map[string]time.Time{}}
	}
	for {
		state, a, failed := r.ask(ctx, repository, r.pr.Number)
		if failed {
			return pullState{}, a, true
		}
		missing, refused := state.verdicts(logins)
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
		left := deadline.Sub(now())
		if left <= 0 {
			fmt.Fprintf(r.o.Stderr, "itos: no review by %s on %s after %s seconds (ci.watch.timeout), so auto-merge is not turned on; itos push --ready again waits again.\n",
				strings.Join(missing, ", "), r.pr.URL, value.Number(*watch.Timeout))
			return state, r.report(temporary, "review-timeout"), true
		}
		if st != nil {
			if a, done := r.stalled(ctx, repository, w, st, state, missing); done {
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
// (missing) from those among them whose reviews are all refusals (refused, in
// the config's order). A login that wrote a review-state comment has the verdict
// that comment gives and no other; else any review of its that is not a refusal
// is one.
func (s pullState) verdicts(logins []string) (missing []string, refused []refusal) {
	for _, login := range logins {
		if completed, stated := s.stateOf(login); stated {
			if !completed {
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

// stateOf reads the login's review-state comment, the last it wrote with a
// "<!-- codex-review-state:v1 {json} -->" line: stated whether it wrote one,
// completed whether both of its reviews are.
func (s pullState) stateOf(login string) (completed, stated bool) {
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
			var state struct {
				CodeReview struct {
					Status string `json:"status"`
				} `json:"code_review"`
				SecurityReview struct {
					Status string `json:"status"`
				} `json:"security_review"`
			}
			stated = true
			completed = json.Unmarshal([]byte(text), &state) == nil &&
				state.CodeReview.Status == "completed" && state.SecurityReview.Status == "completed"
		}
	}
	return completed, stated
}

// ask puts reviewQuery to GitHub (gh api graphql, which takes no --repo, so
// the repository is its owner and name) about the pull request: failed, with
// pull-request-failed reported, when gh does or its answer has not the
// reviews and the review threads (comments, which an answer from before slice
// 187 lacks, are none then).
func (r *pushRun) ask(ctx context.Context, repository string, number int) (pullState, Answer, bool) {
	owner, name, _ := strings.Cut(repository, "/")
	answered, a, failed := r.gh(ctx, "api", "graphql", "-f", "query="+reviewQuery, "-f", "owner="+owner, "-f", "name="+name,
		"-F", "number="+strconv.Itoa(number))
	if failed {
		return pullState{}, a, true
	}
	var got struct {
		Data struct {
			Repository struct {
				PullRequest *struct {
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
		return pullState{}, r.ghFailed("gh api graphql", "its answer is not a pull request's reviews and review threads"), true
	}
	var state pullState
	state.reviews, state.comments = pull.Reviews.authored(), pull.Comments.authored()
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
	return state, Answer{}, false
}

// authoredNodes are GraphQL's reviews or comments; a node whose author is gone
// is no one's.
type authoredNodes struct {
	Nodes []struct {
		Author *struct {
			Login string `json:"login"`
		} `json:"author"`
		Body string `json:"body"`
	} `json:"nodes"`
}

func (a *authoredNodes) authored() []authored {
	var out []authored
	if a == nil {
		return out
	}
	for _, n := range a.Nodes {
		if n.Author != nil {
			out = append(out, authored{n.Author.Login, n.Body})
		}
	}
	return out
}
