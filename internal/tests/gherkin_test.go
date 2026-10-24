package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

// Tagged reads a Gherkin kind's scenarios by a tag on their own tag line or
// their file's, live or not, and leaves out a tag that only starts another.
func TestTagged(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.feature", "Feature: A\n\n  @ID-A-01 @slice-9 @wip\n  Scenario: one\n\n  @ID-A-02 @slice-90\n  Scenario: two\n\n  @ID-A-03 @slice-9\n  Scenario: three\n")
	write("b.feature", "@slice-9\nFeature: B\n\n  @ID-B-01\n  Scenario: four\n")
	root, id := dir, `ID-[A-Z]+-\d+`
	cfg := &config.Loaded{}
	cfg.Tests.Keys = []string{"scenario"}
	cfg.Tests.Values = map[string]config.Kind{"scenario": {Root: &root, ID: &id, TagPrefix: "@", WipTag: "@wip"}}
	k := cfg.Tests.Values["scenario"]
	k.Adapter.Name = "gherkin"
	cfg.Tests.Values["scenario"] = k
	got, err := Tagged(cfg, "@slice-9", "worktree")
	if err != nil {
		t.Fatal(err)
	}
	want := []Test{{ID: "@ID-A-01", File: "a.feature"}, {ID: "@ID-A-03", File: "a.feature", Live: true}, {ID: "@ID-B-01", File: "b.feature", Live: true}}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %+v, not %+v", i, got[i], want[i])
		}
	}
}

// TaggedExamples gives the Examples blocks tagged @wip under a live scenario
// of the tag, named or, with no name, by their line, as Gherkin reads them:
// among other tags, under a Rule, written Scenarios:; a block with another
// tag, a scenario that is not live, one Gherkin cannot read or one whose tag
// line Gherkin gives to no scenario has none; Tagged gives no block.
func TestTaggedExamples(t *testing.T) {
	dir := t.TempDir()
	text := "Feature: A\n" + // 1
		"\n" + // 2
		"  @ID-A-01 @slice-9\n" + // 3
		"  Scenario Outline: named\n" + // 4
		"    When it runs <n>\n" + // 5
		"\n" + // 6
		"    Examples: live\n" + // 7
		"      | n |\n" + // 8
		"      | 1 |\n" + // 9
		"\n" + // 10
		"    @slow @wip\n" + // 11
		"    Examples: waiting\n" + // 12
		"      | n |\n" + // 13
		"      | 2 |\n" + // 14
		"\n" + // 15
		"    @wip\n" + // 16
		"    Scenarios:\n" + // 17
		"      | n |\n" + // 18
		"      | 3 |\n" + // 19
		"\n" + // 20
		"    @slow\n" + // 21
		"    Examples: slow\n" + // 22
		"      | n |\n" + // 23
		"      | 4 |\n" + // 24
		"\n" + // 25
		"  @ID-A-02 @slice-9 @wip\n" + // 26
		"  Scenario Outline: not live\n" + // 27
		"    When it runs <n>\n" + // 28
		"\n" + // 29
		"    @wip\n" + // 30
		"    Examples: waiting\n" + // 31
		"      | n |\n" + // 32
		"      | 5 |\n" + // 33
		"\n" + // 34
		"  Rule: r\n" + // 35
		"\n" + // 36
		"    @ID-A-03 @slice-9\n" + // 37
		"    Scenario Outline: in a rule\n" + // 38
		"      When it runs <n>\n" + // 39
		"\n" + // 40
		"      @wip\n" + // 41
		"      Examples: in the rule\n" + // 42
		"        | n |\n" + // 43
		"        | 6 |\n" // 44
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.feature", text)
	write("b.feature", "Feature: B\n\n  @ID-B-01 @slice-9\n  Nonsense\n")
	write("c.feature", "Feature: C\n\n  @ID-C-01 @slice-9\n  Rule: r\n\n    @ID-C-02 @slice-9\n    Scenario Outline: other\n      When it runs <n>\n\n      @wip\n      Examples: x\n        | n |\n        | 1 |\n")
	root, id := dir, `ID-[A-Z]+-\d+`
	cfg := &config.Loaded{}
	cfg.Tests.Keys = []string{"scenario"}
	cfg.Tests.Values = map[string]config.Kind{"scenario": {Root: &root, ID: &id, TagPrefix: "@", WipTag: "@wip"}}
	k := cfg.Tests.Values["scenario"]
	k.Adapter.Name = "gherkin"
	cfg.Tests.Values["scenario"] = k
	tagged, got, err := TaggedExamples(cfg, "@slice-9", "worktree")
	if err != nil {
		t.Fatal(err)
	}
	a := func(id string) Test { return Test{ID: id, File: "a.feature", Live: id != "@ID-A-02"} }
	want := []WipExample{
		{Test: a("@ID-A-01"), Name: "waiting", Line: 12},
		{Test: a("@ID-A-01"), Name: "", Line: 17},
		{Test: a("@ID-A-03"), Name: "in the rule", Line: 42},
		{Test: Test{ID: "@ID-C-02", File: "c.feature", Live: true}, Name: "x", Line: 11},
	}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %+v, not %+v", i, got[i], want[i])
		}
	}
	plain, err := Tagged(cfg, "@slice-9", "worktree")
	if err != nil || len(plain) != len(tagged) {
		t.Errorf("Tagged gives %+v, TaggedExamples %+v: %v", plain, tagged, err)
	}
}
