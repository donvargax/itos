package next

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/providers"
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
		"verdict": nil, "checks": "none", "threads_open": 1.0, "attempt": nil, "unrequested": []any{}, "auto_merge": false}
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
		"attempt": map[string]any{"role": "implementer", "attempt": 2.0, "verdict": "accepted", "outcome": "done", "head": headSHA}, "unrequested": []any{}, "auto_merge": false}
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
		"the head":            func(f *Facts) { f.Observed.Pull.Head = olderSHA },
		"no pull request":     func(f *Facts) { f.Observed.Pull = nil },
		"the verdict":         func(f *Facts) { f.Observed.Verdict = "changes" },
		"the checks":          func(f *Facts) { f.Observed.Checks = "pending" },
		"a thread":            func(f *Facts) { f.Observed.ThreadsOpen = 1 },
		"auto-merge on":       func(f *Facts) { f.Observed.AutoMerge = true },
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
