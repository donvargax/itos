package work

import (
	"os"
	"testing"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/providers"
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
	cfg, err := config.Load("itos.yaml")
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
