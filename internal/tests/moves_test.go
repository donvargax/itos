package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
		{"added, lost and a header changed",
			map[string]string{"features/a.feature": "Feature: A, retitled\n\n  @ID-A-03\n  Scenario: Locks\n    When I lock it\n\n  @ID-A-02 @wip\n  Scenario: Closes\n    When I close it\n"},
			[]string{
				"adds the live scenario ID-A-03 to features/a.feature",
				"loses the scenario ID-A-01 of features/a.feature",
				"changes the header or Background of features/a.feature",
			}},
	} {
		got := MoveProblems(before, featureSet(t, c.after), renames)
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
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Chdir(dir)
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
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
	moves := NewMoves(cfg)
	found, err := moves.Merge("test", first, "HEAD", nil)
	if err != nil || len(found) != 0 {
		t.Fatalf("a merge with no own change judged by what its topic brings: %v, %v", found, err)
	}
	found, err = moves.Merge("test", first, "HEAD", []string{"features/b.feature"})
	if err != nil || len(found) != 0 {
		t.Fatalf("a merge whose own change is to a wip scenario: %v, %v", found, err)
	}
	found, err = moves.Merge("test", first, "HEAD", []string{"features/a.feature"})
	if err != nil || len(found) != 1 || !strings.Contains(found[0].Message, "a test commit changes the live scenario ID-A-01") {
		t.Fatalf("a merge whose own change is to a live scenario: %v, %v", found, err)
	}
}
