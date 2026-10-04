package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v3/internal/config"
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
