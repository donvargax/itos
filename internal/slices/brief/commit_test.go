package brief

import (
	"os"
	"strings"
	"testing"
)

// The registry of TestSpecCommitType: a slice with a scenario, one with none,
// a task with a ledger entry, one with none in a phase of no file yet, and one
// with no phase.
const commitRegistry = `phases: { 1: null, 2: null }
items:
  - { id: slice-9, title: Tagged, phase: 1, owner: null, status: todo, depends_on: [] }
  - { id: slice-8, title: Untagged, phase: 1, owner: null, status: todo, depends_on: [] }
  - { id: T-001, title: Entered, phase: 1, owner: null, status: todo, kind: task, depends_on: [] }
  - { id: T-002, title: Unwritten, phase: 2, owner: null, status: todo, kind: task, depends_on: [] }
  - { id: T-003, title: Phaseless, owner: null, status: todo, kind: task, depends_on: [] }
`

const (
	commitConfig = "version: 1\nwork:\n  registry: work-items.yaml\nledger:\n  files: tasks/phase-{group}.yaml\n  id: \"T-\\\\d+\"\n"
	scenarioKind = "tests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\n"
	taskFooter   = "    Task: { source: ledger, required_for: [test, chore] }\n"
)

// itemFooter is a footer of the registry, standing in for Task: in those types.
func itemFooter(types string) string {
	return "    Item: { source: registry, in_place_of: { Task: [" + types + "] } }\n"
}

// commitFiles are the repository of the table: its config, the registry, the
// ledger and a feature file tagged for slice-9.
func commitFiles(config string) map[string]string {
	return map[string]string{
		"itos.yaml":               config,
		"work-items.yaml":         commitRegistry,
		"tasks/work-items.yaml":   commitRegistry,
		"AGENTS.md":               "Rules.\n",
		"tasks/phase-1.yaml":      "- id: T-001\n  type: chore\n  title: Entered\n  done_when:\n    - { run: go vet ./... }\n",
		"features/tagged.feature": "Feature: Tagged\n\n  @ID-A-01 @wip @slice-9\n  Scenario: one\n    When it runs\n",
	}
}

// commitSentence is the drafter's sentences on the commit and the push, for the
// type and flag chosen.
func commitSentence(id, typ, flag string) string {
	body := "each scenario's failing step."
	if strings.HasPrefix(id, "T-") {
		body = "how each check fails today."
	}
	return "Commit with `" + (specCommit{Flag: flag}).command(id) + "`: the header `" + typ + ": specify " + id + ", <short rule>`, the body the problem, the rule and " + body + " Push with `itos push`."
}

// Each row is a config and an item, and the type and flag chosen, or where the
// type is "" the stop under the flag. The rows named ID-BRIEF-29 to 41 are those
// features/brief.feature's description lists; no scenario catches a dropped row.
func TestSpecCommitType(t *testing.T) {
	own, err := os.ReadFile("../../../itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name, config, id, typ, flag string
	}{
		{
			"ID-BRIEF-29: nothing tagged, judged under the kind's root",
			commitConfig + "commits:\n  types: [feat, test, chore, docs]\n  footers:\n" + taskFooter + itemFooter("test, docs, chore") + "  scopes:\n    docs: { only: [\"features/**\"] }\ntests:\n  scenario: { root: acceptance, id: \"ID-[A-Z]+-\\\\d+\" }\n",
			"slice-8", "test", "--item",
		},
		{
			"ID-BRIEF-30: the first type stood in for that passes, in commits.types's order",
			commitConfig + "commits:\n  types: [feat, refactor, build, test, chore, docs]\n  footers:\n    Task: { source: ledger, required_for: [refactor, build, test, chore] }\n" + itemFooter("chore, refactor, build, test, docs") +
				"  scopes:\n    docs: { only: [\"**/*.md\"] }\n    refactor: { never: [\"features/**\"] }\n    build: { must_touch: [go.mod] }\n" + scenarioKind,
			"slice-9", "test", "--item",
		},
		{
			"ID-BRIEF-31: commits.types without docs",
			commitConfig + "commits:\n  types: [feat, test, chore]\n  footers:\n" + taskFooter + itemFooter("test, chore") + scenarioKind,
			"slice-9", "test", "--item",
		},
		{
			"ID-BRIEF-32: docs needs a footer Item: does not stand in for",
			commitConfig + "commits:\n  types: [feat, test, docs]\n  footers:\n    Task: { source: ledger, required_for: [docs, test] }\n" + itemFooter("test") + scenarioKind,
			"slice-9", "test", "--item",
		},
		{
			"ID-BRIEF-35: a task's ledger file",
			commitConfig + "commits:\n  types: [test, chore, docs]\n  footers:\n" + taskFooter + itemFooter("test, docs, chore") + "  scopes:\n    docs: { only: [\"**/*.md\"] }\n    test: { only: [\"features/**\"] }\n" + scenarioKind,
			"T-001", "chore", "--item",
		},
		{
			"ID-BRIEF-37: a task with no entry, the file of its phase",
			commitConfig + "commits:\n  types: [test, chore, docs]\n  footers:\n" + taskFooter + itemFooter("test, docs, chore") + "  scopes:\n    docs: { only: [\"**/*.md\"] }\n    test: { only: [\"features/**\"] }\n    chore: { only: [tasks/phase-2.yaml] }\n" + scenarioKind,
			"T-002", "chore", "--item",
		},
		{
			"ID-BRIEF-38: a task with no registry footer, every type its ledger footer is required for barred",
			commitConfig + "commits:\n  footers:\n" + taskFooter + "  scopes:\n    docs: { only: [\"**/*.md\"] }\n    test: { only: [\"features/**\"] }\n    chore: { only: [\"features/**\"] }\n" + scenarioKind,
			"T-001", "", "--task",
		},
		{
			"ID-BRIEF-39: this repository's config, read from the tree so that it cannot drift, keeps a slice's sentence whole, though commits.types has test before docs",
			string(own),
			"slice-9", "docs", "--item",
		},
		{
			"ID-BRIEF-41: no commits section",
			commitConfig + scenarioKind,
			"slice-9", "docs", "",
		},
		{
			"ID-BRIEF-41: no commits section and no Gherkin kind, docs winning over the stop",
			commitConfig,
			"slice-8", "docs", "",
		},
		{
			"a task with no commits section keeps plain itos commit",
			commitConfig + scenarioKind,
			"T-001", "docs", "",
		},
		{
			"a task with no registry footer passes over a type its ledger footer is not required for",
			commitConfig + "commits:\n  types: [feat, test, chore, docs]\n  footers:\n    Task: { source: ledger, required_for: [chore] }\n  scopes:\n    docs: { only: [\"**/*.md\"] }\n" + scenarioKind,
			"T-001", "chore", "--task",
		},
		{
			"a task with neither footer keeps docs and plain itos commit",
			commitConfig + "commits:\n  types: [test, docs]\n" + scenarioKind,
			"T-001", "docs", "",
		},
		{
			"a task with neither footer has docs alone",
			commitConfig + "commits:\n  types: [test, docs]\n  scopes:\n    docs: { only: [\"**/*.md\"] }\n" + scenarioKind,
			"T-001", "", "",
		},
		{
			"a slice with only a ledger footer never takes its flag",
			commitConfig + "commits:\n  footers:\n" + taskFooter + scenarioKind,
			"slice-9", "docs", "",
		},
		{
			"an Item footer with no in_place_of, docs barred",
			commitConfig + "commits:\n  footers:\n" + taskFooter + "    Item: { source: registry }\n  scopes:\n    docs: { only: [\"**/*.md\"] }\n" + scenarioKind,
			"slice-9", "", "--item",
		},
		{
			"a type that requires a footer of free text is passed over",
			commitConfig + "commits:\n  types: [test, chore, docs]\n  footers:\n" + taskFooter + itemFooter("test, chore") + "    Upgrading: { source: text, required_for: [test] }\n  scopes:\n    docs: { only: [\"**/*.md\"] }\n" + scenarioKind,
			"slice-9", "chore", "--item",
		},
		{
			"no tests section under a commits section",
			commitConfig + "commits:\n  footers:\n    Item: { source: registry }\n",
			"slice-8", "", "--item",
		},
		{
			"a command adapter alone under a commits section",
			commitConfig + "commits:\n  types: [docs]\ntests:\n  unit: { root: checks, id: \"U-\\\\d+\", adapter: { command: ./list } }\n",
			"slice-8", "", "",
		},
		{
			"a command kind before the Gherkin one, judged under the Gherkin kind's root",
			commitConfig + "commits:\n  types: [feat, test, chore, docs]\n  footers:\n" + taskFooter + itemFooter("test, docs, chore") + "  scopes:\n    docs: { only: [\"checks/**\"] }\ntests:\n  unit: { root: checks, id: \"U-\\\\d+\", adapter: { command: ./list } }\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\n",
			"slice-8", "test", "--item",
		},
		{
			"a task whose file ledger.files cannot name",
			commitConfig + "commits:\n  footers:\n    Item: { source: registry }\n" + scenarioKind,
			"T-003", "", "--item",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			text := brief(t, over(commitFiles(row.config)), row.id, "drafter").Text
			holdCommit(t, text, row.id, row.typ, row.flag)
		})
	}
}

// holdCommit holds a drafter's brief to the type and flag, or the stop.
func holdCommit(t *testing.T, text, id, typ, flag string) {
	t.Helper()
	if typ != "" {
		if want := commitSentence(id, typ, flag); !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
		if strings.Contains(text, "No commit type") {
			t.Errorf("a stop beside the type:\n%s", text)
		}
		return
	}
	command := (specCommit{Flag: flag}).command(id)
	want := "No commit type of this repository's config can carry this spec: none may touch the spec's files and be committed with `" + command + "`. Stop and report that before writing the spec; do not commit it under another type. Do the work yourself; start no agents."
	if !strings.Contains(text, want) {
		t.Errorf("no %q in:\n%s", want, text)
	}
	if strings.Contains(text, "the header `") || strings.Contains(text, "Push with `itos push`") {
		t.Errorf("a commit sentence beside the stop:\n%s", text)
	}
	if strings.HasPrefix(id, "T-") && strings.Contains(strings.ReplaceAll(text, "There are no scenarios to write", ""), "scenario") {
		t.Errorf("a task's stop speaks of scenarios:\n%s", text)
	}
}

// The request is a digest of the item, the role, the spec commit and the reads:
// a config choosing another type changes the text and not the request.
func TestSpecCommitTypeLeavesTheRequest(t *testing.T) {
	footers := "commits:\n  types: [feat, test, chore, docs]\n  footers:\n" + taskFooter + itemFooter("test, docs, chore")
	lets := brief(t, over(commitFiles(commitConfig+footers+scenarioKind)), "slice-9", "drafter")
	bars := brief(t, over(commitFiles(commitConfig+footers+"  scopes:\n    docs: { only: [\"**/*.md\"] }\n"+scenarioKind)), "slice-9", "drafter")
	holdCommit(t, lets.Text, "slice-9", "docs", "--item")
	holdCommit(t, bars.Text, "slice-9", "test", "--item")
	if lets.Request != bars.Request {
		t.Errorf("the request changed with the type: %s, %s", lets.Request, bars.Request)
	}
	if lets.Text == bars.Text {
		t.Error("the text did not change with the type")
	}
}

// Only the drafter is judged by the path rules: a glob they cannot compile
// refuses its brief, and no other role's.
func TestSpecCommitBadGlob(t *testing.T) {
	p := over(commitFiles(commitConfig + "commits:\n  footers:\n    Item: { source: registry }\n  scopes:\n    docs: { only: [\"?x\"] }\n" + scenarioKind))
	if _, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9", Role: "drafter"}); err == nil {
		t.Error("a drafter's brief under a bad glob: no error")
	}
	brief(t, p, "slice-9", "implementer")
}
