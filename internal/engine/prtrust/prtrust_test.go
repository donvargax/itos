package prtrust

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/plan"
	"github.com/donvargax/itos/v8/internal/providers"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

const configText = `version: 1
push: { mode: pr }
ledger:
  files: "tasks/phase-{group}.yaml"
  id: "T-\\d+"
commits:
  footers:
    Task: { source: ledger }
    Scenarios: { source: { tests: scenario }, strip_prefix: "@" }
tests:
  scenario:
    adapter: gherkin
    root: features
    id: "ID-[A-Z]+-\\d+"
    tag_prefix: "@"
    wip_tag: "@wip"
    run: { whole: "e2e", select: "e2e --grep {pattern}", ids_pattern: "@(?:{ids})" }
ci:
  steps:
    - { tests: scenario }
    - whole
  prose: { paths: ["**/*.md"], steps: [prose] }
work:
  registry: work-items.yaml
`

const (
	ledgerFile = "tasks/phase-1.yaml"
	checked    = `{ run: "true", cost: static }`
	pullURL    = "https://github.example/o/r/pull/7"
	built      = "feat: build\n\nTask: T-1\n"
)

// entry is a ledger file holding the task with the checks given, each a flow
// mapping; none gives it no done_when.
func entry(id string, checks ...string) string {
	text := "- id: " + id + "\n  type: chore\n  title: Adopt\n"
	if len(checks) > 0 {
		text += "  done_when:\n"
	}
	for _, c := range checks {
		text += "    - " + c + "\n"
	}
	return text
}

// ledgerOf are the ledger's files holding the texts, one file each, phase-1
// first.
func ledgerOf(files ...string) map[string]string {
	ledger := map[string]string{}
	for i, text := range files {
		ledger[strings.Replace(ledgerFile, "1", string(rune('1'+i)), 1)] = text
	}
	return ledger
}

// tree is a tree of files with the changes laid over it, a text of "" taking
// the file out.
func tree(base map[string]string, changes ...map[string]string) map[string]string {
	t := maps.Clone(base)
	for _, c := range changes {
		for path, text := range c {
			if text == "" {
				delete(t, path)
			} else {
				t[path] = text
			}
		}
	}
	return t
}

func files(kv ...string) map[string]string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

// scene is a merged pull request in a clone. Main's commits are b0, o1 (a
// registry commit of main's own) and r1, the pull request's commit rebased;
// the head h1 is on b0. The range is b0..r1 and GitHub lists pull request 7,
// merged as r1 from h1, whose run passed.
type scene struct {
	history  []vcsfake.Commit
	late     []vcsfake.Commit // commits a fetch of the head brings
	from, to string
	config   string
	pulls    []providers.Pull
	pullsErr error
	run      providers.Run
	runFound bool
	runErr   error
	trees    map[string]map[string]string // the ledger's files by commit, when not the commits' trees
	noTrees  bool                         // the repository gives no commit's tree
	mutate   func(*broken)
	// what was asked
	pullsAsked, runsAsked []string
	fetched               []string
	talks                 []vcsport.Talk
	deadline              time.Duration
}

var base = files(ledgerFile, entry("T-1", checked), "work-items.yaml", "r0\n", "src/app.go", "a\n")

func newScene() *scene {
	main := tree(base, files("work-items.yaml", "r1\n"))
	return &scene{
		history: []vcsfake.Commit{
			{Name: "b0", Message: "chore: base\n", Tree: base},
			{Name: "o1", Parents: []string{"b0"}, Message: "docs: add an idea\n", Tree: main},
			{Name: "r1", Parents: []string{"o1"}, Message: built, Tree: tree(main, files("src/app.go", "b\n"))},
			{Name: "h1", Parents: []string{"b0"}, Message: built, Tree: tree(base, files("src/app.go", "b\n"))},
		},
		from:     "b0",
		to:       "r1",
		config:   configText,
		pulls:    []providers.Pull{{Number: 7, URL: pullURL, Merged: true, MergeCommit: "r1", Head: "h1"}},
		run:      passed("u-head"),
		runFound: true,
	}
}

func passed(url string) providers.Run {
	return providers.Run{URL: url, Status: "completed", Conclusion: "success",
		Jobs: []providers.Job{{Name: "ci", Status: "completed", Conclusion: "success"}}}
}

// broken is the fake VCS with what a test makes fail or happen.
type broken struct {
	*vcsfake.VCS
	paths error
	// log, when set, is asked first and its error fails the log.
	log func() error
	// diff, when set, is asked first and its error fails the diff.
	diff  func() error
	fetch func(ctx context.Context, remote, refspec string, talk vcsport.Talk) error
}

func (b broken) Diff(ctx context.Context, from, to string) ([]string, error) {
	if b.diff != nil {
		if err := b.diff(); err != nil {
			return nil, err
		}
	}
	return b.VCS.Diff(ctx, from, to)
}

func (b broken) Log(ctx context.Context, r vcsport.Range) ([]vcsport.Entry, error) {
	if b.log != nil {
		if err := b.log(); err != nil {
			return nil, err
		}
	}
	return b.VCS.Log(ctx, r)
}

func (b broken) CommitPaths(ctx context.Context, rev string) ([]string, error) {
	if b.paths != nil {
		return nil, b.paths
	}
	return b.VCS.CommitPaths(ctx, rev)
}

func (b broken) Fetch(ctx context.Context, remote, refspec string, talk vcsport.Talk) (string, error) {
	if b.fetch != nil {
		return "", b.fetch(ctx, remote, refspec, talk)
	}
	return b.VCS.Fetch(ctx, remote, refspec, talk)
}

// judge is Judge over the scene.
func (s *scene) judge(t *testing.T) Result {
	t.Helper()
	v := &vcsfake.VCS{
		Located:  map[string]vcsport.Location{"": {Top: "/r", GitDir: ".git", Common: ".git"}},
		History:  slices.Clone(s.history),
		Named:    map[string]string{"HEAD": s.to},
		OnBranch: "main",
	}
	b := broken{VCS: v}
	b.fetch = func(ctx context.Context, remote, refspec string, talk vcsport.Talk) error {
		s.fetched = append(s.fetched, remote+" "+refspec)
		s.talks = append(s.talks, talk)
		if d, ok := ctx.Deadline(); ok {
			s.deadline = time.Until(d)
		}
		v.History = append(v.History, s.late...)
		return nil
	}
	if s.mutate != nil {
		s.mutate(&b)
	}
	trees := treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": base}}
	for _, c := range s.history {
		trees.Files[c.Name] = c.Tree
	}
	for _, c := range s.late {
		trees.Files[c.Name] = c.Tree
	}
	maps.Copy(trees.Files, s.trees)
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(b, trees, src, "itos.yaml", s.config, false)
	if err != nil {
		t.Fatal(err)
	}
	if s.noTrees {
		cfg.Trees = treefake.Trees{}
	}
	look := providers.MergedPulls{
		Pulls: func(sha string) ([]providers.Pull, error) {
			s.pullsAsked = append(s.pullsAsked, sha)
			return s.pulls, s.pullsErr
		},
		Run: func(sha string) (providers.Run, bool, error) {
			s.runsAsked = append(s.runsAsked, sha)
			return s.run, s.runFound, s.runErr
		},
	}
	return Judge(t.Context(), cfg, look, s.from, s.to)
}

// note is the line that says the pull request's head was not trusted.
func note(reason string) string { return noteOf("h1", reason) }

// noteOf is note for a pull request whose head is another commit.
func noteOf(head, reason string) string {
	return "Not trusted: the pull request " + pullURL + " merged its head " + head + " as r1, but " + reason + "."
}

// A range of main's registry commit and the pull request's commit rebased,
// whose head's run passed, is trusted: the trust names the pull request, the
// head in full and the run, and its line says so.
func TestJudgeTrustsTheMergedHeadsRun(t *testing.T) {
	s := newScene()
	got := s.judge(t)
	want := &plan.Trust{PullRequest: pullURL, Head: "h1", Run: "u-head",
		Line: "Trusted: the pull request " + pullURL + " merged its head h1 as r1, the same commits with the same trees but for the work registry and the ledger, and the head's CI run passed: u-head. Only the prose steps run, and the named tasks' static and `prose: true` checks."}
	if got.Trust == nil || *got.Trust != *want || got.Note != "" {
		t.Errorf("Judge = %+v, want %+v", got, want)
	}
	if !slices.Equal(s.pullsAsked, []string{"r1"}) || !slices.Equal(s.runsAsked, []string{"h1"}) || len(s.fetched) != 0 {
		t.Errorf("asked pulls of %v, the run of %v, fetched %v", s.pullsAsked, s.runsAsked, s.fetched)
	}
}

// A ledger commit of main's trusts too, and so does a range of the pull
// request's commits alone and one of several commits, each with its own.
func TestJudgeTrustsOtherRanges(t *testing.T) {
	s := newScene()
	s.history[1].Tree = tree(base, files(ledgerFile, entry("T-1", checked)+entry("T-2")))
	s.history[2].Tree = tree(s.history[1].Tree, files("src/app.go", "b\n"))
	if got := s.judge(t); got.Trust == nil {
		t.Errorf("a ledger commit of main's: %+v", got)
	}

	s = newScene()
	s.history = []vcsfake.Commit{s.history[0], {Name: "r1", Parents: []string{"b0"}, Message: built, Tree: s.history[3].Tree}, s.history[3]}
	if got := s.judge(t); got.Trust == nil {
		t.Errorf("nothing of main's: %+v", got)
	}

	s = newScene()
	two := tree(s.history[2].Tree, files("src/b.go", "c\n"))
	s.history = append(s.history[:3], vcsfake.Commit{Name: "r2", Parents: []string{"r1"}, Message: "feat: more\n", Tree: two},
		s.history[3], vcsfake.Commit{Name: "h2", Parents: []string{"h1"}, Message: "feat: more\n", Tree: tree(s.history[3].Tree, files("src/b.go", "c\n"))})
	s.to, s.pulls[0].MergeCommit, s.pulls[0].Head = "r2", "r2", "h2"
	if got := s.judge(t); got.Trust == nil || got.Trust.Head != "h2" {
		t.Errorf("two commits: %+v", got)
	}
}

// Nothing is asked of GitHub where the rule does not apply: not under
// push.mode pr, with no prose steps, with a start that is empty, all zeros,
// not an ancestor of the end, or no end.
func TestJudgeAsksNothingWhereItDoesNotApply(t *testing.T) {
	for name, change := range map[string]func(*scene){
		"trunk": func(s *scene) {
			s.config = strings.Replace(configText, "push: { mode: pr }\n", "push: { mode: trunk }\n", 1)
		},
		"no push mode": func(s *scene) { s.config = strings.Replace(configText, "push: { mode: pr }\n", "", 1) },
		"no prose": func(s *scene) {
			s.config = strings.Replace(configText, "  prose: { paths: [\"**/*.md\"], steps: [prose] }\n", "", 1)
		},
		"no prose steps":   func(s *scene) { s.config = strings.Replace(configText, "steps: [prose]", "steps: []", 1) },
		"an empty start":   func(s *scene) { s.from = "" },
		"an all-zeros one": func(s *scene) { s.from = "0000000000000000000000000000000000000000" },
		"a start that is not an ancestor": func(s *scene) {
			s.from = "h1"
		},
		"no end": func(s *scene) { s.to = "" },
		"an end that names nothing": func(s *scene) {
			s.to = "nowhere"
		},
	} {
		s := newScene()
		change(s)
		if got := s.judge(t); got != (Result{}) || len(s.pullsAsked) != 0 {
			t.Errorf("%s: %+v, GitHub asked %v", name, got, s.pullsAsked)
		}
	}
}

// A look that fails, no pull request that counts and a pull request that is
// open, merged as another commit, or listed with no head change nothing and
// say nothing.
func TestJudgeSaysNothingWhenNoMergedPullRequestCounts(t *testing.T) {
	for name, change := range map[string]func(*scene){
		"a look that fails":    func(s *scene) { s.pullsErr = errors.New("down") },
		"none":                 func(s *scene) { s.pulls = nil },
		"an open one":          func(s *scene) { s.pulls[0].Merged = false },
		"another merge commit": func(s *scene) { s.pulls[0].MergeCommit = "o1" },
		"a pull with no head":  func(s *scene) { s.pulls[0].Head = "" },
	} {
		s := newScene()
		change(s)
		if got := s.judge(t); got != (Result{}) || len(s.runsAsked) != 0 {
			t.Errorf("%s: %+v, run asked %v", name, got, s.runsAsked)
		}
	}
	s := newScene()
	s.pulls = []providers.Pull{{Number: 3, URL: "other", Merged: true, MergeCommit: "o1", Head: "x"}, {Number: 4, URL: "open", MergeCommit: "r1", Head: "y"},
		{Number: 7, URL: pullURL, Merged: true, MergeCommit: "r1", Head: "h1"}}
	if got := s.judge(t); got.Trust == nil || got.Trust.PullRequest != pullURL {
		t.Errorf("the one of several that counts: %+v", got)
	}
}

// A head the clone lacks is fetched once from the current branch's upstream
// remote, else origin, never prompting, within the fetch timeout, naming no
// destination; a head the clone has is never fetched, and a fetch that fails
// or brings nothing is no trust, the line saying so.
func TestJudgeFetchesAHeadTheCloneLacks(t *testing.T) {
	s := newScene()
	s.late, s.history = s.history[3:], s.history[:3]
	got := s.judge(t)
	if got.Trust == nil || !slices.Equal(s.fetched, []string{"origin refs/pull/7/head"}) || len(s.talks) != 1 || !s.talks[0].NoPrompt ||
		s.deadline <= 0 || s.deadline > 20*time.Second {
		t.Errorf("fetched: %+v, %v, %v, %v", got, s.fetched, s.talks, s.deadline)
	}

	s = newScene()
	s.late, s.history = s.history[3:], s.history[:3]
	s.mutate = func(b *broken) {
		b.VCS.Tracking = map[string][2]string{"main": {"upstream", "refs/heads/main"}}
	}
	if got := s.judge(t); got.Trust == nil || !slices.Equal(s.fetched, []string{"upstream refs/pull/7/head"}) {
		t.Errorf("an upstream remote: %+v, %v", got, s.fetched)
	}

	s = newScene()
	s.late, s.history = s.history[3:], s.history[:3]
	s.mutate = func(b *broken) { b.VCS.OnBranch = "" }
	if got := s.judge(t); got.Trust == nil || !slices.Equal(s.fetched, []string{"origin refs/pull/7/head"}) {
		t.Errorf("a detached HEAD: %+v, %v", got, s.fetched)
	}

	s = newScene()
	s.late, s.history = s.history[3:], s.history[:3]
	s.mutate = func(b *broken) {
		b.fetch = func(context.Context, string, string, vcsport.Talk) error { return errors.New("no") }
	}
	got = s.judge(t)
	if got.Trust != nil || got.Note != note("this clone lacks h1 and could not fetch it from origin's refs/pull/7/head") || len(s.runsAsked) != 0 {
		t.Errorf("a fetch that fails: %+v", got)
	}

	s = newScene()
	s.history = s.history[:3]
	got = s.judge(t)
	if got.Trust != nil || got.Note != note("this clone lacks h1 and could not fetch it from origin's refs/pull/7/head") || len(s.fetched) != 1 {
		t.Errorf("a fetch that brings nothing: %+v, %v", got, s.fetched)
	}
}

// Each way the commits can fail the rule gives its own line, the first that
// fails, and asks for no run.
func TestJudgeNamesWhyTheCommitsDoNotQualify(t *testing.T) {
	more := files("src/more.go", "m\n")
	for name, c := range map[string]struct {
		change func(*scene)
		reason string
	}{
		"a merge commit in the range": {func(s *scene) {
			s.history[2].Parents = []string{"o1", "h1"}
		}, "the range or the head holds a merge commit"},
		"a merge commit in the head": {func(s *scene) {
			s.history[3].Parents = []string{"b0", "o1"}
		}, "the range or the head holds a merge commit"},
		"a head with no commit of its own": {func(s *scene) {
			s.pulls[0].Head = "o1"
		}, "the range or the head holds a merge commit"},
		"fewer commits than the head": {func(s *scene) {
			s.history = append(s.history, vcsfake.Commit{Name: "h2", Parents: []string{"h1"}, Message: "feat: more\n", Tree: s.history[3].Tree})
			s.pulls[0].Head = "h2"
		}, "the range does not end with the head's 2 commits"},
		"a message that differs": {func(s *scene) {
			s.history[2].Message = built + "Scenarios: ID-A-01\n"
		}, "the range does not end with the head's 1 commits"},
		"a commit of main's that changes a path": {func(s *scene) {
			s.history[1].Tree = tree(s.history[1].Tree, files("src/other.go", "o\n"))
			s.history[2].Tree = tree(s.history[2].Tree, files("src/other.go", "o\n"))
		}, "the range holds o1, which is not the pull request's and changes src/other.go"},
		"a commit of main's that changes several": {func(s *scene) {
			s.history[1].Tree = tree(s.history[1].Tree, files("src/other.go", "o\n", "docs.md", "d\n", "x", "x"))
			s.history[2].Tree = tree(s.history[2].Tree, files("src/other.go", "o\n", "docs.md", "d\n", "x", "x"))
		}, "the range holds o1, which is not the pull request's and changes docs.md and 2 more"},
		"a commit of main's that names a task": {func(s *scene) {
			s.history[1].Message = "docs: add an idea\n\nTask: T-1\n"
		}, "the range holds o1, which is not the pull request's and names T-1"},
		"a commit of main's that names a test": {func(s *scene) {
			s.history[1].Message = "docs: add an idea\n\nScenarios: ID-A-01\n"
		}, "the range holds o1, which is not the pull request's and names ID-A-01"},
		"a commit of main's that names a task and a test": {func(s *scene) {
			s.history[1].Message = "docs: add an idea\n\nScenarios: ID-A-01\nTask: T-1\n"
		}, "the range holds o1, which is not the pull request's and names T-1"},
		"a commit that differs from its counterpart": {func(s *scene) {
			s.history[2].Tree = tree(s.history[2].Tree, files("src/app.go", "c\n"))
		}, "r1 differs from the pull request's commit h1 in src/app.go"},
		"a commit that differs in several paths": {func(s *scene) {
			s.history[2].Tree = tree(s.history[2].Tree, more, files("NOTES.md", "n\n"))
		}, "r1 differs from the pull request's commit h1 in NOTES.md and 1 more"},
	} {
		s := newScene()
		c.change(s)
		got := s.judge(t)
		want := noteOf(s.pulls[0].Head, c.reason)
		if got.Trust != nil || got.Note != want || len(s.runsAsked) != 0 {
			t.Errorf("%s: %+v, want %q", name, got, want)
		}
	}
}

// A commit of main's that touches the registry and the ledger alone is
// bookkeeping even when it is empty or names an item; the ledger's
// registry is not a path that counts.
func TestJudgeAcceptsBookkeepingCommitsOfMains(t *testing.T) {
	s := newScene()
	s.history[1].Tree = s.history[0].Tree
	s.history[2].Tree = s.history[3].Tree
	s.history[1].Message = "docs: take it\n\nItem: slice-1\n"
	if got := s.judge(t); got.Trust == nil {
		t.Errorf("an empty commit of main's naming an item: %+v", got)
	}
}

// What git cannot give leaves the range untrusted and says nothing: a log,
// the paths of a commit, or a diff that fails, whichever of the rule's reads
// it is.
func TestJudgeSaysNothingWhenGitCannotGiveAReading(t *testing.T) {
	fails := errors.New("git failed")
	for name, change := range map[string]func(*broken){
		"the head's commits":           func(b *broken) { b.log = nthFails(1, fails) },
		"the range's commits":          func(b *broken) { b.log = nthFails(2, fails) },
		"a commit's paths":             func(b *broken) { b.paths = fails },
		"a commit and its counterpart": func(b *broken) { b.diff = nthFails(1, fails) },
		"the head and the end":         func(b *broken) { b.diff = nthFails(2, fails) },
	} {
		s := newScene()
		s.mutate = change
		if got := s.judge(t); got != (Result{}) || len(s.runsAsked) != 0 {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

// nthFails fails the nth call: of the diffs, the first is a commit's against
// its counterpart, the second the head's against the end's; of the logs, the
// first is the head's commits, the second the range's.
func nthFails(n int, err error) func() error {
	calls := 0
	return func() error {
		calls++
		if calls == n {
			return err
		}
		return nil
	}
}

// The tasks the range names are compared when a ledger file differs between
// the head and the end: other checks are a line of their own, a ledger that
// cannot be read at either commit another, and the ledger is not read when
// only the registry differs or no task is named.
func TestJudgeComparesTheNamedTasksChecks(t *testing.T) {
	late := entry("T-1", checked, `{ run: "exit 1", cost: late }`)
	ledgerChange := func(s *scene, text string) {
		s.history[1].Tree = tree(s.history[1].Tree, files(ledgerFile, text))
		s.history[2].Tree = tree(s.history[2].Tree, files(ledgerFile, text))
	}
	s := newScene()
	ledgerChange(s, late)
	got := s.judge(t)
	if got.Trust != nil || got.Note != note("T-1's checks in the ledger differ from the head's") || len(s.runsAsked) != 0 {
		t.Errorf("a late check: %+v", got)
	}

	s = newScene()
	ledgerChange(s, entry("T-1", checked)+entry("T-2", checked))
	if got := s.judge(t); got.Trust == nil {
		t.Errorf("another task's entry: %+v", got)
	}

	s = newScene()
	ledgerChange(s, late)
	s.history[2].Message, s.history[3].Message = "feat: build\n", "feat: build\n"
	if got := s.judge(t); got.Trust == nil {
		t.Errorf("no task named: %+v", got)
	}

	s = newScene()
	s.history[1].Tree = tree(s.history[1].Tree, files("work-items.yaml", "r2\n"))
	s.history[2].Tree = tree(s.history[2].Tree, files("work-items.yaml", "r2\n"))
	s.trees = map[string]map[string]string{"h1": nil, "r1": nil}
	if got := s.judge(t); got.Trust == nil {
		t.Errorf("a registry change reads no ledger: %+v", got)
	}

	for name, c := range map[string]struct {
		trees map[string]map[string]string
		at    string
	}{
		"not a list at the head": {map[string]map[string]string{"h1": ledgerOf("a: b\n")}, "h1"},
		"not YAML at the head":   {map[string]map[string]string{"h1": ledgerOf("- [\n")}, "h1"},
		"no folder at the head":  {map[string]map[string]string{"h1": nil}, "h1"},
		"not a list at the end":  {map[string]map[string]string{"r1": ledgerOf("a: b\n")}, "r1"},
		"no folder at the end":   {map[string]map[string]string{"r1": nil}, "r1"},
		"both, the head's named": {map[string]map[string]string{"h1": ledgerOf("a: b\n"), "r1": ledgerOf("a: b\n")}, "h1"},
	} {
		s = newScene()
		ledgerChange(s, late)
		s.trees = c.trees
		got := s.judge(t)
		if got.Trust != nil || got.Note != note("the ledger at "+c.at+" cannot be read") || len(s.runsAsked) != 0 {
			t.Errorf("%s: %+v", name, got)
		}
	}

	s = newScene()
	ledgerChange(s, late)
	s.noTrees = true
	if got := s.judge(t); got.Trust != nil || got.Note != note("the ledger at h1 cannot be read") {
		t.Errorf("trees the repository cannot give: %+v", got)
	}
}

// The head's run decides last: one that is going, failed, was a draft's (it
// lists no job, or only skipped ones), or cannot be found or looked at is no
// trust, each with its line.
func TestJudgeNeedsTheHeadsRunToHavePassed(t *testing.T) {
	skipped := providers.Job{Name: "ci", Status: "completed", Conclusion: "skipped"}
	for name, c := range map[string]struct {
		run    providers.Run
		found  bool
		err    error
		reason string
	}{
		"still going": {providers.Run{URL: "u-head", Status: "in_progress"}, true, nil, "the head's CI run u-head has not passed"},
		"failed": {providers.Run{URL: "u-head", Status: "completed", Conclusion: "failure",
			Jobs: []providers.Job{{Name: "ci", Status: "completed", Conclusion: "failure"}}}, true, nil, "the head's CI run u-head has not passed"},
		"cancelled": {providers.Run{URL: "u-head", Status: "completed", Conclusion: "cancelled"}, true, nil, "the head's CI run u-head has not passed"},
		"no jobs":   {providers.Run{URL: "u-head", Status: "completed", Conclusion: "success"}, true, nil, "the head's CI run u-head has not passed"},
		"all skipped": {providers.Run{URL: "u-head", Status: "completed", Conclusion: "success", Jobs: []providers.Job{skipped, skipped}},
			true, nil, "the head's CI run u-head has not passed"},
		"none":              {providers.Run{}, false, nil, "no CI run of the head was found"},
		"a look that fails": {providers.Run{URL: "u-head", Status: "completed", Conclusion: "success"}, true, errors.New("down"), "no CI run of the head was found"},
	} {
		s := newScene()
		s.run, s.runFound, s.runErr = c.run, c.found, c.err
		got := s.judge(t)
		if got.Trust != nil || got.Note != note(c.reason) {
			t.Errorf("%s: %+v, want %q", name, got, note(c.reason))
		}
	}
	s := newScene()
	s.run.Jobs = append(s.run.Jobs, skipped)
	if got := s.judge(t); got.Trust == nil {
		t.Errorf("a skipped job beside a job that ran: %+v", got)
	}
}

// The paths that are neither the registry nor a ledger file, as ledger.files
// lays the ledger out; with no ledger section no path is one, and a path is
// cleaned as git gives it.
func TestOutsideBookkeeping(t *testing.T) {
	s := newScene()
	cfg := configOf(t, s.config)
	files := []string{"widget.md", "work-items.yaml", "tasks/phase-2.yaml", "tasks/notes.yaml", "tasks/sub/phase-1.yaml", "other/phase-1.yaml", "tasks/phase-1.yml"}
	want := []string{"widget.md", "tasks/notes.yaml", "tasks/sub/phase-1.yaml", "other/phase-1.yaml", "tasks/phase-1.yml"}
	if got, ledgers := OutsideBookkeeping(cfg, files); !slices.Equal(got, want) || !ledgers {
		t.Errorf("OutsideBookkeeping = %q, %v, want %q", got, ledgers, want)
	}
	if got, ledgers := OutsideBookkeeping(cfg, []string{"./work-items.yaml", "tasks/./phase-1.yaml"}); len(got) != 0 || !ledgers {
		t.Errorf("paths to clean: %q, %v", got, ledgers)
	}
	if got, ledgers := OutsideBookkeeping(cfg, []string{"work-items.yaml"}); len(got) != 0 || ledgers {
		t.Errorf("the registry alone: %q, %v", got, ledgers)
	}
	if got, ledgers := OutsideBookkeeping(cfg, nil); len(got) != 0 || ledgers {
		t.Errorf("no paths: %q, %v", got, ledgers)
	}
	bare := configOf(t, strings.NewReplacer("ledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n", "",
		"    Task: { source: ledger }\n", "").Replace(configText))
	if got, ledgers := OutsideBookkeeping(bare, []string{"work-items.yaml", "tasks/phase-1.yaml"}); !slices.Equal(got, []string{"tasks/phase-1.yaml"}) || ledgers {
		t.Errorf("with no ledger section: %q, %v", got, ledgers)
	}
}

func configOf(t *testing.T, text string) *config.Loaded {
	t.Helper()
	v := &vcsfake.VCS{Located: map[string]vcsport.Location{"": {Top: "/r", GitDir: ".git", Common: ".git"}}}
	trees := treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": base}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(v, trees, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// Whether the task's checks are the same at the head and at the landing
// commit (bug 137): in every key the ledger reads of a check, as many entries
// as there are, and no entry whose checks cannot be read; the task's title,
// type and why, another task's entries and the file the entry is in are not
// compared.
func TestSameChecks(t *testing.T) {
	timed := `{ run: "true", cost: static, timeout: 5 }`
	late := `{ run: "exit 1", cost: late }`
	for name, c := range map[string]struct {
		head, landing map[string]string
		same          bool
	}{
		"no entry at either":                        {ledgerOf(entry("T-2", checked)), ledgerOf(entry("T-2", checked)), true},
		"the same checks":                           {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", checked)), true},
		"no checks at either":                       {ledgerOf(entry("T-1")), ledgerOf(entry("T-1")), true},
		"an empty list and none":                    {ledgerOf(entry("T-1") + "  done_when: []\n"), ledgerOf(entry("T-1")), true},
		"another title, type and why":               {ledgerOf(entry("T-1", checked)), ledgerOf("- { id: T-1, type: fix, title: Other, why: Why, done_when: [" + checked + "] }\n"), true},
		"another task's checks":                     {ledgerOf(entry("T-1", checked) + entry("T-2")), ledgerOf(entry("T-1", checked) + entry("T-2", late)), true},
		"another task added":                        {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", checked) + entry("T-3", late)), true},
		"moved to another file":                     {ledgerOf(entry("T-1", checked), entry("T-2")), ledgerOf(entry("T-2"), entry("T-1", checked)), true},
		"gone and added with the same":              {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-9") + entry("T-1", checked)), true},
		"a check added":                             {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", checked, late)), false},
		"a check added to an entry with none":       {ledgerOf(entry("T-1")), ledgerOf(entry("T-1", checked)), false},
		"a check removed":                           {ledgerOf(entry("T-1", checked, late)), ledgerOf(entry("T-1", checked)), false},
		"the checks reordered":                      {ledgerOf(entry("T-1", checked, late)), ledgerOf(entry("T-1", late, checked)), false},
		"the entry gone":                            {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-2", checked)), false},
		"the entry new":                             {ledgerOf(entry("T-2", checked)), ledgerOf(entry("T-1", checked)), false},
		"gone and added with other checks":          {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-9") + entry("T-1", `{ run: "false", cost: static }`)), false},
		"a second entry in another file":            {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", checked), entry("T-1", checked)), false},
		"a second entry at the head":                {ledgerOf(entry("T-1", checked), entry("T-1", checked)), ledgerOf(entry("T-1", checked)), false},
		"another command":                           {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "false", cost: static }`)), false},
		"fails for run":                             {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ fails: "true", cost: static }`)), false},
		"another fails":                             {ledgerOf(entry("T-1", `{ fails: "a" }`)), ledgerOf(entry("T-1", `{ fails: "b" }`)), false},
		"after alone":                               {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: static, after: push }`)), false},
		"timeout alone":                             {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", timed)), false},
		"another timeout":                           {ledgerOf(entry("T-1", timed)), ledgerOf(entry("T-1", `{ run: "true", cost: static, timeout: 6 }`)), false},
		"cost alone":                                {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: late }`)), false},
		"prose alone":                               {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: static, prose: true }`)), false},
		"evidence alone":                            {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: static, evidence: { inputs: [a] } }`)), false},
		"other evidence":                            {ledgerOf(entry("T-1", `{ run: "true", evidence: { inputs: [a] } }`)), ledgerOf(entry("T-1", `{ run: "true", evidence: { inputs: [b] } }`)), false},
		"snapshot alone":                            {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: static, snapshot: false }`)), false},
		"checks that cannot be read at the head":    {ledgerOf(entry("T-1") + "  done_when: nope\n"), ledgerOf(entry("T-1")), false},
		"checks that cannot be read at the landing": {ledgerOf(entry("T-1")), ledgerOf(entry("T-1") + "  done_when: nope\n"), false},
		"a check with no command at both":           {ledgerOf(entry("T-1", `{ cost: late }`)), ledgerOf(entry("T-1", `{ cost: late }`)), false},
	} {
		cfg := configOf(t, configText)
		cfg.Trees = treefake.Trees{Common: ".git", Files: map[string]map[string]string{"head": c.head, "landing": c.landing}}
		if same, unreadable := SameChecks(t.Context(), cfg, "T-1", "head", "landing"); same != c.same || unreadable != "" {
			t.Errorf("%s: SameChecks = %v, %q, want %v", name, same, unreadable, c.same)
		}
	}
	good := ledgerOf(entry("T-1", checked))
	for name, c := range map[string]struct {
		head, landing map[string]string
		at            string
	}{
		"not YAML at the head":      {ledgerOf("- [\n"), good, "head"},
		"not a list at the head":    {ledgerOf("a: b\n"), good, "head"},
		"no folder at the head":     {nil, good, "head"},
		"not YAML at the landing":   {good, ledgerOf("- [\n"), "landing"},
		"not a list at the landing": {good, ledgerOf("a: b\n"), "landing"},
		"no folder at the landing":  {good, nil, "landing"},
		"both":                      {ledgerOf("a: b\n"), ledgerOf("a: b\n"), "head"},
	} {
		cfg := configOf(t, configText)
		cfg.Trees = treefake.Trees{Common: ".git", Files: map[string]map[string]string{"head": c.head, "landing": c.landing}}
		if same, unreadable := SameChecks(t.Context(), cfg, "T-1", "head", "landing"); same || unreadable != c.at {
			t.Errorf("%s: SameChecks = %v, %q, want it unreadable at %q", name, same, unreadable, c.at)
		}
	}
	cfg := configOf(t, configText)
	cfg.Trees = treefake.Trees{}
	if same, unreadable := SameChecks(t.Context(), cfg, "T-1", "head", "landing"); same || unreadable != "head" {
		t.Errorf("trees the repository cannot give: %v, %q", same, unreadable)
	}
	cfg.Trees = treefake.Trees{Common: ".git", Files: map[string]map[string]string{"head": good}}
	if same, unreadable := SameChecks(t.Context(), cfg, "T-1", "head", "landing"); same || unreadable != "landing" {
		t.Errorf("a landing commit with no files: %v, %q", same, unreadable)
	}
}

// The fetch names the pull request's ref and no destination, never prompts,
// and leaves the clone as it was when it fails.
func TestFetchHead(t *testing.T) {
	var got []string
	var talk vcsport.Talk
	var left time.Duration
	v := broken{VCS: &vcsfake.VCS{}, fetch: func(ctx context.Context, remote, refspec string, tk vcsport.Talk) error {
		got, talk = append(got, remote, refspec), tk
		d, _ := ctx.Deadline()
		left = time.Until(d)
		return errors.New("refused")
	}}
	FetchHead(t.Context(), v, "upstream", 12)
	if !slices.Equal(got, []string{"upstream", "refs/pull/12/head"}) || !talk.NoPrompt || left <= 0 || left > 20*time.Second {
		t.Errorf("FetchHead fetched %v, %+v, within %v", got, talk, left)
	}
}

// The SHAs a line names are abbreviated to seven characters, in the trust's
// line and in the not trusted one alike.
func TestJudgeAbbreviatesTheCommitsItNames(t *testing.T) {
	long := func(s *scene) {
		name := func(n string) string { return n + "0123456789abcdef" }
		for i := range s.history {
			s.history[i].Name = name(s.history[i].Name)
			for j, p := range s.history[i].Parents {
				s.history[i].Parents[j] = name(p)
			}
		}
		s.from, s.to = name(s.from), name(s.to)
		s.pulls[0].MergeCommit, s.pulls[0].Head = name("r1"), name("h1")
	}
	s := newScene()
	long(s)
	got := s.judge(t)
	if got.Trust == nil || got.Trust.Head != "h10123456789abcdef" ||
		!strings.HasPrefix(got.Trust.Line, "Trusted: the pull request "+pullURL+" merged its head h101234 as r101234, the same commits") {
		t.Errorf("trusted: %+v", got)
	}
	s = newScene()
	long(s)
	s.run.Conclusion = "failure"
	if got := s.judge(t); got.Note != "Not trusted: the pull request "+pullURL+" merged its head h101234 as r101234, but the head's CI run u-head has not passed." {
		t.Errorf("not trusted: %+v", got)
	}
	s = newScene()
	long(s)
	s.history[2].Tree = tree(s.history[2].Tree, files("src/app.go", "c\n"))
	if got := s.judge(t); !strings.HasSuffix(got.Note, "but r101234 differs from the pull request's commit h101234 in src/app.go.") {
		t.Errorf("a commit that differs: %+v", got)
	}
	s = newScene()
	long(s)
	s.history[1].Tree = tree(s.history[1].Tree, files("src/other.go", "o\n"))
	if got := s.judge(t); !strings.HasSuffix(got.Note, "but the range holds o101234, which is not the pull request's and changes src/other.go.") {
		t.Errorf("a commit of main's: %+v", got)
	}
}
