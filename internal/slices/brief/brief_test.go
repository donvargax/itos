package brief

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// ports are the brief command's ports over the fakes: a repository whose
// HEAD is c1, the trees (the working tree holds the config, HEAD the
// data), the entries git would list for the paths asked, and the last
// commit touching the spec's files.
type ports struct {
	*vcsfake.VCS
	trees      treefake.Trees
	configErr  error
	entries    map[string]TreeEntry
	entriesErr error
	asked      []string
	last       string
	lastErr    error
	lastPaths  []string
	tweak      func(*config.Loaded)
}

// Config is the config and its data as HEAD has them, as the real port
// reads them; a tree that cannot be read is a missing repository.
func (p *ports) Config(ctx context.Context) (*config.Loaded, error) {
	if p.configErr != nil {
		return nil, p.configErr
	}
	src, err := p.trees.At(ctx, "HEAD")
	if err != nil {
		return nil, kind.Else(kind.Missing, err)
	}
	cfg, err := config.Load(p.VCS, p.trees, src, "itos.yaml")
	if err == nil && p.tweak != nil {
		p.tweak(cfg)
	}
	return cfg, err
}

// TreeEntries answers as git ls-tree does for the paths asked, which it
// keeps: a path's own entry, and the entries under a folder asked for with
// a path below it; a path outside the top is refused, as git refuses it.
func (p *ports) TreeEntries(_ context.Context, _ string, paths []string) (map[string]TreeEntry, error) {
	p.asked = paths
	if p.entriesErr != nil {
		return nil, p.entriesErr
	}
	for _, asked := range paths {
		if asked == ".." || strings.HasPrefix(asked, "../") || strings.HasPrefix(asked, "/") {
			return nil, errors.New(asked + " is outside repository")
		}
	}
	found := map[string]TreeEntry{}
	for _, asked := range paths {
		for name, e := range p.entries {
			if name == asked || strings.HasPrefix(name, asked+"/") {
				found[name] = e
			}
		}
	}
	return found, nil
}

func (p *ports) LastTouching(_ context.Context, _ string, paths []string) (string, error) {
	p.lastPaths = paths
	return p.last, p.lastErr
}

const top = "/work/project"

const itosYAML = "version: 1\nwork:\n  registry: work-items.yaml\ntests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\nledger:\n  files: tasks/phase-{group}.yaml\n"

// over is the ports of a repository at top whose HEAD holds files, each
// its path from the top to its text, its blob the file's name under
// "blob:", and the config unless given; the working tree's config names
// another registry, which a brief read from HEAD never sees.
func over(files map[string]string) *ports {
	head := map[string]string{"itos.yaml": itosYAML}
	entries := map[string]TreeEntry{}
	for p, text := range files {
		head[p] = text
		entries[p] = TreeEntry{Kind: "blob", Mode: "100644", Object: "blob:" + p}
	}
	worktree := map[string]string{"itos.yaml": strings.Replace(itosYAML, "work-items.yaml", "other.yaml", 1)}
	return &ports{
		VCS: &vcsfake.VCS{Root: top, Located: map[string]vcsport.Location{"": {Top: top, GitDir: ".git", Common: ".git"}},
			History: []vcsfake.Commit{{Name: "c1"}}, Named: map[string]string{"HEAD": "c1"}},
		trees:   treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": worktree, "HEAD": head}},
		entries: entries,
		last:    "spec-sha",
	}
}

const registry = `phases: { 1: null }
items:
  - { id: slice-8, title: Earlier, phase: 1, owner: null, status: done, depends_on: [] }
  - id: slice-9
    title: Say the next step
    phase: 1
    owner: null
    status: todo
    depends_on: [slice-8, slice-7]
    refs: [internal/foo, docs/plan.md, internal/bar, docs/missing.md, internal/baz/]
  - { id: T-001, title: Tidy, phase: 1, owner: null, status: todo, kind: task, depends_on: [] }
  - { id: bug-7, title: A bug, phase: 1, owner: a, status: doing, kind: bug, depends_on: [] }
  - { id: p1-next, title: Later, phase: 1, owner: null, status: todo, kind: idea, depends_on: [], why: Later. }
`

// lean is the registry with slice-9's refs left out.
var lean = regexp.MustCompile(`(?m)^    refs: .*\n`).ReplaceAllString(registry, "")

const tagged = "Feature: Tagged\n\n  @ID-A-01 @wip @slice-9\n  Scenario: one\n    When it runs\n\n  @ID-A-02 @slice-9\n  Scenario: two\n    When it runs\n"

// specified is the repository of the full brief: the registry, AGENTS.md,
// two feature files tagged for slice-9, the refs (a package with a doc.go,
// a file, a package without one, one with a trailing slash) and the notes.
func specified() *ports {
	return over(map[string]string{
		"work-items.yaml":         registry,
		"AGENTS.md":               "Stage only your own files.\n",
		"features/tagged.feature": tagged,
		"features/b.feature":      "Feature: B\n\n  @ID-B-01 @slice-9\n  Scenario: three\n    When it runs\n",
		"internal/foo/doc.go":     "// Package foo says the next step.\n",
		"internal/foo/foo.go":     "package foo\n",
		"docs/plan.md":            "# The plan\n",
		"internal/bar/bar.go":     "package bar\n",
		"internal/baz/doc.go":     "// Package baz.\n",
		"docs/ORCHESTRATING.md":   "# Our notes\r\n\r\n## Independent work\r\n\r\nRepairs go first.\r\n\r\n## What a brief adds here\r\n\r\n- **Reads:** `PLAN.md`.\r\n\r\n### Also\r\n\r\n- **The neighbours:** run the areas.\r\n\r\n\r\n# Lessons\r\n\r\nThe inbox is read first.\r\n",
	})
}

func handle(t *testing.T, p *ports, id, role string) Answer {
	t.Helper()
	a, err := NewHandler(p).Handle(t.Context(), Query{ID: id, Role: role})
	if err != nil {
		t.Fatalf("Handle(%s, %s): %v", id, role, err)
	}
	return a
}

func brief(t *testing.T, p *ports, id, role string) *Brief {
	t.Helper()
	a := handle(t, p, id, role)
	if a.Brief == nil {
		t.Fatalf("Handle(%s, %s) refused: %+v", id, role, a.Refused)
	}
	return a.Brief
}

// The implementer's brief is the template filled from the item at HEAD:
// its id, kind and title, the top as git prints it and its folder, the
// scenarios by file and which are live, the dependencies with their
// statuses, the reads and what was left out, the marked placeholders, and
// the repository's own brief lines after it, the section's body alone.
func TestImplementerBrief(t *testing.T) {
	p := specified()
	b := brief(t, p, "slice-9", "implementer")
	for _, want := range []string{
		"Implement slice-9 (a slice) of project, in the repository at /work/project: Say the next step. The scenarios are @ID-B-01 in features/b.feature and @ID-A-01 and @ID-A-02 in features/tagged.feature; @ID-B-01 and @ID-A-02 are live. It depends on slice-8 (done) and slice-7 (unknown). Take it with `itos work take slice-9` and push that first.\n\n",
		"Run `itos guide work` first, then `itos work show slice-9`, and read AGENTS.md, features/b.feature, features/tagged.feature, internal/foo/doc.go, docs/plan.md and internal/baz/doc.go. Left out: internal/bar has no doc.go at HEAD; docs/missing.md is not in the tree at HEAD.\n\n",
		"\n\n<settled decisions, filled by the coordinator>\n\n",
		"Writing the missing step definitions is the test commit's job: an undefined step is work, not a reason to stop.",
		"beyond what it names: <neighbouring scenarios and checks, filled by the coordinator>.\n\n",
		"itos commit --item slice-9",
		"before the work, the scenarios turned green, <the item's own questions, filled by the coordinator>, the ideas",
		"\n\n- **Reads:** `PLAN.md`.\n\n### Also\n\n- **The neighbours:** run the areas.\n",
	} {
		if !strings.Contains(b.Text, want) {
			t.Errorf("the brief does not say %q:\n%s", want, b.Text)
		}
	}
	for _, left := range []string{"<item>", "<project>", "<absolute path>", "<one line>", "<ids>", "<What is settled", "<the neighbouring", "<the item's own questions>", "## What a brief", "Repairs go first", "The inbox", "\r"} {
		if strings.Contains(b.Text, left) {
			t.Errorf("the brief still says %q:\n%s", left, b.Text)
		}
	}
	if got := strings.Count(b.Text, "<"); got != 3 {
		t.Errorf("%d placeholders left, not the three marked:\n%s", got, b.Text)
	}
	if !strings.HasSuffix(b.Text, "run the areas.\n") || strings.Contains(b.Text, "\n\n\n") {
		t.Errorf("the brief's end: %q", b.Text[len(b.Text)-40:])
	}
	wantReads := []Read{
		{"AGENTS.md", "blob:AGENTS.md", "the repository's agent rules", true},
		{"features/b.feature", "blob:features/b.feature", "the item's scenarios", true},
		{"features/tagged.feature", "blob:features/tagged.feature", "the item's scenarios", true},
		{"internal/foo/doc.go", "blob:internal/foo/doc.go", "the item's ref internal/foo", false},
		{"docs/plan.md", "blob:docs/plan.md", "the item's ref docs/plan.md", false},
		{"internal/baz/doc.go", "blob:internal/baz/doc.go", "the item's ref internal/baz/", false},
	}
	if !slices.Equal(b.Reads, wantReads) {
		t.Errorf("reads %+v", b.Reads)
	}
	if !slices.Equal(b.Omitted, []Omitted{{"internal/bar", "no doc.go at HEAD"}, {"docs/missing.md", "not in the tree at HEAD"}}) {
		t.Errorf("omitted %+v", b.Omitted)
	}
	if b.Item != "slice-9" || b.Role != "implementer" || b.Spec != "spec-sha" || !slices.Equal(p.lastPaths, []string{"features/b.feature", "features/tagged.feature"}) {
		t.Errorf("item %s, role %s, spec %s, last touching %v", b.Item, b.Role, b.Spec, p.lastPaths)
	}
	h := sha256.New()
	h.Write([]byte("slice-9\nimplementer\nspec-sha\nAGENTS.md blob:AGENTS.md\nfeatures/b.feature blob:features/b.feature\nfeatures/tagged.feature blob:features/tagged.feature\ninternal/foo/doc.go blob:internal/foo/doc.go\ndocs/plan.md blob:docs/plan.md\ninternal/baz/doc.go blob:internal/baz/doc.go\n"))
	if want := "sha256:" + hex.EncodeToString(h.Sum(nil)); b.Request != want {
		t.Errorf("request %s, not %s", b.Request, want)
	}
	if again := brief(t, p, "slice-9", "implementer"); again.Request != b.Request || again.Text != b.Text {
		t.Error("the same inputs give another brief")
	}
	if other := brief(t, p, "slice-9", "reviewer"); other.Request == b.Request || !strings.HasPrefix(other.Text, "Review slice-9 (a slice) of project, in the repository at /work/project: Say the next step. The scenarios are @ID-B-01 in features/b.feature") {
		t.Errorf("the reviewer's: request %s, text %.120q", other.Request, other.Text)
	}
}

// The scenarios sentence says all `@wip`, all live, or which one is live;
// a ref read twice is read once; a title the registry lacks is the id; a
// bug is a bug.
// A ref is looked up by its clean path and named as written; one naming
// the top or outside it is left out; a symbolic link, as a ref, as a
// folder's doc.go or as AGENTS.md, is left out and said so, in the text
// too.
func TestRefForms(t *testing.T) {
	p := over(map[string]string{
		"work-items.yaml":     "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [], refs: [./docs/plan.md, ./internal/foo/, ., ../x, link.md, internal/bar] }\n",
		"AGENTS.md":           "Rules.\n",
		"docs/plan.md":        "# The plan\n",
		"internal/foo/doc.go": "// Package foo.\n",
		"link.md":             "docs/plan.md",
		"internal/bar/doc.go": "../foo/doc.go",
	})
	p.entries["link.md"] = TreeEntry{Kind: "blob", Mode: "120000", Object: "blob:link.md"}
	p.entries["internal/bar/doc.go"] = TreeEntry{Kind: "blob", Mode: "120000", Object: "blob:internal/bar/doc.go"}
	b := brief(t, p, "slice-9", "drafter")
	wantReads := []Read{
		{"AGENTS.md", "blob:AGENTS.md", "the repository's agent rules", true},
		{"docs/plan.md", "blob:docs/plan.md", "the item's ref ./docs/plan.md", false},
		{"internal/foo/doc.go", "blob:internal/foo/doc.go", "the item's ref ./internal/foo/", false},
	}
	if !slices.Equal(b.Reads, wantReads) {
		t.Errorf("reads %+v", b.Reads)
	}
	wantOmitted := []Omitted{{".", notInTree}, {"../x", notInTree}, {"link.md", symlinked}, {"internal/bar", symlinked}}
	if !slices.Equal(b.Omitted, wantOmitted) {
		t.Errorf("omitted %+v", b.Omitted)
	}
	if !strings.Contains(b.Text, "Left out: . is not in the tree at HEAD; ../x is not in the tree at HEAD; link.md is a symbolic link at HEAD; internal/bar is a symbolic link at HEAD.") {
		t.Errorf("the text: %.400q", b.Text)
	}
	p.entries["AGENTS.md"] = TreeEntry{Kind: "blob", Mode: "120000", Object: "blob:AGENTS.md"}
	if b = brief(t, p, "slice-9", "drafter"); len(b.Reads) != 2 || b.Omitted[0] != (Omitted{"AGENTS.md", symlinked}) {
		t.Errorf("AGENTS.md a link: reads %+v, omitted %+v", b.Reads, b.Omitted)
	}
}

// A ref naming the top, above it, absolute, a drive's, with a backslash
// or pathspec magic is never asked of git, which refuses the whole listing
// for one (on Windows, the drive's and the backslash's too): each is left
// out as not in the tree and the brief is still written.
func TestRefOutside(t *testing.T) {
	p := over(map[string]string{
		"work-items.yaml": "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [], refs: [., '..', '../x', /etc/hosts, ':!x', 'C:/x', '..\\x', docs/plan.md] }\n",
		"AGENTS.md":       "Rules.\n",
		"docs/plan.md":    "# The plan\n",
	})
	b := brief(t, p, "slice-9", "drafter")
	if !slices.Equal(p.asked, []string{"AGENTS.md", "docs/plan.md", "docs/plan.md/doc.go"}) {
		t.Errorf("asked git for %q", p.asked)
	}
	if len(b.Reads) != 2 || b.Reads[1].Path != "docs/plan.md" {
		t.Errorf("reads %+v", b.Reads)
	}
	if want := []Omitted{{".", notInTree}, {"..", notInTree}, {"../x", notInTree}, {"/etc/hosts", notInTree}, {":!x", notInTree}, {"C:/x", notInTree}, {`..\x`, notInTree}}; !slices.Equal(b.Omitted, want) {
		t.Errorf("omitted %+v", b.Omitted)
	}
}

// A spec file HEAD's entries hold as a symbolic link, or lack (HEAD moved
// between reading the scenarios and listing the entries), is left out and
// said so, never recorded as a read.
func TestSpecFileLeftOut(t *testing.T) {
	p := over(map[string]string{
		"work-items.yaml":         lean,
		"AGENTS.md":               "Rules.\n",
		"features/tagged.feature": tagged,
		"features/b.feature":      "Feature: B\n\n  @ID-B-01 @slice-9\n  Scenario: three\n    When it runs\n",
	})
	p.entries["features/b.feature"] = TreeEntry{Kind: "blob", Mode: "120000", Object: "blob:features/b.feature"}
	delete(p.entries, "features/tagged.feature")
	b := brief(t, p, "slice-9", "implementer")
	if !slices.Equal(b.Reads, []Read{{"AGENTS.md", "blob:AGENTS.md", "the repository's agent rules", true}}) {
		t.Errorf("reads %+v", b.Reads)
	}
	if !slices.Equal(b.Omitted, []Omitted{{"features/b.feature", symlinked}, {"features/tagged.feature", notInTree}}) {
		t.Errorf("omitted %+v", b.Omitted)
	}
	if !strings.Contains(b.Text, "and read AGENTS.md. Left out: features/b.feature is a symbolic link at HEAD; features/tagged.feature is not in the tree at HEAD.") {
		t.Errorf("the text: %.500q", b.Text)
	}
}

func TestSentences(t *testing.T) {
	p := over(map[string]string{
		"work-items.yaml":        "phases: { 1: null }\nitems:\n  - { id: bug-7, phase: 1, owner: a, status: doing, kind: bug, depends_on: [], refs: [AGENTS.md, features/fixes.feature] }\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [] }\n",
		"AGENTS.md":              "Rules.\n",
		"features/fixes.feature": "Feature: Fixes\n\n  @ID-F-01 @bug-7\n  Scenario: one\n    When it runs\n\n  @ID-F-02 @bug-7 @wip\n  Scenario: two\n    When it runs\n\n  @ID-F-03 @bug-7 @wip\n  Scenario: three\n    When it runs\n",
		"features/a.feature":     "Feature: A\n\n  @ID-A-01 @slice-9\n  Scenario: one\n    When it runs\n\n  @ID-A-02 @slice-9\n  Scenario: two\n    When it runs\n",
	})
	b := brief(t, p, "bug-7", "implementer")
	if !strings.HasPrefix(b.Text, "Implement bug-7 (a bug) of project, in the repository at /work/project: bug-7. The scenarios are @ID-F-01, @ID-F-02 and @ID-F-03 in features/fixes.feature; @ID-F-01 is live. Take it") {
		t.Errorf("the bug's brief: %.200q", b.Text)
	}
	if len(b.Reads) != 2 || b.Reads[0].Path != "AGENTS.md" || b.Reads[0].Reason != "the repository's agent rules" || b.Reads[1].Path != "features/fixes.feature" || !b.Reads[1].Required || len(b.Omitted) != 0 {
		t.Errorf("a ref already read is read once: %+v, %+v", b.Reads, b.Omitted)
	}
	if b = brief(t, p, "slice-9", "implementer"); !strings.Contains(b.Text, "The scenarios are @ID-A-01 and @ID-A-02 in features/a.feature, all live. Take it") {
		t.Errorf("all live: %.200q", b.Text)
	}
	p.trees.Files["HEAD"]["features/a.feature"] = "@wip\nFeature: A\n\n  @ID-A-01 @slice-9\n  Scenario: one\n    When it runs\n"
	if b = brief(t, p, "slice-9", "implementer"); !strings.Contains(b.Text, "The scenarios are @ID-A-01 in features/a.feature, all `@wip`. Take it") {
		t.Errorf("all wip: %.200q", b.Text)
	}
}

// The drafter is briefed on an item nothing is tagged for, its spec commit
// none and its reads AGENTS.md alone; without AGENTS.md too, that read
// omitted; the implementer and the reviewer are refused.
func TestNoSpec(t *testing.T) {
	p := over(map[string]string{"work-items.yaml": lean, "AGENTS.md": "Rules.\n"})
	b := brief(t, p, "slice-9", "drafter")
	if !strings.HasPrefix(b.Text, "Specify slice-9 (a slice) of project, in the repository at /work/project: Say the next step. No scenario is tagged @slice-9 yet. It depends on slice-8 (done) and slice-7 (unknown). Your job") {
		t.Errorf("the drafter's brief: %.250q", b.Text)
	}
	if b.Spec != "" || p.lastPaths != nil || len(b.Reads) != 1 || b.Reads[0].Path != "AGENTS.md" || !strings.Contains(b.Text, "and read AGENTS.md. Read in full") {
		t.Errorf("spec %q, last touching %v, reads %+v", b.Spec, p.lastPaths, b.Reads)
	}
	if !strings.Contains(b.Text, "\n\n<settled decisions, filled by the coordinator>\n\n") || strings.Count(b.Text, "<") != 3 || !strings.Contains(b.Text, "<short rule>") {
		t.Errorf("the drafter's placeholders: %s", b.Text)
	}
	for _, role := range []string{"implementer", "reviewer"} {
		a := handle(t, p, "slice-9", role)
		if len(a.Refused) != 1 || a.Refused[0].Rule != "brief-no-spec" || !strings.Contains(a.Refused[0].Message, "no scenario is tagged @slice-9 yet") || a.Refused[0].Fix != "itos brief slice-9 --role drafter briefs the drafter, who writes it" {
			t.Errorf("%s: %+v", role, a.Refused)
		}
	}
	delete(p.trees.Files["HEAD"], "AGENTS.md")
	delete(p.entries, "AGENTS.md")
	b = brief(t, p, "slice-9", "drafter")
	if len(b.Reads) != 0 || !slices.Equal(b.Omitted, []Omitted{{"AGENTS.md", "not in the tree at HEAD"}}) || !strings.Contains(b.Text, "and read nothing. Left out: AGENTS.md is not in the tree at HEAD. Read in full") {
		t.Errorf("without AGENTS.md: reads %+v, omitted %+v, text %.300q", b.Reads, b.Omitted, b.Text)
	}
}

// A task's brief names its ledger checks and reads its ledger file, the
// spec commit the last touching that file; one with no checks says so; one
// with no ledger entry has no spec, and the drafter is told.
func TestTask(t *testing.T) {
	p := over(map[string]string{
		"work-items.yaml":    registry,
		"AGENTS.md":          "Rules.\n",
		"tasks/phase-1.yaml": "- id: T-001\n  type: chore\n  title: Tidy\n  done_when:\n    - { run: go vet ./... }\n    - { fails: go test ./... }\n",
	})
	b := brief(t, p, "T-001", "implementer")
	if !strings.HasPrefix(b.Text, "Implement T-001 (a task) of project, in the repository at /work/project: Tidy. The task is T-001, whose checks are `go vet ./...` and `go test ./...`. Take it") {
		t.Errorf("the task's brief: %.250q", b.Text)
	}
	if b.Spec != "spec-sha" || !slices.Equal(p.lastPaths, []string{"tasks/phase-1.yaml"}) || len(b.Reads) != 2 || b.Reads[1] != (Read{"tasks/phase-1.yaml", "blob:tasks/phase-1.yaml", "the task's ledger entry", true}) {
		t.Errorf("spec %q, last touching %v, reads %+v", b.Spec, p.lastPaths, b.Reads)
	}
	p.trees.Files["HEAD"]["tasks/phase-1.yaml"] = "- id: T-001\n  type: chore\n  title: Tidy\n"
	if b = brief(t, p, "T-001", "reviewer"); !strings.Contains(b.Text, "The task is T-001, which has no checks yet. Your job") {
		t.Errorf("no checks: %.250q", b.Text)
	}
	p.trees.Files["HEAD"]["tasks/phase-1.yaml"] = "- id: T-002\n  type: chore\n  title: Other\n"
	if a := handle(t, p, "T-001", "implementer"); len(a.Refused) != 1 || a.Refused[0].Rule != "brief-no-spec" || !strings.HasPrefix(a.Refused[0].Message, "T-001 has no ledger entry yet, so T-001 has no spec") {
		t.Errorf("no ledger entry: %+v", a.Refused)
	}
	if b = brief(t, p, "T-001", "drafter"); !strings.Contains(b.Text, ": Tidy. T-001 has no ledger entry yet. Your job") || b.Spec != "" {
		t.Errorf("the drafter on a task with no entry: %q, %.250q", b.Spec, b.Text)
	}
	// Under ledger.group.numeric, phase-01.yaml's group is 1, as Tasks
	// reads it: the file is still the task's.
	p = over(map[string]string{
		"work-items.yaml":     registry,
		"AGENTS.md":           "Rules.\n",
		"itos.yaml":           itosYAML + "  group: { pattern: \"\\\\d+\", numeric: true }\n",
		"tasks/phase-01.yaml": "- id: T-001\n  type: chore\n  title: Tidy\n",
	})
	if b = brief(t, p, "T-001", "implementer"); len(b.Reads) != 2 || b.Reads[1].Path != "tasks/phase-01.yaml" || !slices.Equal(p.lastPaths, []string{"tasks/phase-01.yaml"}) {
		t.Errorf("a numeric group's file: reads %+v, last touching %v", b.Reads, p.lastPaths)
	}
}

// Refused, each with its rule and fix: a role itos does not know (before
// anything is read), no registry at HEAD, an id the registry does not
// have, an idea.
func TestRefusals(t *testing.T) {
	p := over(map[string]string{"work-items.yaml": registry})
	p.configErr = errors.New("unread")
	a := handle(t, p, "slice-9", "tester")
	if len(a.Refused) != 1 || a.Refused[0].Rule != "brief-unknown-role" || a.Refused[0].Message != "no brief for the role tester" || !strings.Contains(a.Refused[0].Fix, "drafter, implementer or reviewer") {
		t.Errorf("unknown role: %+v", a.Refused)
	}
	p.configErr = nil
	for id, want := range map[string][2]string{
		"slice-7": {"work-unknown-item", "no item slice-7 in the registry"},
		"p1-next": {"brief-idea", "p1-next is an idea, which has no spec and no kind to brief for"},
	} {
		a := handle(t, p, id, "drafter")
		if len(a.Refused) != 1 || a.Refused[0].Rule != want[0] || a.Refused[0].Message != want[1] {
			t.Errorf("%s: %+v", id, a.Refused)
		}
	}
	if a := handle(t, p, "p1-next", "drafter"); !strings.HasPrefix(a.Refused[0].Fix, "make it work first: itos work promote p1-next --kind slice, or --kind task") {
		t.Errorf("the idea's fix: %q", a.Refused[0].Fix)
	}
	delete(p.trees.Files["HEAD"], "work-items.yaml")
	if a := handle(t, p, "slice-9", "drafter"); len(a.Refused) != 1 || a.Refused[0].Rule != "work-registry-missing" {
		t.Errorf("no registry at HEAD: %+v", a.Refused)
	}
}

// An error, each with its kind: the config, no commit to read from, a
// tree that cannot be read, a registry that is not YAML, a kind whose ID
// pattern does not compile, the ledger's folder missing, the entries, the
// spec commit and the repository's location git cannot give.
func TestErrors(t *testing.T) {
	ctx := context.Background()
	fails := func(name string, p *ports, id string, k kind.Kind, words string) {
		t.Helper()
		_, err := NewHandler(p).Handle(ctx, Query{ID: id, Role: "implementer"})
		if err == nil || kind.Of(err) != k || !strings.Contains(err.Error(), words) {
			t.Errorf("%s: %v (kind %d), want kind %d saying %q", name, err, kind.Of(err), k, words)
		}
	}
	p := specified()
	p.configErr = kind.Wrap(kind.Usage, errors.New("no config"))
	fails("config", p, "slice-9", kind.Usage, "no config")
	p = specified()
	p.Named = nil
	fails("no HEAD", p, "slice-9", kind.Missing, "has no commit")
	p = specified()
	p.trees.Common = ""
	fails("no tree", p, "slice-9", kind.Missing, "HEAD cannot be read")
	p = specified()
	p.trees.Files["HEAD"]["work-items.yaml"] = "items: [\n"
	fails("registry", p, "slice-9", kind.Usage, "did not find expected")
	p = specified()
	p.tweak = func(cfg *config.Loaded) {
		k := cfg.Tests.Values["scenario"]
		bad := "("
		k.ID = &bad
		cfg.Tests.Values["scenario"] = k
	}
	fails("ID pattern", p, "slice-9", kind.Unknown, "error parsing regexp")
	p = specified()
	fails("ledger folder", p, "T-001", kind.Unknown, "the ledger's folder tasks")
	p = specified()
	p.trees.Files["HEAD"]["tasks/phase-1.yaml"] = "- [\n"
	fails("ledger file", p, "T-001", kind.Usage, "did not find expected")
	p = specified()
	p.entriesErr = errors.New("ls-tree failed")
	fails("entries", p, "slice-9", kind.Unknown, "ls-tree failed")
	p = specified()
	p.lastErr = errors.New("rev-list failed")
	fails("spec commit", p, "slice-9", kind.Unknown, "rev-list failed")
	p = specified()
	p.Located = nil
	fails("location", p, "slice-9", kind.Missing, "not a git repository")
}

// The kind an item is briefed as: its kind when it gives one, else by its
// id as the registry's header reads it, a bug by bug-<n>, an idea for any
// other.
func TestItemKind(t *testing.T) {
	taskID := regexp.MustCompile(`^T-\d+$`)
	for text, want := range map[string]string{
		"{ id: p1-x, kind: bug }":     "bug",
		"{ id: slice-3 }":             "slice",
		"{ id: T-12 }":                "task",
		"{ id: bug-7 }":               "bug",
		"{ id: p1-x }":                "idea",
		"{ id: slice-3, kind: idea }": "idea",
	} {
		raw, err := value.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if got := itemKind(raw.(*value.Map), taskID); got != want {
			t.Errorf("itemKind(%s) = %s, not %s", text, got, want)
		}
	}
}

// The section's body: none without the file or the section, the section
// up to the next heading of level one or two, its sub-headings kept, its
// blank lines at either end dropped, CRLF read as LF.
func TestLines(t *testing.T) {
	for text, want := range map[string]string{
		"":                                  "",
		"# A\n\n## Other\n\nx\n":            "",
		"## What a brief adds here\n":       "",
		"## What a brief adds here\n\n\n":   "",
		"## What a brief adds here\nline\n": "line\n",
		"## What a brief adds here\r\n\r\nline\r\n\r\n# Next\r\nno\r\n":         "line\n",
		"x\n## What a brief adds here\n\n- a\n\n### sub\n- b\n\n## Next\n- c\n": "- a\n\n### sub\n- b\n",
		"## What a brief adds here   \nline  \n":                                "line\n",
	} {
		files := map[string]string{"itos.yaml": itosYAML}
		if text != "" {
			files["docs/ORCHESTRATING.md"] = text
		}
		trees := treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": files, "HEAD": files}}
		src, _ := trees.At(t.Context(), "HEAD")
		v := &vcsfake.VCS{Root: top, Located: map[string]vcsport.Location{"": {Top: top, GitDir: ".git", Common: ".git"}}}
		cfg, err := config.Load(v, trees, src, "itos.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if got := lines(t.Context(), cfg, src); got != want {
			t.Errorf("lines(%q) = %q, not %q", text, got, want)
		}
		if got := lines(t.Context(), cfg, unreadable{src}); got != "" {
			t.Errorf("lines over a file that cannot be read = %q", got)
		}
	}
}

// unreadable is a source whose every file is there and cannot be read.
type unreadable struct{ treeport.Source }

func (unreadable) Has(context.Context, string) bool { return true }

func (unreadable) Read(context.Context, string) (string, error) {
	return "", errors.New("unreadable")
}

// list names items as prose does.
func TestList(t *testing.T) {
	for i, want := range []string{"", "a", "a and b", "a, b and c"} {
		if got := list([]string{"a", "b", "c"}[:i]); got != want {
			t.Errorf("list of %d = %q, not %q", i, got, want)
		}
	}
	if got := omittedWhy("other reason"); got != "has other reason" {
		t.Errorf("omittedWhy = %q", got)
	}
}

// Parse: an id and --role, both required, nothing else.
func TestParse(t *testing.T) {
	for args, want := range map[string]string{
		"":                   "brief needs an item's id: itos brief <id> --role drafter|implementer|reviewer",
		"--role drafter":     "brief needs an item's id: itos brief <id> --role drafter|implementer|reviewer",
		"a b --role drafter": "brief takes one id, not a b: itos brief <id> --role drafter|implementer|reviewer",
		"slice-9":            "brief needs --role: itos brief <id> --role drafter|implementer|reviewer",
	} {
		_, err := Parse(strings.Fields(args), false)
		var usage out.UsageError
		if err == nil || err.Error() != want || !errors.As(err, &usage) {
			t.Errorf("Parse(%q) = %v, want the usage error %q", args, err, want)
		}
	}
	if c, err := Parse([]string{"--role", "tester", "slice-9"}, true); err != nil || c != (CLI{Query: Query{ID: "slice-9", Role: "tester"}, JSON: true}) {
		t.Errorf("Parse = %+v, %v", c, err)
	}
}

// Run prints the brief, or --json's one object with the request, the spec
// commit (null for none), the reads, the omitted and the brief as printed;
// a refusal is its problems on stderr with their fixes, or under --json
// the ok-false object, exit 1; an error is the command line's.
func TestRun(t *testing.T) {
	call := func(p *ports, json bool, id, role string) (int, string, string, error) {
		var stdout, stderr bytes.Buffer
		code, err := CLI{Query: Query{ID: id, Role: role}, JSON: json}.Run(t.Context(), &stdout, &stderr, NewHandler(p))
		return code, stdout.String(), stderr.String(), err
	}
	p := specified()
	code, stdout, stderr, err := call(p, false, "slice-9", "implementer")
	want := brief(t, p, "slice-9", "implementer")
	if code != 0 || err != nil || stdout != want.Text || stderr != "" {
		t.Errorf("text: exit %d, %v, stdout %.80q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = call(p, true, "slice-9", "implementer")
	var got struct {
		Schema  int
		OK      bool
		Item    string
		Role    string
		Request string
		Spec    struct{ Commit *string }
		Reads   []Read
		Omitted []Omitted
		Brief   string
	}
	if code != 0 || err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &got) != nil {
		t.Fatalf("json: exit %d, %v, stderr %q, stdout %s", code, err, stderr, stdout)
	}
	if got.Schema != 1 || !got.OK || got.Item != "slice-9" || got.Role != "implementer" || got.Request != want.Request || got.Spec.Commit == nil || *got.Spec.Commit != "spec-sha" || !slices.Equal(got.Reads, want.Reads) || !slices.Equal(got.Omitted, want.Omitted) || got.Brief != want.Text {
		t.Errorf("json: %+v", got)
	}
	if !strings.HasPrefix(stdout, "{\n  \"schema\": 1,\n  \"ok\": true,\n  \"item\": \"slice-9\",\n  \"role\": \"implementer\",\n  \"request\": \"sha256:") || !strings.Contains(stdout, "\"omitted\": [\n    {") {
		t.Errorf("the keys' order: %.200s", stdout)
	}
	p = over(map[string]string{"work-items.yaml": lean, "AGENTS.md": "Rules.\n"})
	code, stdout, _, err = call(p, true, "slice-9", "drafter")
	if code != 0 || err != nil || !strings.Contains(stdout, "\"spec\": {\n    \"commit\": null\n  }") || !strings.Contains(stdout, "\"omitted\": []") {
		t.Errorf("the drafter's json: exit %d, %v, %s", code, err, stdout)
	}
	code, stdout, stderr, err = call(p, false, "slice-9", "implementer")
	if code != 1 || err != nil || stdout != "" || !strings.HasPrefix(stderr, "no scenario is tagged @slice-9 yet, so slice-9 has no spec to brief from\n  fix: itos brief slice-9 --role drafter") {
		t.Errorf("refused: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = call(p, true, "slice-9", "tester")
	if code != 1 || err != nil || stderr != "" || !strings.HasPrefix(stdout, "{\n  \"schema\": 1,\n  \"ok\": false,\n  \"problems\": [\n    {\n      \"rule\": \"brief-unknown-role\"") {
		t.Errorf("refused json: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	p.configErr = errors.New("unread")
	if code, _, _, err = call(p, false, "slice-9", "implementer"); err == nil || err.Error() != "unread" || code != 0 {
		t.Errorf("error: exit %d, %v", code, err)
	}
}

// An item of no kind is a task by its id only where ledger.id says what a
// task's id is: without it, bug-7 is a bug, p1-next an idea, refused, and
// T-001 an idea too; with it, T-001 is a task.
func TestTaskIDs(t *testing.T) {
	p := over(map[string]string{
		"work-items.yaml":        "phases: { 1: null }\nitems:\n  - { id: bug-7, phase: 1, owner: null, status: todo, depends_on: [] }\n  - { id: p1-next, phase: 1, owner: null, status: todo, depends_on: [] }\n  - { id: T-001, phase: 1, owner: null, status: todo, depends_on: [] }\n",
		"AGENTS.md":              "Rules.\n",
		"features/fixes.feature": "Feature: Fixes\n\n  @ID-F-01 @bug-7\n  Scenario: one\n    When it runs\n",
		"tasks/phase-1.yaml":     "- id: T-001\n  type: chore\n  title: Tidy\n",
	})
	if b := brief(t, p, "bug-7", "implementer"); !strings.HasPrefix(b.Text, "Implement bug-7 (a bug) of project") {
		t.Errorf("an untyped bug: %.120q", b.Text)
	}
	for _, id := range []string{"p1-next", "T-001"} {
		if a := handle(t, p, id, "drafter"); len(a.Refused) != 1 || a.Refused[0].Rule != "brief-idea" {
			t.Errorf("%s without ledger.id: %+v", id, a.Refused)
		}
	}
	p.trees.Files["HEAD"]["itos.yaml"] = itosYAML + "  id: 'T-\\d+'\n"
	if b := brief(t, p, "T-001", "implementer"); !strings.HasPrefix(b.Text, "Implement T-001 (a task) of project") {
		t.Errorf("T-001 under ledger.id: %.120q", b.Text)
	}
}
