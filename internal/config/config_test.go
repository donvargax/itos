package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	vcsgit "github.com/donvargax/itos/v8/internal/vcs/adapter"

	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/value"
	vcstest "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

func load(t *testing.T, text string) (*Loaded, error) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(&vcstest.VCS{}, adapter.Trees{VCS: vcsgit.Git{}}, adapter.Worktree, file)
}

// CommentChar is the VCS's for a loaded config, its copies with other data
// among them, and # for one Load did not make.
func TestCommentChar(t *testing.T) {
	loaded, err := FromText(&vcstest.VCS{Comment: ";"}, adapter.Trees{VCS: vcsgit.Git{}}, adapter.Worktree, "itos.yaml", "version: 1\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.DataFrom(adapter.Worktree).CommentChar(); got != ";" {
		t.Errorf("CommentChar of a loaded config = %q", got)
	}
	if got := (&Loaded{}).CommentChar(); got != "#" {
		t.Errorf("CommentChar of a bare config = %q", got)
	}
}

func TestLoadReadsRequires(t *testing.T) {
	c, err := load(t, "version: 1\nrequires: \">=0.1.0\"\n")
	if err != nil || c.Requires == nil || *c.Requires != ">=0.1.0" {
		t.Fatalf("got %+v, %v", c, err)
	}
	c, err = load(t, "version: 1.0\n")
	if err != nil || c.Requires != nil {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func messages(t *testing.T, err error) []string {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v, want a config error", err)
	}
	var found []string
	for _, p := range e.Problems {
		found = append(found, p.Message)
	}
	return found
}

func TestLoadProblems(t *testing.T) {
	cases := map[string][]string{
		"version: 2\n":                   {"version 2 is not 1"},
		"version: 1.5\n":                 {"version 1.5 is not 1"},
		"requires: x\n":                  {"version is missing"},
		"version: one\nrequires: [a]\n":  {"version should be a number, not string", "requires should be a string, not a list"},
		"- version: 1\n":                 {"the file should be a mapping, not a list"},
		"":                               {"the file should be a mapping, not null"},
		"version: 1\nrequires: {a: b}\n": {"requires should be a string, not object"},
		// YAML 1.1's forms are text to the core schema: 1_0 is no number.
		"version: 1_0\n": {"version should be a number, not string"},
		"version: 1\nhooks: { bim: x, shims: true }\n": {
			"unknown key hooks.bim", "unknown key hooks.shims",
		},
		"version: 1\nwork: { tags: [a, '', a] }\n": {
			"work.tags[1] is empty", `work.tags declares "a" twice`,
		},
		"version: 1\ncommits: { scopes: { docs: { only: [$prose] } } }\n": {
			"commits.scopes.docs.only names $prose, which commits.path_sets does not have",
		},
		// A step runs a command or a kind's tests; the step that ran the done
		// tasks' checks is refused as the one v8 removed (slice 107).
		"version: 1\nci: { steps: [{ tests: x }], nightly: { steps: [{ cost: static }, { tasks: done, cost: static }] } }\n": {
			"a CI step runs tests: x, and tests.x.run.whole is missing",
			`a nightly step needs exactly one of run and tests: {"cost":"static"}`,
			`a nightly step runs tasks: done, which v8 removed: a task's checks are its own progress and leave the ledger at work done: {"tasks":"done","cost":"static"}`,
		},
		// A kind the config declares needs its run.whole for a step to run it.
		"version: 1\ntests: { k: { root: f }, e: { root: g, run: { whole: \"\" } } }\nci: { steps: [{ tests: k }, { tests: e }] }\n": {
			"a CI step runs tests: k, and tests.k.run.whole is missing",
			"a CI step runs tests: e, and tests.e.run.whole is missing",
		},
	}
	for text, want := range cases {
		_, err := load(t, text)
		if got := messages(t, err); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

// An unknown key's fix renames it to the key it misspells, else lists the
// keys there are.
func TestUnknownKeyFix(t *testing.T) {
	_, err := load(t, "version: 1\nhooks: { bim: x, shims: true }\n")
	var e *Error
	errors.As(err, &e)
	if e.Problems[0].Fix != "rename hooks.bim to bin" {
		t.Errorf("near: %q", e.Problems[0].Fix)
	}
	if e.Problems[1].Fix != "remove hooks.shims; the keys here are bin, pre_push, commit_msg" {
		t.Errorf("far: %q", e.Problems[1].Fix)
	}
}

// v5.0.0's removed keys and values are refused as removed, each problem
// saying what to do instead (slice 85), and a value it never had is still an
// enum's problem.
func TestRemovedInV5(t *testing.T) {
	_, err := load(t, "version: 1\nci: { range: { provider: command, command: x, github: { branch: main } }, "+
		"watch: { provider: command, command: y, nightly_command: z } }\n"+
		"work: { identity: { provider: command, command: \"echo a\" } }\n")
	var e *Error
	if !errors.As(err, &e) || len(e.Problems) != 8 {
		t.Fatalf("got %v", err)
	}
	for _, p := range e.Problems {
		if p.Rule != "config-removed" || !strings.Contains(p.Message, " was removed in v5; ") || p.Fix == "" {
			t.Errorf("got %+v", p)
		}
	}
	got := messages(t, func() error { _, err := load(t, "version: 1\nci: { watch: { provider: gitlab } }\n"); return err }())
	if !slices.Equal(got, []string{`ci.watch.provider should be one of github, none, not "gitlab"`}) {
		t.Errorf("an unknown provider: %q", got)
	}
}

// A key declared renamed carries the key it was into its Node, the spec it
// copies left as it was, and the top's Node carries the keys removed
// (slice 113).
func TestSchemaDeclarations(t *testing.T) {
	declared := renamed("ci.watch.timeout", about("How long to wait.", num))
	if n := node(declared); n.RenamedFrom != "ci.watch.timeout" || n.Kind != "number" || n.Description != "How long to wait." {
		t.Errorf("got %+v", n)
	}
	if num.renamedFrom != "" {
		t.Error("the spec renamed copies was changed")
	}
	was := removedHere
	defer func() { removedHere = was }()
	removedHere = []string{"ci.watch.timeout"}
	if got := Schema().Removed; !slices.Equal(got, removedHere) {
		t.Errorf("the top's removed keys are %v", got)
	}
}

// The file is laid over the one table: what it leaves out is the table's,
// work.registry beside its ledger, each kind over the per-kind defaults.
func TestLoadLaysTheFileOverTheDefaults(t *testing.T) {
	c, err := load(t, "version: 1\nledger: { files: \"work/phase-{group}.yaml\", check: { timeout: 5 } }\ntests: { k: { root: f, run: { whole: w } } }\n")
	if err != nil {
		t.Fatal(err)
	}
	if c.Ledger.Check.Timeout != 5 || c.Ledger.Group.Label != "phase" || c.Ledger.ID != nil {
		t.Errorf("ledger: %+v", c.Ledger)
	}
	if c.Work.Registry != "work/work-items.yaml" || c.Hooks.Bin != "itos" || !c.CI.StopAtFirstFailure {
		t.Errorf("registry %q, bin %q, stop %v", c.Work.Registry, c.Hooks.Bin, c.CI.StopAtFirstFailure)
	}
	if c.Work.Asks != "work/asks.yaml" {
		t.Errorf("asks %q, not beside the registry", c.Work.Asks)
	}
	k, ok := c.Tests.Get("k")
	if !ok || *k.Root != "f" || *k.Run.Whole != "w" || k.Run.Join.Sep != "|" || k.TagPrefix != "@" || k.Adapter.Name != "gherkin" || !k.Smoke.EveryFile {
		t.Errorf("kind: %+v", k)
	}
	if !c.HasSection("ledger") || c.HasSection("ci") || c.Section("ci") == nil {
		t.Error("sections")
	}
}

// work.asks is beside the registry the file names, wherever that is, unless
// the file names it too (slice 62).
func TestAsksBesideTheRegistry(t *testing.T) {
	for text, want := range map[string]string{
		"version: 1\n": "tasks/asks.yaml",
		"version: 1\nwork: { registry: work-items.yaml }\n":                     "asks.yaml",
		"version: 1\nwork: { registry: data/reg.yaml }\n":                       "data/asks.yaml",
		"version: 1\nwork: { registry: data/reg.yaml, asks: questions.yaml }\n": "questions.yaml",
	} {
		c, err := load(t, text)
		if err != nil {
			t.Fatal(err)
		}
		if c.Work.Asks != want {
			t.Errorf("%q: asks %q, want %q", text, c.Work.Asks, want)
		}
	}
}

// A work.registry the file leaves empty is none: the default's, beside its
// ledger, is the registry work.asks is beside.
func TestAnEmptyRegistryLeavesAsksBesideTheDefault(t *testing.T) {
	for text, want := range map[string]string{
		"version: 1\nwork: { registry: \"\" }\n":                                                 "tasks/asks.yaml",
		"version: 1\nledger: { files: \"work/phase-{group}.yaml\" }\nwork: { registry: \"\" }\n": "work/asks.yaml",
	} {
		c, err := load(t, text)
		if err != nil {
			t.Fatal(err)
		}
		if c.Work.Asks != want {
			t.Errorf("%q: asks %q, want %q", text, c.Work.Asks, want)
		}
	}
}

// hooks.bin as a command's first word is also read as itos, and only then.
func TestReadings(t *testing.T) {
	c, err := load(t, "version: 1\nci: { steps: [], cost: { static: [\"^itos work check$\"] } }\nhooks: { bin: tools/bin/itos }\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Readings("  tools/bin/itos   work check "); !slices.Equal(got, []string{"tools/bin/itos   work check", "itos   work check"}) {
		t.Errorf("readings: %q", got)
	}
	if !c.MatchesStatic("tools/bin/itos work  check") || c.MatchesStatic("other/itos work check") {
		t.Error("matches")
	}
}

func TestPath(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	if Path(&vcstest.VCS{}, adapter.Worktree) != "itos.yaml" {
		t.Errorf("Path = %q without ITOS_CONFIG", Path(&vcstest.VCS{}, adapter.Worktree))
	}
	t.Setenv("ITOS_CONFIG", "other.yaml")
	if Path(&vcstest.VCS{}, adapter.Worktree) != "other.yaml" {
		t.Errorf("Path = %q with ITOS_CONFIG=other.yaml", Path(&vcstest.VCS{}, adapter.Worktree))
	}
}

func TestKnownKey(t *testing.T) {
	for _, key := range []string{
		"version", "hooks.bin", "hooks.commit_msg.check_timeout", "tests.scenario.root",
		"tests.scenario.adapter.command", "commits.footers.Task.source", "ci.steps", "pin",
	} {
		if known, _ := KnownKey(key); !known {
			t.Errorf("%s is not known", key)
		}
	}
	cases := map[string][]string{
		"hooks.no_such_key": {"bin", "pre_push", "commit_msg"},
		"hooks.bin.x":       nil,
		"ci.steps.0":        nil,
		"hooks.":            {"bin", "pre_push", "commit_msg"},
	}
	for key, want := range cases {
		known, keys := KnownKey(key)
		if known || !slices.Equal(keys, want) {
			t.Errorf("%s: got %v %v, want false %v", key, known, keys, want)
		}
	}
}

func TestGet(t *testing.T) {
	c, err := load(t, "version: 1\nhooks: { bin: bin/mine }\ntests: { scenario: { root: features } }\n")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]any{
		"hooks.bin":                      c.Get("hooks.bin"),
		"hooks.commit_msg.check_timeout": c.Get("hooks.commit_msg.check_timeout"),
		"tests.scenario.wip_tag":         c.Get("tests.scenario.wip_tag"),
		"ledger.id":                      c.Get("ledger.id"),
		"hooks.bin.x":                    c.Get("hooks.bin.x"),
	}
	want := map[string]any{
		"hooks.bin":                      "bin/mine",
		"hooks.commit_msg.check_timeout": 60.0,
		"tests.scenario.wip_tag":         "@wip",
		"ledger.id":                      value.Undefined,
		"hooks.bin.x":                    value.Undefined,
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s: got %v, want %v", key, got[key], w)
		}
	}
}

// A footer's in_place_of names other footers of IDs, for commit types
// (slice 63); a footer of free text neither stands in nor is stood in for.
func TestInPlaceOfProblems(t *testing.T) {
	const head = "version: 1\ncommits:\n  types: [test, docs]\n  footers:\n" +
		"    Task: { source: ledger, required_for: [test] }\n" +
		"    Upgrading: { source: text }\n"
	cases := map[string][]string{
		"    Item: { source: registry, in_place_of: { Task: [test, docs] } }\n": nil,
		"    Item: { source: registry, in_place_of: { Task: all } }\n":          nil,
		"    Item: { source: registry, in_place_of: { Item: [test] } }\n": {
			"commits.footers.Item.in_place_of names Item itself",
		},
		"    Item: { source: registry, in_place_of: { Tusk: [test] } }\n": {
			"commits.footers.Item.in_place_of names Tusk, which is not one of commits.footers",
		},
		"    Item: { source: registry, in_place_of: { Upgrading: [test] } }\n": {
			"commits.footers.Item.in_place_of names Upgrading, a footer of free text, which no ID stands in for",
		},
		"    Item: { source: registry, in_place_of: { Task: [chore] } }\n": {
			"commits.footers.Item.in_place_of.Task names chore, which is not one of commits.types",
		},
		"    Note: { source: text, in_place_of: { Task: [test] } }\n": {
			"commits.footers.Note.in_place_of is for a footer of IDs, and commits.footers.Note is free text (source: text)",
		},
	}
	for footer, want := range cases {
		_, err := load(t, head+footer)
		if want == nil {
			if err != nil {
				t.Errorf("%q: got %v, want none", footer, err)
			}
			continue
		}
		if got := messages(t, err); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", footer, got, want)
		}
	}
}

// A ci section may hold the watch alone (bug 26); a tool that needs its
// steps asks for ci.steps, a key below the section, by its dotted path.
func TestSectionReadsADottedKey(t *testing.T) {
	c, err := load(t, "version: 1\nci:\n  watch: { provider: github }\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Section("ci"); err != nil {
		t.Fatalf("ci: %v", err)
	}
	if got := messages(t, c.Section("ci.steps")); !slices.Equal(got, []string{"ci.steps is missing"}) {
		t.Fatalf("ci.steps: %v", got)
	}
	if got := messages(t, c.Section("ledger.files")); !slices.Equal(got, []string{"ledger.files is missing"}) {
		t.Fatalf("ledger.files: %v", got)
	}
	c, err = load(t, "version: 1\nci:\n  steps: []\n")
	if err != nil || c.Section("ci.steps") != nil {
		t.Fatalf("ci.steps written: %v", err)
	}
}

// noWorktree is trees whose working tree cannot be opened.
type noWorktree struct{ porttest.Trees }

func (noWorktree) At(context.Context, string) (port.Source, error) {
	return nil, errors.New("no working tree")
}

// A config the source does not hold is read from the working tree, which
// the trees open: one they cannot open leaves it unreadable, saying why. The
// loaded config carries the trees and the source its data is read from.
func TestLoadReadsFromTheSourceElseTheWorkingTree(t *testing.T) {
	trees := porttest.Trees{Common: ".git", Files: map[string]map[string]string{
		"index":    {"itos.yaml": "version: 1\n"},
		"worktree": {"other.yaml": "version: 1\n"},
	}}
	index, err := trees.At(context.Background(), "index")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Load(&vcstest.VCS{}, trees, index, "itos.yaml")
	if err != nil || c.Source.Tree(context.Background()) != "index" || c.Trees == nil {
		t.Fatalf("the index's config: %+v, %v", c, err)
	}
	if _, err := Load(&vcstest.VCS{}, trees, index, "other.yaml"); err != nil {
		t.Errorf("a config only the working tree holds: %v", err)
	}
	var e *Error
	if _, err := Load(&vcstest.VCS{}, noWorktree{trees}, index, "other.yaml"); !errors.As(err, &e) || !strings.Contains(e.Problems[0].Message, "no working tree") {
		t.Errorf("a working tree that cannot be opened: %v", err)
	}
}

// DataFrom is the config with its data read from another tree, the config
// it is made from left reading its own.
func TestDataFrom(t *testing.T) {
	trees := porttest.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": {"itos.yaml": "version: 1\n"}}}
	worktree, _ := trees.At(context.Background(), "worktree")
	commit, _ := trees.At(context.Background(), "abc")
	c, err := Load(&vcstest.VCS{}, trees, worktree, "itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	at := c.DataFrom(commit)
	if at.Source.Tree(context.Background()) != "abc" || c.Source.Tree(context.Background()) != "worktree" || at.Path != c.Path {
		t.Errorf("DataFrom: %s from %s, path %s", at.Source.Tree(context.Background()), c.Source.Tree(context.Background()), at.Path)
	}
}

// FromText judges a config's text as Load judges its file, the stealth one's
// data beside it as it says, and a text that is no YAML is unreadable.
func TestFromText(t *testing.T) {
	text := "version: 1\nledger: { files: \"work/phase-{group}.yaml\" }\n"
	loaded, err := FromText(&vcstest.VCS{}, adapter.Trees{VCS: vcsgit.Git{}}, adapter.Worktree, filepath.Join("g", "itos", "itos.yaml"), text, true)
	if err != nil || !loaded.Stealth || loaded.Ledger.Files != filepath.Join("g", "itos", "work", "phase-{group}.yaml") {
		t.Fatalf("stealth: %v, %+v", err, loaded)
	}
	if loaded, err = FromText(&vcstest.VCS{}, adapter.Trees{VCS: vcsgit.Git{}}, adapter.Worktree, "itos.yaml", text, false); err != nil || loaded.Stealth || loaded.Ledger.Files != "work/phase-{group}.yaml" {
		t.Fatalf("not stealth: %v, %+v", err, loaded)
	}
	var e *Error
	if _, err := FromText(&vcstest.VCS{}, adapter.Trees{VCS: vcsgit.Git{}}, adapter.Worktree, "p.yaml", "version: [1", false); !errors.As(err, &e) || e.Problems[0].Rule != "config-unreadable" {
		t.Errorf("no YAML: %v", err)
	}
}

// A tree the typed config cannot be read from fails the load, the file's or
// the one laid over the defaults; none the schema accepts does.
func TestLoadFailsWhereTheTypedConfigCannotBeRead(t *testing.T) {
	was := decode
	t.Cleanup(func() { decode = was })
	for failing := 1; failing <= 2; failing++ {
		calls := 0
		decode = func(tree *value.Map, into *Config) error {
			if calls++; calls == failing {
				return errors.New("unreadable")
			}
			return was(tree, into)
		}
		if _, err := FromText(&vcstest.VCS{}, adapter.Trees{VCS: vcsgit.Git{}}, adapter.Worktree, "itos.yaml", "version: 1\n", false); err == nil || err.Error() != "unreadable" {
			t.Errorf("decode %d failing: %v", failing, err)
		}
	}
}

// A range check's code (slice 134): its $path_sets expanded as
// commits.scopes' are, a set the config lacks a problem; and refused, as
// allowed_renames is, on a check that is not builtin: moves.
func TestRangeCheckCode(t *testing.T) {
	loaded, err := FromText(&vcstest.VCS{}, adapter.Trees{VCS: vcsgit.Git{}}, adapter.Worktree, "itos.yaml", `version: 1
commits: { path_sets: { app: ["src/**", "go.mod"] } }
tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    range_checks: [{ name: moves, builtin: moves, code: [$app, "cmd/**"] }]
`, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loaded.Tests.Values["scenario"].RangeChecks[0].Code, []string{"src/**", "go.mod", "cmd/**"}; !slices.Equal(got, want) {
		t.Errorf("code %q, want %q", got, want)
	}
	_, err = FromText(&vcstest.VCS{}, adapter.Trees{VCS: vcsgit.Git{}}, adapter.Worktree, "itos.yaml", `version: 1
tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    range_checks:
      - { name: renames, staged: "true", allowed_renames: { ID-A-01: Opens }, code: ["src/**"] }
      - { name: moves, builtin: moves, code: [$app] }
`, false)
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v", err)
	}
	var got []string
	for _, p := range e.Problems {
		got = append(got, p.Message)
	}
	want := []string{
		"tests.scenario.range_checks[1].code names $app, which commits.path_sets does not have",
		"tests.scenario.range_checks[0] has allowed_renames, which only builtin: moves reads",
		"tests.scenario.range_checks[0] has code, which only builtin: moves reads",
	}
	if !slices.Equal(got, want) {
		t.Errorf("problems %q, want %q", got, want)
	}
}

// A config's folder is its VCS's, the one itos runs in; with no VCS, the
// process's.
func TestLoadedFolder(t *testing.T) {
	if got := (&Loaded{VCS: &vcstest.VCS{Root: "/run"}}).Folder(); got != "/run" {
		t.Errorf("Folder = %q, want the VCS's", got)
	}
	if got := (&Loaded{}).Folder(); got != "" {
		t.Errorf("Folder with no VCS = %q", got)
	}
}
