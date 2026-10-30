package providers

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// PullStateQuery is the one question a pull request's state is read with
// (gh api graphql, since gh pr view has no threads): its reviews, its
// comments (with the times they were made and last edited, a page of them
// after the cursor), the time it was last marked ready and its review
// threads, on a single line, since the recording gh of the scenarios keeps a
// call to one line.
var PullStateQuery = strings.Join(strings.Fields(`
	query($owner: String!, $name: String!, $number: Int!, $cursor: String) {
	repository(owner: $owner, name: $name) { pullRequest(number: $number) {
	timelineItems(last: 1, itemTypes: [READY_FOR_REVIEW_EVENT]) { nodes { ... on ReadyForReviewEvent { createdAt } } }
	reviews(first: 100) { nodes { author { login } body } }
	comments(first: 100, after: $cursor) { pageInfo { hasNextPage endCursor } nodes { author { login } body createdAt updatedAt } }
	reviewThreads(first: 100) { nodes { id isResolved path line
	comments(first: 1) { nodes { author { login } body } } } } } } }`), " ")

// CommentPages is how many pages of a hundred comments AskPullState reads at
// most.
const CommentPages = 20

// ErrNoPullState is an answer to PullStateQuery that is not a pull request's
// reviews and review threads.
var ErrNoPullState = errors.New("its answer is not a pull request's reviews and review threads")

// PullState is what GitHub says of a pull request: its reviews and comments,
// by the logins GraphQL gives, when it was last marked ready (zero when it
// does not say) and its review threads.
type PullState struct {
	Reviews  []Authored
	Comments []Authored
	ReadyAt  time.Time
	Threads  []ReviewThread
}

// Authored is a review or a comment: who wrote it, its body and, for a
// comment, when it was made and last edited (zero when GitHub does not say).
type Authored struct {
	Login, Body      string
	Created, Updated time.Time
}

// ReviewThread is a review thread: its node id, whether it is resolved,
// where it is (line 0 for an outdated one, which GitHub gives none), and the
// login of its first comment's author and that comment's body ("" for none).
type ReviewThread struct {
	ID       string
	Resolved bool
	Path     string
	Line     int
	Author   string
	Body     string
}

// AskPullState puts PullStateQuery to GitHub (gh api graphql, which takes no
// --repo, so the repository, "owner/name", is its owner and name) about the
// pull request numbered: gh is the call, its errors passed on as they come;
// an answer that has not the reviews and the review threads is
// ErrNoPullState (comments, which an answer from before slice 187 lacks, are
// none then, and the times from before slice 200 are none). The reviews and
// the threads are those of the first answer; the comments are all the pages
// of them, a hundred each, up to CommentPages, since a request or a review
// state that sits past the first hundred is read as none (slice 200).
func AskPullState(ctx context.Context, gh func(ctx context.Context, args ...string) (string, error),
	repository string, number int) (PullState, error) {
	owner, name, _ := strings.Cut(repository, "/")
	var state PullState
	cursor := ""
	for page := 0; ; page++ {
		args := []string{"api", "graphql", "-f", "query=" + PullStateQuery, "-f", "owner=" + owner, "-f", "name=" + name,
			"-F", "number=" + strconv.Itoa(number)}
		if cursor != "" {
			args = append(args, "-f", "cursor="+cursor)
		}
		answered, err := gh(ctx, args...)
		if err != nil {
			return PullState{}, err
		}
		got, next, ok := pullStatePage(answered)
		if !ok {
			return PullState{}, ErrNoPullState
		}
		if page == 0 {
			state = got
		} else {
			state.Comments = append(state.Comments, got.Comments...)
		}
		if next == "" || page+1 >= CommentPages {
			return state, nil
		}
		cursor = next
	}
}

// pullStatePage reads one answer to PullStateQuery: the state it gives, the
// cursor of the next page of comments ("" for none), and whether it has the
// reviews and the review threads.
func pullStatePage(answered string) (state PullState, next string, ok bool) {
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
		return PullState{}, "", false
	}
	state.Reviews, state.Comments = pull.Reviews.authored(), pull.Comments.authored()
	if pull.TimelineItems != nil && len(pull.TimelineItems.Nodes) > 0 {
		state.ReadyAt = githubTime(pull.TimelineItems.Nodes[0].CreatedAt)
	}
	for _, t := range pull.ReviewThreads.Nodes {
		thread := ReviewThread{ID: t.ID, Resolved: t.IsResolved, Path: t.Path}
		if t.Line != nil {
			thread.Line = *t.Line
		}
		if len(t.Comments.Nodes) > 0 {
			c := t.Comments.Nodes[0]
			thread.Body = c.Body
			if c.Author != nil {
				thread.Author = c.Author.Login
			}
		}
		state.Threads = append(state.Threads, thread)
	}
	return state, pull.Comments.next(), true
}

// githubTime is a time as GitHub writes it; zero for none or one that cannot
// be read.
func githubTime(text string) time.Time {
	at, _ := time.Parse(time.RFC3339, text)
	return at
}

// authoredNodes are GraphQL's reviews or comments; a node whose author is
// gone is no one's.
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

func (a *authoredNodes) authored() []Authored {
	var out []Authored
	if a == nil {
		return out
	}
	for _, n := range a.Nodes {
		if n.Author != nil {
			out = append(out, Authored{n.Author.Login, n.Body, githubTime(n.CreatedAt), githubTime(n.UpdatedAt)})
		}
	}
	return out
}
