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

// config check's findings: the values no tool can use are the config's
// problems, each after the file's name, exit 1; a registry that cannot be
// read is an error, and no exit code of config check's own.
func TestConfigFindings(t *testing.T) {
	scratchFolder(t)
	gitIn(t, "init", "-q", "-b", "main")
	writeFiles(t, map[string]string{
		"itos.yaml": "version: 1\nshell: []\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\", group: { label: skip } }\n" +
			"work: { people: { source: yaml, file: people.yaml } }\n",
		"tasks/phase-1.yaml":    "- { id: T-001, type: build, title: One }\n",
		"tasks/work-items.yaml": "phases: {}\nitems: []\n",
		"people.yaml":           "- a\n",
	})
	code, found, _, _, err := configFindings("")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range found {
		got = append(got, f.Area+": "+f.Message)
	}
	want := []string{
		"config: itos.yaml: shell names no program, so no command can run under it",
		"config: itos.yaml: ledger.group.label is skip, and --skip is already another flag of itos task, so it never takes a group",
	}
	if code != ExitPolicy || !slices.Equal(got, want) {
		t.Errorf("got %d %q, want %d %q", code, got, ExitPolicy, want)
	}

	writeFiles(t, map[string]string{"tasks/work-items.yaml": "phases: [\n"})
	if code, found, _, _, err := configFindings(""); err == nil || code != 0 || found != nil {
		t.Errorf("a registry that cannot be read: %d %v %v", code, found, err)
	}
}
