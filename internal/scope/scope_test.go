package scope

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v4/internal/config"
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

// except takes a path back out of never, $sets and all, for the check and
// for the types a rejection's fix names; it leaves only alone.
func TestExceptTakesPathsOutOfNever(t *testing.T) {
	r := rules(t, `version: 1
commits:
  types: [chore, docs]
  path_sets:
    themes: ["src/games/*/themes/**"]
  scopes:
    chore: { never: ["src/**"], except: [$themes] }
    docs: { only: ["docs/**"], never: ["docs/drafts/**"], except: ["**/*.md"] }
`)
	if found, err := r.Issues("chore", []string{"src/games/cielo/themes/colours.ts"}); err != nil || len(found) != 0 {
		t.Errorf("chore on a theme: %v, %v", found, err)
	}
	found, err := r.Issues("chore", []string{"src/games/cielo/sim/nudo.ts"})
	if err != nil || len(found) != 1 || found[0].Rule != "scope-never" {
		t.Errorf("chore on the sim: %v, %v", found, err)
	}
	if found, err := r.Issues("docs", []string{"docs/drafts/a.md"}); err != nil || len(found) != 0 {
		t.Errorf("docs on a draft's Markdown: %v, %v", found, err)
	}
	if found, err := r.Issues("docs", []string{"README.md"}); err != nil || len(found) != 1 || found[0].Rule != "scope-only" {
		t.Errorf("docs on README.md, only's alone: %v, %v", found, err)
	}
	if types, err := r.TypesFor("src/games/cielo/themes/a.ts"); err != nil || len(types) != 1 || types[0] != "chore" {
		t.Errorf("TypesFor a theme: %v, %v", types, err)
	}
	if types, err := r.TypesFor("src/a.ts"); err != nil || len(types) != 0 {
		t.Errorf("TypesFor src/a.ts: %v, %v", types, err)
	}
}
