package configcheck

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// taskFlags stand in for itos task's flags that name no group.
var taskFlags = []string{"--config", "--root", "--json", "--skip", "--pending", "--title"}

// The group label is a flag of itos task's (slice 129): one word that is no
// other of taskFlags.
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
		found := groupLabelProblems(cfg, taskFlags)
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
		if found := groupLabelProblems(cfg, taskFlags); len(found) != 1 || found[0].Fix != fix {
			t.Errorf("%q: got %+v, want the fix %q", label, found, fix)
		}
	}
}

// Tagged puts each problem in the area, as it was.
func TestTagged(t *testing.T) {
	problems := []out.Problem{{Rule: "a", Message: "one"}, {Rule: "b", Message: "two", Fix: "do"}}
	got := Tagged("ledger", problems)
	want := []Found{{problems[0], "ledger"}, {problems[1], "ledger"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := Tagged("ledger", nil); got == nil || len(got) != 0 {
		t.Errorf("none: got %#v, want an empty list", got)
	}
}

// configText names the ledger, its group label and the people.
const configText = "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { people: { source: yaml, file: people.yaml } }\n"

// sound is a project config check finds nothing in.
func sound() map[string]string {
	return map[string]string{
		"itos.yaml":             configText,
		"tasks/phase-1.yaml":    "[]\n",
		"tasks/work-items.yaml": "phases: { 1: null }\nitems: [{ id: p1-a, title: A, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: A }]\n",
		"people.yaml":           "- a\n",
	}
}

// with is files, each of more set over them, "" removing one.
func with(files map[string]string, more map[string]string) map[string]string {
	all := map[string]string{}
	for p, text := range files {
		all[p] = text
	}
	for p, text := range more {
		if text == "" {
			delete(all, p)
			continue
		}
		all[p] = text
	}
	return all
}

// result is what Findings gives.
type result struct {
	code            int
	found, warnings []Found
	lines           []string
	err             error
}

// findingsOf is Findings over the working tree holding files, from an empty
// folder with no ITOS_CONFIG, the ledger file given.
func findingsOf(t *testing.T, files map[string]string, ledgerFile string) result {
	t.Helper()
	t.Setenv("ITOS_CONFIG", "")
	t.Chdir(t.TempDir())
	trees := treefake.Trees{Files: map[string]map[string]string{"worktree": files}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	var r result
	r.code, r.found, r.warnings, r.lines, r.err = Findings(t.Context(), &vcsfake.VCS{}, trees, src, ledgerFile, taskFlags, nil)
	return r
}

// messages are each finding's area and message.
func messages(found []Found) []string {
	var got []string
	for _, f := range found {
		got = append(got, f.Area+": "+f.Message)
	}
	return got
}

// A sound project: no problem, no warning, code 0, and the lines that say
// so, the ledger files counted, the given one alone when there is one.
func TestFindingsSound(t *testing.T) {
	r := findingsOf(t, sound(), "")
	want := []string{"itos.yaml is valid, and so are the 1 ledger files it reads", "tasks/work-items.yaml: sound"}
	if r.err != nil || r.code != 0 || len(r.found) != 0 || len(r.warnings) != 0 || !slices.Equal(r.lines, want) {
		t.Errorf("got %+v, want the lines %q", r, want)
	}
	r = findingsOf(t, with(sound(), map[string]string{"tasks/phase-2.yaml": "[]\n"}), "tasks/phase-1.yaml")
	if r.err != nil || r.code != 0 || r.lines[0] != "itos.yaml is valid, and so are the 1 ledger files it reads" {
		t.Errorf("one ledger file given: %+v", r)
	}
	r = findingsOf(t, with(sound(), map[string]string{"tasks/phase-2.yaml": "[]\n"}), "")
	if r.err != nil || r.code != 0 || r.lines[0] != "itos.yaml is valid, and so are the 2 ledger files it reads" {
		t.Errorf("two ledger files: %+v", r)
	}
}

// A config that is invalid, or names a since this repository does not
// have, and a ledger folder that is missing, are the one problem, code 2.
func TestFindingsCannotRead(t *testing.T) {
	for name, c := range map[string]struct {
		files map[string]string
		want  string
	}{
		"invalid": {
			with(sound(), map[string]string{"itos.yaml": "version: 1\nnope: 1\n"}),
			"config: itos.yaml: ",
		},
		"unknown since": {
			with(sound(), map[string]string{"itos.yaml": configText + "commits: { since: 0123456789abcdef0123456789abcdef01234567 }\n"}),
			"config: itos.yaml: commits.since names 0123456789abcdef0123456789abcdef01234567, which is not a commit of this repository",
		},
		"no ledger folder": {
			with(sound(), map[string]string{"tasks/phase-1.yaml": "", "itos.yaml": "version: 1\nledger: { files: \"ledger/phase-{group}.yaml\" }\nwork: { people: { source: yaml, file: people.yaml } }\n"}),
			"ledger: ",
		},
	} {
		r := findingsOf(t, c.files, "")
		got := messages(r.found)
		if r.err != nil || r.code != 2 || len(got) != 1 || !strings.HasPrefix(got[0], c.want) || r.warnings != nil || r.lines != nil {
			t.Errorf("%s: got %d %q %+v %q %v, want one problem starting %q", name, r.code, got, r.warnings, r.lines, r.err, c.want)
		}
	}
}

// The values nothing reads as written, the ledger layout's, the registry's,
// the questions', the smoke set's and the decision records' are problems,
// code 1, the config's each after the file's name; the layout's keep the
// ledger's files from being read.
func TestFindingsProblems(t *testing.T) {
	r := findingsOf(t, with(sound(), map[string]string{
		"itos.yaml":             "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", group: { label: skip } }\nwork: { people: { source: yaml, file: people.yaml } }\n",
		"tasks/work-items.yaml": "phases: {}\nitems: [{ id: p1-a, title: A, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: A }]\n",
		"tasks/asks.yaml":       "questions:\n  - { id: q-1, item: p1-a, question: \"Why?\" }\n  - { id: q-2, item: p1-b, question: \"How?\" }\n",
	}), "")
	want := []string{
		"config: itos.yaml: ledger.group.label is skip, and --skip is already another flag of itos task, so it never takes a group",
		"registry: p1-a: skip 1 is not listed",
		"asks: tasks/asks.yaml: q-2 names the item p1-b, which tasks/work-items.yaml does not have; " +
			"by hand, set q-2's item in tasks/asks.yaml to p1-b's new id, or remove q-2's item key",
	}
	if got := messages(r.found); r.err != nil || r.code != 1 || !slices.Equal(got, want) {
		t.Errorf("got %d %q %v, want %q", r.code, got, r.err, want)
	}

	r = findingsOf(t, with(sound(), map[string]string{
		"itos.yaml":       "version: 1\nledger: { files: \"tasks/{group}/phase.yaml\" }\nwork: { registry: tasks/work-items.yaml, people: { source: yaml, file: people.yaml } }\n",
		"tasks/asks.yaml": "questions:\n  - { id: q-1, question: \"Why?\" }\n  - { id: q-1, question: \"How?\" }\n",
	}), "")
	want = []string{
		"config: itos.yaml: ledger.files has {group} in its folder: tasks/{group}/phase.yaml; the ledger is one folder, and a group is a part of a file's name",
		"asks: tasks/asks.yaml: two questions have the id q-1",
	}
	if got := messages(r.found); r.err != nil || r.code != 1 || !slices.Equal(got, want) || r.lines[0] != "itos.yaml is valid, and so are the 0 ledger files it reads" {
		t.Errorf("a layout: got %d %q %q %v, want %q", r.code, got, r.lines, r.err, want)
	}
	if r.found[1].Rule != "asks-invalid" || r.found[1].Fix != "correct tasks/asks.yaml to what itos decision writes, or undo the hand edit" {
		t.Errorf("an invalid questions' file: %+v", r.found[1])
	}

	r = findingsOf(t, with(sound(), map[string]string{"docs/decisions/0001-one.md": "# One\n", "docs/decisions/0001-two.md": "# Two\n"}), "")
	if r.err != nil || r.code != 1 || len(r.found) != 1 || r.found[0].Area != "decisions" || r.found[0].Rule != "decisions-number-twice" {
		t.Errorf("a decision record: %+v", r)
	}
}

// A registry whose alias the reader refuses is the registry's problem
// alone, so the questions name no unknown item beside it.
func TestFindingsRegistryRefused(t *testing.T) {
	r := findingsOf(t, with(sound(), map[string]string{
		"tasks/asks.yaml":       "questions:\n  - { id: q-1, item: p1-a, question: \"Why?\" }\n",
		"tasks/work-items.yaml": "phases: {}\nitems: &a [*a]\n",
	}), "")
	if r.err != nil || r.code != 1 || slices.ContainsFunc(r.found, func(f Found) bool { return f.Area == "asks" }) {
		t.Errorf("a registry with a cycle: %+v", r)
	}
}

// A registry, a questions' file, a ledger folder or a decisions folder that
// cannot be read is an error, with no code, finding or line.
func TestFindingsUnreadable(t *testing.T) {
	for name, more := range map[string]map[string]string{
		"registry":  {"tasks/work-items.yaml": "phases: [\n"},
		"questions": {"tasks/asks.yaml/x": "x"},
		"ledger folder": {
			"itos.yaml": "version: 1\nledger: { files: \"ledger/phase-{group}.yaml\" }\nwork: { people: { source: yaml, file: people.yaml } }\n",
			"ledger":    "x",
		},
		"decisions": {
			"itos.yaml":  "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { decisions: decided.md, people: { source: yaml, file: people.yaml } }\n",
			"decided.md": "x",
		},
	} {
		r := findingsOf(t, with(sound(), more), "")
		if r.err == nil || r.code != 0 || r.found != nil || r.warnings != nil || r.lines != nil {
			t.Errorf("%s: got %+v, want an error alone", name, r)
		}
	}
}

// The ledger's, the people's and the answered questions' warnings never
// set the code; an open question naming no item of the registry's is a
// problem (bug 105).
func TestFindingsWarnings(t *testing.T) {
	r := findingsOf(t, with(sound(), map[string]string{
		"people.yaml": "",
		"tasks/asks.yaml": "questions:\n  - { id: q-1, item: p1-gone, question: \"Why?\", answer: \"So.\" }\n" +
			"  - { id: q-2, item: p1-a, question: \"How?\", answer: \"Thus.\" }\n  - { id: q-3, question: \"When?\", answer: \"Now.\" }\n",
	}), "")
	if r.err != nil || r.code != 0 || len(r.found) != 0 || len(r.warnings) != 2 || r.warnings[0].Area != "people" {
		t.Fatalf("answered: %+v", r)
	}
	want := Found{Area: "asks"}
	want.Rule = "asks-answered-unknown-item"
	want.Fix = "by hand, set q-1's item in tasks/asks.yaml to p1-gone's new id, or remove q-1's item key"
	want.Message = "tasks/asks.yaml: q-1, answered, names the item p1-gone, which tasks/work-items.yaml does not have; " + want.Fix
	if r.warnings[1] != want {
		t.Errorf("warning %+v, want %+v", r.warnings[1], want)
	}
	r = findingsOf(t, with(sound(), map[string]string{"tasks/asks.yaml": "questions:\n  - { id: q-1, item: p1-gone, question: \"Why?\" }\n"}), "")
	want = Found{Area: "asks"}
	want.Rule = "asks-unknown-item"
	want.Fix = "by hand, set q-1's item in tasks/asks.yaml to p1-gone's new id, or remove q-1's item key"
	want.Message = "tasks/asks.yaml: q-1 names the item p1-gone, which tasks/work-items.yaml does not have; " + want.Fix
	if r.err != nil || r.code != 1 || len(r.found) != 1 || r.found[0] != want || len(r.warnings) != 0 {
		t.Errorf("open: %+v", r)
	}
}

// The repair names the questions' file the config sets, not tasks/asks.yaml,
// in the message and in the fix, for an answered question and an open one.
func TestFindingsRepairNamesTheConfiguredAsks(t *testing.T) {
	r := findingsOf(t, with(sound(), map[string]string{
		"itos.yaml": "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { asks: data/questions.yaml, people: { source: yaml, file: people.yaml } }\n",
		"data/questions.yaml": "questions:\n  - { id: q-1, item: p1-gone, question: \"Why?\", answer: \"So.\" }\n" +
			"  - { id: q-2, item: p1-lost, question: \"How?\" }\n",
	}), "")
	if r.err != nil || r.code != 1 || len(r.found) != 1 || len(r.warnings) != 1 {
		t.Fatalf("got %+v", r)
	}
	for _, c := range []struct {
		got          Found
		message, fix string
	}{
		{r.warnings[0], "data/questions.yaml: q-1, answered, names the item p1-gone, which tasks/work-items.yaml does not have; ",
			"by hand, set q-1's item in data/questions.yaml to p1-gone's new id, or remove q-1's item key"},
		{r.found[0], "data/questions.yaml: q-2 names the item p1-lost, which tasks/work-items.yaml does not have; ",
			"by hand, set q-2's item in data/questions.yaml to p1-lost's new id, or remove q-2's item key"},
	} {
		if c.got.Message != c.message+c.fix || c.got.Fix != c.fix {
			t.Errorf("got %+v, want message %q and fix %q", c.got, c.message+c.fix, c.fix)
		}
	}
}

const smokedConfig = `version: 1
tests:
  scenario: { root: features, id: "ID-[A-Z]+-\\d+", tag_prefix: "@", smoke: { file: features/smoke.yaml, every_file: false } }
  every: { root: other, id: "OT-[A-Z]+-\\d+", tag_prefix: "@", smoke: { file: other/smoke.yaml, every_file: true } }
  unsmoked: { root: features, id: "ID-[A-Z]+-\\d+" }
`

// smokedOf is the config smokedConfig over the working tree holding files,
// written to the folder it runs in too, where the tests' kinds find their
// files.
func smokedOf(t *testing.T, files map[string]string) *config.Loaded {
	t.Helper()
	t.Chdir(t.TempDir())
	for p, text := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	trees := treefake.Trees{Files: map[string]map[string]string{"worktree": files}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(&vcsfake.VCS{}, trees, src, "itos.yaml", smokedConfig, false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// The kinds judged picks are the ones judged, among those with a smoke set
// (unsmoked has none): a sound set prints its line (every file's with
// every_file, else that every smoke test is live), a set's own problems are
// in the area smoke, and a set that cannot be read is one smoke-unreadable
// problem.
func TestJudgedSmokeFindings(t *testing.T) {
	files := map[string]string{
		"features/a.feature":  "Feature: A\n\n  @ID-A-01\n  Scenario: One\n    When it runs\n",
		"features/smoke.yaml": "- file: a.feature\n  scenarios: [{ id: \"@ID-A-01\", why: listed }]\n",
		"other/b.feature":     "Feature: B\n\n  @OT-B-01\n  Scenario: One\n    When it runs\n",
		"other/smoke.yaml":    "- file: b.feature\n  scenarios: [{ id: \"@OT-B-01\", why: listed }]\n",
	}
	cfg := smokedOf(t, files)
	found, lines := SmokeFindings(cfg, &tests.Warnings{})
	want := []string{"features/smoke.yaml: every smoke test is live (1 in all)", "other/smoke.yaml: every file has a smoke test (1 in all)"}
	if len(found) != 0 || !slices.Equal(lines, want) {
		t.Errorf("sound sets: %v %q", found, lines)
	}
	var asked []string
	found, lines = JudgedSmokeFindings(cfg, func(name, file string) bool {
		asked = append(asked, name+" "+file)
		return name == "every"
	}, nil)
	if len(found) != 0 || !slices.Equal(lines, want[1:]) || !slices.Equal(asked, []string{"scenario features/smoke.yaml", "every other/smoke.yaml"}) {
		t.Errorf("a kind left out: %v %q, asked %q", found, lines, asked)
	}

	cfg = smokedOf(t, with(files, map[string]string{"features/smoke.yaml": "- file: a.feature\n  scenarios: [{ id: \"@ID-A-09\", why: gone }]\n"}))
	found, _ = JudgedSmokeFindings(cfg, nil, nil)
	if len(found) != 1 || found[0].Area != "smoke" || found[0].Rule == "smoke-unreadable" {
		t.Errorf("a set naming no live test: %+v", found)
	}

	cfg = smokedOf(t, with(files, map[string]string{"features/smoke.yaml": "- [\n"}))
	found, _ = JudgedSmokeFindings(cfg, nil, nil)
	if len(found) != 1 || found[0].Rule != "smoke-unreadable" || found[0].Fix != "correct features/smoke.yaml" || found[0].Area != "smoke" {
		t.Errorf("a set that cannot be read: %v", found)
	}
}
