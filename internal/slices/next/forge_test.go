package next

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/reviewers"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/providers"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/work"
)

// gh is the forge's fake gh: its answers by the call's first two words,
// the calls it was given, the error it fails every call with, and those it
// fails a call with by its first two words.
type gh struct {
	answers map[string][]string
	calls   [][]string
	fail    error
	failing map[string]error
}

// call records the arguments and gives the next answer for their first
// two words, the last one again once they run out.
func (g *gh) call(args []string) (string, error) {
	g.calls = append(g.calls, args)
	key := strings.Join(args[:2], " ")
	if g.fail != nil {
		return "", g.fail
	}
	if err := g.failing[key]; err != nil {
		return "", err
	}
	answers := g.answers[key]
	if len(answers) == 0 {
		return "", nil
	}
	answer := answers[0]
	if len(answers) > 1 {
		g.answers[key] = answers[1:]
	}
	return answer, nil
}

// called are the first two words of each call.
func (g *gh) called() []string {
	var words []string
	for _, c := range g.calls {
		words = append(words, strings.Join(c[:2], " "))
	}
	return words
}

const (
	headSHA  = "1111111111111111111111111111111111111111"
	olderSHA = "2222222222222222222222222222222222222222"
	prConfig = configText + "push:\n  mode: pr\n"
)

// listed is gh pr list's answer of the pull requests, each "<number>
// <state> <draft> <head>".
func listed(pulls ...string) string {
	var entries []string
	for _, p := range pulls {
		var number int
		var state, head string
		var draft bool
		if _, err := fmt.Sscan(p, &number, &state, &draft, &head); err != nil {
			panic(err)
		}
		merged := "null"
		if state == "MERGED" {
			merged = `"2026-10-24T10:00:00Z"`
		}
		entries = append(entries, fmt.Sprintf(`{"number":%d,"url":"https://github.example/o/r/pull/%d","state":%q,"isDraft":%t,"mergedAt":%s,"headRefOid":%q}`,
			number, number, state, draft, merged, head))
	}
	return "[" + strings.Join(entries, ",") + "]"
}

// pullAnswer is an answer to providers.PullStateQuery: the head, its
// checks' roll-up state ("" for none), the threads, resolved or not, and
// the comments.
func pullAnswer(head, rollup string, resolved []bool, comments ...string) string {
	checks := "null"
	if rollup != "" {
		checks = fmt.Sprintf(`{"state":%q}`, rollup)
	}
	var threads []string
	for i, r := range resolved {
		threads = append(threads, fmt.Sprintf(`{"id":"T%d","isResolved":%t,"path":"a.go","line":1,"comments":{"nodes":[]}}`, i, r))
	}
	return `{"data":{"repository":{"pullRequest":{"headRefOid":"` + head + `",` +
		`"commits":{"nodes":[{"commit":{"statusCheckRollup":` + checks + `}}]},` +
		`"reviews":{"nodes":[]},"comments":{"nodes":[` + strings.Join(comments, ",") + `]},` +
		`"reviewThreads":{"nodes":[` + strings.Join(threads, ",") + `]}}}}}`
}

// autoMerged is the answer with auto-merge enabled for the pull request.
func autoMerged(answer string) string {
	return strings.Replace(answer, `"pullRequest":{`, `"pullRequest":{"autoMergeRequest":{"enabledAt":"2026-10-31T00:00:00Z"},`, 1)
}

// verdictBy is a comment by the login recording the verdict of the item's
// stage at the head.
func verdictBy(login, item, stage, verdict, head string) string {
	body := work.Verdict{Record: "verdict", Item: item, Stage: stage, Verdict: verdict, Head: head, Round: 1}.Marker()
	text, _ := json.Marshal(map[string]any{"author": map[string]any{"login": login}, "body": "itos: review\n\n" + body})
	return string(text)
}

// line is a journal line of the role's attempt with the outcome and the
// verdict at the head ("" for null).
func line(role string, attempt int, outcome, verdict, head string) string {
	quoted := func(s string) string {
		if s == "" {
			return "null"
		}
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf(`{"schema":1,"role":%q,"attempt":%d,"request":"sha256:0","base":null,"outcome":%s,"head":%s,"notes":null,"verdict":%q,"sha256":"sha256:1"}`,
		role, attempt, quoted(outcome), quoted(head), verdict)
}

// forgeOver is the ports of a repository under push.mode pr whose slice-9
// has the status and a scenario at HEAD, whose gh answers so, and whose
// journal holds the lines.
func forgeOver(t *testing.T, status, questions string, answers map[string][]string, lines ...string) ports {
	t.Helper()
	p := over(t, fixture{
		config: prConfig,
		files: map[string]string{"work-items.yaml": registryOf(
			item("id: slice-9, title: Nine, phase: 1, owner: ana, status: " + status + ", depends_on: []"),
		), "people.yaml": "- ana\n"},
		head:      map[string]string{"features/a.feature": feature("@ID-A-01 @wip @slice-9")},
		questions: questions,
	})
	p.repository, p.gh = "o/r", &gh{answers: answers}
	if len(lines) > 0 {
		p.journal = []byte(strings.Join(lines, "\n") + "\n")
	}
	return p
}

// Each state the forge and the journal give, through Handle: the pull
// request listed with --state all, the one question for an open one and
// none for a merged one or none at all.
func TestHandleForgeStates(t *testing.T) {
	asked := func(answered ...bool) string {
		text := "questions:\n  - id: q-1\n    item: slice-8\n    question: \"Other?\"\n    answer: \"Yes.\"\n"
		for i, a := range answered {
			text += fmt.Sprintf("  - id: q-%d\n    item: slice-9\n    question: \"Which %d?\"\n", i+2, i)
			if a {
				text += "    answer: \"This.\"\n"
			}
		}
		return text
	}
	draft := listed("7 OPEN true " + olderSHA)
	ready := listed("7 OPEN false " + olderSHA)
	for _, test := range []struct {
		name, status, questions string
		list, pull              string
		lines                   []string
		state, reason           string
		argv                    []string
		calls                   []string
	}{
		{name: "no pull request, todo", status: "todo", list: "[]", state: "spec-proposed",
			argv: []string{"itos", "brief", "slice-9", "--role", "drafter"}, calls: []string{"pr list"}},
		{name: "a closed one counts for nothing", status: "todo", list: listed("3 CLOSED false " + olderSHA), state: "spec-proposed",
			argv: []string{"itos", "brief", "slice-9", "--role", "drafter"}, calls: []string{"pr list"}},
		{name: "a draft with no verdict", status: "todo", list: draft, pull: pullAnswer(headSHA, "", nil), state: "spec-review",
			argv: []string{"itos", "brief", "slice-9", "--role", "reviewer"}, calls: []string{"pr list", "api graphql"}},
		{name: "an approve at the head", status: "todo", list: draft,
			pull: pullAnswer(headSHA, "", nil, verdictBy("ANA", "slice-9", "spec", "approve", strings.ToUpper(headSHA))), state: "spec-accepted",
			argv: []string{"itos", "work", "take", "slice-9"}, calls: []string{"pr list", "api graphql"}},
		{name: "changes after the approve", status: "todo", list: draft,
			pull:  pullAnswer(headSHA, "", nil, verdictBy("ana", "slice-9", "spec", "approve", headSHA), verdictBy("ana", "slice-9", "spec", "changes", headSHA)),
			state: "spec-changes", argv: []string{"itos", "brief", "slice-9", "--role", "drafter"}, calls: []string{"pr list", "api graphql"}},
		{name: "markers that do not count", status: "todo", list: draft,
			pull: pullAnswer(headSHA, "", nil, verdictBy("ana", "slice-9", "spec", "approve", olderSHA), verdictBy("someone", "slice-9", "spec", "approve", headSHA),
				verdictBy("ana", "slice-9", "diff", "approve", headSHA), verdictBy("ana", "slice-8", "spec", "approve", headSHA), `{"author":{"login":"ana"},"body":"prose"}`),
			state: "spec-review", argv: []string{"itos", "brief", "slice-9", "--role", "reviewer"}, calls: []string{"pr list", "api graphql"}},
		{name: "ready with a failed check", status: "todo", list: ready, pull: pullAnswer(headSHA, "FAILURE", nil), state: "repair",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list", "api graphql"}},
		{name: "ready with an errored check", status: "doing", list: ready, pull: pullAnswer(headSHA, "ERROR", nil), state: "repair",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list", "api graphql"}},
		{name: "ready with an open thread", status: "doing", list: ready, pull: pullAnswer(headSHA, "SUCCESS", []bool{true, false}), state: "repair",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list", "api graphql"}},
		{name: "ready and passed", status: "doing", list: ready, pull: autoMerged(pullAnswer(headSHA, "SUCCESS", []bool{true})), state: "merging",
			calls: []string{"pr list", "api graphql"}},
		{name: "ready, expected", status: "doing", list: ready, pull: autoMerged(pullAnswer(headSHA, "EXPECTED", nil)), state: "merging",
			calls: []string{"pr list", "api graphql"}},
		{name: "ready, no checks", status: "doing", list: ready, pull: autoMerged(pullAnswer(headSHA, "", nil)), state: "merging",
			calls: []string{"pr list", "api graphql"}},
		{name: "ready, auto-merge off", status: "doing", list: ready, pull: pullAnswer(headSHA, "SUCCESS", []bool{true}), state: "handed-back",
			argv: []string{"itos", "push", "--ready"}, calls: []string{"pr list", "api graphql"}},
		{name: "ready, auto-merge off, todo", status: "todo", list: ready, pull: pullAnswer(headSHA, "PENDING", nil), state: "handed-back",
			argv: []string{"itos", "push", "--ready"}, calls: []string{"pr list", "api graphql"}},
		{name: "ready, auto-merge off, before the journal", status: "doing", list: ready, pull: pullAnswer(headSHA, "", nil),
			lines: []string{line("implementer", 1, "spec_defect", "accepted", "")}, state: "handed-back",
			argv: []string{"itos", "push", "--ready"}, calls: []string{"pr list", "api graphql"}},
		{name: "ready, auto-merge on, a check failed", status: "doing", list: ready, pull: autoMerged(pullAnswer(headSHA, "FAILURE", nil)), state: "repair",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list", "api graphql"}},
		{name: "merged, asked nothing more", status: "doing", list: listed("3 CLOSED false "+olderSHA, "7 MERGED false "+headSHA), state: "merged",
			argv: []string{"itos", "work", "done", "slice-9"}, calls: []string{"pr list"}},
		{name: "open wins over merged", status: "doing", list: listed("5 MERGED false "+olderSHA, "7 OPEN false "+olderSHA), pull: autoMerged(pullAnswer(headSHA, "PENDING", nil)),
			state: "merging", calls: []string{"pr list", "api graphql"}},
		{name: "doing, no pull request", status: "doing", list: "[]", state: "implementing",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list"}},
		{name: "doing, accepted done at the head", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil),
			lines: []string{line("implementer", 1, "done", "accepted", headSHA), line("drafter", 2, "done", "accepted", headSHA)},
			state: "handed-back", argv: []string{"itos", "push", "--ready"}, calls: []string{"pr list", "api graphql"}},
		{name: "doing, accepted done at an older head", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil),
			lines: []string{line("implementer", 1, "done", "accepted", olderSHA)}, state: "implementing",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list", "api graphql"}},
		{name: "doing, rotated done at the head", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil),
			lines: []string{line("implementer", 1, "done", "rotated", headSHA)}, state: "implementing",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list", "api graphql"}},
		{name: "doing, rotated with no outcome", status: "doing", list: "[]",
			lines: []string{line("implementer", 1, "", "rotated", "")}, state: "implementing",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list"}},
		{name: "doing, done with no head", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil),
			lines: []string{line("implementer", 1, "done", "accepted", "")}, state: "implementing",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list", "api graphql"}},
		{name: "a needs_decision with no question", status: "doing", list: "[]",
			lines: []string{line("implementer", 1, "needs_decision", "accepted", "")}, state: "blocked", reason: "attempt", calls: []string{"pr list"}},
		{name: "a blocked spent by an answered question", status: "doing", questions: asked(true), list: "[]",
			lines: []string{line("implementer", 1, "blocked", "accepted", "")}, state: "implementing",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list"}},
		{name: "the second record, one question", status: "doing", questions: asked(true), list: "[]",
			lines: []string{line("implementer", 1, "blocked", "accepted", ""), line("implementer", 2, "needs_decision", "accepted", "")},
			state: "blocked", reason: "attempt", calls: []string{"pr list"}},
		{name: "the second record, the second question answered", status: "todo", questions: asked(true, true), list: "[]",
			lines: []string{line("implementer", 1, "blocked", "accepted", ""), line("implementer", 2, "needs_decision", "accepted", "")},
			state: "spec-proposed", argv: []string{"itos", "brief", "slice-9", "--role", "drafter"}, calls: []string{"pr list"}},
		{name: "a spec defect, the item doing", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil),
			lines: []string{line("implementer", 1, "spec_defect", "accepted", "")}, state: "spec-changes",
			argv: []string{"itos", "brief", "slice-9", "--role", "drafter"}, calls: []string{"pr list", "api graphql"}},
		{name: "a spec defect spent by a drafter record", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil),
			lines: []string{line("implementer", 1, "spec_defect", "accepted", ""), line("drafter", 1, "blocked", "accepted", "")}, state: "implementing",
			argv: []string{"itos", "brief", "slice-9", "--role", "implementer"}, calls: []string{"pr list", "api graphql"}},
		{name: "a drafter record before the defect spends nothing", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil),
			lines: []string{line("drafter", 1, "done", "accepted", ""), line("implementer", 1, "spec_defect", "accepted", "")}, state: "spec-changes",
			argv: []string{"itos", "brief", "slice-9", "--role", "drafter"}, calls: []string{"pr list", "api graphql"}},
		{name: "neither todo nor doing", status: "review", list: draft, pull: pullAnswer(headSHA, "", nil), state: "in-flight",
			calls: []string{"pr list", "api graphql"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			answers := map[string][]string{"pr list": {test.list}}
			if test.pull != "" {
				answers["api graphql"] = []string{test.pull}
			}
			p := forgeOver(t, test.status, test.questions, answers, test.lines...)
			a, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9"})
			if err != nil || a.Next == nil {
				t.Fatalf("Handle = %+v, %v", a, err)
			}
			n := a.Next
			if n.State != test.state || n.BlockedReason != test.reason || !reflect.DeepEqual(n.Argv, test.argv) || n.WaitsOn != nil {
				t.Errorf("state %s, reason %q, argv %v, waits on %v; want %s, %q, %v", n.State, n.BlockedReason, n.Argv, n.WaitsOn, test.state, test.reason, test.argv)
			}
			if got := p.gh.called(); !reflect.DeepEqual(got, test.calls) {
				t.Errorf("gh was called %v, want %v", got, test.calls)
			}
		})
	}
}

// The --json of a draft in spec review: observed exact, the list asked
// with --state all and the fields, the question about the number; then a
// merged one's head is gh pr list's, and a journal's latest implementer
// record is observed.attempt, not its last line.
func TestForgeJSON(t *testing.T) {
	p := forgeOver(t, "todo", "", map[string][]string{"pr list": {listed("7 OPEN true " + olderSHA)}, "api graphql": {pullAnswer(headSHA, "", []bool{false})}})
	code, stdout, _, err := run(t, p, "--json", "slice-9")
	if err != nil || code != 0 {
		t.Fatalf("exit %d, %v", code, err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"pr": map[string]any{"number": 7.0, "draft": true, "head": headSHA, "merged": false},
		"verdict": nil, "checks": "none", "threads_open": 1.0, "attempt": nil, "unrequested": []any{}, "auto_merge": false, "review_stuck": nil}
	if !reflect.DeepEqual(got["observed"], want) || got["state"] != "spec-review" {
		t.Errorf("observed %v in %s", got["observed"], stdout)
	}
	list := strings.Join(p.gh.calls[0], " ")
	if list != "pr list --repo o/r --head slice-9 --state all --json number,url,state,isDraft,mergedAt,headRefOid" {
		t.Errorf("gh pr list was called %q", list)
	}
	if call := p.gh.calls[1]; !reflect.DeepEqual(call, []string{"api", "graphql", "-f", "query=" + providers.PullStateQuery, "-f", "owner=o", "-f", "name=r", "-F", "number=7"}) {
		t.Errorf("gh api graphql was called %q", call)
	}

	p = forgeOver(t, "doing", "", map[string][]string{"pr list": {listed("7 MERGED false " + headSHA)}},
		line("implementer", 1, "done", "rotated", ""), line("implementer", 2, "done", "accepted", headSHA), line("drafter", 1, "blocked", "accepted", ""))
	_, stdout, _, err = run(t, p, "--json", "slice-9")
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	want = map[string]any{"pr": map[string]any{"number": 7.0, "draft": false, "head": headSHA, "merged": true},
		"verdict": nil, "checks": "none", "threads_open": 0.0,
		"attempt": map[string]any{"role": "implementer", "attempt": 2.0, "verdict": "accepted", "outcome": "done", "head": headSHA}, "unrequested": []any{}, "auto_merge": false, "review_stuck": nil}
	if !reflect.DeepEqual(got["observed"], want) || got["state"] != "merged" {
		t.Errorf("observed %v in %s", got["observed"], stdout)
	}
	p = forgeOver(t, "todo", "", map[string][]string{"pr list": {listed("7 OPEN true " + olderSHA)},
		"api graphql": {pullAnswer(headSHA, "PENDING", nil, verdictBy("ana", "slice-9", "spec", "approve", headSHA))}})
	if _, verdict, _, _ := run(t, p, "--json", "slice-9"); !strings.Contains(verdict, `"verdict": "approve",
    "checks": "pending"`) {
		t.Errorf("observed.verdict is not the approve:\n%s", verdict)
	}
	if !strings.Contains(stdout, `"verdict": "accepted",
      "outcome": "done"`) {
		t.Errorf("observed.attempt's keys are not role, attempt, verdict, outcome, head in that order:\n%s", stdout)
	}
	p = forgeOver(t, "doing", "", map[string][]string{"pr list": {listed("7 OPEN false " + olderSHA)},
		"api graphql": {autoMerged(pullAnswer(headSHA, "SUCCESS", nil))}})
	if _, merging, _, _ := run(t, p, "--json", "slice-9"); !strings.Contains(merging, `"unrequested": [],
    "auto_merge": true`) {
		t.Errorf("observed.auto_merge is not true, after unrequested:\n%s", merging)
	}
}

// Outside push.mode pr the forge is not asked and nothing is observed; a
// state judged before in-flight asks it nothing either.
func TestForgeNotAsked(t *testing.T) {
	p := forgeOver(t, "doing", "", nil)
	p.cfg.Push.Mode = ""
	a, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9"})
	if err != nil || a.Next.State != "in-flight" || a.Next.Observed != nil || len(p.gh.calls) != 0 {
		t.Errorf("trunk: %+v, %v, gh called %v", a.Next, err, p.gh.calls)
	}
	p = forgeOver(t, "done", "", nil)
	a, err = NewHandler(p).Handle(t.Context(), Query{ID: "slice-9"})
	if err != nil || a.Next.State != "done" || a.Next.Observed != nil || len(p.gh.calls) != 0 {
		t.Errorf("done: %+v, %v, gh called %v", a.Next, err, p.gh.calls)
	}
	_, stdout, _, _ := run(t, p, "--json", "slice-9")
	if strings.Contains(stdout, "observed") {
		t.Errorf("observed without the forge asked:\n%s", stdout)
	}
}

// Under push.mode pr the scenarios are read at origin/<id> when the clone
// has the ref and it holds some, else at HEAD; the ref read is digested.
func TestSpecRef(t *testing.T) {
	registry := registryOf(item("id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: []"))
	files := map[string]string{"work-items.yaml": registry, "people.yaml": "- ana\n"}
	held := map[string]string{"features/b.feature": feature("@ID-B-01 @slice-9")}
	for _, test := range []struct {
		name  string
		head  map[string]string
		refs  map[string]map[string]string
		ref   string
		ids   []string
		state string
	}{
		{"at the ref", map[string]string{}, map[string]map[string]string{"refs/remotes/origin/slice-9": held}, "origin/slice-9", []string{"@ID-B-01"}, "spec-proposed"},
		{"a ref with none, HEAD's", map[string]string{"features/a.feature": feature("@ID-A-01 @slice-9")},
			map[string]map[string]string{"refs/remotes/origin/slice-9": {}}, "HEAD", []string{"@ID-A-01"}, "spec-proposed"},
		{"no ref, HEAD's", map[string]string{"features/a.feature": feature("@ID-A-01 @slice-9")}, nil, "HEAD", []string{"@ID-A-01"}, "spec-proposed"},
		{"neither", map[string]string{}, map[string]map[string]string{"refs/remotes/origin/slice-9": {}}, "HEAD", nil, "needs-spec"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := over(t, fixture{config: prConfig, files: files, head: test.head, refs: test.refs})
			p.repository, p.gh = "o/r", &gh{answers: map[string][]string{"pr list": {"[]"}}}
			facts, _, _, err := read(t.Context(), p.cfg, p.tracker, p, "slice-9")
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, s := range facts.Scenarios {
				ids = append(ids, s.ID)
			}
			if facts.SpecRef != test.ref || !reflect.DeepEqual(ids, test.ids) || Judge(facts).State != test.state {
				t.Errorf("read %v at %q, %s; want %v at %q, %s", ids, facts.SpecRef, Judge(facts).State, test.ids, test.ref, test.state)
			}
			moved := facts
			moved.SpecRef = "elsewhere"
			if Revision(moved) == Revision(facts) {
				t.Error("the ref read is not digested")
			}
		})
	}
}

// Under push.mode pr a task's checks are read at origin/<id> when the clone
// has the ref and the ledger there holds an entry for it, else in the working
// tree, with no ref read; the rows no scenario can fail without.
func TestTaskSpecRef(t *testing.T) {
	const ledgerFile = "tasks/phase-1.yaml"
	entry := func(id, doneWhen string) string {
		return "- { id: " + id + ", type: chore, title: One" + doneWhen + " }\n"
	}
	vet, none := entry("T-1", `, done_when: [{ run: "go vet" }]`), entry("T-1", ", done_when: []")
	bad := map[string]string{ledgerFile: "{ id: T-1, done_when: ["}
	for _, test := range []struct {
		name   string
		config string
		id     string
		status string
		tree   string
		held   map[string]string
		ref    string
		checks []string
		state  string
		// The read's error: its rule, or any at all.
		rule       string
		unreadable bool
		noTrees    bool
	}{
		{
			name: "trunk, a check only at the ref", config: configText, id: "T-1", status: "todo",
			tree: none, held: map[string]string{ledgerFile: vet}, state: "needs-spec",
		},
		{
			name: "no ref, the working tree's", config: prConfig, id: "T-1", status: "todo",
			tree: vet, checks: []string{"go vet"}, state: "spec-proposed",
		},
		{
			name: "no entry at the ref, the working tree's", config: prConfig, id: "T-1", status: "todo",
			tree: vet, held: map[string]string{ledgerFile: entry("T-10", ", done_when: []")},
			checks: []string{"go vet"}, state: "spec-proposed",
		},
		{
			name: "done, an unreadable ledger at the ref", config: prConfig, id: "T-1", status: "done",
			tree: vet, held: bad, checks: []string{"go vet"}, state: "done",
		},
		{
			name: "dropped, an unreadable ledger at the ref", config: prConfig, id: "T-1", status: "dropped",
			tree: vet, held: bad, checks: []string{"go vet"}, state: "parked",
		},
		{
			name: "a task's scenario at the ref counts for nothing", config: prConfig, id: "T-1", status: "todo",
			tree: none, held: map[string]string{ledgerFile: entry("T-10", ", done_when: []"), "features/b.feature": feature("@ID-B-01 @T-1")},
			state: "needs-spec",
		},
		{
			name: "a slice's ledger entry at the ref counts for nothing", config: prConfig, id: "slice-9", status: "todo",
			tree: none, held: map[string]string{ledgerFile: entry("slice-9", `, done_when: [{ run: "go vet" }]`)},
			ref: "HEAD", state: "needs-spec",
		},
		{
			name: "a check at the ref", config: prConfig, id: "T-1", status: "todo",
			tree: none, held: map[string]string{ledgerFile: vet}, ref: "origin/T-1", checks: []string{"go vet"}, state: "spec-proposed",
		},
		{
			name: "an entry at the ref with no done_when key, whatever the working tree's", config: prConfig, id: "T-1", status: "todo",
			tree: vet, held: map[string]string{ledgerFile: entry("T-1", "")}, ref: "origin/T-1", state: "needs-spec",
		},
		{
			name: "no ledger folder at the ref", config: prConfig, id: "T-1", status: "todo",
			tree: vet, held: map[string]string{}, rule: "ledger-folder-missing",
		},
		{
			name: "deferred, an unreadable ledger at the ref", config: prConfig, id: "T-1", status: "todo, deferred: later",
			tree: vet, held: bad, unreadable: true,
		},
		{
			name: "a ref that cannot be opened", config: prConfig, id: "T-1", status: "todo",
			tree: vet, held: map[string]string{ledgerFile: vet}, noTrees: true, unreadable: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry := registryOf(item("id: " + test.id + ", status: " + test.status + ", title: One, phase: 1, owner: null, depends_on: []"))
			files := map[string]string{"work-items.yaml": registry, "people.yaml": "- ana\n", ledgerFile: test.tree}
			var refs map[string]map[string]string
			if test.held != nil {
				refs = map[string]map[string]string{"refs/remotes/origin/" + test.id: test.held}
			}
			p := over(t, fixture{config: test.config, files: files, head: map[string]string{}, refs: refs})
			p.repository, p.gh = "o/r", &gh{answers: map[string][]string{"pr list": {"[]"}}}
			if test.noTrees {
				p.cfg.Trees = treefake.Trees{}
			}
			facts, _, _, err := read(t.Context(), p.cfg, p.tracker, p, test.id)
			var refused *config.Error
			switch {
			case test.rule != "":
				if !errors.As(err, &refused) || refused.Problems[0].Rule != test.rule {
					t.Fatalf("read: %v, want the rule %s", err, test.rule)
				}
				return
			case test.unreadable:
				if err == nil {
					t.Fatalf("read %+v, want its error", facts)
				}
				return
			case err != nil:
				t.Fatal(err)
			}
			var checks []string
			for _, c := range facts.Checks {
				checks = append(checks, c.Command())
			}
			if facts.SpecRef != test.ref || !reflect.DeepEqual(checks, test.checks) || Judge(facts).State != test.state {
				t.Errorf("read %v at %q, %s; want %v at %q, %s", checks, facts.SpecRef, Judge(facts).State, test.checks, test.ref, test.state)
			}
		})
	}
}

// SpecCommit is the commit of origin/<id> when the reading TestSpecRef and
// TestTaskSpecRef hold finds the item's spec there, by its kind alone.
func TestSpecCommit(t *testing.T) {
	const (
		ledgerFile = "tasks/phase-1.yaml"
		commit     = "0123456789abcdef0123456789abcdef01234567"
	)
	entry := func(id string) string { return "- { id: " + id + ", type: chore, title: One, done_when: [] }\n" }
	scenario := map[string]string{"features/b.feature": feature("@ID-B-01 @slice-9")}
	bug := map[string]string{"features/b.feature": feature("@ID-B-01 @bug-7")}
	for _, test := range []struct {
		name, k, id string
		held        map[string]string
		want        string
		failed      bool
	}{
		{name: "a slice's scenario", k: "slice", id: "slice-9", held: scenario, want: commit},
		{name: "a bug's scenario", k: "bug", id: "bug-7", held: bug, want: commit},
		{name: "another item's scenario", k: "slice", id: "slice-9", held: bug},
		{name: "no ref", k: "slice", id: "slice-9"},
		{name: "a task's entry, with no check", k: "task", id: "T-1", held: map[string]string{ledgerFile: entry("T-1")}, want: commit},
		{name: "another task's entry", k: "task", id: "T-1", held: map[string]string{ledgerFile: entry("T-10")}},
		{name: "a task with no ref", k: "task", id: "T-1"},
		{name: "a task's scenario", k: "task", id: "T-1", held: map[string]string{ledgerFile: entry("T-10"), "features/b.feature": feature("@ID-B-01 @T-1")}},
		{name: "a slice's ledger entry", k: "slice", id: "slice-9", held: map[string]string{ledgerFile: entry("slice-9")}},
		{name: "a ledger that cannot be read", k: "task", id: "T-1", held: map[string]string{ledgerFile: "{ id: T-1, done_when: ["}, failed: true},
		{name: "no ledger folder", k: "task", id: "T-1", held: map[string]string{}, failed: true},
		{name: "a kind with no spec", k: "idea", id: "slice-9", held: scenario},
	} {
		t.Run(test.name, func(t *testing.T) {
			var refs map[string]map[string]string
			if test.held != nil {
				refs = map[string]map[string]string{"refs/remotes/origin/" + test.id: test.held}
			}
			p := over(t, fixture{config: prConfig, files: map[string]string{"people.yaml": "- ana\n", ledgerFile: entry("T-1")}, head: map[string]string{}, refs: refs})
			got, err := SpecCommit(t.Context(), p.cfg, test.k, test.id)
			if (err != nil) != test.failed || (!test.failed && got != test.want) {
				t.Errorf("SpecCommit = %q, %v; want %q, an error %v", got, err, test.want, test.failed)
			}
		})
	}
}

// A forge that cannot be asked is exit 3 with its problem, under --json
// the problems JSON, nothing else printed; RevisionOf gives it as an error
// of kind Missing. A journal that cannot be read is the error.
func TestForgeFailures(t *testing.T) {
	signedOut := &providers.GhFailed{Words: "HTTP 401: Bad credentials\n", Err: errors.New("exit status 1")}
	for _, test := range []struct {
		name       string
		repository string
		fail       error
		answers    map[string][]string
		rule, says string
	}{
		{"no repository", "", nil, nil, "no-github-repository", "no GitHub repository"},
		{"gh signed out", "o/r", signedOut, nil, "next-forge-failed", "gh pr list failed: HTTP 401: Bad credentials"},
		{"no gh", "o/r", providers.ErrNoGh, nil, "next-forge-failed", "gh pr list failed: " + providers.ErrNoGh.Error()},
		{"a list that is no list", "o/r", nil, map[string][]string{"pr list": {"{}"}}, "next-forge-failed", "gh pr list failed: its answer is not a list"},
		{"an answer without the threads", "o/r", nil, map[string][]string{"pr list": {listed("7 OPEN true " + olderSHA)}, "api graphql": {`{"data":{}}`}},
			"next-forge-failed", "gh api graphql failed: " + providers.ErrNoPullState.Error()},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := forgeOver(t, "todo", "", test.answers)
			p.repository, p.gh.fail = test.repository, test.fail
			code, stdout, stderr, err := run(t, p, "slice-9")
			if err != nil || code != 3 || stdout != "" || !strings.Contains(stderr, test.rule+": ") || !strings.Contains(stderr, test.says) {
				t.Errorf("exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
			}
			code, stdout, _, err = run(t, p, "--json", "slice-9")
			if err != nil || code != 3 || !strings.Contains(stdout, `"rule": "`+test.rule+`"`) || strings.Contains(stdout, `"state"`) {
				t.Errorf("--json: exit %d, %v, %s", code, err, stdout)
			}
			revision, refused, err := RevisionOf(t.Context(), p.cfg, p.tracker, p, "slice-9")
			if revision != "" || refused != nil || kind.Of(err) != kind.Missing || !strings.Contains(err.Error(), test.says) {
				t.Errorf("RevisionOf = %q, %v, %v", revision, refused, err)
			}
		})
	}

	p := forgeOver(t, "todo", "", map[string][]string{"pr list": {listed("7 OPEN true " + olderSHA)}})
	p.gh.failing = map[string]error{"api graphql": &providers.GhFailed{Words: "HTTP 502", Err: errors.New("exit status 1")}}
	a, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9"})
	if err != nil || a.Failed == nil || !strings.Contains(a.Failed.Message, "gh api graphql failed: HTTP 502") {
		t.Errorf("a graphql call that fails: %+v, %v", a, err)
	}

	p = forgeOver(t, "todo", "", map[string][]string{"pr list": {"[]"}})
	p.journalErr = errors.New("cannot read")
	if _, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9"}); err == nil || err.Error() != "cannot read" {
		t.Errorf("a journal that cannot be read: %v", err)
	}
	p = forgeOver(t, "todo", "", map[string][]string{"pr list": {"[]"}}, "not json", "")
	if _, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9"}); kind.Of(err) != kind.Usage || !strings.Contains(err.Error(), "line 1 of slice-9's journal") {
		t.Errorf("a journal line that is no record: %v", err)
	}
	p = forgeOver(t, "todo", "", map[string][]string{"pr list": {"[]"}}, `{"attempt":1}`)
	if _, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9"}); kind.Of(err) != kind.Usage {
		t.Errorf("a journal line with no role: %v", err)
	}
}

// The names next gives the head's statusCheckRollup state.
func TestChecksOf(t *testing.T) {
	for state, want := range map[string]string{"SUCCESS": "success", "FAILURE": "failure", "ERROR": "failure",
		"PENDING": "pending", "EXPECTED": "pending", "": "none", "OTHER": "none"} {
		if got := checksOf(state); got != want {
			t.Errorf("checksOf(%q) = %q, want %q", state, got, want)
		}
	}
}

// Under push.mode pr the revision is the same for the same forge facts and
// another for a change to any: the pull request, the verdict, the checks,
// the open threads, auto-merge turning on, a journal record, a question
// naming the item. Auto-merge off writes nothing: the facts' revision is the
// one they had before slice-216.
func TestRevisionForge(t *testing.T) {
	base := func() Facts {
		f := facts(t, "{ id: slice-9, status: todo }", "slice", nil, nil, live, nil)
		f.SpecRef = "origin/slice-9"
		f.Observed = &Observed{Pull: &Pull{Number: 7, Draft: true, Head: headSHA}, Verdict: "approve", Checks: "none",
			Records: []Record{{Role: "implementer", Attempt: 1, Verdict: "accepted", Outcome: ptr("done"), Head: ptr(headSHA)}}}
		f.Asked = []ask.Question{{ID: "q-2", Item: "slice-9", Question: "Which?"}}
		return f
	}
	first := Revision(base())
	if first != "sha256:7592b165b62e596533f2d3e82ce549e2f40affabaa609e16048b5889829db63e" {
		t.Errorf("the revision with auto-merge off is %s, not the one before slice-216", first)
	}
	if first != Revision(base()) {
		t.Error("the same forge facts give another revision")
	}
	for name, change := range map[string]func(f *Facts){
		"the head":        func(f *Facts) { f.Observed.Pull.Head = olderSHA },
		"no pull request": func(f *Facts) { f.Observed.Pull = nil },
		"the verdict":     func(f *Facts) { f.Observed.Verdict = "changes" },
		"the checks":      func(f *Facts) { f.Observed.Checks = "pending" },
		"a thread":        func(f *Facts) { f.Observed.ThreadsOpen = 1 },
		"auto-merge on":   func(f *Facts) { f.Observed.AutoMerge = true },
		"a review stuck": func(f *Facts) {
			f.Observed.ReviewStuck = &reviewers.Stop{Login: "a", Review: "code", Head: headSHA, Request: "#1"}
		},
		"a record":            func(f *Facts) { f.Observed.Records[0].Verdict = "rotated" },
		"a question answered": func(f *Facts) { f.Asked[0].Answer = "This." },
		"the ref":             func(f *Facts) { f.SpecRef = "HEAD" },
	} {
		f := base()
		change(&f)
		if Revision(f) == first {
			t.Errorf("%s leaves the revision as it was", name)
		}
	}
}

// The logins of TestForgeReviewStuck's pull requests.
const (
	codexA = "chatgpt-codex-connector"
	codexB = "codex-b"
)

// stuckConfig is the config with review_stall 60, review_by the logins and any
// more of push.pr after them.
func stuckConfig(more string, logins ...string) string {
	return prConfig + "  pr:\n    draft: true\n    review_by: [" + strings.Join(logins, ", ") + "]\n    review_stall: 60\n" + more
}

// stuckOver is the ports of a pull request whose config is the text and whose
// answer is the pull request's, listed at draft or ready.
func stuckOver(t *testing.T, config, status, list, pull string, lines ...string) ports {
	t.Helper()
	p := over(t, fixture{
		config: config,
		files: map[string]string{"work-items.yaml": registryOf(
			item("id: slice-9, title: Nine, phase: 1, owner: ana, status: " + status + ", depends_on: []"),
		), "people.yaml": "- ana\n"},
		head: map[string]string{"features/a.feature": feature("@ID-A-01 @wip @slice-9")},
	})
	p.repository, p.gh = "o/r", &gh{answers: map[string][]string{"pr list": {list}, "api graphql": {pull}}}
	if len(lines) > 0 {
		p.journal = []byte(strings.Join(lines, "\n") + "\n")
	}
	return p
}

// ago is the time that many seconds ago, as GitHub writes it; "" for a negative one,
// a time GitHub does not give.
func ago(seconds int) string {
	if seconds < 0 {
		return ""
	}
	return time.Now().Add(-time.Duration(seconds) * time.Second).UTC().Format(time.RFC3339)
}

// timedComment is a comment by the login made and last edited those many seconds
// ago, at the address.
func timedComment(login, body string, created, edited int, url string) string {
	text, _ := json.Marshal(map[string]any{"author": map[string]any{"login": login}, "body": body,
		"createdAt": ago(created), "updatedAt": ago(edited), "url": url})
	return string(text)
}

// asked is a comment asking the login for the review, "code" or "security", at
// the head, posted those many seconds ago, at #<id>.
func asked(login, review, head string, seconds int, id string) string {
	req := reviewers.CodexReview(login)
	if review == "security" {
		req = reviewers.CodexSecurity(login)
	}
	bodies := req.Bodies(head)
	return timedComment("someone", bodies[len(bodies)-1], seconds, seconds, "#"+id)
}

// stateOf is the login's review-state comment, its two reviews each a status and
// the commit it is of, last edited those many seconds ago.
func stateOf(login, code, codeSHA, security, securitySHA string, edited int) string {
	review := func(status, sha string) string {
		text, _ := json.Marshal(map[string]any{"status": nullable(status), "commit_sha": nullable(sha)})
		return string(text)
	}
	return timedComment(login, `<!-- codex-review-state:v1 {"code_review":`+review(code, codeSHA)+`,"security_review":`+review(security, securitySHA)+`} -->`, edited, edited, "#state")
}

// nullable is the text, or null for none.
func nullable(text string) any {
	if text == "" {
		return nil
	}
	return text
}

// securityNotStarted is the login's state with its code review completed for the
// head and its security review null of no commit.
func securityNotStarted(login string) string {
	return stateOf(login, "completed", headSHA, "", "", 400)
}

// securityStuck are the comments of a login whose security review has not started
// after its two requests, the later past review_stall.
func securityStuck(login string) []string {
	return []string{securityNotStarted(login), asked(login, "security", headSHA, 300, "1"), asked(login, "security", headSHA, 120, "2")}
}

// The state's rows, the first of which is the stop itself: a stop under each state
// that keeps its place over it, and each answer --ready would judge otherwise. The
// answers name the review where --ready would stop, as it reads them.
func TestForgeReviewStuck(t *testing.T) {
	draft := listed("7 OPEN true " + olderSHA)
	ready := listed("7 OPEN false " + olderSHA)
	handed := line("implementer", 1, "done", "accepted", headSHA)
	one := stuckConfig("", codexA)
	two := stuckConfig("", codexA, codexB)
	stuck := securityStuck(codexA)
	code := func(request string) *reviewers.Stop {
		return &reviewers.Stop{Login: codexA, Review: "code", Head: headSHA, Request: request}
	}
	security := func(login, request string) *reviewers.Stop {
		return &reviewers.Stop{Login: login, Review: "security", Head: headSHA, Request: request}
	}
	for _, test := range []struct {
		name, config, status, list, pull string
		lines                            []string
		state                            string
		want                             *reviewers.Stop
		stalled                          bool
	}{
		{name: "checks pending", status: "doing", pull: pullAnswer(headSHA, "PENDING", nil, stuck...), state: "review-stuck", want: security(codexA, "#2")},
		{name: "no checks", status: "doing", pull: pullAnswer(headSHA, "", nil, stuck...), state: "review-stuck", want: security(codexA, "#2")},
		{name: "checks passed", status: "doing", pull: pullAnswer(headSHA, "SUCCESS", nil, stuck...), state: "review-stuck", want: security(codexA, "#2")},
		{name: "a todo item with no journal", status: "todo", pull: pullAnswer(headSHA, "", nil, stuck...), state: "review-stuck", want: security(codexA, "#2")},
		{name: "a draft", status: "doing", list: draft, pull: pullAnswer(headSHA, "", nil, stuck...), lines: []string{handed}, state: "handed-back"},
		{name: "a failed check", status: "doing", pull: pullAnswer(headSHA, "FAILURE", nil, stuck...), state: "repair"},
		{name: "an open thread", status: "doing", pull: pullAnswer(headSHA, "", []bool{false}, stuck...), state: "repair"},
		{name: "a resolved thread", status: "doing", pull: pullAnswer(headSHA, "", []bool{true}, stuck...), state: "review-stuck", want: security(codexA, "#2")},
		{name: "auto-merge on", status: "doing", pull: autoMerged(pullAnswer(headSHA, "", nil, stuck...)), state: "merging"},
		{name: "an unrequested reviewer", config: stuckConfig("    reviewers:\n      - { login: \"cursor[bot]\", kind: bugbot }\n", codexA), status: "doing",
			pull: pullAnswer(headSHA, "", nil, stuck...), state: "review-unrequested"},
		{name: "a login reviewers names and review_by does not", config: stuckConfig("    reviewers:\n      - { login: "+codexA+", kind: codex }\n", "someone-else"), status: "doing",
			pull: pullAnswer(headSHA, "", nil, stuck...), state: "handed-back"},
		{name: "no review-state comment", status: "doing",
			pull: pullAnswer(headSHA, "", nil, asked(codexA, "security", headSHA, 300, "1"), asked(codexA, "security", headSHA, 120, "2")), state: "handed-back"},
		{name: "a request with no time", status: "doing",
			pull: pullAnswer(headSHA, "", nil, securityNotStarted(codexA), asked(codexA, "security", headSHA, -1, "1"), asked(codexA, "security", headSHA, -1, "2")), state: "handed-back"},
		{name: "one request past review_stall", status: "doing",
			pull: pullAnswer(headSHA, "", nil, securityNotStarted(codexA), asked(codexA, "security", headSHA, 300, "1")), state: "handed-back"},
		{name: "the later request inside review_stall", status: "doing",
			pull: pullAnswer(headSHA, "", nil, securityNotStarted(codexA), asked(codexA, "security", headSHA, 300, "1"), asked(codexA, "security", headSHA, 5, "2")), state: "handed-back"},
		{name: "two requests listed later first", status: "doing",
			pull:  pullAnswer(headSHA, "", nil, securityNotStarted(codexA), asked(codexA, "security", headSHA, 120, "2"), asked(codexA, "security", headSHA, 300, "1")),
			state: "review-stuck", want: security(codexA, "#2")},
		{name: "two under way requests older than the last edit, past it", status: "doing",
			pull:  pullAnswer(headSHA, "", nil, stateOf(codexA, "running", headSHA, "completed", headSHA, 100), asked(codexA, "code", headSHA, 300, "1"), asked(codexA, "code", headSHA, 200, "2")),
			state: "review-stuck", want: code("#2"), stalled: true},
		{name: "one under way request older than the last edit", status: "doing",
			pull:  pullAnswer(headSHA, "", nil, stateOf(codexA, "running", headSHA, "completed", headSHA, 100), asked(codexA, "code", headSHA, 300, "1")),
			state: "handed-back"},
		{name: "the code review under way inside review_stall of a request", status: "doing",
			pull:  pullAnswer(headSHA, "", nil, stateOf(codexA, "running", headSHA, "completed", headSHA, 100), asked(codexA, "code", headSHA, 5, "1")),
			state: "handed-back"},
		{name: "the security review under way with one older code request", status: "doing",
			pull:  pullAnswer(headSHA, "", nil, stateOf(codexA, "completed", headSHA, "running", headSHA, 100), asked(codexA, "code", headSHA, 300, "1")),
			state: "review-stuck", want: security(codexA, "#1"), stalled: true},
		{name: "both reviews under way", status: "doing",
			pull:  pullAnswer(headSHA, "", nil, stateOf(codexA, "running", headSHA, "running", headSHA, 300), asked(codexA, "code", headSHA, 120, "1")),
			state: "review-stuck", want: code("#1"), stalled: true},
		{name: "the code review with one request before the security review with two", status: "doing",
			pull: pullAnswer(headSHA, "", nil, stateOf(codexA, "", "", "", "", 400),
				asked(codexA, "code", headSHA, 120, "1"), asked(codexA, "security", headSHA, 300, "2"), asked(codexA, "security", headSHA, 120, "3")),
			state: "handed-back"},
		{name: "the code review with two before the security review with one", status: "doing",
			pull: pullAnswer(headSHA, "", nil, stateOf(codexA, "", "", "", "", 400),
				asked(codexA, "code", headSHA, 300, "1"), asked(codexA, "code", headSHA, 120, "2"), asked(codexA, "security", headSHA, 120, "3")),
			state: "review-stuck", want: code("#2")},
		{name: "a first login's missing request, the other stuck", config: two, status: "doing",
			pull: pullAnswer(headSHA, "", nil, append(securityStuck(codexB), securityNotStarted(codexA))...), state: "handed-back"},
		{name: "a second login's missing request, the first stuck", config: two, status: "doing",
			pull: pullAnswer(headSHA, "", nil, append(securityStuck(codexA), securityNotStarted(codexB))...), state: "handed-back"},
		{name: "two logins stuck, in the config's order", config: two, status: "doing",
			pull: pullAnswer(headSHA, "", nil, append(securityStuck(codexB), securityStuck(codexA)...)...), state: "review-stuck", want: security(codexA, "#2")},
		{name: "a login inside its clock passed over", config: two, status: "doing",
			pull:  pullAnswer(headSHA, "", nil, append(securityStuck(codexB), securityNotStarted(codexA), asked(codexA, "security", headSHA, 300, "3"), asked(codexA, "security", headSHA, 5, "4"))...),
			state: "review-stuck", want: security(codexB, "#2")},
		{name: "a login with a verdict passed over", config: two, status: "doing",
			pull:  pullAnswer(headSHA, "", nil, append(securityStuck(codexB), stateOf(codexA, "completed", headSHA, "completed", headSHA, 400))...),
			state: "review-stuck", want: security(codexB, "#2")},
		{name: "another login refused", config: two, status: "doing",
			pull: strings.Replace(pullAnswer(headSHA, "", nil, securityStuck(codexB)...), `"reviews":{"nodes":[]}`,
				`"reviews":{"nodes":[{"author":{"login":"`+codexA+`"},"body":"Copilot was unable to review this pull request."}]}`, 1),
			state: "review-stuck", want: security(codexB, "#2")},
		{name: "a ready time past ci.watch.timeout", config: stuckConfig("", codexA) + "ci:\n  watch:\n    timeout: 10\n", status: "doing",
			pull: strings.Replace(pullAnswer(headSHA, "", nil, stuck...), `"pullRequest":{`,
				`"pullRequest":{"timelineItems":{"nodes":[{"createdAt":"`+ago(3600)+`"}]},`, 1),
			state: "review-stuck", want: security(codexA, "#2")},
	} {
		t.Run(test.name, func(t *testing.T) {
			config, list := test.config, test.list
			if config == "" {
				config = one
			}
			if list == "" {
				list = ready
			}
			a, err := NewHandler(stuckOver(t, config, test.status, list, test.pull, test.lines...)).Handle(t.Context(), Query{ID: "slice-9"})
			if err != nil || a.Next == nil {
				t.Fatalf("Handle = %+v, %v", a, err)
			}
			n := a.Next
			if test.want != nil {
				test.want.Stalled = test.stalled
			}
			if n.State != test.state || !reflect.DeepEqual(n.Observed.ReviewStuck, test.want) {
				t.Errorf("state %s, stuck %+v; want %s, %+v", n.State, n.Observed.ReviewStuck, test.state, test.want)
			}
			if test.state == "review-stuck" && (n.Actor != "person" || n.Action != "look" || n.Argv != nil) {
				t.Errorf("the step is %s %s %v, not person look with no argv", n.Actor, n.Action, n.Argv)
			}
		})
	}
}

// With no review_stall nothing is stuck, whatever the times.
func TestForgeReviewStuckNoStall(t *testing.T) {
	config := prConfig + "  pr:\n    draft: true\n    review_by: [" + codexA + "]\n"
	a, err := NewHandler(stuckOver(t, config, "doing", listed("7 OPEN false "+olderSHA), pullAnswer(headSHA, "", nil, securityStuck(codexA)...))).Handle(t.Context(), Query{ID: "slice-9"})
	if err != nil || a.Next.State != "handed-back" || a.Next.Observed.ReviewStuck != nil {
		t.Errorf("Handle = %+v, %v", a.Next, err)
	}
}

// The line says what is stuck after the step, with no command to run.
func TestForgeReviewStuckLine(t *testing.T) {
	p := stuckOver(t, stuckConfig("", codexA), "doing", listed("7 OPEN false "+olderSHA), pullAnswer(headSHA, "", nil, securityStuck(codexA)...))
	_, stdout, _, err := run(t, p, "slice-9")
	want := "slice-9: review-stuck; next: person look; security review by " + codexA + " of " + headSHA + ", last asked at #2\n"
	if err != nil || stdout != want {
		t.Errorf("%q, %v; want %q", stdout, err, want)
	}
}

// A kept state over a stop has one revision either side of the stop's clock, and
// nothing stuck on either side: the clock moves no revision in a state it does not
// change. Each has its second request five seconds old, then past review_stall.
func TestForgeReviewStuckKeepsTheRevision(t *testing.T) {
	approve := verdictBy("ana", "slice-9", "spec", "approve", headSHA)
	for _, test := range []struct {
		name, status, list, state, rollup string
		also                              []string
	}{
		{name: "spec-accepted on a draft", status: "todo", list: listed("7 OPEN true " + olderSHA), state: "spec-accepted", also: []string{approve}},
		{name: "repair on a red pull request", status: "doing", list: listed("7 OPEN false " + olderSHA), state: "repair", rollup: "FAILURE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// One comment's address at each age, so only the clock moves between them;
			// the times a kept state does not read are not in its revision.
			of := func(seconds int) (string, *Observed) {
				comments := append([]string{securityNotStarted(codexA), asked(codexA, "security", headSHA, 300, "1"), asked(codexA, "security", headSHA, seconds, "2")}, test.also...)
				a, err := NewHandler(stuckOver(t, stuckConfig("", codexA), test.status, test.list, pullAnswer(headSHA, test.rollup, nil, comments...))).Handle(t.Context(), Query{ID: "slice-9"})
				if err != nil || a.Next.State != test.state {
					t.Fatalf("Handle = %+v, %v", a.Next, err)
				}
				return a.Next.Revision, a.Next.Observed
			}
			young, seen := of(5)
			if seen.ReviewStuck != nil {
				t.Errorf("five seconds old: stuck %+v", seen.ReviewStuck)
			}
			past, seen := of(120)
			if seen.ReviewStuck != nil {
				t.Errorf("past review_stall: stuck %+v", seen.ReviewStuck)
			}
			if young != past {
				t.Errorf("the revision moved with the clock: %s, %s", young, past)
			}
		})
	}
}
