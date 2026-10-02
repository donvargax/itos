package scope

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/internal/config"
)

func rules(t *testing.T, text string) *Rules {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Of(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const project = `version: 1
commits:
  types: [docs, style, wip, bad]
  scopes:
    docs: { only: ["**/*.md"] }
    style: {}
    bad: { only: ["*.md", "?x"] }
`

// A type with empty rules is ruled and passes; one without rules is not
// ruled; a bad glob is an error only once a path reaches it, as `some`
// reaches it.
func TestIssues(t *testing.T) {
	r := rules(t, project)
	if !r.Ruled("style") || r.Ruled("wip") {
		t.Errorf("Ruled: style %v, wip %v", r.Ruled("style"), r.Ruled("wip"))
	}
	if found, err := r.Issues("style", []string{"src/a.ts"}); err != nil || len(found) != 0 {
		t.Errorf("style: %v, %v", found, err)
	}
	if found, err := r.Issues("bad", []string{"a.md"}); err != nil || len(found) != 0 {
		t.Errorf("bad on a.md: %v, %v", found, err)
	}
	if _, err := r.Issues("bad", []string{"a.ts"}); err == nil {
		t.Error("bad on a.ts: no error")
	}
	// docs rejects a.ts, and the fix's types reach bad's glob.
	if _, err := r.Issues("docs", []string{"a.ts"}); err == nil {
		t.Error("docs on a.ts: no error from the fix's types")
	}
}
