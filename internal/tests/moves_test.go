package tests

import (
	"reflect"
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

// The cases moves.yaml gives the commit-msg hook, which is not ported yet,
// judged by the comparison it will call.
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
