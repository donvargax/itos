package message

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v3/internal/config"
)

// A footer's IDs over several lines, split at whitespace and commas, the
// prefix taken off where it is given; a line that only mentions the key is
// not the footer.
func TestIDs(t *testing.T) {
	text := "feat: x\n\nScenarios: @ID-A-1, @ID-B-2\nScenarios:ID-C-3 @ID-D-4\r\nSee Scenarios: ID-E-5\n"
	got := IDs(text, "Scenarios", "@")
	if want := []string{"ID-A-1", "ID-B-2", "ID-C-3", "ID-D-4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IDs %q", got)
	}
	if got := Type("feat(scope): x"); got != "feat" {
		t.Errorf("Type %q", got)
	}
	if got := Type(": x"); got != "" {
		t.Errorf("Type %q", got)
	}
}

// A free-text footer's texts, a line each, trimmed, the empty ones kept; a
// line that only mentions the key is not the footer. None is the word alone,
// in lower case.
func TestTexts(t *testing.T) {
	text := "chore: x\n\nUpgrading:  rename a key \nUpgrading:\nSee Upgrading: below\nUpgrading: none\n"
	if got, want := Texts(text, "Upgrading"), []string{"rename a key", "", "none"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Texts %q", got)
	}
	for text, want := range map[string]bool{"none": true, "None": false, "none.": false, "": false} {
		if None(text) != want {
			t.Errorf("None(%q) is %v", text, !want)
		}
	}
}

func load(t *testing.T, text string) *config.Loaded {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// The delegate's report: each problem line with its level and fix, the rest
// left out, and a failure with no error among them one header-lint problem.
func TestParseReport(t *testing.T) {
	cfg := load(t, "version: 1\n")
	report := "⧗   input: x\n  ✖   subject may not be empty [subject-empty]  \n⚠  lines are long [body-max-line-length]\n✖ [type-empty]\n"
	got, err := ParseReport(cfg, report, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []Leveled{
		{Rule: "subject-empty", Message: "subject may not be empty", Fix: fixes["subject-empty"], Level: "error"},
		{Rule: "body-max-line-length", Message: "lines are long", Fix: fixes["body-max-line-length"], Level: "warning"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseReport %+v", got)
	}
	got, _ = ParseReport(cfg, "\n  no config  \n", true)
	if len(got) != 1 || got[0].Rule != "header-lint" || got[0].Message != "no config" {
		t.Errorf("ParseReport of a bare failure %+v", got)
	}
}

// The footer a flag of itos commit writes is the first of its source, found
// by the source and never the key; its IDs are packed into lines within the
// limit, the key on each, an ID too long for any line alone on its own.
func TestFooterLines(t *testing.T) {
	cfg := load(t, `version: 1
ledger: { files: "tasks/phase-{group}.yaml", id: "T-\\d+" }
tests: { scenario: { root: features, id: "ID-[A-Z]+-\\d+" } }
commits:
  footers:
    Why: { source: text }
    Covers: { source: { tests: scenario } }
    Work: { source: ledger }
    Also: { source: ledger }
`)
	if key, ok := LedgerFooter(cfg); key != "Work" || !ok {
		t.Errorf("LedgerFooter %q %v", key, ok)
	}
	if key, ok := TestsFooter(cfg); key != "Covers" || !ok {
		t.Errorf("TestsFooter %q %v", key, ok)
	}
	if key, ok := TestsFooter(load(t, "version: 1\n")); key != "" || ok {
		t.Errorf("TestsFooter of no footers %q %v", key, ok)
	}
	if got, want := SplitIDs(" @ID-A-01,@ID-A-02  @ID-A-03, "), []string{"@ID-A-01", "@ID-A-02", "@ID-A-03"}; !reflect.DeepEqual(got, want) {
		t.Errorf("SplitIDs %q", got)
	}
	var ids []string
	for range 10 {
		ids = append(ids, "@ID-COMMITCMD-01")
	}
	long := "@ID-" + strings.Repeat("X", 120)
	lines := FooterLines("Scenarios", append(ids, long, "@ID-A-01"))
	for i, l := range lines {
		if !strings.HasPrefix(l, "Scenarios: @") || (len(l) > maxLength && l != "Scenarios: "+long) {
			t.Errorf("line %d %q", i, l)
		}
	}
	if len(lines) != 4 || lines[2] != "Scenarios: "+long || lines[3] != "Scenarios: @ID-A-01" {
		t.Errorf("FooterLines %q", lines)
	}
	if got := IDs(strings.Join(lines, "\n"), "Scenarios", ""); len(got) != 12 {
		t.Errorf("the lines read back as %d IDs", len(got))
	}
	if got := FooterLines("Task", nil); got != nil {
		t.Errorf("FooterLines of no IDs %q", got)
	}
}

// Under a stealth config the footer rules read the links from the note,
// never the message: one typed into the message is its footer's problem,
// naming the itos commit flag that writes it, and a missing one says the
// same; a footer of free text is the message's in either mode (slice 36); a
// project's config reads the message and no note.
func TestStealthFooters(t *testing.T) {
	cfg := load(t, `version: 1
ledger: { files: "tasks/phase-{group}.yaml", id: "T-\\d+" }
commits:
  footers:
    Task: { source: ledger, required_for: [chore], validate_for: [] }
    Why: { source: text, required_for: [docs] }
`)
	problems := func(message, note string) []string {
		t.Helper()
		found, err := FooterProblems(cfg, message, Reading{Note: note})
		if err != nil {
			t.Fatal(err)
		}
		var said []string
		for _, p := range found {
			said = append(said, p.Rule+": "+p.Message+" | "+p.Fix)
		}
		return said
	}
	if got := problems("chore: x\n", "Task: T-1"); len(got) != 1 || !strings.Contains(got[0], "need a") {
		t.Errorf("a project's config read the note: %q", got)
	}
	cfg.Stealth = true
	if got := problems("chore: x\n", "Task: T-1\n"); got != nil {
		t.Errorf("the note's footer %q", got)
	}
	got := problems("chore: x\n\nTask: T-1\n", "Task: T-1")
	if len(got) != 1 || !strings.HasPrefix(got[0], "task-footer: the Task: footer is in the message") ||
		!strings.Contains(got[0], "itos commit --task writes it") || !strings.Contains(got[0], "| take the footer out") {
		t.Errorf("a typed footer %q", got)
	}
	got = problems("chore: x\n", "")
	if len(got) != 1 || !strings.Contains(got[0], `need a "Task: T-…" footer, which in stealth mode itos commit --task`) ||
		!strings.Contains(got[0], "| commit with itos commit --task <id>") {
		t.Errorf("a missing footer %q", got)
	}
	if got = problems("chore: x\n\nWhy: because\n", "Task: T-1"); got != nil {
		t.Errorf("a footer of free text in the message %q", got)
	}
	if got = problems("docs: x\n", "Why: because"); len(got) != 1 || !strings.HasPrefix(got[0], "why-footer: docs commits need") {
		t.Errorf("a footer of free text read from the note %q", got)
	}
}

// Each footer of free text has a flag of its name in lower case, unless a
// built-in flag has that name; Missing names the required footers a commit
// lacks, links read from one text and content from another.
func TestTextFlagsAndMissing(t *testing.T) {
	cfg := load(t, `version: 1
ledger: { files: "tasks/phase-{group}.yaml", id: "T-\\d+" }
commits:
  footers:
    Task: { source: ledger, required_for: [chore] }
    Upgrading: { source: text, required_for: [chore] }
    Breaking: { source: text }
`)
	if got := TextFlags(cfg); len(got) != 1 || got["--upgrading"] != "Upgrading" {
		t.Errorf("text flags %q", got)
	}
	for key, want := range map[string]string{"Task": "--task <id>", "Upgrading": "--upgrading <text|none>", "Breaking": " "} {
		if flag, what := Flag(cfg, key); flag+" "+what != want {
			t.Errorf("Flag(%s) = %q %q", key, flag, what)
		}
	}
	if got := Missing(cfg, "chore", "Upgrading: none", "Task: T-1"); !slices.Equal(got, []string{"Task", "Upgrading"}) {
		t.Errorf("missing from the wrong texts %q", got)
	}
	if got := Missing(cfg, "chore", "Task: T-1", "Upgrading: none"); got != nil {
		t.Errorf("missing %q", got)
	}
	if got := Missing(cfg, "feat", "", ""); got != nil {
		t.Errorf("missing for a type none requires %q", got)
	}
}

// Links are the IDs of every footer of IDs, in the config's order of
// footers, each footer's prefix taken off; a footer of free text gives none.
func TestLinks(t *testing.T) {
	cfg := load(t, `version: 1
ledger: { files: "tasks/phase-{group}.yaml", id: "T-\\d+" }
commits:
  footers:
    Scenarios: { source: { tests: scenario }, strip_prefix: "@" }
    Task: { source: ledger }
    Upgrading: { source: text }
tests:
  scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
`)
	got := Links(cfg, "fix: x\n\nTask: T-1\nScenarios: @ID-A-01, ID-B-02\nUpgrading: none\n")
	want := []Link{{Key: "Scenarios", ID: "ID-A-01", Tests: "scenario"}, {Key: "Scenarios", ID: "ID-B-02", Tests: "scenario"}, {Key: "Task", ID: "T-1"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Links %+v", got)
	}
}

// A footer of the work registry is written by --item and stands in for the
// ledger's where its in_place_of says (slice 63): a type it does not name
// still needs its own, and a stand-in with no ID stands in for nothing. Its
// IDs are links, the items' own.
func TestInPlaceOf(t *testing.T) {
	cfg := load(t, `version: 1
ledger: { files: "tasks/phase-{group}.yaml", id: "T-\\d+" }
commits:
  footers:
    Task: { source: ledger, required_for: [test, refactor] }
    Item: { source: registry, in_place_of: { Task: [test, docs] } }
`)
	if flag, what := Flag(cfg, "Item"); flag != "--item" || what != "<id>" {
		t.Errorf("Flag(Item) = %q %q", flag, what)
	}
	if key, ok := RegistryFooter(cfg); !ok || key != "Item" {
		t.Errorf("RegistryFooter = %q %v", key, ok)
	}
	for _, c := range []struct {
		typ, links string
		want       []string
	}{
		{"test", "Item: slice-9", nil},
		{"test", "Task: T-1", nil},
		{"test", "Item:", []string{"Task"}},
		{"refactor", "Item: slice-9", []string{"Task"}},
	} {
		if got := Missing(cfg, c.typ, c.links, ""); !slices.Equal(got, c.want) {
			t.Errorf("Missing(%s, %q) = %q, want %q", c.typ, c.links, got, c.want)
		}
	}
	if got, want := Links(cfg, "docs: x\n\nItem: slice-9\n"), []Link{{Key: "Item", ID: "slice-9"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("Links %+v", got)
	}
}
