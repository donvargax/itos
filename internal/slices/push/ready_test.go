package push

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// pulls is a gh for push.pr.draft (slice 179): it lists the open pull
// request, a draft or not, opens pull requests at created, answers pr view
// with the heads in turn, the last for good, and api graphql with the reviews
// and review threads it was given, or with what graph makes of the number of
// the ask; a command in fail fails with its error. The head a review state
// is of, in the answers, is written "@head" and is the head --ready judges:
// the last of the heads pr view answers when it answers some, else the clone's
// HEAD, as current gives it.
type pulls struct {
	open, created string
	title         string // the open pull request's title
	draft         bool
	heads         []string
	reviews       []string // the logins of the reviews' authors, "" for an author that is gone
	threads       []string // review threads, as GraphQL's JSON nodes
	graph         func(ask int) string
	fail          map[string]error
	asks, viewed  int
	current       func() string
	view          string // the answer to a pr view that asks for the pull request's state, "{}" without
	activity      string // the answer to the branch's activity, "" for none
}

// judged is the head the answers' "@head" stands for.
func (p *pulls) judged() string {
	if len(p.heads) > 0 {
		return p.heads[len(p.heads)-1]
	}
	return p.current()
}

func (p *pulls) gh(args []string) (string, error) {
	if args[0] == "api" && strings.HasPrefix(args[1], "repos/") {
		return strings.ReplaceAll(p.activity, "@head", p.judged()), p.fail["api repos"]
	}
	command := strings.Join(args[:2], " ")
	if err := p.fail[command]; err != nil {
		return "", err
	}
	switch command {
	case "pr list":
		if p.open == "" {
			return "[]\n", nil
		}
		return fmt.Sprintf(`[{"number":7,"url":%q,"isDraft":%t,"title":%q}]`, p.open, p.draft, p.title) + "\n", nil
	case "pr create":
		return "Creating the pull request\n" + p.created + "\n", nil
	case "pr view":
		if len(args) > 0 && strings.Contains(args[len(args)-1], "state") {
			if p.view != "" {
				return strings.ReplaceAll(p.view, "@head", p.judged()) + "\n", nil
			}
			return "{}\n", nil
		}
		head := p.heads[min(p.viewed, len(p.heads)-1)]
		p.viewed++
		return fmt.Sprintf(`{"headRefOid":%q}`+"\n", head), nil
	case "api graphql":
		p.asks++
		if p.graph != nil {
			return strings.ReplaceAll(p.graph(p.asks), "@head", p.judged()), nil
		}
		var reviews []string
		for _, login := range p.reviews {
			if login == "" {
				reviews = append(reviews, `{"author":null}`)
			} else {
				reviews = append(reviews, fmt.Sprintf(`{"author":{"login":%q}}`, login))
			}
		}
		return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[%s]},"comments":{"nodes":[]},"reviewThreads":{"nodes":[%s]}}}}}`,
			strings.Join(reviews, ","), strings.Join(p.threads, ",")), nil
	}
	return "", nil
}

// reviewsBy is the answer of a pull request with reviews by the logins and
// no thread.
func reviewsBy(logins ...string) string {
	p := &pulls{reviews: logins}
	answer, _ := p.gh([]string{"api", "graphql"})
	return answer
}

// A review thread as GraphQL gives it.
func thread(resolved bool, path, line string, comments ...string) string {
	var nodes []string
	for _, body := range comments {
		nodes = append(nodes, fmt.Sprintf(`{"body":%q}`, body))
	}
	return fmt.Sprintf(`{"isResolved":%t,"path":%q,"line":%s,"comments":{"nodes":[%s]}}`, resolved, path, line, strings.Join(nodes, ","))
}

// draftRepo is prRepo under push.pr, whose settings are the text given, with
// the more config after it; its gh is the pull requests p holds, which open
// pull requests at opened.
func draftRepo(t *testing.T, pr string, more ...string) (*fake, *pulls) {
	t.Helper()
	f := prRepo(t)
	f.cfg = loadedWith(t, "version: 1\npush: { mode: pr, pr: { "+pr+" } }\n"+strings.Join(more, ""),
		map[string]string{registryFile: "items:\n  - { id: " + item + " }\n"})
	p := &pulls{created: opened, current: func() string { return f.head(t) }}
	f.gh = p.gh
	return f, p
}

// watching is ci.watch on github with the settings, and a watcher whose runs
// all end as the conclusion says, the commits it was asked about kept.
func watching(f *fake, conclusion string) *[]string {
	asked := new([]string)
	f.watcher, f.watchOK = providers.Watcher{Look: func(sha string) (providers.Run, bool, error) {
		*asked = append(*asked, sha)
		return providers.Run{URL: "https://ci/1", Status: "completed", Conclusion: conclusion,
			Jobs: []providers.Job{{Name: "ci", Status: "completed", Conclusion: conclusion}}}, true, nil
	}}, true
	return asked
}

const github = "ci: { watch: { provider: github, interval: 0, timeout: 30 } }\n"

// never fails the test when a run is looked at.
func never(t *testing.T, f *fake) {
	t.Helper()
	f.watcher, f.watchOK = providers.Watcher{Look: func(string) (providers.Run, bool, error) {
		t.Error("a run was looked at")
		return providers.Run{}, false, nil
	}}, true
}

// pushed is the line a push of HEAD to the item's branch says, with the pull
// request's address.
func pushedLine(f *fake, t *testing.T, address string) string {
	t.Helper()
	head := f.head(t)
	return "Pushed " + head[:len(head)-1] + " to origin/slice-9. Pull request: " + address + "\n"
}

const (
	list     = "pr list --repo owner/name --head slice-9 --state open --json number,url,isDraft,title"
	activity = "api repos/owner/name/activity?ref=refs/heads/slice-9&per_page=1"
	readBack = "pr view slice-9 --repo owner/name --json state,mergedAt,autoMergeRequest"
	ready    = "pr ready slice-9 --repo owner/name"
	sayDraft = "The pull request is a draft: nothing judges or merges it until itos push --ready marks it ready.\n"
	sayReady = "The pull request is ready: itos push --ready waits for its checks and reviews, then turns on auto-merge.\n"
)

// merged are the calls --ready ends with when auto-merge goes on: the merge,
// pinned to the head judged, and the read of the pull request that follows.
func merged(head string) []string {
	return []string{"pr merge slice-9 --repo owner/name --auto --rebase --match-head-commit " + head, readBack}
}

// graphql is the gh api graphql call for the pull request, the query one
// line and the repository its owner and name.
func graphql(number int) string {
	return fmt.Sprintf("api graphql -f query=%s -f owner=owner -f name=name -F number=%d", reviewQuery, number)
}

// Under push.pr.draft a plain push opens the pull request as a draft and
// does nothing more: no auto-merge, no rebase, no look at a run, ci.watch or
// not. A pull request already open, a draft or ready, is left as it is.
func TestPushUnderDraftOpensADraftAndNothingMore(t *testing.T) {
	f, _ := draftRepo(t, "draft: true", github)
	never(t, f)
	code, stdout, stderr, _ := call(t, f)
	if code != 0 || stderr != "" || stdout != pushedLine(f, t, opened)+sayDraft {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if got, want := f.calls(), []string{list, "pr create --repo owner/name --base main --head slice-9 --title=slice-9 --body=slice-9 --draft"}; !reflect.DeepEqual(got, want) {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}

	f, _ = draftRepo(t, "draft: true", github)
	never(t, f)
	code, stdout, stderr, _ = call(t, f, "--json")
	r := reported(t, stdout)
	if code != 0 || r.Outcome != "pushed" || r.PR == nil || !r.PR.Draft || r.PR.AutoMerge || !r.PR.Created || r.PR.Updated || r.CI != "" || stderr != "" {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
	f, _ = draftRepo(t, "draft: true")
	if code, stdout, _, _ := call(t, f, "-q"); code != 0 || stdout != "" {
		t.Errorf("-q: exit %d, %q", code, stdout)
	}

	// An open pull request, which main has moved past: no rebase is asked.
	for name, c := range map[string]struct {
		draft bool
		note  string
	}{"a draft": {true, sayDraft}, "a ready pull request": {false, sayReady}} {
		t.Run(name, func(t *testing.T) {
			f, p := draftRepo(t, "draft: true", github)
			never(t, f)
			p.open, p.draft = open7, c.draft
			f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
			f.on(item, "nw000")
			code, stdout, stderr, _ := call(t, f, "--json")
			r := reported(t, stdout)
			if code != 0 || r.PR == nil || r.PR.URL != open7 || r.PR.Draft != c.draft || r.PR.AutoMerge || r.PR.Created || r.PR.Updated || stderr != "" {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
			if got := f.calls(); !reflect.DeepEqual(got, []string{list}) {
				t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
			}
			f.Remote["origin"].Refs["refs/heads/slice-9"] = "nw000"
			if code, stdout, _, _ := call(t, f); code != 0 || stdout != "Nothing to push: slice-9 is at origin/slice-9. Pull request: "+open7+"\n"+c.note {
				t.Errorf("nothing to push: exit %d\n%s", code, stdout)
			}
		})
	}
}

// A pull request is a draft in --json when GitHub lists it so, whatever the
// config says, and the push that turns on auto-merge says so.
func TestPushWithoutDraftTurnsOnAutoMergeAndSaysSo(t *testing.T) {
	f := prRepo(t)
	code, stdout, _, _ := call(t, f, "--json")
	if r := reported(t, stdout); code != 0 || r.PR == nil || r.PR.Draft || !r.PR.AutoMerge {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	f = prRepo(t)
	f.gh = func(args []string) (string, error) {
		if args[1] == "list" {
			return `[{"number":7,"url":"` + open7 + `","isDraft":true}]`, nil
		}
		return "", nil
	}
	if _, stdout, _, _ := call(t, f, "--json"); !reported(t, stdout).PR.Draft {
		t.Errorf("a draft listed: %s", stdout)
	}
}

// --ready marks the draft ready, asks for the run of the head pushed and for
// the review of each push.pr.review_by login, which GraphQL names without
// "[bot]", looks at the review threads, and only then turns on auto-merge by
// rebase; each gh call names the repository, the query is one line and the
// number is the pull request's.
func TestPushReadyMarksTheDraftReadyThenTurnsOnAutoMerge(t *testing.T) {
	const login = "copilot-pull-request-reviewer"
	f, p := draftRepo(t, `draft: true, review_by: ["copilot-pull-request-reviewer[bot]"]`, github)
	asked := watching(f, "success")
	p.open, p.draft, p.reviews = open7, true, []string{login}
	code, stdout, stderr, _ := call(t, f, "--ready")
	address := open7
	waiting := "Waiting on " + address + " at " + short(f.head(t)) + ": "
	want := pushedLine(f, t, address) + waiting + "run waiting, copilot-pull-request-reviewer[bot] not asked.\n" +
		"CI run: https://ci/1\n  ci: success\nCI passed: https://ci/1\n" +
		"Waiting for reviews by copilot-pull-request-reviewer[bot] on " + address + ".\n" +
		waiting + "run succeeded, copilot-pull-request-reviewer[bot] completed.\n" +
		"Auto-merge is on: GitHub merges " + address + " by rebase once its required checks pass.\n"
	if code != 0 || stderr != "" || stdout != want {
		t.Fatalf("exit %d\n%s%s\nwant\n%s", code, stdout, stderr, want)
	}
	if got, want := f.calls(), append([]string{list, ready, graphql(7), activity}, merged(f.head(t))...); !reflect.DeepEqual(got, want) {
		t.Errorf("gh was called\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if strings.ContainsAny(reviewQuery, "\n\t") || strings.Contains(reviewQuery, "  ") || !strings.HasPrefix(reviewQuery, "query($owner: String!, ") {
		t.Errorf("the query is not on one line: %q", reviewQuery)
	}
	if !reflect.DeepEqual(*asked, []string{f.head(t)}) {
		t.Errorf("the runs asked for: %v", *asked)
	}

	f, p = draftRepo(t, `draft: true, review_by: ["copilot-pull-request-reviewer[bot]"]`, github)
	watching(f, "success")
	p.open, p.draft, p.reviews = open7, true, []string{login}
	code, stdout, stderr, _ = call(t, f, "--json", "--ready")
	r := reported(t, stdout)
	if code != 0 || !r.OK || r.Outcome != "pushed" || r.CI != "success" || r.PR == nil || r.PR.Draft || !r.PR.AutoMerge || r.PR.Created || r.PR.Updated ||
		!strings.Contains(stderr, "Waiting for reviews by copilot-pull-request-reviewer[bot] on "+address+".\n") {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
	f, p = draftRepo(t, `draft: true, review_by: ["copilot-pull-request-reviewer[bot]"]`, github)
	watching(f, "success")
	p.open, p.draft, p.reviews = open7, true, []string{login}
	if code, stdout, stderr, _ := call(t, f, "-q", "--ready"); code != 0 || stdout != "" || stderr != "" {
		t.Errorf("-q: exit %d\n%s%s", code, stdout, stderr)
	}
}

// With no pull request --ready opens it ready, so there is nothing to mark;
// one that is ready already is not marked either. With no ci.watch provider no
// run is waited for, and with no review_by the threads are asked once.
func TestPushReadyOpensOrKeepsTheReadyPullRequest(t *testing.T) {
	f, p := draftRepo(t, "draft: true")
	code, stdout, _, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != 0 || r.PR == nil || !r.PR.Created || r.PR.Draft || !r.PR.AutoMerge || r.CI != "" {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	create := "pr create --repo owner/name --base main --head slice-9 --title=slice-9 --body=slice-9"
	if got, want := f.calls(), append([]string{list, create, graphql(1)}, merged(f.head(t))...); !reflect.DeepEqual(got, want) || p.asks != 1 {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}
	f, p = draftRepo(t, "draft: true")
	p.open = open7
	code, stdout, _, _ = call(t, f, "--ready")
	if want := pushedLine(f, t, open7) + "Auto-merge is on: GitHub merges " + open7 + " by rebase once its required checks pass.\n"; code != 0 || stdout != want {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	if got, want := f.calls(), append([]string{list, graphql(7)}, merged(f.head(t))...); !reflect.DeepEqual(got, want) {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}
}

// With nothing to push --ready still marks the pull request ready and
// finishes it, the outcome nothing-to-push, the run watched that of HEAD.
func TestPushReadyWithNothingToPush(t *testing.T) {
	f, p := draftRepo(t, "draft: true", github)
	asked := watching(f, "success")
	p.open, p.draft = open7, true
	if code, _, _, _ := call(t, f); code != 0 {
		t.Fatal("the first push failed")
	}
	f.ghCalls, *asked = nil, nil
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	r := reported(t, stdout)
	if code != 0 || r.Outcome != "nothing-to-push" || r.PR == nil || !r.PR.AutoMerge || r.PR.Draft || r.CI != "success" {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if got, want := f.calls(), append([]string{list, ready, graphql(7)}, merged(f.head(t))...); !reflect.DeepEqual(got, want) || !reflect.DeepEqual(*asked, []string{f.head(t)}) {
		t.Errorf("gh was called\n%s\nruns asked for %v", strings.Join(got, "\n"), *asked)
	}
	f.ghCalls = nil
	code, stdout, _, _ = call(t, f, "--ready")
	waiting := "Waiting on " + open7 + " at " + short(f.head(t)) + ": "
	want := "Nothing to push: slice-9 is at origin/slice-9. Pull request: " + open7 + "\n" + waiting + "run waiting.\n" +
		"CI run: https://ci/1\n  ci: success\nCI passed: https://ci/1\n" + waiting + "run succeeded.\n" +
		"Auto-merge is on: GitHub merges " + open7 + " by rebase once its required checks pass.\n"
	if code != 0 || stdout != want {
		t.Errorf("exit %d\n%s", code, stdout)
	}
}

// --no-wait ends --ready once the pull request is ready and, when main moved,
// asked to be rebased: no run, no review, no api graphql and no auto-merge,
// which unwatched would let GitHub merge before the review.
func TestPushReadyNoWait(t *testing.T) {
	f, p := draftRepo(t, `draft: true, review_by: [a]`, github)
	never(t, f)
	p.open, p.draft = open7, true
	f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
	f.on(item, "nw000")
	code, stdout, stderr, _ := call(t, f, "--ready", "--no-wait")
	want := pushedLine(f, t, open7) + open7 + " is ready for review; itos push --ready again waits for its checks and reviews, then turns on auto-merge.\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if got, want := f.calls(), []string{list, ready, "pr update-branch slice-9 --repo owner/name --rebase"}; !reflect.DeepEqual(got, want) {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}
	f, p = draftRepo(t, `draft: true`, github)
	never(t, f)
	p.open, p.draft = open7, true
	code, stdout, _, _ = call(t, f, "--json", "--ready", "--no-wait")
	if r := reported(t, stdout); code != 0 || r.PR == nil || r.PR.Draft || r.PR.AutoMerge || r.Outcome != "pushed" {
		t.Errorf("--json: exit %d\n%s", code, stdout)
	}
	f.Remote["origin"].Refs["refs/heads/slice-9"] = f.head(t)
	code, stdout, _, _ = call(t, f, "--json", "--ready", "--no-wait")
	if r := reported(t, stdout); code != 0 || r.Outcome != "nothing-to-push" || r.PR.AutoMerge {
		t.Errorf("nothing to push: exit %d\n%s", code, stdout)
	}
	code, stdout, _, _ = call(t, f, "--ready", "--no-wait")
	if code != 0 || !strings.HasPrefix(stdout, "Nothing to push: slice-9 is at origin/slice-9. Pull request: "+open7+"\n"+open7+" is ready for review;") {
		t.Errorf("nothing to push: exit %d\n%s", code, stdout)
	}
}

// A run that does not succeed leaves auto-merge off, whatever it was: exit 1
// for one that failed, 3 for a watch that could not be made, with the watch's
// outcome, and what to do next.
func TestPushReadyWhereTheRunDoesNotSucceed(t *testing.T) {
	const left = "itos: auto-merge is not turned on: the run did not succeed. Fix what failed and push, or wait for the run, then itos push --ready again.\n"
	f, p := draftRepo(t, `draft: true, review_by: [a]`, github)
	watching(f, "failure")
	p.open, p.draft = open7, true
	code, stdout, stderr, _ := call(t, f, "--ready")
	if code != policy || !strings.Contains(stderr, "CI failure: https://ci/1") || !strings.HasSuffix(stderr, left) || !strings.HasPrefix(stdout, "Pushed ") {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	if got, want := f.calls(), []string{list, ready}; !reflect.DeepEqual(got, want) {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}
	code, stdout, stderr, _ = call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != policy || r.OK || r.Outcome != "nothing-to-push" || r.CI != "failure" || r.PR.AutoMerge || !strings.HasSuffix(stderr, left) {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
	f, p = draftRepo(t, `draft: true, review_by: [a]`, github)
	watching(f, "failure")
	p.open, p.draft = open7, true
	if _, stdout, _, _ := call(t, f, "--json", "--ready"); reported(t, stdout).Outcome != "pushed" {
		t.Errorf("pushed: %s", stdout)
	}

	f, p = draftRepo(t, `draft: true`, github)
	f.watchErr, f.watchOK = errors.New("no token"), false
	p.open, p.draft = open7, true
	code, _, stderr, _ = call(t, f, "--ready")
	if code != missing || !strings.Contains(stderr, "itos: no token; the commits are pushed, and itos ci watch ") || !strings.HasSuffix(stderr, left) {
		t.Errorf("a watcher that cannot be made: exit %d\n%s", code, stderr)
	}
}

// When main moved, --ready asks GitHub to rebase the pull request once it is
// ready and watches the run of the head GitHub makes, as a plain push does;
// none within ci.watch.timeout, or a gh pr view that fails, leaves auto-merge
// off.
func TestPushReadyWhereGitHubRebases(t *testing.T) {
	behind := func(t *testing.T, heads ...string) (*fake, *pulls, *[]string) {
		f, p := draftRepo(t, "draft: true", "ci: { watch: { provider: github, interval: 2, timeout: 5 } }\n")
		asked := watching(f, "success")
		f.History = append(f.History, vcsfake.Commit{Name: "rb000", Parents: []string{up}, Message: "test: more\n", Tree: maps.Clone(f.History[1].Tree)})
		p.open, p.draft, p.heads = open7, true, heads
		f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
		f.on(item, "nw000")
		return f, p, asked
	}
	view := "pr view slice-9 --repo owner/name --json headRefOid"
	update := "pr update-branch slice-9 --repo owner/name --rebase"
	f, _, asked := behind(t, "rb000")
	clocked(t)
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	r := reported(t, stdout)
	if code != 0 || r.PR == nil || !r.PR.Updated || !r.PR.AutoMerge || r.CI != "success" || !strings.Contains(stderr, "GitHub rebased the pull request onto origin/main") ||
		!reflect.DeepEqual(*asked, []string{"rb000"}) {
		t.Fatalf("exit %d\n%s%s\nruns asked for %v", code, stdout, stderr, *asked)
	}
	if got, want := f.calls(), append([]string{list, ready, update, view, graphql(7)}, merged("rb000")...); !reflect.DeepEqual(got, want) {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}

	f, _, asked = behind(t, mine)
	clocked(t)
	code, _, stderr, _ = call(t, f, "--ready")
	if code != temporary || len(*asked) != 0 || !strings.Contains(stderr, "GitHub has not rebased the pull request onto origin/main after 5 seconds") ||
		!strings.Contains(stderr, "itos: auto-merge is not turned on: the run did not succeed.") || strings.Contains(strings.Join(f.calls(), "\n"), "api graphql") {
		t.Errorf("no rebase: exit %d\n%s", code, stderr)
	}

	f, p, _ := behind(t, "rb000")
	p.fail = map[string]error{"pr view": &providers.GhFailed{Words: "HTTP 502\n", Err: errors.New("exit status 1")}}
	code, _, stderr, _ = call(t, f, "--ready")
	if code != missing || !strings.Contains(stderr, "gh pr view failed: HTTP 502") || strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
		t.Errorf("gh pr view fails: exit %d\n%s", code, stderr)
	}
}

// The reviews are asked for at once and each ci.watch.interval, never faster
// than reviewPoll, until each push.pr.review_by login has one. A login matches
// an author when the two are equal once a trailing "[bot]" is dropped from each.
func TestPushReadyWaitsForTheReviews(t *testing.T) {
	waiting := func(t *testing.T, logins, watch string) (*fake, *pulls) {
		f, p := draftRepo(t, "draft: true, review_by: "+logins, "ci: { watch: { "+watch+" } }\n")
		p.open, p.draft = open7, true
		return f, p
	}
	f, p := waiting(t, "[a]", "interval: 2, timeout: 30")
	p.graph = func(ask int) string {
		if ask < 3 {
			return reviewsBy("b")
		}
		return reviewsBy("b", "a")
	}
	c := clocked(t)
	code, stdout, _, _ := call(t, f, "--ready")
	if code != 0 || p.asks != 3 || !reflect.DeepEqual(c.slept, []time.Duration{2 * time.Second, 2 * time.Second}) ||
		strings.Count(stdout, "Waiting for reviews by a on "+open7+".\n") != 1 {
		t.Errorf("exit %d, asks %d, slept %v\n%s", code, p.asks, c.slept, stdout)
	}

	// The same login in the config and in GitHub's answer, with and without "[bot]".
	for _, c := range []struct {
		config, author string
		ok             bool
	}{{"a", "a", true}, {"a[bot]", "a", true}, {"a", "a[bot]", true}, {"a[bot]", "a[bot]", true}, {"a", "ab", false}, {"a", "b[bot]", false},
		{"a", "A", false}, {"[bot]", "a", false}} {
		f, p := waiting(t, fmt.Sprintf("[%q]", c.config), "interval: 2, timeout: 1")
		p.reviews = []string{c.author}
		clocked(t)
		if code, _, _, _ := call(t, f, "--ready"); (code == 0) != c.ok {
			t.Errorf("a review by %q for %q: exit %d", c.author, c.config, code)
		}
	}

	// Time runs out: 75, each login without a review named as the config
	// writes it, auto-merge off. The asks are at 0, 2 and 4 seconds, then at 5.
	f, p = waiting(t, `[a, "b[bot]", c]`, "interval: 2, timeout: 5")
	p.reviews = []string{"b"}
	c = clocked(t)
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	r := reported(t, stdout)
	if code != temporary || r.OK || r.Outcome != "review-timeout" || r.PR == nil || r.PR.AutoMerge || p.asks != 4 ||
		!reflect.DeepEqual(c.slept, []time.Duration{2 * time.Second, 2 * time.Second, time.Second}) ||
		!strings.Contains(stderr, "itos: no review by a, c on "+open7+" after 5 seconds (ci.watch.timeout), so auto-merge is not turned on; itos push --ready again waits again.\n") ||
		strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
		t.Errorf("exit %d, asks %d, slept %v\n%s%s", code, p.asks, c.slept, stdout, stderr)
	}
	code, _, stderr, _ = call(t, f, "--ready")
	if code != temporary || !strings.Contains(stderr, "itos: no review by a, c on ") {
		t.Errorf("exit %d\n%s", code, stderr)
	}

	// An interval of 0 is no reason to spin; a longer one is kept.
	f, _ = waiting(t, "[a]", "interval: 0, timeout: 1")
	c = clocked(t)
	if code, _, _, _ := call(t, f, "--ready"); code != temporary || len(c.slept) != 20 || c.slept[0] != reviewPoll || c.slept[19] != reviewPoll {
		t.Errorf("exit %d, slept %d times: %v", code, len(c.slept), c.slept)
	}
	f, _ = waiting(t, "[a]", "interval: 0.2, timeout: 1")
	c = clocked(t)
	if code, _, _, _ := call(t, f, "--ready"); code != temporary || c.slept[0] != 200*time.Millisecond || len(c.slept) != 5 {
		t.Errorf("exit %d, slept %v", code, c.slept)
	}

	// A review whose author is gone is no review by anyone.
	f, p = waiting(t, "[a]", "interval: 2, timeout: 30")
	p.graph = func(int) string {
		return `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[{"author":null},{"author":{"login":"a"}}]},"comments":{"nodes":[]},"reviewThreads":{"nodes":[]}}}}}`
	}
	if code, _, _, _ := call(t, f, "--ready"); code != 0 {
		t.Errorf("a review whose author is gone: exit %d", code)
	}
}

// The progress line of the wait for reviews goes where the watch's goes:
// stdout, stderr under --json, nowhere under -q, and is printed once.
func TestPushReadySaysItWaitsForReviewsWhereTheWatchSpeaks(t *testing.T) {
	for _, c := range []struct {
		args           []string
		stdout, stderr bool
	}{{nil, true, false}, {[]string{"--json"}, false, true}, {[]string{"-q"}, false, false}} {
		f, p := draftRepo(t, "draft: true, review_by: [a, b]")
		p.open, p.draft = open7, true
		p.graph = func(ask int) string {
			if ask < 3 {
				return reviewsBy()
			}
			return reviewsBy("a", "b")
		}
		clocked(t)
		code, stdout, stderr, _ := call(t, f, append(slices.Clone(c.args), "--ready")...)
		line := "Waiting for reviews by a, b on " + open7 + ".\n"
		if code != 0 || strings.Count(stdout, line) != btoi(c.stdout) || strings.Count(stderr, line) != btoi(c.stderr) {
			t.Errorf("%v: exit %d\n%s%s", c.args, code, stdout, stderr)
		}
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// A thread whose isResolved is false refuses the merge, exit 1, and is named
// by where it is and the first line of its first comment, in GitHub's order;
// one with no line is an outdated one; a resolved thread is not named, nor
// is a second line.
func TestPushReadyRefusesUnresolvedReviewThreads(t *testing.T) {
	f, p := draftRepo(t, "draft: true", github)
	watching(f, "success")
	p.open, p.draft = open7, true
	p.threads = []string{
		thread(false, "widget.md", "3", "Rename this.", "It clashes."),
		thread(true, "gadget.md", "1", "Fine now."),
		thread(false, "old.md", "null", "  Outdated one.  "),
		thread(false, "bare.md", "2"),
	}
	code, stdout, stderr, _ := call(t, f, "--ready")
	want := "itos push: auto-merge is not turned on: " + open7 + " has unresolved review threads:\n" +
		"  widget.md:3 Rename this.\n  old.md Outdated one.\n  bare.md:2\n" +
		"Address each (fix it and push, or reply on the pull request), resolve it, then itos push --ready again.\n" +
		"itos push threads lists them with their ids, and itos push thread <id> --reply <text> --resolve answers one.\n"
	if code != policy || stderr != want || !strings.HasPrefix(stdout, "Pushed ") || strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	code, stdout, stderr, _ = call(t, f, "--json", "--ready")
	r := reported(t, stdout)
	if code != policy || r.OK || r.Outcome != "review-threads-unresolved" || r.PR == nil || r.PR.AutoMerge || r.CI != "success" || !strings.HasSuffix(stderr, want) {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
}

// What GitHub answers that is no pull request's reviews and review threads,
// and a gh api graphql that fails, is a gh call that fails: exit 3, naming it.
func TestPushReadyWhereGhFails(t *testing.T) {
	empty := reviewsBy("a")
	for name, c := range map[string]struct {
		set  func(*pulls)
		says string
	}{
		"api graphql": {func(p *pulls) {
			p.fail = map[string]error{"api graphql": &providers.GhFailed{Words: "HTTP 502\n", Err: errors.New("exit status 1")}}
		}, "gh api graphql failed: HTTP 502\n"},
		"not json": {func(p *pulls) { p.graph = func(int) string { return "no" } }, "gh api graphql failed: its answer is not a pull request's reviews and review threads\n"},
		"no pull request": {func(p *pulls) {
			p.graph = func(int) string { return `{"data":{"repository":{"pullRequest":null}}}` }
		}, "gh api graphql failed: its answer is not"},
		"no repository": {func(p *pulls) { p.graph = func(int) string { return `{"data":{"repository":null}}` } }, "gh api graphql failed: its answer is not"},
		"no reviews": {func(p *pulls) {
			p.graph = func(int) string {
				return `{"data":{"repository":{"pullRequest":{"comments":{"nodes":[]},"reviewThreads":{"nodes":[]}}}}}`
			}
		}, "gh api graphql failed: its answer is not"},
		"no threads": {func(p *pulls) {
			p.graph = func(int) string {
				return `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[]},"comments":{"nodes":[]}}}}}`
			}
		}, "gh api graphql failed: its answer is not"},
		"merge": {func(p *pulls) {
			p.graph = func(int) string { return empty }
			p.fail = map[string]error{"pr merge": &providers.GhFailed{Words: "HTTP 405\n", Err: errors.New("exit status 1")}}
		}, "gh pr merge failed: HTTP 405\n"},
		"ready": {func(p *pulls) {
			p.fail = map[string]error{"pr ready": &providers.GhFailed{Words: "HTTP 403\n", Err: errors.New("exit status 1")}}
		}, "gh pr ready failed: HTTP 403\n"},
	} {
		for _, review := range []string{"", "review_by: [a]"} {
			t.Run(name+review, func(t *testing.T) {
				f, p := draftRepo(t, "draft: true, "+review)
				p.open, p.draft, p.reviews = open7, true, []string{"a"}
				c.set(p)
				code, stdout, stderr, _ := call(t, f, "--ready")
				if code != missing || !strings.Contains(stderr, c.says) || len(f.pushed) != 1 ||
					!strings.Contains(stderr, "Fix that, then itos push --ready again: it opens the pull request ready or marks it ready, waits for its checks and reviews, then turns on auto-merge.\n") {
					t.Errorf("exit %d\n%s%s", code, stdout, stderr)
				}
				f, p = draftRepo(t, "draft: true, "+review)
				p.open, p.draft, p.reviews = open7, true, []string{"a"}
				c.set(p)
				_, stdout, _, _ = call(t, f, "--json", "--ready")
				if r := reported(t, stdout); r.OK || r.Outcome != "pull-request-failed" || r.PR == nil || r.PR.AutoMerge {
					t.Errorf("--json: %s", stdout)
				}
			})
		}
	}
	// The number is the end of the address gh pr create printed, and the
	// repository's owner and name are gh api graphql's, which takes no --repo.
	f, p := draftRepo(t, "draft: true")
	p.created = "https://github.example/o/r/pull/12"
	if code, _, _, _ := call(t, f, "--ready"); code != 0 || f.calls()[2] != graphql(12) {
		t.Errorf("exit %d\n%s", code, strings.Join(f.calls(), "\n"))
	}
}

// A plain push under push.pr.draft that gh fails says how to finish: another
// itos push, which opens the pull request as a draft.
func TestPushUnderDraftWhereGhFails(t *testing.T) {
	f, p := draftRepo(t, "draft: true")
	p.fail = map[string]error{"pr create": &providers.GhFailed{Words: "HTTP 403\n", Err: errors.New("exit status 1")}}
	code, _, stderr, _ := call(t, f)
	if code != missing || !strings.Contains(stderr, "gh pr create failed: HTTP 403\n") ||
		!strings.Contains(stderr, "Fix that, then itos push again: it opens the pull request as a draft (gh pr create --draft).\n") {
		t.Errorf("exit %d\n%s", code, stderr)
	}
}

// --ready is for push.mode pr with push.pr.draft true: anywhere else it is
// refused once the config is read, before anything is fetched or pushed, and
// from main once main is known.
func TestPushReadyIsRefusedWhereItHasNothingToDo(t *testing.T) {
	const says = "itos push: --ready is for push.mode pr with push.pr.draft true, which this config does not set; nothing was fetched or pushed.\n" +
		"Push without --ready, or set push.mode: pr and push.pr.draft: true.\n"
	for name, set := range map[string]func(*fake){
		"pr without draft": func(f *fake) {},
		"draft false":      func(f *fake) { f.cfg = loadedWith(t, "version: 1\npush: { mode: pr, pr: { draft: false } }\n", nil) },
		"trunk":            func(f *fake) { f.cfg = loadedWith(t, "version: 1\npush: { mode: trunk, pr: { draft: true } }\n", nil) },
		"no config":        func(f *fake) { f.found = false },
		"unreadable":       func(f *fake) { f.loadErr = errors.New("bad") },
	} {
		t.Run(name, func(t *testing.T) {
			f := prRepo(t)
			set(f)
			code, stdout, stderr, _ := call(t, f, "--ready")
			if code != policy || stdout != "" || stderr != says || len(f.pushed) != 0 || len(f.ghCalls) != 0 || len(f.quiet) != 0 {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
			f = prRepo(t)
			set(f)
			_, stdout, _, _ = call(t, f, "--json", "--ready")
			if r := reported(t, stdout); r.OK || r.Outcome != "ready-without-draft" {
				t.Errorf("--json: %s", stdout)
			}
		})
	}
	f, _ := draftRepo(t, "draft: true")
	f.on("main", mine)
	code, stdout, stderr, _ := call(t, f, "--ready")
	if code != policy || stdout != "" || len(f.pushed) != 0 || len(f.ghCalls) != 0 ||
		stderr != "itos push: --ready marks an item's pull request ready, and HEAD is on main, which has none; nothing was fetched or pushed.\n"+
			"Run itos push --ready from the item's branch.\n" {
		t.Errorf("from main: exit %d\n%s%s", code, stdout, stderr)
	}
	_, stdout, _, _ = call(t, f, "--json", "--ready")
	if r := reported(t, stdout); r.OK || r.Outcome != "ready-on-main" {
		t.Errorf("from main, --json: %s", stdout)
	}
	// A branch naming no item is not one to mark ready either.
	f, _ = draftRepo(t, "draft: true")
	f.on("other", mine)
	if code, _, _, _ := call(t, f, "--json", "--ready"); code != policy || len(f.ghCalls) != 0 {
		t.Errorf("a branch naming no item: exit %d", code)
	}
}

// --ready is a flag of its own, and takes no --tag or --ref: it pushes an
// item's branch.
func TestParseReady(t *testing.T) {
	c, err := Parse([]string{"--ready", "--no-wait"}, false, false, "")
	if err != nil || !reflect.DeepEqual(c, CLI{Query: Query{Ready: true, NoWait: true}}) {
		t.Errorf("Parse = %+v, %v", c, err)
	}
	for _, args := range []string{"--ready --tag v1", "--tag v1 --ready", "--ref=b --ready"} {
		_, err := Parse(strings.Fields(args), false, false, "")
		var u out.UsageError
		if !errors.As(err, &u) || err.Error() != "push --ready pushes an item's branch; it takes no --tag or --ref" {
			t.Errorf("Parse(%s) = %v", args, err)
		}
	}
}

// codexState is the comment Codex edits in place, with a state line holding the
// two statuses, both reviews of the head (@head); codexStateAt, of the commits.
func codexState(code, security string) string { return codexStateAt(code, "@head", security, "@head") }

// codexStateAt is a review-state comment whose code review has the status and
// the commit given first and its security review the second pair.
func codexStateAt(code, codeSHA, security, securitySHA string) string {
	return "<!-- codex-pull-request-review-summary -->\n<!-- codex-review-state:v1 " +
		fmt.Sprintf(`{"code_review":{"status":%q,"commit_sha":%q},"security_review":{"status":%q,"commit_sha":%q}}`, code, codeSHA, security, securitySHA) +
		" -->\n## Codex Review Summary"
}

const (
	quota = "Copilot was unable to review this pull request because the user who requested the review has insufficient remaining quota."
	oops  = "Copilot encountered an error and was unable to review this pull request. You can try again by re-requesting a review."
)

// A login whose reviews are all refusals stops the wait at once, however long
// ci.watch.timeout and while another login's verdict is still to come: 75,
// review-refused, each such login in the config's order with its last refusal,
// auto-merge off, the answer asked once.
func TestPushReadyStopsAtARefusal(t *testing.T) {
	f, p := draftRepo(t, `draft: true, review_by: ["b[bot]", c, a]`, "ci: { watch: { interval: 2, timeout: 30 } }\n")
	p.open, p.draft = open7, true
	p.graph = func(int) string {
		return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[`+
			`{"author":{"login":"b"},"body":%q},{"author":{"login":"a"},"body":%q},{"author":{"login":"a"},"body":%q}]},`+
			`"comments":{"nodes":[]},"reviewThreads":{"nodes":[]}}}}}`, quota+"\nmore", quota, oops)
	}
	c := clocked(t)
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	r := reported(t, stdout)
	want := "itos: b[bot] gave no verdict on " + open7 + ": \"" + quota + "\"; auto-merge is not turned on: request its review again, then itos push --ready again.\n" +
		"itos: a gave no verdict on " + open7 + ": \"" + oops + "\"; auto-merge is not turned on: request its review again, then itos push --ready again.\n"
	if code != temporary || r.OK || r.Outcome != "review-refused" || r.PR == nil || r.PR.AutoMerge || p.asks != 1 || len(c.slept) != 0 ||
		!strings.HasSuffix(stderr, want) || strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
		t.Errorf("exit %d, asks %d, slept %v\n%s%s", code, p.asks, c.slept, stdout, stderr)
	}
}

// An answer with no comments (the recording gh of the scenarios before slice
// 187 gave none) has none, and a review of the login counts.
func TestPushReadyWhereTheAnswerHasNoComments(t *testing.T) {
	f, p := draftRepo(t, "draft: true, review_by: [a]")
	p.open, p.draft = open7, true
	p.graph = func(int) string {
		return `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[{"author":{"login":"a"}}]},"reviewThreads":{"nodes":[]}}}}}`
	}
	if code, _, stderr, _ := call(t, f, "--ready"); code != 0 {
		t.Errorf("exit %d\n%s", code, stderr)
	}
}
