package status

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/value"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// pullsAnswer is gh's answer to the pull requests query for d1 and d2: d1's
// newest pull request merged, d2's open and an earlier one merged.
const pullsAnswer = `{"data":{"repository":{
  "p0":{"nodes":[{"number":7,"state":"MERGED","mergedAt":"2026-10-24T10:00:00Z"},{"number":5,"state":"MERGED","mergedAt":"2026-10-20T10:00:00Z"}]},
  "p1":{"nodes":[{"number":9,"state":"OPEN","mergedAt":null},{"number":6,"state":"MERGED","mergedAt":"2026-10-21T10:00:00Z"}]}}}}`

// pulls runs readPulls over the config text for d1 and d2, in progress, with
// the environment, gh answering as given, and gives the standing and the
// calls gh was put.
func pulls(t *testing.T, text string, env map[string]string, answer string, failure error) (*Standing, [][]string, []*value.Map) {
	t.Helper()
	v := world()
	cfg := configOf(t, v, text, nil)
	var calls [][]string
	in := inOf(v, env)
	in.Gh = func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		return answer, failure
	}
	doing := []*value.Map{value.NewMap("id", "d1", "title", "One"), value.NewMap("id", "d2", "title", "Two")}
	st := &Standing{merged: map[any]int{}, remote: "origin"}
	st.readPulls(t.Context(), cfg, doing, in)
	return st, calls, doing
}

const prText = configText + "push:\n  mode: pr\n"

// Under pr, an item in progress whose newest pull request merged is marked
// with its number, one with an open pull request is not, however many
// merged before it; the call is one gh api graphql naming the repository,
// and the item's JSON gains the key.
func TestReadPullsMarksTheMergedItem(t *testing.T) {
	st, calls, doing := pulls(t, prText, nil, pullsAnswer, nil)
	if len(calls) != 1 || len(calls[0]) != 8 || strings.Join(calls[0][:3], " ") != "api graphql -f" ||
		strings.Join(calls[0][4:], " ") != "-f owner=o -f name=n" || !strings.HasPrefix(calls[0][3], "query=") {
		t.Fatalf("gh was put %q", calls)
	}
	query := calls[0][3]
	for _, want := range []string{`p0: pullRequests(headRefName: "d1", states: [OPEN, MERGED], first: 20, `, `p1: pullRequests(headRefName: "d2"`,
		"orderBy: {field: CREATED_AT, direction: DESC}) { nodes { number state mergedAt } }"} {
		if !strings.Contains(query, want) {
			t.Errorf("the query lacks %q:\n%s", want, query)
		}
	}
	if len(st.merged) != 1 || st.merged[doing[0]] != 7 || doing[0].At("merged_pull_request") != 7.0 || doing[1].Has("merged_pull_request") {
		t.Errorf("merged %v; d1 %s; d2 %s", st.merged, value.JSON(doing[0]), value.JSON(doing[1]))
	}
	if len(st.Unread) != 0 {
		t.Errorf("unread %q", st.Unread)
	}
}

// Nothing is asked under trunk, a stealth config, with nothing in progress
// or with no repository to name.
func TestReadPullsAsksNothingWhereItIsNotToBeAsked(t *testing.T) {
	v := world()
	stealth := configOf(t, v, prText, nil)
	stealth.Stealth = true
	nothing := configOf(t, v, prText, nil)
	elsewhere := configOf(t, &vcsfake.VCS{Remote: map[string]*vcsfake.Remote{"origin": {URL: "/srv/git/n.git"}}}, prText, nil)
	for name, c := range map[string]struct {
		cfg   *config.Loaded
		doing []*value.Map
	}{
		"trunk":         {configOf(t, v, configText, nil), []*value.Map{value.NewMap("id", "d1")}},
		"stealth":       {stealth, []*value.Map{value.NewMap("id", "d1")}},
		"nothing doing": {nothing, nil},
		"no repository": {elsewhere, []*value.Map{value.NewMap("id", "d1")}},
	} {
		in := inOf(v, nil)
		in.Gh = func(context.Context, ...string) (string, error) { t.Errorf("%s: gh was asked", name); return "", nil }
		st := &Standing{merged: map[any]int{}, remote: "origin"}
		st.readPulls(t.Context(), c.cfg, c.doing, in)
		if len(st.merged) != 0 || len(st.Unread) != 0 {
			t.Errorf("%s: merged %v, unread %q", name, st.merged, st.Unread)
		}
	}
}

// A gh that is not there says nothing; one that fails, answers wrongly or
// does not answer in time is a line, and no item is marked.
func TestReadPullsSaysWhatItCannotRead(t *testing.T) {
	for name, c := range map[string]struct {
		answer  string
		failure error
		want    string
	}{
		"no gh":      {"", providers.ErrNoGh, ""},
		"no answer":  {"", providers.ErrGhNoAnswer, "The pull requests cannot be read: gh gave no answer within 5 s."},
		"gh failed":  {"", &providers.GhFailed{Words: "HTTP 401\n", Err: errors.New("exit status 1")}, "The pull requests cannot be read: gh api graphql failed: HTTP 401"},
		"not JSON":   {"nope", nil, "The pull requests cannot be read: gh api graphql gave no pull requests: its answer is not JSON: "},
		"no repo":    {`{"data":{"repository":null}}`, nil, "The pull requests cannot be read: gh api graphql gave no pull requests: its answer has no repository"},
		"no alias":   {`{"data":{"repository":{"p0":{"nodes":[]}}}}`, nil, "The pull requests cannot be read: gh api graphql gave no pull requests: its answer has no pull requests for d2"},
		"no nodes":   {`{"data":{"repository":{"p0":null,"p1":null}}}`, nil, "The pull requests cannot be read: gh api graphql gave no pull requests: its answer has no pull requests for d1"},
		"none found": {`{"data":{"repository":{"p0":{"nodes":[]},"p1":{"nodes":[]}}}}`, nil, ""},
	} {
		st, _, _ := pulls(t, prText, nil, c.answer, c.failure)
		if len(st.merged) != 0 {
			t.Errorf("%s: merged %v", name, st.merged)
		}
		if (c.want == "") != (len(st.Unread) == 0) || len(st.Unread) > 0 && !strings.HasPrefix(st.Unread[0], c.want) {
			t.Errorf("%s: unread %q, want %q", name, st.Unread, c.want)
		}
	}
}

// The newest merged pull request is the number, unless one is open, wherever
// in the list; none merged, none.
func TestMergedPull(t *testing.T) {
	for name, c := range map[string]struct {
		pulls []providers.BranchPull
		want  int
	}{
		"none":              {nil, 0},
		"merged":            {[]providers.BranchPull{{Number: 3, Merged: true}}, 3},
		"the newest merged": {[]providers.BranchPull{{Number: 4, Merged: true}, {Number: 3, Merged: true}}, 4},
		"open first":        {[]providers.BranchPull{{Number: 5, Open: true}, {Number: 4, Merged: true}}, 0},
		"open last":         {[]providers.BranchPull{{Number: 4, Merged: true}, {Number: 3, Open: true}}, 0},
	} {
		if got := mergedPull(c.pulls); got != c.want {
			t.Errorf("%s: %d, want %d", name, got, c.want)
		}
	}
}

// The text marks a merged item's line with the work done that closes it,
// and --json gives the number on its entry.
func TestPrintMarksTheMergedItem(t *testing.T) {
	v := world()
	cfg := configOf(t, v, prText, map[string]string{"people.yaml": "- a\n",
		"work-items.yaml": items(nil, "{ id: d1, title: One, phase: 1, owner: a, status: doing, depends_on: [] }",
			"{ id: d2, title: Two, phase: 1, owner: a, status: doing, depends_on: [] }")})
	in := inOf(v, map[string]string{"GITHUB_REPOSITORY": "o/n"})
	in.Gh = func(context.Context, ...string) (string, error) { return pullsAnswer, nil }
	st, _, _, err := Of(t.Context(), cfg, "a", in)
	if err != nil {
		t.Fatal(err)
	}
	if got := printed(st); !strings.Contains(got, "In progress for a:\n  d1  One  merged (#7): itos work done d1\n  d2  Two\n") {
		t.Errorf("the status:\n%s", got)
	}
	if got := emitted(t, st); !strings.Contains(got, `"merged_pull_request": 7`) ||
		strings.Count(got, "merged_pull_request") != 1 {
		t.Errorf("the JSON:\n%s", got)
	}
}
