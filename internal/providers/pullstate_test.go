package providers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The query is one line, so a recording gh keeps the call to one line, and
// it asks for the comments after a cursor.
func TestPullStateQueryIsOneLine(t *testing.T) {
	if strings.ContainsAny(PullStateQuery, "\n\t") || strings.Contains(PullStateQuery, "  ") || !strings.HasPrefix(PullStateQuery, "query($owner: String!, ") {
		t.Errorf("the query is not on one line: %q", PullStateQuery)
	}
	for _, want := range []string{"after: $cursor", "pageInfo { hasNextPage endCursor }", "reviewThreads(first: 100)", "READY_FOR_REVIEW_EVENT"} {
		if !strings.Contains(PullStateQuery, want) {
			t.Errorf("the query lacks %q: %s", want, PullStateQuery)
		}
	}
}

// pageAnswer is an answer to PullStateQuery with the comments given and a
// next page when more is set.
func pageAnswer(comments string, more string) string {
	pageInfo := `"pageInfo":{"hasNextPage":false,"endCursor":"end"},`
	if more != "" {
		pageInfo = fmt.Sprintf(`"pageInfo":{"hasNextPage":true,"endCursor":%q},`, more)
	}
	return `{"data":{"repository":{"pullRequest":{` +
		`"timelineItems":{"nodes":[{"createdAt":"2026-10-24T10:00:00Z"}]},` +
		`"reviews":{"nodes":[{"author":{"login":"codex"},"body":"LGTM"},{"author":null,"body":"gone"}]},` +
		`"comments":{` + pageInfo + `"nodes":[` + comments + `]},` +
		`"reviewThreads":{"nodes":[` +
		`{"id":"T1","isResolved":false,"path":"a.go","line":3,"comments":{"nodes":[{"author":{"login":"ana"},"body":"first line\nsecond"}]}},` +
		`{"id":"T2","isResolved":true,"path":"b.go","line":null,"comments":{"nodes":[]}},` +
		`{"id":"T3","isResolved":false,"path":"c.go","line":null,"comments":{"nodes":[{"author":null,"body":"ghost's"}]}}` +
		`]}}}}}`
}

// The state read from one answer: the reviews and comments by their logins,
// a node whose author is gone left out, the times as GitHub writes them, the
// ready time and every thread with its first comment, an outdated one with
// line 0 and a ghost's with no author.
func TestAskPullStateReadsOneAnswer(t *testing.T) {
	var calls [][]string
	gh := func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		return pageAnswer(`{"author":{"login":"someone"},"body":"hi","createdAt":"2026-10-24T11:00:00Z","updatedAt":"2026-10-24T12:00:00Z"}`, ""), nil
	}
	got, err := AskPullState(context.Background(), gh, "owner/name", 7)
	if err != nil {
		t.Fatal(err)
	}
	at := func(text string) time.Time { return githubTime(text) }
	want := PullState{
		Reviews:  []Authored{{Login: "codex", Body: "LGTM"}},
		Comments: []Authored{{Login: "someone", Body: "hi", Created: at("2026-10-24T11:00:00Z"), Updated: at("2026-10-24T12:00:00Z")}},
		ReadyAt:  at("2026-10-24T10:00:00Z"),
		Threads: []ReviewThread{
			{ID: "T1", Path: "a.go", Line: 3, Author: "ana", Body: "first line\nsecond"},
			{ID: "T2", Resolved: true, Path: "b.go"},
			{ID: "T3", Path: "c.go", Body: "ghost's"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("read\n%+v\nwant\n%+v", got, want)
	}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], []string{"api", "graphql", "-f", "query=" + PullStateQuery, "-f", "owner=owner", "-f", "name=name", "-F", "number=7"}) {
		t.Errorf("gh was called %v", calls)
	}
}

// A second page of comments is asked for with the cursor, and joins the
// first page's; the reviews and threads are the first answer's. The pages
// stop at CommentPages.
func TestAskPullStatePagesTheComments(t *testing.T) {
	var cursors []string
	gh := func(_ context.Context, args ...string) (string, error) {
		cursor := ""
		if len(args) > 10 {
			cursor = args[len(args)-1]
		}
		cursors = append(cursors, cursor)
		if cursor == "" {
			return pageAnswer(`{"author":{"login":"a"},"body":"one"}`, "page2"), nil
		}
		return pageAnswer(`{"author":{"login":"b"},"body":"two"}`, ""), nil
	}
	got, err := AskPullState(context.Background(), gh, "owner/name", 7)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cursors, []string{"", "cursor=page2"}) {
		t.Errorf("the pages asked: %v", cursors)
	}
	if len(got.Comments) != 2 || got.Comments[0].Body != "one" || got.Comments[1].Body != "two" || len(got.Reviews) != 1 || len(got.Threads) != 3 {
		t.Errorf("read %+v", got)
	}

	// Every page has a next one: the read stops at CommentPages.
	asked := 0
	endless := func(_ context.Context, args ...string) (string, error) {
		asked++
		return pageAnswer(`{"author":{"login":"a"},"body":"more"}`, "again"), nil
	}
	got, err = AskPullState(context.Background(), endless, "owner/name", 7)
	if err != nil || asked != CommentPages || len(got.Comments) != CommentPages {
		t.Errorf("asked %d pages, read %d comments, %v", asked, len(got.Comments), err)
	}
}

// gh's error is passed on as it comes; an answer that is not JSON, or has
// no pull request, reviews or threads, is ErrNoPullState; one without
// comments or a timeline reads as having none.
func TestAskPullStateErrors(t *testing.T) {
	failed := &GhFailed{Words: "HTTP 401", Err: errors.New("exit 1")}
	gh := func(_ context.Context, args ...string) (string, error) { return "", failed }
	if _, err := AskPullState(context.Background(), gh, "owner/name", 7); !errors.Is(err, failed) {
		t.Errorf("a gh that fails: %v", err)
	}
	for name, answer := range map[string]string{
		"not JSON":      "nope",
		"no pull":       `{"data":{"repository":{"pullRequest":null}}}`,
		"no reviews":    `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[]}}}}}`,
		"no threads":    `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[]}}}}}`,
		"second page":   "",
		"page not JSON": "",
	} {
		t.Run(name, func(t *testing.T) {
			page := 0
			gh := func(_ context.Context, args ...string) (string, error) {
				page++
				if name == "second page" || name == "page not JSON" {
					if page == 1 {
						return pageAnswer(``, "page2"), nil
					}
					if name == "second page" {
						return `{"data":{"repository":{"pullRequest":null}}}`, nil
					}
					return "{", nil
				}
				return answer, nil
			}
			if _, err := AskPullState(context.Background(), gh, "owner/name", 7); !errors.Is(err, ErrNoPullState) {
				t.Errorf("%s: %v", name, err)
			}
		})
	}
	bare := func(_ context.Context, args ...string) (string, error) {
		return `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[{"author":{"login":"x"},"body":""}]},"reviewThreads":{"nodes":[]}}}}}`, nil
	}
	got, err := AskPullState(context.Background(), bare, "owner/name", 7)
	if err != nil || len(got.Comments) != 0 || !got.ReadyAt.IsZero() || len(got.Reviews) != 1 || len(got.Threads) != 0 {
		t.Errorf("an answer without comments or a timeline: %+v, %v", got, err)
	}
	if !githubTime("not a time").IsZero() {
		t.Error("a time that cannot be read is not zero")
	}
}
