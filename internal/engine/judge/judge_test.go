package judge

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// good is a commit commits.since may name, a full SHA.
var good = strings.Repeat("ab", 20)

// The rules every test judges by: docs may touch Markdown alone.
const rules = `version: 1
commits:
  types: [feat, docs]
  scopes: { feat: {}, docs: { only: ["**/*.md"] } }
  header_lint: { use: builtin }
`

// A history: a root, a docs commit that holds, one touching code that does
// not, a side branch, a merge of it with no change of its own and one, with
// no type, with one.
func history() *vcsfake.VCS {
	root := map[string]string{"README.md": "a\n"}
	added := map[string]string{"README.md": "a\n", "a.md": "a\n"}
	bad := map[string]string{"README.md": "a\n", "a.md": "a\n", "a.go": "package a\n"}
	side := map[string]string{"README.md": "a\n", "b.md": "b\n"}
	merged := map[string]string{"README.md": "a\n", "a.md": "a\n", "a.go": "package a\n", "b.md": "b\n"}
	own := map[string]string{"README.md": "a\n", "a.md": "a\n", "a.go": "package a\n", "b.md": "b\n", "c.md": "c\n"}
	return &vcsfake.VCS{History: []vcsfake.Commit{
		{Name: "root", Message: "docs: start\n", Tree: root},
		{Name: good, Parents: []string{"root"}, Message: "docs: add a\n", Tree: added},
		{Name: "bad1234567", Parents: []string{good}, Message: "docs: add code\n", Tree: bad},
		{Name: "side", Parents: []string{"root"}, Message: "docs: add b\n", Tree: side},
		{Name: "merged", Parents: []string{"bad1234567", "side"}, Message: "Merge branch 'side'\n", Tree: merged},
		{Name: "own", Parents: []string{"merged", "side"}, Message: "Merge branch 'side'\n", Tree: own},
	}}
}

// The config text over repo and an empty working tree.
func configOver(t *testing.T, repo vcsport.VCS, text string) *config.Loaded {
	t.Helper()
	trees := treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": {}}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(repo, trees, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// Over judges each commit of the range: one that holds, one whose paths its
// type may not touch named under the rejection, a merge with no change of
// its own passing, and one with changes of its own refused for its untyped
// first line; the count goes to stdout and the rejections to stderr.
func TestOver(t *testing.T) {
	repo := history()
	cfg := configOver(t, repo, rules)
	var stdout, stderr strings.Builder
	run, err := Over(t.Context(), cfg, repo, "root", "own", Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatal(err)
	}
	want := []Verified{
		{SHA: good, Header: "docs: add a", OK: true},
		{SHA: "bad1234567", Header: "docs: add code", OK: false},
		{SHA: "side", Header: "docs: add b", OK: true},
		{SHA: "merged", Header: "Merge branch 'side'", OK: true},
		{SHA: "own", Header: "Merge branch 'side'", OK: false},
	}
	if !reflect.DeepEqual(run.Results, want) || run.Passed != 3 || !run.Ranged || run.OK() || run.Since != "" {
		t.Errorf("got %+v", run)
	}
	if stdout.String() != "3/5 commits pass the commit rules\n" {
		t.Errorf("stdout: %q", stdout.String())
	}
	for _, line := range []string{"  ^ bad1234 docs: add code\n", "  ^ own Merge branch 'side'\n", "a.go", "(c.md)"} {
		if !strings.Contains(stderr.String(), line) {
			t.Errorf("stderr lacks %q:\n%s", line, stderr.String())
		}
	}
}

// OverUnpushed judges, of the range, the commits no remote branch has
// (slice-213): the side commit a remote branch has, which the merge brings in,
// is left out, and so are commits.since and its ancestors; the range commands
// still start at from, or at commits.since when it is past from.
func TestOverUnpushed(t *testing.T) {
	repo := history()
	repo.Named = map[string]string{"refs/remotes/origin/main": "side"}
	cfg := configOver(t, repo, rules+"  since: "+good+"\n"+`tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    range_checks: [{ name: from, range: "echo from {from}" }]
`)
	var stdout strings.Builder
	run, err := OverUnpushed(t.Context(), cfg, repo, "root", "merged", Out{Stdout: &stdout, Stderr: &strings.Builder{}})
	if err != nil {
		t.Fatal(err)
	}
	want := []Verified{{SHA: "bad1234567", Header: "docs: add code", OK: false}, {SHA: "merged", Header: "Merge branch 'side'", OK: true}}
	if !reflect.DeepEqual(run.Results, want) || run.Passed != 1 {
		t.Errorf("got %+v", run)
	}
	if want := "Not checked: abababa (commits.since) and its ancestors\n1/2 commits pass the commit rules\nfrom " + good + "\n"; stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
}

// Under --json the logs go to stderr, and commits.since and its ancestors
// are said and left out.
func TestOverSinceUnderJSON(t *testing.T) {
	repo := history()
	cfg := configOver(t, repo, rules+"  since: "+good+"\n")
	var stdout, stderr strings.Builder
	run, err := Over(t.Context(), cfg, repo, "", "bad1234567", Out{Stdout: &stdout, Stderr: &stderr, JSON: true})
	if err != nil {
		t.Fatal(err)
	}
	if run.Since != good || len(run.Results) != 1 || run.Results[0].SHA != "bad1234567" || run.OK() {
		t.Errorf("got %+v", run)
	}
	if stdout.String() != "" || !strings.HasPrefix(stderr.String(), "Not checked: abababa (commits.since) and its ancestors\n") ||
		!strings.Contains(stderr.String(), "0/1 commits pass the commit rules\n") {
		t.Errorf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

// Each kind's range command runs once over the range, up to the first that
// fails, which fails the run however its commits did.
func TestOverRangeCommands(t *testing.T) {
	repo := history()
	text := rules + `tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    range_checks:
      - { name: first, range: "echo first {to}" }
      - { name: second, range: "echo second; exit 1" }
      - { name: third, range: "echo third" }
`
	cfg := configOver(t, repo, text)
	var stdout strings.Builder
	run, err := Over(t.Context(), cfg, repo, "root", good, Out{Stdout: &stdout, Stderr: &strings.Builder{}})
	if err != nil {
		t.Fatal(err)
	}
	if run.Ranged || run.OK() || run.Passed != 1 {
		t.Errorf("got %+v", run)
	}
	if want := "1/1 commits pass the commit rules\nfirst " + good + "\nsecond\n"; stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
}

// A message that fails is not judged further, and is named.
func TestOverRefusedMessage(t *testing.T) {
	repo := history()
	repo.History = append(repo.History, vcsfake.Commit{Name: "odd", Parents: []string{good}, Message: "chore: untyped\n", Tree: map[string]string{"x.md": "x\n"}})
	cfg := configOver(t, repo, rules)
	var stderr strings.Builder
	run, err := Over(t.Context(), cfg, repo, good, "odd", Out{Stdout: &strings.Builder{}, Stderr: &stderr})
	if err != nil || len(run.Results) != 1 || run.Results[0].OK {
		t.Fatalf("got %+v, %v", run, err)
	}
	if !strings.HasSuffix(stderr.String(), "  ^ odd chore: untyped\n") {
		t.Errorf("stderr %q", stderr.String())
	}
}

// broken is the repository with one of its reads failing for one commit.
type broken struct {
	*vcsfake.VCS
	at, read string
}

var errRead = errors.New("cannot read")

func (b broken) fails(read, sha string) bool { return b.read == read && b.at == sha }

func (b broken) Show(ctx context.Context, sha string) (vcsport.Entry, error) {
	if b.fails("show", sha) {
		return vcsport.Entry{}, errRead
	}
	return b.VCS.Show(ctx, sha)
}

func (b broken) Parents(ctx context.Context, sha string) ([]string, error) {
	if b.fails("parents", sha) {
		return nil, errRead
	}
	return b.VCS.Parents(ctx, sha)
}

func (b broken) OwnPaths(ctx context.Context, sha string) ([]string, error) {
	if b.fails("own", sha) {
		return nil, errRead
	}
	return b.VCS.OwnPaths(ctx, sha)
}

func (b broken) CommitPaths(ctx context.Context, sha string) ([]string, error) {
	if b.fails("paths", sha) {
		return nil, errRead
	}
	return b.VCS.CommitPaths(ctx, sha)
}

// A read of the repository that fails stops the run with its error: the
// range's commits, a commit's message, its parents and its paths.
func TestOverReadFails(t *testing.T) {
	repo := history()
	cfg := configOver(t, repo, rules)
	o := Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}
	if _, err := Over(t.Context(), cfg, repo, "root", "no-such-commit", o); err == nil {
		t.Error("a range that cannot be read gave no error")
	}
	for _, b := range []broken{
		{repo, good, "show"}, {repo, good, "parents"}, {repo, good, "paths"}, {repo, "merged", "own"},
	} {
		if _, err := Over(t.Context(), cfg, b, "root", "merged", o); !errors.Is(err, errRead) {
			t.Errorf("%s of %s failing: %v", b.read, b.at, err)
		}
	}
}

// A glob the rules cannot read, a path's or the moves rule's, is an error,
// the commit not judged.
func TestOverRulesFail(t *testing.T) {
	repo := history()
	for name, text := range map[string]string{
		"paths": "version: 1\ncommits:\n  types: [docs]\n  scopes: { docs: { only: [\"?x\"] } }\n",
		"moves": `version: 1
commits:
  types: [docs]
tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    range_checks: [{ name: moves, builtin: moves, code: ["?x"] }]
`,
	} {
		cfg := configOver(t, repo, text)
		if _, err := Over(t.Context(), cfg, repo, "root", good, Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}); err == nil {
			t.Errorf("%s: a glob that cannot be read gave no error", name)
		}
	}
}

// A footer whose source cannot be read stops the run with its error.
func TestOverFooterFails(t *testing.T) {
	repo := history()
	repo.History = append(repo.History, vcsfake.Commit{Name: "named", Parents: []string{good}, Message: "docs: x\n\nItem: p1-x\n", Tree: map[string]string{"x.md": "x\n"}})
	cfg := configOver(t, repo, rules+"  footers:\n    Item: { source: registry, validate_for: all, read_at: commit }\n")
	if _, err := Over(t.Context(), cfg, repo, good, "named", Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}); err == nil {
		t.Error("a footer whose source cannot be read gave no error")
	}
}

// Under a stealth config a commit's footers are its note's.
func TestOverStealth(t *testing.T) {
	repo := history()
	repo.History[1].Note = "Scenarios: @ID-A-01\n"
	cfg := configOver(t, repo, rules)
	cfg.Stealth = true
	run, err := Over(t.Context(), cfg, repo, "root", good, Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}})
	if err != nil || !run.OK() {
		t.Errorf("got %+v, %v", run, err)
	}
}

// A config with no commits section sets no path rules to judge by: an
// error once a message holds.
func TestOverNoCommitRules(t *testing.T) {
	repo := history()
	cfg := configOver(t, repo, "version: 1\n")
	if _, err := Over(t.Context(), cfg, repo, "root", good, Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}); err == nil {
		t.Error("no commits section gave no error")
	}
}

// A merge with no type names its own paths, the first three and a count of
// the rest; any other commit, or a typed merge, has no such problem.
func TestUntypedMerge(t *testing.T) {
	cfg := configOver(t, &vcsfake.VCS{}, rules)
	got := UntypedMerge(cfg, Judged{Type: "Merge", Merge: true, Files: []string{"a", "b", "c", "d", "e"}})
	if len(got) != 1 || got[0].Rule != "merge-type" ||
		!strings.Contains(got[0].Message, "(a, b, c and 2 more)") {
		t.Errorf("got %+v", got)
	}
	got = UntypedMerge(cfg, Judged{Type: "Merge", Merge: true, Files: []string{"a"}})
	if len(got) != 1 || !strings.Contains(got[0].Message, "(a) needs") {
		t.Errorf("got %+v", got)
	}
	for _, c := range []Judged{{Type: "docs", Merge: true, Files: []string{"a"}}, {Type: "nothing", Files: []string{"a"}}} {
		if got := UntypedMerge(cfg, c); got != nil {
			t.Errorf("%+v: %+v", c, got)
		}
	}
}

// The moves rule's change is the IDs the footers name, by kind of named
// tests, beside the paths.
func TestMovesChange(t *testing.T) {
	cfg := configOver(t, &vcsfake.VCS{}, `version: 1
commits:
  types: [test]
  footers:
    Scenarios: { source: { tests: scenario }, strip_prefix: "@" }
tests:
  scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
`)
	got := MovesChange(cfg, "test: x\n\nScenarios: @ID-A-01, @ID-A-02\n", []string{"a.go"})
	want := tests.Change{Named: map[string][]string{"scenario": {"ID-A-01", "ID-A-02"}}, Paths: []string{"a.go"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := MovesChange(cfg, "test: x\n", nil); len(got.Named) != 0 || got.Named == nil {
		t.Errorf("no footers: %+v", got)
	}
}
