package cli

import (
	"slices"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

// The group label is a flag of itos task's (slice 129): one word that is no
// other flag of its, a subcommand's or a global one.
func TestGroupLabelProblems(t *testing.T) {
	cases := map[string]string{
		"phase":     "",
		"group":     "",
		"milestone": "",
		"étape":     "",
		"skip":      "ledger.group.label is skip, and --skip is already another flag of itos task, so it never takes a group",
		"pending":   "ledger.group.label is pending, and --pending is already another flag of itos task, so it never takes a group",
		"title":     "ledger.group.label is title, and --title is already another flag of itos task, so it never takes a group",
		"json":      "ledger.group.label is json, and --json is already another flag of itos task, so it never takes a group",
		"root":      "ledger.group.label is root, and --root is already another flag of itos task, so it never takes a group",
		"config":    "ledger.group.label is config, and --config is already another flag of itos task, so it never takes a group",
		"":          `ledger.group.label "" cannot be typed as itos task's flag --`,
		"my phase":  `ledger.group.label "my phase" cannot be typed as itos task's flag --my phase`,
		"a\tb":      `ledger.group.label "a\tb" cannot be typed as itos task's flag --a` + "\tb",
		"-x":        `ledger.group.label "-x" cannot be typed as itos task's flag ---x`,
		"a=b":       `ledger.group.label "a=b" cannot be typed as itos task's flag --a=b`,
		" phase":    `ledger.group.label " phase" cannot be typed as itos task's flag -- phase`,
	}
	for label, want := range cases {
		cfg := &config.Loaded{}
		cfg.Ledger.Group.Label = label
		found := groupLabelProblems(cfg)
		switch {
		case want == "" && len(found) > 0:
			t.Errorf("%q: got %+v, want none", label, found)
		case want != "" && (len(found) != 1 || found[0].Message != want || found[0].Rule != "config-group-label"):
			t.Errorf("%q: got %+v, want %q", label, found, want)
		}
	}
}

func TestGroupLabelProblemsSayWhatToWrite(t *testing.T) {
	for label, fix := range map[string]string{
		"a b":  "set ledger.group.label to one word, with no space, = or leading -, such as phase or milestone",
		"skip": "set ledger.group.label to a word that is no other flag of itos task, such as phase or milestone",
	} {
		cfg := &config.Loaded{}
		cfg.Ledger.Group.Label = label
		if found := groupLabelProblems(cfg); len(found) != 1 || found[0].Fix != fix {
			t.Errorf("%q: got %+v, want the fix %q", label, found, fix)
		}
	}
}

// config check's findings: the values nothing reads as written, the ledger
// layout's and the questions' are problems, each after the file's name, exit
// 1; a value a command misbehaves on is the loader's, exit 2 (slice 130); a
// registry that cannot be read is an error, and no exit code of config
// check's own.
func TestConfigFindings(t *testing.T) {
	scratchFolder(t)
	gitIn(t, "init", "-q", "-b", "main")
	const ledger = "ledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\", group: { label: skip } }\n"
	const work = "work: { people: { source: yaml, file: people.yaml } }\n"
	writeFiles(t, map[string]string{
		"itos.yaml":             "version: 1\nshell: []\n" + ledger + work,
		"tasks/phase-1.yaml":    "- { id: T-001, type: build, title: One }\n",
		"tasks/work-items.yaml": "phases: {}\nitems: [{ id: p1-a, title: A, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: A }]\n",
		"people.yaml":           "- a\n",
	})
	findings := func(wantCode int, want ...string) {
		t.Helper()
		code, found, _, _, err := configFindings("", nil)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range found {
			got = append(got, f.Area+": "+f.Message)
		}
		if code != wantCode || !slices.Equal(got, want) {
			t.Errorf("got %d %q, want %d %q", code, got, wantCode, want)
		}
	}
	findings(2, "config: itos.yaml: shell names no program, so no command can run under it")

	writeFiles(t, map[string]string{
		"itos.yaml":       "version: 1\n" + ledger + work,
		"tasks/asks.yaml": "questions:\n  - { id: q-1, item: p1-a, question: \"Why?\" }\n  - { id: q-2, item: p1-b, question: \"How?\" }\n",
	})
	findings(ExitPolicy,
		"config: itos.yaml: ledger.group.label is skip, and --skip is already another flag of itos task, so it never takes a group",
		"registry: p1-a: skip 1 is not listed",
		"asks: tasks/asks.yaml: q-2 names the item p1-b, which tasks/work-items.yaml does not have")

	writeFiles(t, map[string]string{
		"itos.yaml": "version: 1\nledger: { files: \"tasks/{group}/phase.yaml\" }\n" +
			"work: { registry: tasks/work-items.yaml, people: { source: yaml, file: people.yaml } }\n",
		"tasks/asks.yaml": "questions:\n  - { id: q-1, question: \"Why?\" }\n  - { id: q-1, question: \"How?\" }\n",
	})
	findings(ExitPolicy,
		"config: itos.yaml: ledger.files has {group} in its folder: tasks/{group}/phase.yaml; the ledger is one folder, and a group is a part of a file's name",
		"registry: p1-a: phase 1 is not listed",
		"asks: tasks/asks.yaml: two questions have the id q-1")

	// A registry whose alias the reader refuses is the registry's problem
	// alone, so the questions name no unknown item beside it.
	writeFiles(t, map[string]string{
		"itos.yaml":             "version: 1\n" + ledger + work,
		"tasks/asks.yaml":       "questions:\n  - { id: q-1, item: p1-a, question: \"Why?\" }\n",
		"tasks/work-items.yaml": "phases: {}\nitems: &a [*a]\n",
	})
	code, found, _, _, err := configFindings("", nil)
	if err != nil || code != ExitPolicy || slices.ContainsFunc(found, func(f Found) bool { return f.Area == "asks" }) {
		t.Errorf("a registry with a cycle: %d %+v %v", code, found, err)
	}

	writeFiles(t, map[string]string{"tasks/work-items.yaml": "phases: [\n"})
	if code, found, _, _, err := configFindings("", nil); err == nil || code != 0 || found != nil {
		t.Errorf("a registry that cannot be read: %d %v %v", code, found, err)
	}
}

// A questions' file that is there and cannot be read is an error, as the
// registry's is.
func TestConfigFindingsQuestionsUnreadable(t *testing.T) {
	scratchFolder(t)
	gitIn(t, "init", "-q", "-b", "main")
	writeFiles(t, map[string]string{
		"itos.yaml":             "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { people: { source: yaml, file: people.yaml } }\n",
		"tasks/phase-1.yaml":    "[]\n",
		"tasks/work-items.yaml": "phases: {}\nitems: []\n",
		"people.yaml":           "- a\n",
		"tasks/asks.yaml/x":     "",
	})
	if code, found, _, _, err := configFindings("", nil); err == nil || code != 0 || found != nil {
		t.Errorf("a questions' file that cannot be read: %d %v %v", code, found, err)
	}
}

// An answered question naming an item the registry does not have is a
// warning, in the area asks, and never sets the code; an open one is a
// problem (bug 105).
func TestConfigFindingsAnsweredQuestionsWarn(t *testing.T) {
	scratchFolder(t)
	gitIn(t, "init", "-q", "-b", "main")
	writeFiles(t, map[string]string{
		"itos.yaml":             "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { people: { source: yaml, file: people.yaml } }\n",
		"tasks/phase-1.yaml":    "[]\n",
		"tasks/work-items.yaml": "phases: { 1: null }\nitems: [{ id: p1-a, title: A, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: A }]\n",
		"people.yaml":           "- a\n",
		"tasks/asks.yaml": "questions:\n  - { id: q-1, item: p1-gone, question: \"Why?\", answer: \"So.\" }\n" +
			"  - { id: q-2, item: p1-a, question: \"How?\", answer: \"Thus.\" }\n  - { id: q-3, question: \"When?\", answer: \"Now.\" }\n",
	})
	code, found, warnings, _, err := configFindings("", nil)
	if err != nil || code != 0 || len(found) != 0 || len(warnings) != 1 {
		t.Fatalf("answered: %d %+v %+v %v", code, found, warnings, err)
	}
	want := Found{Area: "asks"}
	want.Rule = "asks-answered-unknown-item"
	want.Message = "tasks/asks.yaml: q-1, answered, names the item p1-gone, which tasks/work-items.yaml does not have"
	want.Fix = "name the item's new id in q-1's item, or remove its item"
	if warnings[0] != want {
		t.Errorf("warning %+v, want %+v", warnings[0], want)
	}
	writeFiles(t, map[string]string{"tasks/asks.yaml": "questions:\n  - { id: q-1, item: p1-gone, question: \"Why?\" }\n"})
	code, found, warnings, _, err = configFindings("", nil)
	if err != nil || code != ExitPolicy || len(found) != 1 || found[0].Rule != "asks-unknown-item" || len(warnings) != 0 {
		t.Errorf("open: %d %+v %+v %v", code, found, warnings, err)
	}
}
