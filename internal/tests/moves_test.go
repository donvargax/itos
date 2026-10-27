package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

var movesOptions = Options{Root: "features", ID: `ID-[A-Z]+-\d+`, TagPrefix: "@", WipTag: "@wip"}

func featureSet(t *testing.T, files map[string]string) *FeatureSet {
	t.Helper()
	set, err := ReadFeatures(files, movesOptions)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

const base = "Feature: A\n\n  @ID-A-01\n  Scenario: Opens\n    When I open it\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it\n"

// The cases moves.yaml gives the commit-msg hook, judged by the comparison
// it calls.
func TestMoveProblems(t *testing.T) {
	before := featureSet(t, map[string]string{"features/a.feature": base})
	renames := map[string]string{"ID-A-01": "Opens again"}
	for _, c := range []struct {
		name  string
		after map[string]string
		want  []string
	}{
		{"a live scenario's steps changed",
			map[string]string{"features/a.feature": "Feature: A\n\n  @ID-A-01\n  Scenario: Opens\n    When I open it twice\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it\n"},
			[]string{"changes the live scenario ID-A-01 in features/a.feature: a moved scenario keeps its ID, name, tags and steps exactly"}},
		{"moved unchanged to a new file with its own header",
			map[string]string{"features/b.feature": "@phase-2\nFeature: B\n\n  Background:\n    Given it is closed\n\n  @ID-A-01\n  Scenario: Opens\n    When I open it\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it\n"},
			nil},
		{"wip scenarios changed and a comment written",
			map[string]string{"features/a.feature": "Feature: A\n\n  # why it opens\n  @ID-A-01\n  Scenario: Opens\n    When I open it\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it at once\n\n  @ID-A-03 @wip\n  Scenario: Locks\n    When I lock it\n"},
			nil},
		{"an allowed rename",
			map[string]string{"features/a.feature": "Feature: A\n\n  @ID-A-01\n  Scenario: Opens again\n    When I open it\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it\n"},
			nil},
		{"a rename not allowed",
			map[string]string{"features/a.feature": "Feature: A\n\n  @ID-A-01\n  Scenario: Opens at last\n    When I open it\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it\n"},
			[]string{"changes the live scenario ID-A-01 in features/a.feature: a moved scenario keeps its ID, name, tags and steps exactly"}},
		{"a live scenario turned wip",
			map[string]string{"features/a.feature": "Feature: A\n\n  @ID-A-01 @wip\n  Scenario: Opens\n    When I open it\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it\n"},
			[]string{"turns the live scenario ID-A-01 wip in features/a.feature"}},
		{"moved into a wip file",
			map[string]string{"features/b.feature": "@wip\nFeature: B\n\n  @ID-A-01\n  Scenario: Opens\n    When I open it\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it\n"},
			[]string{"turns the live scenario ID-A-01 wip in features/b.feature"}},
		{"added, lost and a header changed",
			map[string]string{"features/a.feature": "Feature: A, retitled\n\n  @ID-A-03\n  Scenario: Locks\n    When I lock it\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it\n"},
			[]string{
				"brings the scenario ID-A-03 live in features/a.feature, which needs the check's code set to be allowed outside except_types",
				"loses the scenario ID-A-01 of features/a.feature",
				"changes the header or Background of features/a.feature",
			}},
	} {
		got := MoveProblems(before, featureSet(t, c.after), renames, Live{})
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// A feature's description (slice 125), the lines under its Feature line up
// to a tag line or a Background, Rule, Scenario or Example keyword, blank ones
// included, is dropped; the tags and Feature line above it and all from its
// end on are kept, a later line beginning Feature: (in a doc string) included.
func TestWithoutDescription(t *testing.T) {
	for _, c := range []struct{ name, header, want string }{
		{"no Feature line", "  Some text\n\n  More", "  Some text\n\n  More"},
		{"tags and the Feature line kept", "@phase-1 @wip\nFeature: A\n  Prose,\n\n  more prose.\n", "@phase-1 @wip\nFeature: A"},
		{"up to the Background", "Feature: A\n  Prose.\n\n  Background:\n    Given it is set up",
			"Feature: A\n  Background:\n    Given it is set up"},
		{"up to a Rule", "Feature: A\n  Prose.\n  Rule: R\n    Prose of the rule.", "Feature: A\n  Rule: R\n    Prose of the rule."},
		{"up to an untagged scenario", "Feature: A\n  Prose.\n  Scenario: S\n    When it runs", "Feature: A\n  Scenario: S\n    When it runs"},
		{"up to an outline", "Feature: A\n  Prose.\n  Scenario Outline: S", "Feature: A\n  Scenario Outline: S"},
		{"up to a template", "Feature: A\n  Prose.\n  Scenario Template: S", "Feature: A\n  Scenario Template: S"},
		{"up to an example", "Feature: A\n  Prose.\n  Example: S", "Feature: A\n  Example: S"},
		{"up to a tag line", "Feature: A\n  Prose.\n  @smoke\n  Scenario: S", "Feature: A\n  @smoke\n  Scenario: S"},
		{"a Feature line in the Background's doc string is not a second description",
			"Feature: A\n  Prose.\n  Background:\n    Given the file:\n      \"\"\"\n      Feature: B\n        Its prose.\n      \"\"\"",
			"Feature: A\n  Background:\n    Given the file:\n      \"\"\"\n      Feature: B\n        Its prose.\n      \"\"\""},
	} {
		if got := withoutDescription(c.header); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// A description reworded is no problem; the feature renamed, a tag added
// above it, or its Background changed beside the rewording, each is.
func TestMoveProblemsDescription(t *testing.T) {
	const scenarios = "\n\n  @ID-A-01\n  Scenario: Opens\n    When I open it\n"
	before := featureSet(t, map[string]string{"features/a.feature": "Feature: A\n  It opens." + scenarios})
	changed := []string{"changes the header or Background of features/a.feature"}
	for _, c := range []struct {
		name, after string
		want        []string
	}{
		{"reworded", "Feature: A\n  It opens,\n\n  told another way." + scenarios, nil},
		{"left out", "Feature: A" + scenarios, nil},
		{"renamed", "Feature: B\n  It opens." + scenarios, changed},
		{"tagged", "@phase-2\nFeature: A\n  It opens, reworded." + scenarios, changed},
		{"a Background", "Feature: A\n  It opens, reworded.\n\n  Background:\n    Given it is shut" + scenarios, changed},
	} {
		got := MoveProblems(before, featureSet(t, map[string]string{"features/a.feature": c.after}), nil, Live{})
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// A file tagged wip above its Feature line is wip throughout, and a scenario
// ID written twice keeps its first place and its last block.
func TestReadFeatures(t *testing.T) {
	set := featureSet(t, map[string]string{
		"features/b.feature": "@wip\nFeature: B\n\n  @ID-B-01\n  Scenario: One\n",
		"features/a.feature": "Feature: A\n\n  @ID-A-01\n  Scenario: First\n\n  @ID-A-02\n  Scenario: Two\n\n  @ID-A-01\n  Scenario: Again\n",
	})
	if want := []string{"features/a.feature", "features/b.feature"}; !reflect.DeepEqual(set.Files, want) {
		t.Errorf("files %q, want %q", set.Files, want)
	}
	if want := []string{"ID-A-01", "ID-A-02", "ID-B-01"}; !reflect.DeepEqual(set.IDs, want) {
		t.Errorf("ids %q, want %q", set.IDs, want)
	}
	if got := set.Scenarios["ID-A-01"].Body; got != "  @ID-A-01\n  Scenario: Again" {
		t.Errorf("ID-A-01's block %q", got)
	}
	if !set.Scenarios["ID-B-01"].Wip || set.Scenarios["ID-A-02"].Wip {
		t.Errorf("wip: %+v", set.Scenarios)
	}
}

// A merge is judged by its own changes alone (bug 30): the live scenario its
// topic changed is the topic's, judged in its own commit, and a merge's own
// paths are judged against its first parent, so a wip scenario it changes
// passes and a live one, among its own paths, does not.
func TestMovesMerge(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Chdir(dir)
	gitBin := git.Bin(runtime.GOOS)
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command(gitBin, append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	other := strings.ReplaceAll(base, "ID-A-", "ID-B-")
	write("features/a.feature", base)
	write("features/b.feature", other)
	git("add", ".")
	git("commit", "-q", "-m", "feat: start")
	git("checkout", "-q", "-b", "topic")
	write("features/a.feature", strings.Replace(base, "open it", "open it twice", 1))
	git("commit", "-q", "-am", "feat: open twice")
	git("checkout", "-q", "main")
	write("main.md", "main\n")
	git("add", "main.md")
	git("commit", "-q", "-m", "docs: the main line")
	first := git("rev-parse", "HEAD")
	git("merge", "-q", "--no-ff", "--no-commit", "topic")
	write("features/b.feature", strings.Replace(other, "close it", "close it now", 1))
	git("add", "features/b.feature")
	git("commit", "-q", "-m", "test: merge the topic")
	cfg := load(t, `version: 1
commits: { types: [feat, test, docs] }
tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    tag_prefix: "@"
    range_checks: [{ name: moves, builtin: moves, except_types: [feat] }]
`)
	moves := NewMoves(cfg, nil)
	found, err := moves.Merge("test", first, "HEAD", nil, Change{})
	if err != nil || len(found) != 0 {
		t.Fatalf("a merge with no own change judged by what its topic brings: %v, %v", found, err)
	}
	found, err = moves.Merge("test", first, "HEAD", []string{"features/b.feature"}, Change{})
	if err != nil || len(found) != 0 {
		t.Fatalf("a merge whose own change is to a wip scenario: %v, %v", found, err)
	}
	found, err = moves.Merge("test", first, "HEAD", []string{"features/a.feature"}, Change{})
	if err != nil || len(found) != 1 || !strings.Contains(found[0].Message, "a test commit changes the live scenario ID-A-01") {
		t.Fatalf("a merge whose own change is to a live scenario: %v, %v", found, err)
	}
}

// A scenario going live (slice 134), a wip one made live or a new live one,
// is live's to judge: refused with why when the check has no code, when no
// message was read, when the footer does not name it or the commit changes
// the code under test; "" when it may.
func TestLiveProblem(t *testing.T) {
	what := "the scenario ID-C-01 live in features/c.feature"
	named := map[string]bool{"ID-C-01": true}
	for _, c := range []struct {
		name string
		live Live
		want string
	}{
		{"no code", Live{Known: true, Named: named, Footer: "Scenarios:"},
			"brings " + what + ", which needs the check's code set to be allowed outside except_types"},
		{"no message read", Live{Code: true, Footer: "Scenarios:"},
			"brings " + what + ", which a commit may by naming it in Scenarios: and changing none of the check's code"},
		{"named, no code touched", Live{Code: true, Known: true, Named: named, Footer: "Scenarios:"}, ""},
		{"unnamed", Live{Code: true, Known: true, Footer: "Scenarios:"}, "brings " + what + " without naming it in Scenarios:"},
		{"named, code touched", Live{Code: true, Known: true, Named: named, Touched: []string{"src/a.go", "src/b.go"}, Footer: "Scenarios:"},
			"brings " + what + " while changing the code under test: src/a.go, src/b.go"},
		{"unnamed, code touched", Live{Code: true, Known: true, Touched: []string{"src/a.go"}, Footer: "Scenarios:"},
			"brings " + what + " without naming it in Scenarios:, and while changing the code under test: src/a.go"},
	} {
		if got := c.live.Problem("ID-C-01", what); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// A wip scenario made live, its steps changed too or its file's @wip taken
// off, and a new live one, each go live; a live scenario changed does not,
// whatever live allows.
func TestMoveProblemsGoingLive(t *testing.T) {
	before := featureSet(t, map[string]string{
		"features/a.feature": base,
		"features/b.feature": "@wip\nFeature: B\n\n  @ID-B-01\n  Scenario: One\n    When it runs\n",
	})
	after := featureSet(t, map[string]string{
		"features/a.feature": "Feature: A\n\n  @ID-A-01\n  Scenario: Opens\n    When I open it twice\n\n  @ID-A-02\n  Scenario: Closes\n    When I close it now\n\n  @ID-A-03\n  Scenario: Locks\n",
		"features/b.feature": "Feature: B\n\n  @ID-B-01\n  Scenario: One\n    When it runs\n",
	})
	allowed := Live{Code: true, Known: true, Named: map[string]bool{"ID-A-01": true, "ID-A-02": true, "ID-A-03": true, "ID-B-01": true}}
	want := []string{
		"changes the live scenario ID-A-01 in features/a.feature: a moved scenario keeps its ID, name, tags and steps exactly",
		"changes the header or Background of features/b.feature",
	}
	if got := MoveProblems(before, after, nil, allowed); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	unnamed := Live{Code: true, Known: true, Footer: "Scenarios:"}
	want = []string{
		"changes the live scenario ID-A-01 in features/a.feature: a moved scenario keeps its ID, name, tags and steps exactly",
		"brings the scenario ID-A-02 live in features/a.feature without naming it in Scenarios:",
		"brings the scenario ID-A-03 live in features/a.feature without naming it in Scenarios:",
		"brings the scenario ID-B-01 live in features/b.feature without naming it in Scenarios:",
		"changes the header or Background of features/b.feature",
	}
	if got := MoveProblems(before, after, nil, unnamed); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A check's judgement of a commit (slice 134): the IDs the kind's footers
// name, the paths of the check's code among those it changes, and the
// footer a message names the kind's tests by, or the phrase for a kind no
// footer names; a glob JavaScript would refuse is an error, reached.
func TestMovesLive(t *testing.T) {
	cfg := load(t, `version: 1
commits:
  types: [feat, test]
  path_sets: { app: ["src/**"] }
  footers:
    Scenarios: { source: { tests: scenario }, strip_prefix: "@" }
tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    range_checks: [{ name: moves, builtin: moves, except_types: [feat], code: [$app, "go.mod"] }]
  other:
    root: other
    id: "ID-[A-Z]+-\\d+"
    range_checks: [{ name: moves, builtin: moves, except_types: [feat], code: ["?x"] }]
`)
	m := NewMoves(cfg, nil)
	checks := m.checks()
	got, err := m.live(checks[0], Change{
		Named: map[string][]string{"scenario": {"ID-C-01"}, "other": {"ID-D-01"}},
		Paths: []string{"features/c.feature", "src/app.go", "go.mod"},
	})
	want := Live{Code: true, Known: true, Named: map[string]bool{"ID-C-01": true}, Touched: []string{"src/app.go", "go.mod"}, Footer: "Scenarios:"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, %v, want %+v", got, err, want)
	}
	if _, err := m.live(checks[1], Change{Paths: []string{"x"}}); err == nil {
		t.Error("a glob JavaScript refuses gave no error")
	}
	if got := m.footer("other"); got != "a footer whose source is { tests: other }" {
		t.Errorf("a kind no footer names: %q", got)
	}
	bad := NewMoves(load(t, `version: 1
commits: { types: [feat, test] }
tests:
  other:
    root: other
    id: "ID-[A-Z]+-\\d+"
    range_checks: [{ name: moves, builtin: moves, except_types: [feat], code: ["?x"] }]
`), nil)
	if _, err := bad.Between("test", "HEAD", "index", Change{Paths: []string{"x"}}); err == nil {
		t.Error("the rule judged a commit by a glob JavaScript refuses")
	}
}
