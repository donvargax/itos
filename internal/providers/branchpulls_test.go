package providers

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// answering is a gh that records its arguments and answers as given.
func answering(calls *[][]string, answer string, err error) func(context.Context, ...string) (string, error) {
	return func(_ context.Context, args ...string) (string, error) {
		*calls = append(*calls, args)
		return answer, err
	}
}

// The pull requests of each branch are asked in one gh api graphql call: an
// alias per branch, each branch a GraphQL string, the repository its owner
// and name, and read newest first with whether each is open or merged.
func TestBranchPulls(t *testing.T) {
	var calls [][]string
	answer := `{"data":{"repository":{
	  "p0":{"nodes":[{"number":9,"state":"OPEN","mergedAt":null},{"number":7,"state":"MERGED","mergedAt":"2026-10-24T10:00:00Z"}]},
	  "p1":{"nodes":[]}}}}`
	pulls, err := BranchPulls(t.Context(), answering(&calls, answer, nil), "o/n", []string{"slice-8", `a"b`})
	want := map[string][]BranchPull{"slice-8": {{Number: 9, Open: true}, {Number: 7, Merged: true}}}
	if err != nil || !reflect.DeepEqual(pulls, want) {
		t.Errorf("BranchPulls = %+v, %v, want %+v", pulls, err, want)
	}
	if len(calls) != 1 || len(calls[0]) != 8 || strings.Join(calls[0][:3], " ") != "api graphql -f" ||
		strings.Join(calls[0][4:], " ") != "-f owner=o -f name=n" {
		t.Fatalf("gh was put %q", calls)
	}
	wantQuery := `query=query($owner: String!, $name: String!) { repository(owner: $owner, name: $name) {` +
		` p0: pullRequests(headRefName: "slice-8", states: [OPEN, MERGED], first: 20, orderBy: {field: CREATED_AT, direction: DESC}) { nodes { number state mergedAt } }` +
		` p1: pullRequests(headRefName: "a\"b", states: [OPEN, MERGED], first: 20, orderBy: {field: CREATED_AT, direction: DESC}) { nodes { number state mergedAt } }` +
		` } }`
	if calls[0][3] != wantQuery {
		t.Errorf("the query:\n%s\nwant:\n%s", calls[0][3], wantQuery)
	}
}

// A gh that is not there, or that does not answer in time, is passed on as
// it is, any other failure is told as gh api graphql's, and an answer that is
// not JSON or lacks the repository or a branch's pull requests is one that
// gave none, saying which.
func TestBranchPullsErrors(t *testing.T) {
	failed := &GhFailed{Words: "HTTP 401: Bad credentials\n", Err: errors.New("exit status 1")}
	for name, c := range map[string]struct {
		answer string
		err    error
		is     error
		want   string
	}{
		"no gh":     {"", ErrNoGh, ErrNoGh, ""},
		"no answer": {"", ErrGhNoAnswer, ErrGhNoAnswer, ""},
		"failed":    {"", failed, nil, "gh api graphql failed: HTTP 401: Bad credentials"},
		"not JSON":  {"nope", nil, nil, "gh api graphql gave no pull requests: its answer is not JSON: "},
		"empty":     {"{}", nil, nil, "gh api graphql gave no pull requests: its answer has no repository"},
		"no alias":  {`{"data":{"repository":{"p1":{"nodes":[]}}}}`, nil, nil, "gh api graphql gave no pull requests: its answer has no pull requests for a"},
		"null":      {`{"data":{"repository":{"p0":null}}}`, nil, nil, "gh api graphql gave no pull requests: its answer has no pull requests for a"},
	} {
		var calls [][]string
		pulls, err := BranchPulls(t.Context(), answering(&calls, c.answer, c.err), "o/n", []string{"a"})
		switch {
		case pulls != nil || err == nil:
			t.Errorf("%s: %+v, %v", name, pulls, err)
		case c.is != nil && !errors.Is(err, c.is):
			t.Errorf("%s: %v, want %v", name, err, c.is)
		case c.is == nil && !strings.HasPrefix(err.Error(), c.want):
			t.Errorf("%s: %q, want %q", name, err, c.want)
		}
	}
}
