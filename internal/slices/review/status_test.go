package review

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
)

const (
	refusalWords = "Copilot was unable to review this pull request because the user who requested the review has insufficient remaining quota."
	statusPath   = "api repos/owner/name/statuses/" + head
)

// codexDone is Codex's review-state comment, both its reviews completed and
// of the commit given.
func codexDone(login, sha string) providers.Authored {
	return providers.Authored{Login: login, Body: "<!-- codex-review-state:v1 " +
		`{"code_review":{"status":"completed","commit_sha":"` + sha + `"},"security_review":{"status":"completed","commit_sha":"` + sha + `"}}` + " -->"}
}

// statusFake is a fake under push.mode pr whose review_by are the logins.
func statusFake(logins string) *fake {
	f := newFake()
	f.config = "push:\n  mode: pr\n  pr:\n    review_by: [" + logins + "]\n"
	return f
}

// Parse: status with one item and no flag; no item or two a usage error.
func TestParseStatus(t *testing.T) {
	c, err := Parse([]string{"status", "slice-9"}, "sub", true, true)
	if err != nil || !reflect.DeepEqual(c, CLI{Query: Query{Item: "slice-9"}, Status: true, JSON: true, Quiet: true}) {
		t.Errorf("%+v, %v", c, err)
	}
	for name, args := range map[string][]string{
		"no item":   {"status"},
		"two items": {"status", "slice-9", "slice-8"},
		"a flag":    {"status", "slice-9", "--at", "HEAD"},
	} {
		t.Run(name, func(t *testing.T) {
			var usage out.UsageError
			if _, err := Parse(args, "", false, false); !errors.As(err, &usage) {
				t.Errorf("%v: %v", args, err)
			}
		})
	}
}

// Each state posts one status on the head GitHub gives, the line and --json
// naming it: success once every login reviewed the head, pending naming the
// logins with no verdict, failure for a refusal whatever the others'.
func TestStatusStates(t *testing.T) {
	for name, c := range map[string]struct {
		logins          string
		reviews         []providers.Authored
		comments        []providers.Authored
		state, outcome  string
		description     string
		line            string
		missing, refuse []any
	}{
		"success": {
			logins: `a, "b[bot]"`, reviews: []providers.Authored{{Login: "b", Body: "looks fine"}}, comments: []providers.Authored{codexDone("a", head)},
			state: "success", outcome: "reviewed", description: "Reviewed by a, b[bot].",
			line: "Posted itos/reviews success on 0123456 of " + pull7 + ": reviewed by a, b[bot].\n", missing: []any{}, refuse: []any{},
		},
		"pending, a state of another commit": {
			logins: `a, b`, reviews: []providers.Authored{{Login: "b", Body: ""}}, comments: []providers.Authored{codexDone("a", oldHead)},
			state: "pending", outcome: "review-pending", description: "Waiting on a.",
			line: "Posted itos/reviews pending on 0123456 of " + pull7 + ": waiting on a.\n", missing: []any{"a"}, refuse: []any{},
		},
		"failure, whatever the others": {
			logins: `"c[bot]", a, d`, reviews: []providers.Authored{{Login: "c", Body: refusalWords + "\nmore"}, {Login: "d", Body: refusalWords}},
			state: "failure", outcome: "review-refused", description: "Refused to review: c[bot], d.",
			line:    "Posted itos/reviews failure on 0123456 of " + pull7 + `: c[bot] refused to review: "` + refusalWords + `"; d refused to review: "` + refusalWords + "\".\n",
			missing: []any{"c[bot]", "a", "d"}, refuse: []any{"c[bot]", "d"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := statusFake(c.logins)
			f.reviews, f.comments = c.reviews, c.comments
			code, stdout, stderr := run(t, f, false, "status", "slice-9")
			want := statusPath + " -f state=" + c.state + " -f context=itos/reviews -f description=" + c.description
			if code != 0 || stdout != c.line || stderr != "" || len(f.calls) != 3 || f.calls[2] != want {
				t.Errorf("exit %d\n%s%s\n%s\nwant\n%s%s", code, stdout, stderr, strings.Join(f.calls, "\n"), c.line, want)
			}
			f.calls = nil
			code, stdout, _ = run(t, f, true, "status", "slice-9")
			got := reported(t, stdout)
			if code != 0 || got["ok"] != true || got["outcome"] != c.outcome || got["item"] != "slice-9" || got["state"] != c.state || got["head"] != head ||
				got["description"] != c.description || !reflect.DeepEqual(got["missing"], c.missing) || !reflect.DeepEqual(got["refused"], c.refuse) ||
				!reflect.DeepEqual(got["pull_request"], map[string]any{"url": pull7, "number": float64(7)}) {
				t.Errorf("exit %d, %s", code, stdout)
			}
			at := -1
			for _, key := range []string{"schema", "ok", "outcome", "item", "state", "head", "pull_request", "url", "number", "description", "missing", "refused"} {
				next := strings.Index(stdout, `"`+key+`":`)
				if next <= at {
					t.Errorf("the key %s out of order: %s", key, stdout)
				}
				at = next
			}
			c, err := Parse([]string{"status", "slice-9"}, "", false, true)
			if err != nil {
				t.Fatal(err)
			}
			var quiet strings.Builder
			if code, err := c.Run(t.Context(), &quiet, &quiet, NewHandler(f)); code != 0 || err != nil || quiet.Len() != 0 {
				t.Errorf("-q: exit %d, %v, %q", code, err, quiet.String())
			}
		})
	}
}

// A description past 140 characters is cut at a whole login, the rest
// counted; when not even the first login fits, the logins are counted.
func TestDescribed(t *testing.T) {
	login := func(n int, c string) string { return strings.Repeat(c, n) }
	for name, c := range map[string]struct {
		logins []string
		want   string
	}{
		"fits":         {[]string{"a", "b"}, "Waiting on a, b."},
		"fits exactly": {[]string{login(128, "a")}, "Waiting on " + login(128, "a") + "."},
		"cut":          {[]string{login(57, "a"), login(57, "b"), login(57, "c")}, "Waiting on " + login(57, "a") + ", " + login(57, "b") + " and 1 more."},
		"one fits":     {[]string{login(110, "a"), login(10, "b"), login(10, "c")}, "Waiting on " + login(110, "a") + " and 2 more."},
		"none fits":    {[]string{login(130, "a"), login(5, "b")}, "Waiting on 2 logins."},
	} {
		t.Run(name, func(t *testing.T) {
			got := described("Waiting on ", c.logins)
			if got != c.want || len(got) > descriptionLimit {
				t.Errorf("%q (%d)\nwant %q", got, len(got), c.want)
			}
		})
	}
}

// Refused before gh is asked: no pr mode, no review_by (before the item is
// judged), an unknown item, a missing registry, a remote naming no
// repository (exit 3); refused once gh answers: no open pull request, a gh
// that fails or answers without the shape or a head, a status write that
// fails (exit 3, naming the call), nothing posted.
func TestStatusRefused(t *testing.T) {
	failing := func(words string) error { return &providers.GhFailed{Words: words, Err: errors.New("exit 1")} }
	for name, c := range map[string]struct {
		set     func(f *fake)
		code    int
		rule    string
		message string
		calls   int
	}{
		"no config":     {func(f *fake) { f.noConfig = true }, 1, "review-without-pr-mode", "does not set push.mode: pr", 0},
		"trunk":         {func(f *fake) { f.config = "push:\n  mode: trunk\n" }, 1, "review-without-pr-mode", "does not set push.mode: pr", 0},
		"no review_by":  {func(f *fake) { f.config = "push:\n  mode: pr\n"; f.registry = "" }, 1, "review-without-review-by", "push.pr.review_by names no login", 0},
		"unknown item":  {func(f *fake) { f.registry = "items: []\n" }, 1, "work-unknown-item", "slice-9", 0},
		"no registry":   {func(f *fake) { f.registry = "" }, 1, "work-registry-missing", "", 0},
		"no repository": {func(f *fake) { f.noRepo = true }, 3, "no-github-repository", "names no GitHub repository", 0},
		"no pull":       {func(f *fake) { f.answers["pr list"] = "[]" }, 1, "review-no-pull-request", "slice-9 has no open pull request", 1},
		"pr list fails": {func(f *fake) { f.ghErr["pr list"] = failing("HTTP 401: Bad credentials") }, 3, "review-pull-request-failed", "gh pr list failed: HTTP 401", 1},
		"graphql fails": {func(f *fake) { f.ghErr["api graphql"] = failing("boom") }, 3, "review-pull-request-failed", "gh api graphql failed: boom", 2},
		"graphql shape": {func(f *fake) { f.answers["api graphql"] = "{}" }, 3, "review-pull-request-failed", "not a pull request's reviews", 2},
		"graphql head":  {func(f *fake) { f.head = "" }, 3, "review-pull-request-failed", "names no head", 2},
		"status refused": {func(f *fake) {
			f.ghErr["api repos/owner/name/statuses/"+head] = failing("HTTP 403: Resource not accessible by integration")
		}, 3, "review-pull-request-failed", "gh api repos/owner/name/statuses/" + head + " failed: HTTP 403: Resource not accessible by integration", 3},
	} {
		t.Run(name, func(t *testing.T) {
			f := statusFake("a")
			c.set(f)
			code, stdout, _ := run(t, f, true, "status", "slice-9")
			got := reported(t, stdout)
			if code != c.code || got["ok"] != false || rule(got) != c.rule || !strings.Contains(stdout, c.message) || len(f.calls) != c.calls {
				t.Errorf("exit %d, %s, %v", code, stdout, f.calls)
			}
		})
	}
	f := statusFake("a")
	f.answers["pr list"] = "[]"
	code, stdout, stderr := run(t, f, false, "status", "slice-9")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "  fix: itos push from the item's branch opens its pull request; then itos review status again\n") {
		t.Errorf("exit %d, %s%s", code, stdout, stderr)
	}
}
