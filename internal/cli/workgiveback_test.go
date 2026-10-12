package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/work"
)

// givingRegistry has p's item in progress, q's, and p's item to do, among
// the people p and q.
const givingRegistry = "phases: { 1: null }\nitems:\n" +
	"  - { id: a, title: A, phase: 1, owner: p, status: doing }\n" +
	"  - { id: b, title: B, phase: 1, owner: q, status: doing }\n" +
	"  - { id: c, title: C, phase: 1, owner: p, status: todo }\n"

// givingRepo is a repository whose registry is givingRegistry, its people p
// and q in people.yaml, committed.
func givingRepo(t *testing.T) {
	t.Helper()
	registryWriterRepo(t, givingRegistry)
	config := "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n" +
		"work:\n  registry: registry.yaml\n  people: { source: yaml, file: people.yaml }\n"
	for name, text := range map[string]string{"itos.yaml": config, "people.yaml": "- p\n- q\n"} {
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, "add", "--", "itos.yaml", "people.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: people")
}

// giveBack runs work give-back in a fresh givingRepo, and gives its exit
// code, error, stdout and stderr.
func giveBack(t *testing.T, asJSON bool, args ...string) (int, string, string, error) {
	t.Helper()
	givingRepo(t)
	var stdout, stderr strings.Builder
	code, err := workGiveBack(args, Out{Stdout: &stdout, Stderr: &stderr, JSON: asJSON})
	return code, stdout.String(), stderr.String(), err
}

// Slice 122: the person is work take's, the owner judged against them; one
// id and several end alike for a person who is not among the people.
func TestWorkGiveBack(t *testing.T) {
	code, stdout, stderr, err := giveBack(t, false, "a", "--why", "Blocked.", "--as", "p")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "a is given back, todo again for p: committed ") ||
		!strings.HasSuffix(stdout, " docs: give back a\n") {
		t.Errorf("give-back a --as p: (%d, %v) %q %q", code, err, stdout, stderr)
	}
	if text, _ := os.ReadFile("registry.yaml"); !strings.Contains(string(text), "{ id: a, title: A, phase: 1, owner: p, status: todo }") {
		t.Errorf("give-back a --as p wrote:\n%s", text)
	}
	code, stdout, _, err = giveBack(t, true, "a", "--why", "Blocked.", "--as", "p")
	var got struct {
		OK     bool           `json:"ok"`
		Item   map[string]any `json:"item"`
		Commit *string        `json:"commit"`
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || !got.OK ||
		got.Item["status"] != "todo" || got.Item["owner"] != "p" || got.Commit == nil {
		t.Errorf("give-back a --json: (%d, %v) %s", code, err, stdout)
	}
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{"b", "--why", "Blocked.", "--as", "p"}, ExitPolicy, "b is q's, not p's"},
		{[]string{"c", "--why", "Blocked.", "--as", "p"}, ExitPolicy, "c is todo"},
		{[]string{"a", "--why", "Blocked.", "--as", "r"}, ExitMissing, "r"},
		{[]string{"a", "b", "--why", "Blocked.", "--as", "r"}, ExitMissing, "r"},
		{[]string{"a", "b", "--why", "Blocked.", "--as", "p"}, ExitPolicy, "work give-back refuses b, so nothing is written"},
	} {
		code, _, stderr, err := giveBack(t, false, c.args...)
		if code != c.code || err != nil || !strings.Contains(stderr, c.says) {
			t.Errorf("give-back %q: (%d, %v) %q", c.args, code, err, stderr)
		}
		if text, _ := os.ReadFile("registry.yaml"); string(text) != givingRegistry {
			t.Errorf("give-back %q wrote the registry:\n%s", c.args, text)
		}
	}
	code, stdout, _, err = giveBack(t, true, "a", "b", "--why", "Blocked.", "--as", "r")
	got.OK = true
	if code != ExitMissing || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.OK || !strings.Contains(stdout, "work-no-person") {
		t.Errorf("give-back a b --as r --json: (%d, %v) %s", code, err, stdout)
	}
}

// workPerson, work take's and work give-back's: --as, else the identity
// provider's handle, which must be among the people; a refusal of several
// ids says its problem as an error.
func TestWorkPerson(t *testing.T) {
	givingRepo(t)
	cfg, err := config.Load(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		t.Fatal(err)
	}
	says := func(handle, problem string) providers.Identity {
		return func() providers.Answer { return providers.Answer{Handle: handle, Problem: problem} }
	}
	if person, every, problem := workPerson(cfg, registry, "", says("q", "")); person != "q" || every || problem != nil {
		t.Errorf("the provider's q: %q, %v, %+v", person, every, problem)
	}
	if person, _, problem := workPerson(cfg, registry, "p", says("q", "")); person != "p" || problem != nil {
		t.Errorf("--as p: %q, %+v", person, problem)
	}
	if _, _, problem := workPerson(cfg, registry, "", says("r", "")); problem == nil ||
		problem.Message != "r is not in people.yaml, so cannot own an item" || problem.Fix != "add r to people.yaml, or pass --as <handle>" {
		t.Errorf("the provider's r, not among the people: %+v", problem)
	}
	if _, _, problem := workPerson(cfg, registry, "", says("", "gh is signed out")); problem == nil || !strings.Contains(problem.Message, "gh is signed out") {
		t.Errorf("a provider that cannot say: %+v", problem)
	}
	if err := (refusal{code: ExitMissing, found: []out.Problem{{Message: "nobody"}}}); err.Error() != "nobody" {
		t.Errorf("a refusal says %q", err.Error())
	}
}
