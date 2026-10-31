package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
	"github.com/donvargax/itos/v8/internal/work"
)

const (
	head    = "0123456789abcdef0123456789abcdef01234567"
	oldHead = "fedcba9876543210fedcba9876543210fedcba98"
	pull7   = "https://github.example/o/r/pull/7"
)

// fake is review's ports over no world: a config (nil for none), a registry
// and a people file written as text, the commits git resolves, the comments
// GitHub answers with their head, the gh calls made and the files readable.
type fake struct {
	config     string // the YAML added to the config, "" for no config file
	noConfig   bool
	badYAML    bool
	registry   string
	people     string
	resolved   map[string]string
	remote     string
	noRepo     bool
	head       string
	comments   []providers.Authored
	reviews    []providers.Authored // with their commits
	failWith   string               // a pr comment whose body has it fails
	revision   string               // the item's revision as next gives it
	revErr     error
	revProblem *out.Problem
	ghErr      map[string]error  // the call (its first two words) that fails
	answers    map[string]string // the call's answer
	calls      []string
	files      map[string]string
}

func newFake() *fake {
	return &fake{
		config:   "push:\n  mode: pr\n",
		registry: "items:\n  - { id: slice-9, title: slice-9, phase: 1, owner: null, status: doing }\n",
		people:   "- ana\n- someone\n",
		resolved: map[string]string{"HEAD": head, "HEAD~1": oldHead},
		remote:   "origin",
		head:     head,
		ghErr:    map[string]error{},
		answers:  map[string]string{},
		files:    map[string]string{},
	}
}

func (f *fake) ConfigFound(context.Context) bool { return !f.noConfig }

func (f *fake) LoadConfig(context.Context) (*config.Loaded, error) {
	if f.badYAML {
		return nil, errors.New("itos.yaml: not YAML")
	}
	text := "version: 1\nwork:\n  people:\n    source: yaml\n    file: people.yaml\n" + f.config
	files := map[string]string{"itos.yaml": text}
	if f.registry != "" {
		files["tasks/work-items.yaml"] = f.registry
	}
	if f.people != "" {
		files["people.yaml"] = f.people
	}
	src, err := treefake.Trees{Files: map[string]map[string]string{"worktree": files}}.At(context.Background(), "worktree")
	if err != nil {
		return nil, err
	}
	return config.FromText(&vcsfake.VCS{}, treefake.Trees{}, src, "itos.yaml", text, false)
}

func (f *fake) Resolve(_ context.Context, rev string) (string, error) {
	if sha, ok := f.resolved[rev]; ok {
		return sha, nil
	}
	return "", errors.New("unknown revision " + rev)
}

func (f *fake) Remote(context.Context) string { return f.remote }

func (f *fake) GitHubRepository(context.Context, *config.Loaded, string) string {
	if f.noRepo {
		return ""
	}
	return "owner/name"
}

func (f *fake) Gh(_ context.Context, args ...string) (string, error) {
	call := strings.Join(args, " ")
	f.calls = append(f.calls, call)
	key := strings.Join(args[:2], " ")
	if err, failing := f.ghErr[key]; failing {
		return "", err
	}
	if answer, given := f.answers[key]; given {
		return answer, nil
	}
	switch key {
	case "pr list":
		return `[{"number":7,"url":"` + pull7 + `"}]`, nil
	case "pr comment":
		if f.failWith != "" && strings.Contains(call, f.failWith) {
			return "", &providers.GhFailed{Words: "HTTP 502: Bad Gateway", Err: errors.New("exit 1")}
		}
		return pull7 + "#issuecomment-2\n", nil
	case "api graphql":
		nodes := func(of []providers.Authored) string {
			each := make([]string, 0, len(of))
			for _, c := range of {
				body, _ := json.Marshal(c.Body)
				commit := ""
				if c.Commit != "" {
					commit = fmt.Sprintf(`,"commit":{"oid":%q}`, c.Commit)
				}
				each = append(each, fmt.Sprintf(`{"author":{"login":%q},"body":%s%s}`, c.Login, body, commit))
			}
			return strings.Join(each, ",")
		}
		return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"headRefOid":%q,"reviews":{"nodes":[%s]},"comments":{"nodes":[%s]},"reviewThreads":{"nodes":[]}}}}}`,
			f.head, nodes(f.reviews), nodes(f.comments)), nil
	}
	if strings.HasPrefix(key, "api repos/") {
		return `{"id":1,"html_url":"` + pull7 + `#issuecomment-1"}`, nil
	}
	return "", nil
}

func (f *fake) ReadFile(_ context.Context, path string) ([]byte, error) {
	if text, ok := f.files[path]; ok {
		return []byte(text), nil
	}
	return nil, errors.New("open " + path + ": no such file or directory")
}

func (f *fake) Revision(context.Context, *config.Loaded, string) (string, *out.Problem, error) {
	return f.revision, f.revProblem, f.revErr
}

// A comment recording a verdict as itos writes one.
func verdict(login, item, stage, verdictWord, sha string, round int) providers.Authored {
	q := Query{Item: item, Stage: stage, Verdict: verdictWord}
	return providers.Authored{Login: login, Body: verdictBody(q, sha, "", round)}
}

// run parses the arguments and runs review over f: the exit code and the
// two streams.
func run(t *testing.T, f *fake, json bool, args ...string) (int, string, string) {
	t.Helper()
	c, err := Parse(args, "", json, false)
	if err != nil {
		t.Fatalf("Parse(%v): %v", args, err)
	}
	var stdout, stderr bytes.Buffer
	code, err := c.Run(context.Background(), &stdout, &stderr, NewHandler(f))
	if err != nil {
		t.Fatalf("Run(%v): %v", args, err)
	}
	return code, stdout.String(), stderr.String()
}

// reported is --json's object.
func reported(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	return got
}

// rule is the one problem's rule of a refusal, "" for none.
func rule(got map[string]any) string {
	problems, _ := got["problems"].([]any)
	if len(problems) != 1 {
		return ""
	}
	p, _ := problems[0].(map[string]any)
	r, _ := p["rule"].(string)
	return r
}

// Parse: post with an item, --verdict and --at, --stage spec by default or
// diff, --body-file from where the person stood; everything else a usage
// error, the verdict's word not judged here.
func TestParse(t *testing.T) {
	c, err := Parse([]string{"post", "slice-9", "--verdict", "reject", "--at", "HEAD"}, "", true, true)
	if err != nil || !reflect.DeepEqual(c, CLI{Query: Query{Item: "slice-9", Verdict: "reject", At: "HEAD", Stage: "spec"}, JSON: true, Quiet: true}) {
		t.Errorf("%+v, %v", c, err)
	}
	c, err = Parse([]string{"post", "slice-9", "--verdict", "changes", "--at", "HEAD", "--stage", "diff", "--body-file", "review.md"}, "sub", false, false)
	if err != nil || c.Stage != "diff" || c.BodyFile != filepath.Join("sub", "review.md") {
		t.Errorf("%+v, %v", c, err)
	}
	abs, err := filepath.Abs(filepath.Join("elsewhere", "review.md"))
	if err != nil {
		t.Fatal(err)
	}
	if c, err := Parse([]string{"post", "slice-9", "--verdict", "changes", "--at", "HEAD", "--body-file", abs}, "sub", false, false); err != nil || c.BodyFile != abs {
		t.Errorf("an absolute body file: %+v, %v", c, err)
	}
	for name, args := range map[string][]string{
		"review alone":      {},
		"another word":      {"list"},
		"no item":           {"post", "--verdict", "approve", "--at", "HEAD"},
		"two items":         {"post", "slice-9", "slice-8", "--verdict", "approve", "--at", "HEAD"},
		"no verdict":        {"post", "slice-9", "--at", "HEAD"},
		"no at":             {"post", "slice-9", "--verdict", "approve"},
		"bad stage":         {"post", "slice-9", "--verdict", "approve", "--at", "HEAD", "--stage", "code"},
		"another flag":      {"post", "slice-9", "--verdict", "approve", "--at", "HEAD", "--ready"},
		"body file no path": {"post", "slice-9", "--verdict", "approve", "--at", "HEAD", "--body-file"},
		"empty verdict":     {"post", "slice-9", "--verdict", "", "--at", "HEAD"},
	} {
		t.Run(name, func(t *testing.T) {
			var usage out.UsageError
			if _, err := Parse(args, "", false, false); !errors.As(err, &usage) {
				t.Errorf("%v: %v", args, err)
			}
		})
	}
}

// approve at the head with no marker posts the line and the marker as round
// 1, one plain issue comment with the body its last field; --json gives
// every field; -q says nothing.
func TestPostsRoundOne(t *testing.T) {
	f := newFake()
	code, stdout, stderr := run(t, f, false, "post", "slice-9", "--verdict", "approve", "--at", "HEAD")
	if code != 0 || stderr != "" || stdout != "Posted the spec review of slice-9 at 0123456: approve, round 1, on "+pull7+".\n" {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	want := []string{
		"pr list --repo owner/name --head slice-9 --state open --json number,url",
		"api graphql -f query=" + providers.PullStateQuery + " -f owner=owner -f name=name -F number=7",
		"api repos/owner/name/issues/7/comments -f body=itos: spec review of slice-9 at 0123456: approve\n\n" +
			`<!-- itos:v1 {"record":"verdict","item":"slice-9","stage":"spec","verdict":"approve","head":"` + head + `","round":1} -->`,
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("gh was called\n%s\nwant\n%s", strings.Join(f.calls, "\n"), strings.Join(want, "\n"))
	}

	f = newFake()
	code, stdout, _ = run(t, f, true, "post", "slice-9", "--verdict", "approve", "--at", "HEAD")
	got := reported(t, stdout)
	if code != 0 || got["ok"] != true || got["outcome"] != "posted" || got["item"] != "slice-9" || got["stage"] != "spec" ||
		got["verdict"] != "approve" || got["head"] != head || got["round"] != 1.0 || got["comment"] != pull7+"#issuecomment-1" {
		t.Errorf("--json: exit %d, %s", code, stdout)
	}

	f = newFake()
	c, _ := Parse([]string{"post", "slice-9", "--verdict", "approve", "--at", "HEAD"}, "", false, true)
	var quiet bytes.Buffer
	if code, err := c.Run(context.Background(), &quiet, &quiet, NewHandler(f)); code != 0 || err != nil || quiet.Len() != 0 {
		t.Errorf("-q: exit %d, %v, %q", code, err, quiet.String())
	}
}

// --stage diff with a body file puts the file's text between the line and
// the marker, \r\n read as \n and the trailing line ends dropped; a full
// SHA is taken as given, git not asked, in lower case as GitHub writes the
// head it is held to.
func TestPostsTheBodyFile(t *testing.T) {
	f := newFake()
	f.files["review.md"] = "P2: the help says less.\r\nThe rest reads well.\r\n\r\n"
	f.resolved = nil
	code, _, stderr := run(t, f, false, "post", "slice-9", "--verdict", "changes", "--at", strings.ToUpper(head), "--stage", "diff", "--body-file", "review.md")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	want := "api repos/owner/name/issues/7/comments -f body=itos: diff review of slice-9 at 0123456: changes\n\nP2: the help says less.\nThe rest reads well.\n\n" +
		`<!-- itos:v1 {"record":"verdict","item":"slice-9","stage":"diff","verdict":"changes","head":"` + head + `","round":1} -->`
	if f.calls[2] != want {
		t.Errorf("posted\n%s\nwant\n%s", f.calls[2], want)
	}
}

// The round is one more than the people's markers of the item and stage,
// compared regardless of case; an outsider's, another stage's, another
// item's, the review-request marker, prose and a marker that is not JSON
// count for nothing. With no people every login counts.
func TestCountsTheRound(t *testing.T) {
	f := newFake()
	f.people = "- Ana\n- someone\n"
	f.comments = []providers.Authored{
		verdict("someone", "slice-9", "spec", "changes", oldHead, 1),
		verdict("ana", "slice-9", "spec", "changes", head, 2),
		verdict("someone", "slice-9", "diff", "approve", head, 1),
		verdict("someone", "slice-8", "spec", "approve", head, 1),
		{Login: "someone", Body: "@codex review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector " + head + " -->"},
		{Login: "ana", Body: "I approve of this spec; the second round reads well."},
		{Login: "ana", Body: "<!-- itos:v1 not json -->"},
		{Login: "ana", Body: "<!-- itos:v1 {\"record\":\"attempt\"} -->"},
		{Login: "ana", Body: "<!-- itos:v1 no end"},
		verdict("copilot-pull-request-reviewer", "slice-9", "spec", "approve", head, 3),
	}
	code, stdout, _ := run(t, f, true, "post", "slice-9", "--verdict", "approve", "--at", "HEAD")
	if got := reported(t, stdout); code != 0 || got["outcome"] != "posted" || got["round"] != 3.0 {
		t.Errorf("exit %d, %s", code, stdout)
	}

	for name, people := range map[string]string{"no people file": "", "nobody listed": "[]\n"} {
		t.Run(name, func(t *testing.T) {
			f := newFake()
			f.people = people
			f.comments = []providers.Authored{verdict("copilot-pull-request-reviewer", "slice-9", "spec", "approve", head, 1)}
			code, stdout, _ := run(t, f, true, "post", "slice-9", "--verdict", "approve", "--at", "HEAD")
			if got := reported(t, stdout); code != 0 || got["outcome"] != "already" || got["round"] != 1.0 || len(f.calls) != 2 {
				t.Errorf("exit %d, %s, %v", code, stdout, f.calls)
			}
		})
	}
}

// Already is the latest counted marker recording this verdict at this head:
// nothing is posted, the round is its, --json has no comment. Another
// verdict at the head, the same verdict at another head, or an earlier
// marker recording it with a later one that does not, is the next round.
func TestAlreadyIsTheLatestMarkers(t *testing.T) {
	f := newFake()
	f.comments = []providers.Authored{verdict("ana", "slice-9", "spec", "approve", head, 1)}
	code, stdout, stderr := run(t, f, false, "post", "slice-9", "--verdict", "approve", "--at", "HEAD")
	if code != 0 || stderr != "" || stdout != "The spec review of slice-9 at 0123456: approve already, round 1, on "+pull7+"; nothing was posted.\n" || len(f.calls) != 2 {
		t.Errorf("exit %d\n%s%s\n%v", code, stdout, stderr, f.calls)
	}
	f = newFake()
	f.comments = []providers.Authored{verdict("ana", "slice-9", "spec", "approve", head, 1)}
	_, stdout, _ = run(t, f, true, "post", "slice-9", "--verdict", "approve", "--at", "HEAD")
	got := reported(t, stdout)
	if _, has := got["comment"]; has || got["outcome"] != "already" || got["verdict"] != "approve" || got["round"] != 1.0 {
		t.Errorf("--json: %s", stdout)
	}

	for name, c := range map[string]struct {
		comments []providers.Authored
		verdict  string
		round    float64
	}{
		"another verdict at the head": {[]providers.Authored{verdict("someone", "slice-9", "spec", "approve", head, 1)}, "changes", 2},
		"the verdict at another head": {[]providers.Authored{verdict("someone", "slice-9", "spec", "approve", oldHead, 1)}, "approve", 2},
		"an earlier marker records it": {[]providers.Authored{
			verdict("ana", "slice-9", "spec", "approve", head, 1), verdict("ana", "slice-9", "spec", "changes", head, 2)}, "approve", 3},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake()
			f.comments = c.comments
			code, stdout, _ := run(t, f, true, "post", "slice-9", "--verdict", c.verdict, "--at", "HEAD")
			if got := reported(t, stdout); code != 0 || got["outcome"] != "posted" || got["round"] != c.round || len(f.calls) != 3 {
				t.Errorf("exit %d, %s, %d calls", code, stdout, len(f.calls))
			}
		})
	}
}

// Refused before gh is asked, each naming its rule, the message and fix on
// stderr without --json: no pr mode (no config, one that cannot be read, or
// trunk), an unknown item, a missing registry, a bad verdict, a body file
// that cannot be read, a commit git cannot resolve, and a remote naming no
// repository (exit 3).
func TestRefusedBeforeGh(t *testing.T) {
	for name, c := range map[string]struct {
		set  func(f *fake)
		args []string
		rule string
		code int
	}{
		"no config":         {func(f *fake) { f.noConfig = true }, nil, "review-without-pr-mode", 1},
		"unreadable config": {func(f *fake) { f.badYAML = true }, nil, "review-without-pr-mode", 1},
		"trunk":             {func(f *fake) { f.config = "" }, nil, "review-without-pr-mode", 1},
		"unknown item":      {func(*fake) {}, []string{"post", "slice-7", "--verdict", "approve", "--at", "HEAD"}, "work-unknown-item", 1},
		"no registry":       {func(f *fake) { f.registry = "" }, nil, "work-registry-missing", 1},
		"bad verdict":       {func(*fake) {}, []string{"post", "slice-9", "--verdict", "Approve", "--at", "HEAD"}, "review-bad-verdict", 1},
		"unreadable body":   {func(*fake) {}, []string{"post", "slice-9", "--verdict", "approve", "--at", "HEAD", "--body-file", "missing.md"}, "review-body-file", 1},
		"unknown commit":    {func(*fake) {}, []string{"post", "slice-9", "--verdict", "approve", "--at", "nosuchref"}, "review-unknown-commit", 1},
		"no repository":     {func(f *fake) { f.noRepo = true }, nil, "no-github-repository", 3},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake()
			c.set(f)
			args := c.args
			if args == nil {
				args = []string{"post", "slice-9", "--verdict", "approve", "--at", "HEAD"}
			}
			code, stdout, stderr := run(t, f, true, args...)
			if got := reported(t, stdout); code != c.code || got["ok"] != false || rule(got) != c.rule || stderr != "" || len(f.calls) != 0 {
				t.Errorf("exit %d\n%s%s\n%v", code, stdout, stderr, f.calls)
			}
			f = newFake()
			c.set(f)
			code, stdout, stderr = run(t, f, false, args...)
			if code != c.code || stdout != "" || !strings.Contains(stderr, "nothing was posted") || !strings.Contains(stderr, "  fix: ") {
				if c.rule == "work-unknown-item" || c.rule == "work-registry-missing" {
					if code != c.code || stdout != "" || !strings.Contains(stderr, "  fix: ") {
						t.Errorf("text: exit %d\n%s%s", code, stdout, stderr)
					}
					return
				}
				t.Errorf("text: exit %d\n%s%s", code, stdout, stderr)
			}
		})
	}
	f := newFake()
	f.registry = "items: 3\n"
	c, _ := Parse([]string{"post", "slice-9", "--verdict", "approve", "--at", "HEAD"}, "", false, false)
	var buf bytes.Buffer
	if _, err := c.Run(context.Background(), &buf, &buf, NewHandler(f)); kind.Of(err) != kind.Usage {
		t.Errorf("a registry of the wrong shape: %v", err)
	}
}

// Once gh is asked: no open pull request (exit 1, after gh pr list alone),
// a head that moved (the new head and the command named, an unknown full
// SHA judged the same), and gh that fails or answers without the shape
// (exit 3, naming the call), nothing posted.
func TestRefusedByGitHub(t *testing.T) {
	f := newFake()
	f.answers["pr list"] = "[]"
	code, stdout, _ := run(t, f, true, "post", "slice-9", "--verdict", "approve", "--at", "HEAD")
	if got := reported(t, stdout); code != 1 || rule(got) != "review-no-pull-request" || len(f.calls) != 1 {
		t.Errorf("exit %d, %s, %v", code, stdout, f.calls)
	}

	for at, named := range map[string]string{"HEAD~1": oldHead, "ffffffffffffffffffffffffffffffffffffffff": "ffffffffffffffffffffffffffffffffffffffff"} {
		f = newFake()
		code, stdout, stderr := run(t, f, false, "post", "slice-9", "--verdict", "approve", "--at", at)
		want := "itos: slice-9's pull request " + pull7 + " is at " + head + ", not " + named + "; nothing was posted.\n" +
			"  fix: review 0123456 and run itos review post slice-9 --verdict approve --at " + head + "\n"
		if code != 1 || stdout != "" || stderr != want || len(f.calls) != 2 {
			t.Errorf("--at %s: exit %d\n%s%s\nwant\n%s", at, code, stdout, stderr, want)
		}
	}

	for name, c := range map[string]struct {
		set     func(f *fake)
		message string
		calls   int
	}{
		"gh missing": {func(f *fake) { f.ghErr["pr list"] = providers.ErrNoGh }, "gh pr list failed: gh is not on the PATH", 1},
		"pr list fails": {func(f *fake) {
			f.ghErr["pr list"] = &providers.GhFailed{Words: "HTTP 401: Bad credentials\n", Err: errors.New("exit 1")}
		}, "gh pr list failed: HTTP 401: Bad credentials", 1},
		"pr list not a list": {func(f *fake) { f.answers["pr list"] = "nope" }, "gh pr list failed: its answer is not a list of pull requests", 1},
		"graphql fails":      {func(f *fake) { f.ghErr["api graphql"] = &providers.GhFailed{Words: "boom", Err: errors.New("exit 1")} }, "gh api graphql failed: boom", 2},
		"graphql no shape":   {func(f *fake) { f.answers["api graphql"] = "{}" }, "gh api graphql failed: its answer is not a pull request's reviews", 2},
		"graphql no head":    {func(f *fake) { f.head = "" }, "gh api graphql failed: its answer names no head", 2},
		"post fails": {func(f *fake) {
			f.ghErr["api repos/owner/name/issues/7/comments"] = &providers.GhFailed{Words: "HTTP 403", Err: errors.New("exit 1")}
		}, "gh api repos/owner/name/issues/7/comments failed: HTTP 403", 3},
		"post answers no url": {func(f *fake) { f.answers["api repos/owner/name/issues/7/comments"] = "{}" }, "names no comment (html_url); the comment may be posted", 3},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake()
			c.set(f)
			code, stdout, _ := run(t, f, true, "post", "slice-9", "--verdict", "approve", "--at", "HEAD")
			got := reported(t, stdout)
			problems, _ := got["problems"].([]any)
			p, _ := problems[0].(map[string]any)
			if code != 3 || rule(got) != "review-pull-request-failed" || !strings.Contains(p["message"].(string), c.message) || len(f.calls) != c.calls {
				t.Errorf("exit %d, %s, %d calls", code, stdout, len(f.calls))
			}
		})
	}
}

// work.ReadVerdict reads only a marker whose JSON is a verdict's, and
// verdictMarker writes one it reads back, the keys in order.
func TestMarkers(t *testing.T) {
	q := Query{Item: "slice-9", Stage: "spec", Verdict: "approve"}
	written := verdictMarker(q, head, 2)
	if written != `<!-- itos:v1 {"record":"verdict","item":"slice-9","stage":"spec","verdict":"approve","head":"`+head+`","round":2} -->` {
		t.Errorf("written %s", written)
	}
	m, ok := work.ReadVerdict("itos: spec review of slice-9 at 0123456: approve\r\n\r\n" + written + "\r\n")
	if !ok || m != (work.Verdict{Record: "verdict", Item: "slice-9", Stage: "spec", Verdict: "approve", Head: head, Round: 2}) {
		t.Errorf("read %+v, %t", m, ok)
	}
	// The review's text quotes a marker: the one appended after it is read.
	quoted := verdictMarker(Query{Item: "slice-9", Stage: "spec", Verdict: "changes"}, oldHead, 1)
	if m, ok := work.ReadVerdict("itos: spec review of slice-9 at 0123456: approve\n\nRound 1 ended with " + quoted + ", now fixed.\n\n" + written); !ok || m.Verdict != "approve" || m.Round != 2 {
		t.Errorf("read the quoted marker: %+v, %t", m, ok)
	}
	for _, body := range []string{"", "prose", "<!-- itos:v1 -->", "<!-- itos:v1 {} -->", "<!-- itos:v1 {\"record\":\"verdict\"", "<!-- itos-review-request:v1 x " + head + " -->"} {
		if _, ok := work.ReadVerdict(body); ok {
			t.Errorf("%q read as a verdict", body)
		}
	}
	if short("abc") != "abc" {
		t.Error("a short SHA is cut")
	}
}
