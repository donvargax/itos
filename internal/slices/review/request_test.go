package review

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
)

const (
	revision      = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	otherRevision = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	bothReviewers = "push:\n  mode: pr\n  pr:\n    reviewers:\n      - { login: chatgpt-codex-connector, kind: codex }\n      - { login: 'cursor[bot]', kind: bugbot }\n"
)

// requestFake is the fake with Codex and Bugbot configured, at the item's
// current revision.
func requestFake() *fake {
	f := newFake()
	f.config = bothReviewers
	f.revision = revision
	return f
}

// posted are the pr comment calls f was given.
func (f *fake) posted() []string {
	var calls []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "pr comment ") {
			calls = append(calls, c)
		}
	}
	return calls
}

// commentOf is the pr comment call of the line asked of the login at the head.
func commentOf(line, login string) string {
	return "pr comment 7 --repo owner/name --body=" + line + "\n\n<!-- itos-review-request:v1 " + login + " " + head + " -->"
}

// Parse: request with one item, and --expect a revision; no item, two, a
// malformed --expect, or post's flags are usage errors.
func TestParseRequest(t *testing.T) {
	c, err := Parse([]string{"request", "slice-9", "--expect", revision}, "", true, false)
	if err != nil || !reflect.DeepEqual(c, CLI{Query: Query{Request: true, Item: "slice-9", Expect: revision}, JSON: true}) {
		t.Errorf("%+v, %v", c, err)
	}
	if c, err := Parse([]string{"request", "slice-9"}, "", false, false); err != nil || c.Expect != "" || !c.Request {
		t.Errorf("without --expect: %+v, %v", c, err)
	}
	for name, args := range map[string][]string{
		"no item":           {"request"},
		"two items":         {"request", "slice-9", "slice-8"},
		"a short revision":  {"request", "slice-9", "--expect", "sha256:abc"},
		"upper case":        {"request", "slice-9", "--expect", strings.ToUpper(revision)},
		"no prefix":         {"request", "slice-9", "--expect", revision[len("sha256:"):]},
		"--expect no value": {"request", "slice-9", "--expect"},
		"post's flag":       {"request", "slice-9", "--verdict", "approve"},
	} {
		t.Run(name, func(t *testing.T) {
			var usage out.UsageError
			if _, err := Parse(args, "", false, false); !errors.As(err, &usage) {
				t.Errorf("%v: %v", args, err)
			}
		})
	}
}

// Nobody asked: Codex's two requests, then Bugbot's, one gh pr comment each
// by number, each the line, an empty line and the marker; the text names
// each, --json lists them in order.
func TestRequestAsksEachReviewer(t *testing.T) {
	f := requestFake()
	code, stdout, stderr := run(t, f, false, "request", "slice-9")
	want := []string{
		commentOf("@codex review", "chatgpt-codex-connector"),
		commentOf("@codex security review", "chatgpt-codex-connector"),
		commentOf("@cursor review", "cursor[bot]"),
	}
	if code != 0 || stderr != "" || !reflect.DeepEqual(f.posted(), want) {
		t.Errorf("exit %d, comments\n%s\n%s", code, strings.Join(f.posted(), "\n"), stderr)
	}
	lines := "Asked chatgpt-codex-connector at 0123456 on " + pull7 + ": @codex review.\n" +
		"Asked chatgpt-codex-connector at 0123456 on " + pull7 + ": @codex security review.\n" +
		"Asked cursor[bot] at 0123456 on " + pull7 + ": @cursor review.\n"
	if stdout != lines {
		t.Errorf("said\n%s", stdout)
	}
	if strings.Contains(strings.Join(f.calls, "\n"), "api repos/") {
		t.Errorf("an issue comment was posted through gh api: %v", f.calls)
	}

	f = requestFake()
	code, stdout, _ = run(t, f, true, "request", "slice-9", "--expect", revision)
	got := reported(t, stdout)
	posted := []any{
		map[string]any{"login": "chatgpt-codex-connector", "kind": "codex", "request": "@codex review"},
		map[string]any{"login": "chatgpt-codex-connector", "kind": "codex", "request": "@codex security review"},
		map[string]any{"login": "cursor[bot]", "kind": "bugbot", "request": "@cursor review"},
	}
	if code != 0 || got["outcome"] != "requested" || got["item"] != "slice-9" || got["head"] != head || !reflect.DeepEqual(got["posted"], posted) {
		t.Errorf("--json: exit %d, %s", code, stdout)
	}

	f = requestFake()
	c, _ := Parse([]string{"request", "slice-9"}, "", false, true)
	var quiet bytes.Buffer
	if code, err := c.Run(context.Background(), &quiet, &quiet, NewHandler(f)); code != 0 || err != nil || quiet.Len() != 0 || len(f.posted()) != 3 {
		t.Errorf("-q: exit %d, %v, %q", code, err, quiet.String())
	}
}

// Every request on the head, by a marked request or the reviewer's own
// review: already, nothing posted, whatever --expect says.
func TestRequestAlready(t *testing.T) {
	f := requestFake()
	f.comments = []providers.Authored{
		{Login: "ana", Body: "@codex review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector " + head + " -->"},
		{Login: "chatgpt-codex-connector", Body: `<!-- codex-review-state:v1 {"code_review":{"commit_sha":"` + oldHead + `"},"security_review":{"commit_sha":"` + head + `"}} -->`},
	}
	f.reviews = []providers.Authored{{Login: "cursor", Body: "<!-- BUGBOT_REVIEW -->\nno issues", Commit: head}}
	code, stdout, _ := run(t, f, false, "request", "slice-9", "--expect", otherRevision)
	if code != 0 || len(f.posted()) != 0 || stdout != "Every reviewer of slice-9 has a request or a review for 0123456 on "+pull7+"; nothing was posted.\n" {
		t.Errorf("exit %d, %v\n%s", code, f.posted(), stdout)
	}
	code, stdout, _ = run(t, f, true, "request", "slice-9")
	got := reported(t, stdout)
	if code != 0 || got["outcome"] != "already" || !reflect.DeepEqual(got["posted"], []any{}) || got["head"] != head {
		t.Errorf("--json: exit %d, %s", code, stdout)
	}
}

// --expect: a revision that is not the item's is review-stale, nothing
// posted, the current one named; an item next gives no revision for posts
// as without it; what next cannot read is the error.
func TestRequestExpect(t *testing.T) {
	f := requestFake()
	code, stdout, _ := run(t, f, true, "request", "slice-9", "--expect", otherRevision)
	got := reported(t, stdout)
	problems, _ := got["problems"].([]any)
	if code != 1 || rule(got) != "review-stale" || len(f.posted()) != 0 ||
		!strings.Contains(problems[0].(map[string]any)["message"].(string), revision) ||
		!strings.Contains(problems[0].(map[string]any)["fix"].(string), "itos next slice-9") {
		t.Errorf("stale: exit %d, %v, %s", code, f.posted(), stdout)
	}

	f = requestFake()
	f.revProblem = &out.Problem{Rule: "work-unknown-item"}
	if code, _, _ := run(t, f, false, "request", "slice-9", "--expect", otherRevision); code != 0 || len(f.posted()) != 3 {
		t.Errorf("no revision: exit %d, %v", code, f.posted())
	}

	f = requestFake()
	f.revErr = kind.Wrap(kind.Missing, errors.New("gh is not on the PATH"))
	c, _ := Parse([]string{"request", "slice-9", "--expect", otherRevision}, "", false, false)
	var sink bytes.Buffer
	if _, err := c.Run(context.Background(), &sink, &sink, NewHandler(f)); err == nil || len(f.posted()) != 0 {
		t.Errorf("next cannot read: %v, %v", err, f.posted())
	}

	f = requestFake()
	f.registry = "items: [\n"
	if _, err := NewHandler(f).Handle(context.Background(), Query{Request: true, Item: "slice-9"}); err == nil || len(f.calls) != 0 {
		t.Errorf("a registry that is not YAML: %v, %v", err, f.calls)
	}
}

// The first gh pr comment that fails stops the command, exit 3, naming gh's
// words, the requests before it posted and none after it tried.
func TestRequestStopsAtTheFirstFailure(t *testing.T) {
	f := requestFake()
	f.failWith = "@codex security review"
	code, stdout, _ := run(t, f, true, "request", "slice-9")
	got := reported(t, stdout)
	problems, _ := got["problems"].([]any)
	if code != 3 || rule(got) != "review-pull-request-failed" ||
		!strings.Contains(problems[0].(map[string]any)["message"].(string), "gh pr comment failed: HTTP 502: Bad Gateway") ||
		!strings.Contains(problems[0].(map[string]any)["fix"].(string), "itos review request slice-9") {
		t.Errorf("exit %d, %s", code, stdout)
	}
	if want := []string{commentOf("@codex review", "chatgpt-codex-connector"), commentOf("@codex security review", "chatgpt-codex-connector")}; !reflect.DeepEqual(f.posted(), want) {
		t.Errorf("comments\n%s", strings.Join(f.posted(), "\n"))
	}
}

// Refused before gh is asked: outside push.mode pr, an unknown item, no
// reviewers (review_by names none to ask); then no repository, no open pull
// request and a gh that fails, each naming review request to run again.
func TestRequestRefusals(t *testing.T) {
	for name, test := range map[string]struct {
		edit    func(*fake)
		rule    string
		code    int
		message string
		asked   bool
	}{
		"trunk": {func(f *fake) { f.config = "" }, "review-without-pr-mode", 1, "a request goes on an item's pull request", false},
		"unknown item": {func(f *fake) {
			f.registry = "items:\n  - { id: slice-8, title: slice-8, phase: 1, owner: null, status: doing }\n"
		}, "work-unknown-item", 1, "slice-9", false},
		"no reviewers": {func(f *fake) { f.config = "push:\n  mode: pr\n  pr:\n    review_by: [chatgpt-codex-connector]\n" },
			"review-no-reviewers", 1, "push.pr.reviewers names no reviewer to ask; nothing was posted.", false},
		"no repository": {func(f *fake) { f.noRepo = true }, "no-github-repository", 3, "names no GitHub repository", false},
		"no pull request": {func(f *fake) { f.answers["pr list"] = "[]" }, "review-no-pull-request", 1,
			"slice-9 has no open pull request", true},
		"graphql fails": {func(f *fake) { f.ghErr["api graphql"] = &providers.GhFailed{Words: "boom", Err: errors.New("exit 1")} },
			"review-pull-request-failed", 3, "gh api graphql failed: boom", true},
	} {
		t.Run(name, func(t *testing.T) {
			f := requestFake()
			test.edit(f)
			code, stdout, _ := run(t, f, true, "request", "slice-9")
			got := reported(t, stdout)
			problems, _ := got["problems"].([]any)
			if code != test.code || rule(got) != test.rule || !strings.Contains(problems[0].(map[string]any)["message"].(string), test.message) {
				t.Errorf("exit %d, %s", code, stdout)
			}
			if (len(f.calls) > 0) != test.asked || len(f.posted()) != 0 {
				t.Errorf("gh was called %v", f.calls)
			}
			if fix := problems[0].(map[string]any)["fix"].(string); test.asked && !strings.Contains(fix, "itos review request again") {
				t.Errorf("the fix %q", fix)
			}
		})
	}
}
