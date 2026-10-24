package work

import (
	"errors"
	"os"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// A project whose people file is missing or cannot be read has no people:
// the registry's owners go unchecked, any session is listed, and only
// PeopleProblem says why; a people file that reads holds owners to it.
func TestRegistryWithoutPeople(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("ITOS_CONFIG", "itos.yaml")
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("itos.yaml", "version: 1\nwork: { registry: work-items.yaml, people: { source: yaml, file: people.yaml } }\n")
	write("work-items.yaml", "phases: { 1: z }\nitems:\n  - { id: x, title: X, phase: 1, owner: q, status: todo }\n")
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, "itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	nobody := func() providers.Answer { return providers.Answer{Problem: "nobody"} }
	for _, people := range []string{"", "people: [a]\n"} {
		if people != "" {
			write("people.yaml", people)
		}
		r, err := Load(cfg, cfg.Work.Registry)
		if err != nil {
			t.Fatalf("people %q: %v", people, err)
		}
		if found, err := Issues(cfg, r, cfg.Work.Registry); err != nil || len(found) > 0 || r.People {
			t.Errorf("people %q: %v, %v, %v", people, r.People, found, err)
		}
		if who := Whoami(r, "people.yaml", "q", nobody); who.Handle != "q" || !who.Listed {
			t.Errorf("people %q: --as q is %+v", people, who)
		}
		if PeopleProblem(cfg) == nil {
			t.Errorf("people %q: no warning", people)
		}
	}
	write("people.yaml", "- a\n")
	r, err := Load(cfg, cfg.Work.Registry)
	if err != nil {
		t.Fatal(err)
	}
	if found, _ := Issues(cfg, r, cfg.Work.Registry); len(found) != 2 || PeopleProblem(cfg) != nil {
		t.Errorf("with the people, the problems are %v", found)
	}
	if who := Whoami(r, "people.yaml", "q", nobody); who.Problem == "" {
		t.Errorf("--as q, not among the people, is %+v", who)
	}
}

// Whoami asks the identity provider only when the people list several (bug
// 46): one person is the session, and no one listed is nobody.
func TestWhoamiAsksOnlyAmongSeveral(t *testing.T) {
	asked := 0
	ask := func() providers.Answer { asked++; return providers.Answer{Handle: "b"} }
	cases := []struct {
		name  string
		r     Registry
		want  Identity
		asked int
	}{
		{"no people file", Registry{}, Identity{}, 0},
		{"nobody listed", Registry{People: true}, Identity{}, 0},
		{"one person", Registry{People: true, Logins: []string{"a"}}, Identity{Handle: "a", Listed: true}, 0},
		{"several", Registry{People: true, Logins: []string{"a", "b"}}, Identity{Handle: "b", Listed: true}, 1},
	}
	for _, c := range cases {
		asked = 0
		if who := Whoami(c.r, "people.yaml", "", ask); who != c.want || asked != c.asked {
			t.Errorf("%s: %+v, asked %d times", c.name, who, asked)
		}
	}
	one := Registry{People: true, Logins: []string{"a"}}
	if who := Whoami(one, "people.yaml", "z", ask); who.Problem == "" {
		t.Errorf("--as z, not the one person, is %+v", who)
	}
}

// A registry with no items, the key absent or null, reads as one with none,
// as the TypeScript's `raw.items ?? []` did; items that are not a list are
// refused.
func TestRegistryWithoutItems(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("itos.yaml", []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, "itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"phases: { 1: z }\n", "phases: { 1: z }\nitems: null\n"} {
		if r, err := parse(cfg, "w.yaml", text); err != nil || len(r.Items) != 0 {
			t.Errorf("%q: items %v, %v", text, r.Items, err)
		}
	}
}

// A registry of the wrong shape is refused as a problem of the registry (bug
// 134) with its rule, naming the file and the key, and the fix: the error
// Load returns for the commands that read it carries the same message.
func TestRegistryOfTheWrongShape(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("itos.yaml", []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, "itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, text, rule, message, fix string }{
		{"empty", "# nothing\n", "work-registry-empty", "w.yaml: the registry is empty", "restore w.yaml, or write its phases: and items:"},
		{"null", "null\n", "work-registry-empty", "w.yaml: the registry is empty", "restore w.yaml, or write its phases: and items:"},
		{"items", "phases: { 1: z }\nitems: 3\n", "work-items-not-list", "w.yaml: items: is not a list of work items",
			"write items: in w.yaml as a list, or remove it"},
		{"item", "phases: { 1: z }\nitems: [{ id: a }, ~]\n", "work-item-empty", "w.yaml: entry 2 of items: is empty",
			"give entry 2 of items: in w.yaml an id and a title, or remove it"},
		{"documents", "phases: { 1: z }\n---\nphases: { 1: z }\n", "work-registry-several-documents",
			"w.yaml: the YAML has more than one document", "keep one document in w.yaml: remove the --- line that starts the second, and what follows it"},
		{"groups", "phases: one\n", "work-groups-not-mapping", "w.yaml: phases: is not a mapping (it is one)",
			"write phases: in w.yaml as a mapping of each group to its owner, or remove it"},
	} {
		_, err := parse(cfg, "w.yaml", c.text)
		var shape *shapeError
		if !errors.As(err, &shape) {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := shape.found; got.Rule != c.rule || got.Message != c.message || got.Fix != c.fix || err.Error() != c.message {
			t.Errorf("%s: %+v, error %q", c.name, got, err)
		}
	}
}

// ProblemsAt reports a registry of the wrong shape as its problem, where a
// YAML syntax error stays an error.
func TestProblemsAtWrongShape(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("ITOS_CONFIG", "itos.yaml")
	if err := os.WriteFile("itos.yaml", []byte("version: 1\nwork: { registry: w.yaml }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, "itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("w.yaml", []byte("phases: { 1: z }\nitems: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if found, err := ProblemsAt(cfg, "w.yaml", false); err != nil || len(found) != 1 || found[0].Rule != "work-items-not-list" {
		t.Errorf("%v, %v", found, err)
	}
	if err := os.WriteFile("w.yaml", []byte("phases: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if found, err := ProblemsAt(cfg, "w.yaml", false); err == nil {
		t.Errorf("a syntax error is problems %v", found)
	}
}

// A depends_on that is not a list is the item's problem, the rest of the
// registry still judged.
func TestDependsOnNotList(t *testing.T) {
	cfg := addConfigWith(t, "")
	r := registryOf(t, "phases: { 1: z }\nitems:\n  - { id: a, title: A, phase: 1, owner: null, status: todo, depends_on: b }\n")
	found, err := Issues(cfg, r, "w.yaml")
	if err != nil || len(found) != 1 || found[0].Rule != "work-depends-on-not-list" ||
		found[0].Message != "a: depends_on is not a list of item ids" {
		t.Errorf("%v, %v", found, err)
	}
}
