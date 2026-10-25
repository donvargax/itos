package message

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcstest "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// Gathered reads a range's free-text footers through the config's VCS,
// every commit up to the end from an all-zero start, the merges and the
// footers that say none left out, and the comment character the config's.
func TestGatheredReadsTheVCS(t *testing.T) {
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v := &vcstest.VCS{Comment: ";", History: []vcstest.Commit{
		{Name: "c-a", Message: "feat: a\n\nUpgrading: run x\n"},
		{Name: "c-b", Parents: []string{"c-a"}, Message: "fix: b\n\nUpgrading: none\n; Upgrading: a comment\n"},
		{Name: "c-m", Parents: []string{"c-b", "c-a"}, Message: "Merge\n\nUpgrading: merged\n"},
	}}
	cfg, err := config.Load(v, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, file)
	if err != nil {
		t.Fatal(err)
	}
	want := []Said{{SHA: "c-a", Subject: "feat: a", Text: "run x"}}
	if got, err := Gathered(cfg, "0000000", "c-m", "Upgrading"); !reflect.DeepEqual(got, want) || err != nil {
		t.Errorf("Gathered from zeros = %+v, %v; want %+v", got, err, want)
	}
	if got, err := Gathered(cfg, "nope", "c-m", "Upgrading"); err == nil {
		t.Errorf("Gathered of no commit = %+v", got)
	}
}

// A footer's IDs over several lines, split at whitespace and commas, the
// prefix taken off where it is given, read as git reads trailers (bug 110):
// the last paragraph, a continuation line's IDs the footer's; a line of the
// body beginning with the key, or a footer above the last paragraph, is not
// the footer; under a stealth config every line of the note is.
func TestIDs(t *testing.T) {
	cfg := load(t, "version: 1\ncommits:\n  footers:\n    Scenarios: { source: { tests: scenario }, strip_prefix: \"@\" }\n"+
		"tests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\n")
	text := "feat: x\n\nScenarios: ID-Z-9\n\nScenarios: or a footer, as\nScenarios: says.\n\n" +
		"Scenarios: @ID-A-1, @ID-B-2\nScenarios :ID-C-3 @ID-D-4\r\n  @ID-E-5\nSigned-off-by: A <a@b>\n# a comment\n\n"
	if got, want := IDs(cfg, text, "Scenarios"), []string{"ID-A-1", "ID-B-2", "ID-C-3", "ID-D-4", "ID-E-5"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IDs %q", got)
	}
	cfg.Stealth = true
	if got, want := IDs(cfg, "Scenarios: @ID-A-1\nScenarios: ID-B-2\n", "Scenarios"), []string{"ID-A-1", "ID-B-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IDs of a note %q", got)
	}
	if got := Type("feat(scope): x"); got != "feat" {
		t.Errorf("Type %q", got)
	}
	if got := Type(": x"); got != "" {
		t.Errorf("Type %q", got)
	}
}

// The trailers are git's (trailer.c): the last paragraph, never the first,
// when each of its lines is a trailer or a continuation, or when it holds
// one git writes and a quarter of its lines are trailers; the blank and
// comment lines at the end, and git's scissors line and what follows it,
// are not the message's. BREAKING CHANGE, its space and all, is a key, as
// the Conventional Commits spec has it, where git reads none with a space.
func TestTrailers(t *testing.T) {
	for message, want := range map[string][]trailer{
		"chore: x":                        nil,
		"Task: T-1":                       nil,
		"chore: x\nTask: T-1":             nil,
		"chore: x\n\nTask: T-1":           {{"Task", "T-1"}},
		"chore: x\n\nTask: T-1\n\n\n":     {{"Task", "T-1"}},
		"chore: x\n\nTask: T-1\nprose":    nil,
		"chore: x\n\nprose\nTask: T-1":    nil,
		"chore: x\n\n: T-1":               nil,
		"chore: x\n\nA key: T-1":          nil,
		"chore: x\n\nTask: T-1\n\nprose":  nil,
		"chore: x\n\nTask: T-1\n  T-2":    {{"Task", "T-1\n  T-2"}},
		"chore: x\n\n  T-0\nTask: T-1":    nil,
		"chore: x\n\nTask: T-1\n# a note": {{"Task", "T-1"}},
		"chore: x\n\nTask: T-1\n# ------------------------ >8 ------------------------\nprose": {{"Task", "T-1"}},
		"chore: x\n\nprose\n  more\nSigned-off-by: A\n(cherry picked from commit 1)\nTask: T-1": {
			{"Signed-off-by", "A"}, {"Task", "T-1"}},
		"chore: x\n\nprose\nmore\nyet more\nstill more\nSigned-off-by: A": nil,
		"chore: x\n\nprose\nSigned-off-by: A\n\t T-2\n  T-3":              {{"Signed-off-by", "A\n\t T-2\n  T-3"}},
		"chore: x\n\n\t\nTask: T-1":                                       {{"Task", "T-1"}},
		"chore: x\n\nhttps://x.y/z":                                       nil,
		"chore: x\n\nTask: T-1\n# c\nItem: b":                             {{"Task", "T-1"}, {"Item", "b"}},
		"chore: x\n\n# c\nTask: T-1":                                      {{"Task", "T-1"}},
		"chore: x\n\nprose\n  more\n# c\nSigned-off-by: A":                {{"Signed-off-by", "A"}},
		"chore: x\n\nBREAKING CHANGE: x\nTask: T-1":                       {{"BREAKING CHANGE", "x"}, {"Task", "T-1"}},
		"chore: x\n\nBREAKING CHANGES: x\nTask: T-1":                      nil,
		"chore: x\n\nSee : https://x.y/z":                                 {{"See", "https://x.y/z"}},
		"chore: x\n\nA : T-1":                                             {{"A", "T-1"}},
	} {
		if got := trailers(message, "#"); !reflect.DeepEqual(got, want) {
			t.Errorf("trailers(%q) = %q, want %q", message, got, want)
		}
	}
	if got := unfolded("a\r\n\t b \n c "); got != "a\r b  c" {
		t.Errorf("unfolded %q", got)
	}
	for message, want := range map[string]string{
		"chore: x":                    "chore: x\n\nTask: T-1",
		"chore: x\n\nbody\n":          "chore: x\n\nbody\n\nTask: T-1",
		"chore: x\n\nItem: a\n# note": "chore: x\n\nItem: a\nTask: T-1",
	} {
		if got := Trailed(message, []string{"Task: T-1"}, "#"); got != want {
			t.Errorf("Trailed(%q) = %q, want %q", message, got, want)
		}
	}
	if got := Trailed("chore: x", nil, "#"); got != "chore: x" {
		t.Errorf("Trailed with no footers %q", got)
	}
}

// The headers git writes itself are judged by what they stand for (bug 30):
// a revert's or a reapply's as revert, an amend!, fixup! or squash! commit,
// its prefixes off, as the header it names; a merge's header keeps its word.
// A named header with no type of the commit types is refused, under its own
// rule; a revert named is a revert.
func TestTypeOfGitsHeaders(t *testing.T) {
	for message, want := range map[string]string{
		"Revert \"chore: tidy\"\n\nThis reverts commit abc.\n": "revert",
		"Reapply \"chore: tidy\"\n":                            "revert",
		"revert: chore: tidy\n":                                "revert",
		"Reverted the page\n":                                  "Reverted",
		"fixup! chore: tidy\n\nTask: T-1\n":                    "chore",
		"fixup! squash! amend! docs(x): a page\n":              "docs",
		"fixup!feat: a page\n":                                 "feat",
		"fixup! Revert \"chore: tidy\"\n":                      "revert",
		"fixup! update things\n":                               "update",
		"Merge branch 'topic'\n":                               "Merge",
	} {
		if got := Type(message); got != want {
			t.Errorf("Type(%q) = %q, want %q", message, got, want)
		}
	}
	cfg := load(t, "version: 1\ncommits:\n  types: [feat, fix, chore, docs]\n")
	for message, want := range map[string]string{
		"fixup! chore: tidy\n":            "",
		"fixup! Revert \"chore: tidy\"\n": "",
		"chore: tidy\n":                   "",
		"Revert \"chore: tidy\"\n":        "",
		"fixup! update things\n":          `fixup! names the header "update things", which has no type of [feat, fix, chore, docs]`,
		"squash! fixup! wip: x\n":         `squash! names the header "wip: x", which has no type of [feat, fix, chore, docs]`,
	} {
		got := ""
		if found := namedProblems(cfg, message); len(found) > 0 {
			got = found[0].Message
			if found[0].Rule != "named-type" {
				t.Errorf("namedProblems(%q) under the rule %q", message, found[0].Rule)
			}
		}
		if got != want {
			t.Errorf("namedProblems(%q) = %q, want %q", message, got, want)
		}
	}
	if !Typed(cfg, "docs") || Typed(cfg, "revert") || !Typed(load(t, "version: 1\ncommits: {}\n"), "revert") {
		t.Error("Typed does not take commits.types, or config-conventional's without them")
	}
}

// A free-text footer's texts, a line each, trimmed, the empty ones kept; a
// line that only mentions the key is not the footer. None is the word alone,
// in lower case.
func TestTexts(t *testing.T) {
	text := "chore: x\n\nUpgrading: below\n\nSee Upgrading: below\nUpgrading: so\n\n" +
		"Upgrading:  rename a key \nUpgrading:\nUpgrading: drop\n  the old one\nUpgrading: none\n"
	if got, want := Texts(text, "Upgrading", "#"), []string{"rename a key", "", "drop the old one", "none"}; !reflect.DeepEqual(got, want) {
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
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, file)
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
	if got := IDs(cfg, "feat: x\n\n"+strings.Join(lines, "\n"), "Scenarios"); len(got) != 12 {
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

// A footer read at a commit where its source lists nothing, as a project's
// first commits predate it, is read against the working tree, a warning
// saying so.
func TestFooterOfATreeWithoutItsSourceReadsTheWorkingTree(t *testing.T) {
	t.Chdir(t.TempDir())
	lists := `[ "$3" = worktree ] && t='{"id":"U-1","file":"a_test.go","live":true}'` + "\n" +
		`printf '{"protocol":1,"tests":[%s],"files":[]}\n' "$t"` + "\n"
	if err := os.WriteFile("lists.sh", []byte(lists), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := load(t, `version: 1
commits:
  types: [test]
  footers:
    Tests: { source: { tests: unit }, read_at: commit, validate_for: [test] }
tests:
  unit: { adapter: { command: sh lists.sh, supports_at: true }, tag_prefix: "" }
`)
	for id, unknown := range map[string]bool{"U-1": false, "U-2": true} {
		var warn strings.Builder
		said, err := CheckFooter(cfg, "Tests", "test", "test: x\n\nTests: "+id+"\n", Reading{Warn: &warn})
		if err != nil || (len(said) > 0) != unknown || !strings.Contains(warn.String(), "index has no") ||
			!strings.Contains(warn.String(), "read against the working tree") {
			t.Errorf("Tests: %s said %q, %v, warned %q", id, said, err, warn.String())
		}
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
	if got := Missing(cfg, "chore", "chore: x\n\nUpgrading: none", "chore: x\n\nTask: T-1"); !slices.Equal(got, []string{"Task", "Upgrading"}) {
		t.Errorf("missing from the wrong texts %q", got)
	}
	if got := Missing(cfg, "chore", "chore: x\n\nTask: T-1", "chore: x\n\nUpgrading: none"); got != nil {
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
		if got := Missing(cfg, c.typ, c.typ+": x\n\n"+c.links, ""); !slices.Equal(got, c.want) {
			t.Errorf("Missing(%s, %q) = %q, want %q", c.typ, c.links, got, c.want)
		}
	}
	if got, want := Links(cfg, "docs: x\n\nItem: slice-9\n"), []Link{{Key: "Item", ID: "slice-9"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("Links %+v", got)
	}
}

// A registry footer's source is the registry's file in the config's source:
// present, nothing to say; missing, a config error naming the file.
func TestNeedSourceOfTheRegistry(t *testing.T) {
	cfg := load(t, `version: 1
work: { registry: work-items.yaml }
commits:
  footers:
    Item: { source: registry }
`)
	f, _ := cfg.Commits.Footers.Get("Item")
	for files, want := range map[string]string{
		"work-items.yaml": "",
		"other.yaml":      "the Item: footer names items of the work registry, and its file work-items.yaml does not exist",
	} {
		source, err := porttest.Trees{Files: map[string]map[string]string{"worktree": {files: "items: []\n"}}}.At(t.Context(), "worktree")
		if err != nil {
			t.Fatal(err)
		}
		cfg.Source = source
		got := ""
		if err := needSource(cfg, "Item", f); err != nil {
			got = err.(*config.Error).Problems[0].Message
		}
		if got != want {
			t.Errorf("the registry beside %s: %q", files, got)
		}
	}
}

// A message gives a footer a flag would write, or not, as the hook reads
// footers: the IDs of every line of its key, the strip_prefix off, and the
// texts, unfolded, compared as sets; BREAKING-CHANGE and BREAKING CHANGE are
// one footer; a body line that starts with the key is no footer. A differing
// footer is a problem naming both values, an empty one "nothing".
func TestWrittenAgainstAMessage(t *testing.T) {
	cfg := load(t, "version: 1\ncommits:\n  footers:\n    Scenarios: { source: { tests: scenario }, strip_prefix: \"@\" }\n"+
		"    Upgrading: { source: text }\n"+
		"tests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\n")
	ids := func(values ...string) Written { return Written{Key: "Scenarios", Flag: "--scenarios", Values: values} }
	text := func(key string, values ...string) Written {
		flag := "--" + strings.ToLower(key)
		if key == "BREAKING-CHANGE" {
			flag = "--breaking"
		}
		return Written{Key: key, Flag: flag, Text: true, Values: values}
	}
	for _, c := range []struct {
		name    string
		w       Written
		message string
		want    Standing
		problem string
	}{
		{"no footer", ids("@ID-A-1"), "feat: x\n\nUpgrading: none\n", Absent, ""},
		{"no footer at all", ids("@ID-A-1"), "feat: x\n", Absent, ""},
		{"the same line", ids("@ID-A-1"), "feat: x\n\nScenarios: @ID-A-1\n", Same, ""},
		{"other lines, other order, a repeat", ids("@ID-B-2", "@ID-A-1", "@ID-A-1"),
			"feat: x\n\nScenarios: @ID-A-1\nScenarios: @ID-B-2,\n  @ID-A-1\n", Same, ""},
		{"the prefix is off both sides", ids("ID-A-1"), "feat: x\n\nScenarios: @ID-A-1\n", Same, ""},
		{"case counts", ids("@id-a-1"), "feat: x\n\nScenarios: @ID-A-1\n", Differs,
			"Scenarios: the message gives @ID-A-1, --scenarios gives @id-a-1"},
		{"more IDs in the message", ids("@ID-A-1"), "feat: x\n\nScenarios: @ID-A-1 @ID-B-2\n", Differs,
			"Scenarios: the message gives @ID-A-1, @ID-B-2, --scenarios gives @ID-A-1"},
		{"fewer IDs in the message", ids("@ID-A-1", "@ID-B-2"), "feat: x\n\nScenarios: @ID-A-1\n", Differs,
			"Scenarios: the message gives @ID-A-1, --scenarios gives @ID-A-1, @ID-B-2"},
		{"an empty footer", ids("@ID-A-1"), "feat: x\n\nScenarios:\n", Differs,
			"Scenarios: the message gives nothing, --scenarios gives @ID-A-1"},
		{"a body line is no footer", ids("@ID-A-1"), "feat: x\n\nScenarios: @ID-B-2\nand prose\n\nUpgrading: none\n", Absent, ""},
		{"a comment line is passed over", ids("@ID-A-1"), "feat: x\n\nScenarios: @ID-A-1\n# Scenarios: @ID-B-2\n", Same, ""},
		{"a text unfolded", text("Upgrading", "run x"), "feat: x\n\nUpgrading: run\n  x\n", Same, ""},
		{"a flag's text unfolded", text("Upgrading", "run\n  x"), "feat: x\n\nUpgrading: run x\n", Same, ""},
		{"texts, any order", text("Upgrading", "b", "a"), "feat: x\n\nUpgrading: a\nUpgrading: b\nUpgrading: a\n", Same, ""},
		{"another text", text("Upgrading", "none"), "feat: x\n\nUpgrading: run x\n", Differs,
			`Upgrading: the message gives "run x", --upgrading gives "none"`},
		{"an empty text", text("Upgrading", "none"), "feat: x\n\nUpgrading:\n", Differs,
			`Upgrading: the message gives nothing, --upgrading gives "none"`},
		{"breaking, the spaced form alone is written anyway", text("BREAKING-CHANGE", "x"), "feat: x\n\nBREAKING CHANGE: x\n", Absent, ""},
		{"breaking, the own key", text("BREAKING-CHANGE", "x"), "feat: x\n\nBREAKING-CHANGE: x\n", Same, ""},
		{"breaking, both forms with the same text", text("BREAKING-CHANGE", "x"), "feat: x\n\nBREAKING CHANGE: x\nBREAKING-CHANGE: x\n", Same, ""},
		{"breaking, the own key short of the flag's texts", text("BREAKING-CHANGE", "x", "y"), "feat: x\n\nBREAKING CHANGE: x\nBREAKING-CHANGE: y\n", Absent, ""},
		{"breaking differs", text("BREAKING-CHANGE", "x"), "feat: x\n\nBREAKING CHANGE: y\n", Differs,
			`BREAKING-CHANGE: the message gives "y", --breaking gives "x"`},
	} {
		got, problem := c.w.Against(cfg, c.message)
		if got != c.want || problem.Message != c.problem {
			t.Errorf("%s: %v, %q; want %v, %q", c.name, got, problem.Message, c.want, c.problem)
		}
	}
	_, p := text("BREAKING-CHANGE", "x").Against(cfg, "feat: x\n\nBREAKING-CHANGE: y\n")
	if p.Rule != "breaking-change-footer-differs" || p.Fix != "give BREAKING-CHANGE: once, in the message or with --breaking" {
		t.Errorf("problem %+v", p)
	}
	if got := ids("@ID-A-1", "@ID-B-2").Lines(); !slices.Equal(got, []string{"Scenarios: @ID-A-1 @ID-B-2"}) {
		t.Errorf("lines %q", got)
	}
	if got := text("Upgrading", "a", "b").Lines(); !slices.Equal(got, []string{"Upgrading: a", "Upgrading: b"}) {
		t.Errorf("lines %q", got)
	}
}

// A message that gives a breaking change only under BREAKING CHANGE, the
// spaced form, with the flag's text gets the flag's BREAKING-CHANGE line
// written, as the hook, which reads the declared key alone, would otherwise
// find the footer missing; the spaced form with another text is refused.
func TestBreakingInTheSpacedFormIsWrittenForTheHook(t *testing.T) {
	cfg := load(t, "version: 1\ncommits:\n  types: [feat]\n  footers:\n    BREAKING-CHANGE: { source: text, required_for: [feat] }\n")
	w := Written{Key: "BREAKING-CHANGE", Flag: "--breaking", Text: true, Values: []string{"x"}}
	msg := "feat: x\n\nBREAKING CHANGE: x\n"
	standing, _ := w.Against(cfg, msg)
	if standing != Absent {
		t.Fatalf("standing %v", standing)
	}
	if got := Missing(cfg, "feat", "", msg); !slices.Equal(got, []string{"BREAKING-CHANGE"}) {
		t.Errorf("the message alone is missing %q", got)
	}
	if got := Missing(cfg, "feat", "", Trailed(msg, w.Lines(), "#")); len(got) != 0 {
		t.Errorf("the message with the flag's line is missing %q", got)
	}
	if standing, _ := w.Against(cfg, "feat: x\n\nBREAKING CHANGE: y\n"); standing != Differs {
		t.Errorf("another text: %v", standing)
	}
}

// A ledger footer naming none, in any case, is told it takes a ledger task's
// id and has no none (bug 139), going on to the footer that stands in for it
// in a commit of the type and the itos commit flag that writes that, each as
// the config gives them; the fix says the same. Every other ID is judged as
// before, one a second problem after it, and a ledger task named none is
// known.
func TestLedgerFooterNamingNone(t *testing.T) {
	const (
		first    = "name the task the commit belongs to by an id tasks/ has"
		unknown  = "name an id tasks/ has, or add it first in a docs commit"
		standing = "; a chore commit names its "
	)
	problems := func(cfg *config.Loaded, message, note string) []string {
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
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("tasks", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("work-items.yaml", []byte("items: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("tasks/phase-1.yaml", []byte("- id: T-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := `version: 1
ledger: { files: "tasks/phase-{group}.yaml", group: { label: phase, pattern: "\\d+", numeric: true }, id: "T-\\d+" }
work: { registry: work-items.yaml }
tests:
  scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
  proof: { root: features, id: "P-\\d+" }
commits:
  footers:
    Task: { source: ledger, required_for: [chore, refactor], validate_for: all }
`
	items := ledger + "    Item: { source: registry, validate_for: all, in_place_of: { Task: [chore] } }\n"
	scenarios := ledger + "    Scenarios: { source: { tests: scenario }, in_place_of: { Task: [chore] } }\n"
	proofs := scenarios + "    Proofs: { source: { tests: proof }, in_place_of: { Task: [chore] } }\n"
	noFlag := ledger + "    Scenarios: { source: { tests: scenario } }\n    Proofs: { source: { tests: proof }, in_place_of: { Task: [chore] } }\n"
	sentence := "task-footer: the Task: footer takes a ledger task's id, and has no none"
	for _, c := range []struct {
		name, config, message string
		want                  []string
	}{
		{"no stand-in", ledger, "chore: x\n\nTask: none\n", []string{sentence + " | " + first}},
		{"a type it does not serve", items, "refactor: x\n\nTask: none\n", []string{sentence + " | " + first}},
		{"the registry", items, "chore: x\n\nTask: none\n", []string{
			sentence + standing + "registry item in Item: instead, with itos commit --item <id> | " +
				first + ", or the registry item in Item: with itos commit --item <id>"}},
		{"another case", items, "chore: x\n\nTask: NONE\n", []string{
			sentence + standing + "registry item in Item: instead, with itos commit --item <id> | " +
				first + ", or the registry item in Item: with itos commit --item <id>"}},
		{"named tests", scenarios, "chore: x\n\nTask: None\n", []string{
			sentence + standing + "scenario in Scenarios: instead, with itos commit --scenarios <ids> | " +
				first + ", or the scenario in Scenarios: with itos commit --scenarios <ids>"}},
		{"named tests no flag writes", noFlag, "chore: x\n\nTask: none\n", []string{
			sentence + standing + "proof in Proofs: instead | " + first + ", or the proof in Proofs:"}},
		{"the first stand-in", proofs, "chore: x\n\nTask: none\n", []string{
			sentence + standing + "scenario in Scenarios: instead, with itos commit --scenarios <ids> | " +
				first + ", or the scenario in Scenarios: with itos commit --scenarios <ids>"}},
		{"beside an unknown task", ledger, "chore: x\n\nTask: none, T-999\n", []string{
			sentence + " | " + first, "task-footer: unknown tasks: T-999 | " + unknown}},
		{"beside a known task", ledger, "chore: x\n\nTask: T-1\nTask: none\n", []string{sentence + " | " + first}},
		{"an unknown task", items, "chore: x\n\nTask: T-999\n", []string{"task-footer: unknown tasks: T-999 | " + unknown}},
		{"a known task", items, "chore: x\n\nTask: T-1\n", nil},
		{"none in a footer that is not the ledger's", items, "chore: x\n\nTask: T-1\nItem: none\n", []string{
			"item-footer: unknown items: none | name an id work-items.yaml has, or add it first in a docs commit"}},
	} {
		if got := problems(load(t, c.config), c.message, ""); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}

	if err := os.WriteFile("tasks/phase-1.yaml", []byte("- id: T-1\n- id: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := problems(load(t, items), "chore: x\n\nTask: none\n", ""); got != nil {
		t.Errorf("a ledger task named none: %q", got)
	}
	if err := os.WriteFile("tasks/phase-1.yaml", []byte("- id: T-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := load(t, items)
	cfg.Stealth = true
	got := problems(cfg, "chore: x\n", "Task: none")
	if len(got) != 1 || !strings.HasPrefix(got[0], sentence+standing) || !strings.HasSuffix(got[0], first+", or the registry item in Item: with itos commit --item <id>") {
		t.Errorf("stealth: %q", got)
	}
}
