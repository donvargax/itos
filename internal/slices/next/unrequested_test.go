package next

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// reviewersOver is forgeOver with push.pr.reviewers naming Codex.
func reviewersOver(t *testing.T, status string, answers map[string][]string, lines ...string) ports {
	t.Helper()
	p := over(t, fixture{
		config: prConfig + "  pr:\n    reviewers:\n      - { login: chatgpt-codex-connector, kind: codex }\n",
		files: map[string]string{"work-items.yaml": registryOf(
			item("id: slice-9, title: Nine, phase: 1, owner: ana, status: " + status + ", depends_on: []"),
		), "people.yaml": "- ana\n"},
		head: map[string]string{"features/a.feature": feature("@ID-A-01 @wip @slice-9")},
	})
	p.repository, p.gh = "o/r", &gh{answers: answers}
	if len(lines) > 0 {
		p.journal = []byte(strings.Join(lines, "\n") + "\n")
	}
	return p
}

// requestOf is a comment asking Codex for the line at the head, with itos's
// marker.
func requestOf(line, head string) string {
	text, _ := json.Marshal(map[string]any{"author": map[string]any{"login": "ana"},
		"body": line + "\n\n<!-- itos-review-request:v1 chatgpt-codex-connector " + head + " -->"})
	return string(text)
}

// With push.pr.reviewers, review-unrequested stands where merging and
// handed-back would, after repair, and a todo draft stays in spec review;
// observed's Unrequested names the reviewer until each of its requests is on
// the head, and is empty, never nil, for a merged pull request.
func TestForgeReviewUnrequested(t *testing.T) {
	draft := listed("7 OPEN true " + olderSHA)
	ready := listed("7 OPEN false " + olderSHA)
	both := []string{requestOf("@codex review", headSHA), requestOf("@codex security review", headSHA)}
	request := []string{"itos", "review", "request", "slice-9"}
	handed := line("implementer", 1, "done", "accepted", headSHA)
	for _, test := range []struct {
		name, status, list, pull string
		lines                    []string
		state                    string
		argv, unrequested        []string
	}{
		{name: "ready, nobody asked", status: "doing", list: ready, pull: pullAnswer(headSHA, "PENDING", nil),
			state: "review-unrequested", argv: request, unrequested: []string{"chatgpt-codex-connector"}},
		{name: "ready, only the code review asked", status: "doing", list: ready, pull: pullAnswer(headSHA, "SUCCESS", nil, both[0]),
			state: "review-unrequested", argv: request, unrequested: []string{"chatgpt-codex-connector"}},
		{name: "ready, a check failed", status: "doing", list: ready, pull: pullAnswer(headSHA, "FAILURE", nil),
			state: "repair", argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, unrequested: []string{"chatgpt-codex-connector"}},
		{name: "ready, both asked", status: "doing", list: ready, pull: pullAnswer(headSHA, "PENDING", nil, both...),
			state: "handed-back", argv: []string{"itos", "push", "--ready"}, unrequested: []string{}},
		{name: "ready, both asked, auto-merge on", status: "doing", list: ready, pull: autoMerged(pullAnswer(headSHA, "PENDING", nil, both...)),
			state: "merging", unrequested: []string{}},
		{name: "ready, nobody asked, auto-merge on", status: "doing", list: ready, pull: autoMerged(pullAnswer(headSHA, "PENDING", nil)),
			state: "review-unrequested", argv: request, unrequested: []string{"chatgpt-codex-connector"}},
		{name: "ready, asked at an older head", status: "doing", list: ready,
			pull:  pullAnswer(headSHA, "", nil, requestOf("@codex review", olderSHA), requestOf("@codex security review", olderSHA)),
			state: "review-unrequested", argv: request, unrequested: []string{"chatgpt-codex-connector"}},
		{name: "handed back, nobody asked", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil), lines: []string{handed},
			state: "review-unrequested", argv: request, unrequested: []string{"chatgpt-codex-connector"}},
		{name: "handed back, both asked", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil, both...), lines: []string{handed},
			state: "handed-back", argv: []string{"itos", "push", "--ready"}, unrequested: []string{}},
		{name: "implementing, nobody asked", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil),
			state: "implementing", argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, unrequested: []string{"chatgpt-codex-connector"}},
		{name: "a todo draft", status: "todo", list: draft, pull: pullAnswer(headSHA, "", nil),
			state: "spec-review", argv: []string{"itos", "brief", "slice-9", "--role", "reviewer"}, unrequested: []string{"chatgpt-codex-connector"}},
		{name: "merged", status: "doing", list: listed("7 MERGED false " + headSHA),
			state: "merged", argv: []string{"itos", "work", "done", "slice-9"}, unrequested: []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			answers := map[string][]string{"pr list": {test.list}}
			if test.pull != "" {
				answers["api graphql"] = []string{test.pull}
			}
			a, err := NewHandler(reviewersOver(t, test.status, answers, test.lines...)).Handle(t.Context(), Query{ID: "slice-9"})
			if err != nil || a.Next == nil {
				t.Fatalf("Handle = %+v, %v", a, err)
			}
			n := a.Next
			if n.State != test.state || !reflect.DeepEqual(n.Argv, test.argv) || !reflect.DeepEqual(n.Observed.Unrequested, test.unrequested) {
				t.Errorf("state %s, argv %v, unrequested %#v; want %s, %v, %#v", n.State, n.Argv, n.Observed.Unrequested, test.state, test.argv, test.unrequested)
			}
		})
	}
}

// The revision digests the reviewers not requested: a request posted gives
// a new one, while none unrequested, [] or nil, writes nothing, so a config
// with no reviewers keeps the revisions it had.
func TestRevisionDigestsUnrequested(t *testing.T) {
	of := func(unrequested []string) Facts {
		f := facts(t, "{ id: slice-9, status: doing }", "slice", nil, nil, wip, nil)
		f.Observed = &Observed{Checks: checksNone, Unrequested: unrequested}
		return f
	}
	if Revision(of(nil)) != Revision(of([]string{})) {
		t.Error("none unrequested, nil or empty, gives two revisions")
	}
	if Revision(of([]string{"a"})) == Revision(of([]string{})) || Revision(of([]string{"a"})) == Revision(of([]string{"b"})) {
		t.Error("the reviewers unrequested do not move the revision")
	}
}
