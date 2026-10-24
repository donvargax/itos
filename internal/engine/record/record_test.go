package record

import (
	"context"
	"errors"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/value"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
	"github.com/donvargax/itos/v8/internal/work"
)

// A project's tracker over the fake, registry.yaml its registry and
// ledger.yaml a ledger file, both committed, and a config naming them.
func project(t *testing.T) (*porttest.Tracker, *config.Loaded) {
	t.Helper()
	tracker := &porttest.Tracker{}
	if _, err := tracker.Transact(t.Context(), port.Transaction{Changes: []port.Change{
		{Name: "registry.yaml", Text: "items: []\n", Created: true},
		{Name: "ledger.yaml", Text: "- id: T-1\n", Created: true},
	}, Paragraphs: []string{"chore: start"}}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Loaded{}
	cfg.Work.Registry = "registry.yaml"
	cfg.Work.Asks = "asks.yaml"
	return tracker, cfg
}

// The registry edited and a new ledger file, as task add writes them.
func files() []File {
	return []File{
		{Path: "registry.yaml", Old: "items: []\n", Text: "items:\n  - id: T-2\n"},
		{Path: "new.yaml", Text: "- id: T-2\n", Created: true},
	}
}

// Every file as it was, the new one gone.
func asItWas(t *testing.T, tracker port.Tracker) {
	t.Helper()
	docs, err := tracker.Load(t.Context(), []string{"registry.yaml", "new.yaml"})
	if want := []port.Doc{{Name: "registry.yaml", Text: "items: []\n", There: true}, {Name: "new.yaml"}}; err != nil || !slices.Equal(docs, want) {
		t.Errorf("the documents are %v (%v)", docs, err)
	}
}

func write(t *testing.T, tracker port.Tracker, cfg *config.Loaded, files []File, body string) (string, []out.Problem, string, error) {
	t.Helper()
	var words strings.Builder
	sha, found, err := Write(t.Context(), tracker, cfg, files, Commit{Header: "docs: add T-2", Body: body, Words: &words})
	return sha, found, words.String(), err
}

// A commit a hook refuses puts every file back, the new one removed, and is
// the problem that says so, after the hook's words.
func TestWriteHookRefuses(t *testing.T) {
	tracker, cfg := project(t)
	tracker.Refuse = true
	sha, found, words, err := write(t, tracker, cfg, files(), "Add T-2.")
	want := []out.Problem{{
		Rule:    "work-commit-failed",
		Message: "the commit of registry.yaml and new.yaml failed (git exited 1), so registry.yaml and new.yaml are as they were",
		Fix:     "fix what git or the hook above says, then run the command again",
	}}
	if sha != "" || err != nil || !slices.Equal(found, want) || words != "refused\n" {
		t.Fatalf("got %q, %v, %v, words %q", sha, found, err, words)
	}
	asItWas(t, tracker)
}

// One file a hook refuses is said to be as it was.
func TestWriteOneFileSaysItIsAsItWas(t *testing.T) {
	tracker, cfg := project(t)
	tracker.Refuse = true
	_, found, _, err := write(t, tracker, cfg, files()[:1], "Edit the registry.")
	if err != nil || len(found) != 1 || !strings.HasSuffix(found[0].Message, "failed (git exited 1), so registry.yaml is as it was") {
		t.Fatalf("got %v, %v", found, err)
	}
	asItWas(t, tracker)
}

// refusing is a tracker whose every transaction a step refuses, as Refused
// says.
type refusing struct {
	*porttest.Tracker
	refused *port.Refused
}

func (r refusing) Transact(context.Context, port.Transaction) (string, error) {
	return "", r.refused
}

// A refused step is the problem that names it, with the exit of the command
// that ran it: git add refusing a created file, say.
func TestWriteNamesTheRefusedStep(t *testing.T) {
	tracker, cfg := project(t)
	_, found, _, err := write(t, refusing{tracker, &port.Refused{Step: "git add of new.yaml", Code: 128}}, cfg, files(), "Add T-2.")
	if err != nil || len(found) != 1 || found[0].Message != "git add of new.yaml failed (git exited 128), so registry.yaml and new.yaml are as they were" {
		t.Fatalf("got %v, %v", found, err)
	}
}

// A refused commit that leaves a file that cannot be put back is an error
// that says both, wrapping why, with no problem of its own: the file is not
// as it was.
func TestWriteSaysWhatCannotBePutBack(t *testing.T) {
	tracker, cfg := project(t)
	restore := errors.New("registry.yaml cannot be put back: a folder is there")
	sha, found, _, err := write(t, refusing{tracker, &port.Refused{Step: "the commit of registry.yaml and new.yaml", Code: 1, Restore: restore}}, cfg, files(), "Add T-2.")
	want := "the commit of registry.yaml and new.yaml failed (git exited 1): registry.yaml cannot be put back: a folder is there"
	if sha != "" || found != nil || err == nil || err.Error() != want || !errors.Is(err, restore) {
		t.Fatalf("got %q, %v, %v", sha, found, err)
	}
}

// Any other failure, a write, a git that cannot start or the hooks not
// ready, is the tracker's error as it came, with no problem: the tracker
// has put back what it wrote.
func TestWritePassesOtherErrorsOn(t *testing.T) {
	tracker, cfg := project(t)
	unready := errors.New("itos's commit-msg hook is not declared; run itos hook install")
	tracker.Unready = unready
	sha, found, _, err := write(t, tracker, cfg, files(), "Add T-2.")
	if sha != "" || found != nil || err != unready {
		t.Fatalf("got %q, %v, %v", sha, found, err)
	}
	asItWas(t, tracker)
}

// A file with changes no commit holds is refused before anything is
// written, the problem naming the file and the rule its kind gives: the
// registry's, the questions', a ledger file's, or the one the command gives.
func TestWriteRefusesUncommittedFiles(t *testing.T) {
	for name, want := range map[string]string{
		"registry.yaml": "work-registry-uncommitted",
		"asks.yaml":     "asks-file-uncommitted",
		"ledger.yaml":   "ledger-file-uncommitted",
		"decision.md":   "decision-file-uncommitted",
	} {
		tracker, cfg := project(t)
		tracker.Edit(name, "edited\n")
		changes := []File{{Path: name, Old: "edited\n", Text: "x\n"}, {Path: "new.yaml", Text: "- id: T-2\n", Created: true}}
		if name == "decision.md" {
			changes[0].Rule = want
		}
		sha, found, _, err := write(t, tracker, cfg, changes, "Add T-2.")
		problem := out.Problem{
			Rule:    want,
			Message: name + " has changes no commit holds, which itos's commit of it would sweep in",
			Fix:     "commit or drop the changes to " + name + " first",
		}
		if sha != "" || err != nil || !slices.Equal(found, []out.Problem{problem}) {
			t.Errorf("%s: got %q, %v, %v", name, sha, found, err)
		}
		if docs, _ := tracker.Load(t.Context(), []string{name, "new.yaml"}); docs[0].Text != "edited\n" || docs[1].There {
			t.Errorf("%s: a file was written: %v", name, docs)
		}
	}
}

// recording is a tracker that keeps the transactions it was given.
type recording struct {
	*porttest.Tracker
	given []port.Transaction
}

func (r *recording) Transact(ctx context.Context, t port.Transaction) (string, error) {
	r.given = append(r.given, t)
	return r.Tracker.Transact(ctx, t)
}

// The files are written and committed as the commit says: the header, the
// body, how quiet, the input, where the words go and the environment, its
// result the commit's.
func TestWriteCommitsAsTheCommitSays(t *testing.T) {
	tracker, cfg := project(t)
	r := &recording{Tracker: tracker}
	var words strings.Builder
	stdin := strings.NewReader("")
	c := Commit{Header: "docs: add T-2", Body: "Add\n  T-2.", Quiet: true, Stdin: stdin, Words: &words, Env: []string{"A=1"}}
	sha, found, err := Write(t.Context(), r, cfg, files(), c)
	if sha != strings.Repeat("0", 39)+"2" || found != nil || err != nil {
		t.Fatalf("got %q, %v, %v", sha, found, err)
	}
	given := r.given[0]
	want := []port.Change{
		{Name: "registry.yaml", Old: "items: []\n", Text: "items:\n  - id: T-2\n"},
		{Name: "new.yaml", Text: "- id: T-2\n", Created: true},
	}
	if !slices.Equal(given.Changes, want) || !slices.Equal(given.Paragraphs, []string{"docs: add T-2", "Add T-2."}) ||
		!given.Quiet || given.Stdin != stdin || given.Words != &words || !slices.Equal(given.Env, []string{"A=1"}) {
		t.Errorf("the transaction is %+v", given)
	}
	if docs, _ := tracker.Load(t.Context(), []string{"new.yaml"}); docs[0].Text != "- id: T-2\n" {
		t.Errorf("new.yaml is %v", docs)
	}
}

// The body of a commit itos makes of its own files never starts a line with
// what the header lint reads as a footer, nor with git's comment char, where
// a plain wrap at the same width would (bug 18): ask's "word:", as in docs:
// answer q-10, work done's run address after its "passed:", and a "#". The
// words are kept, in order, and no line is longer than the width.
func TestCommitBodyWrapsClearOfFooters(t *testing.T) {
	tracker, cfg := project(t)
	r := &recording{Tracker: tracker}
	footer := regexp.MustCompile(`^[\w-]+(?::\s|\s+#)`)
	old := "items: []\n"
	for i, text := range []string{
		"recommendation: majors cost nothing under fix-forward.",
		"passed: https://github.com/donvargax/itos/actions/runs/37228229727.",
		"#12 is the issue it closes.",
	} {
		// The words before the text fill the first line to the width, so a
		// plain wrap starts the second with the text.
		lead := "Set slice-71 (\"itos ask record writes MADR 4 records\") to done, with itos work done. HEAD's CI run"
		lead += strings.Repeat(" x", (BodyWidth-len(lead))/2)
		if len(lead) < BodyWidth {
			lead += "s"
		}
		body := lead + " " + text
		if plain := value.Wrap(body, BodyWidth); !strings.Contains(plain, "\n"+text) {
			t.Fatalf("a plain wrap does not start a line with %q:\n%s", text, plain)
		}
		files := []File{{Path: "registry.yaml", Old: old, Text: strings.Repeat("# edit\n", i+1) + old}}
		if sha, found, _, err := write(t, r, cfg, files, body); sha == "" || found != nil || err != nil {
			t.Fatalf("got %q, %v, %v", sha, found, err)
		}
		old = files[0].Text
		got := r.given[i].Paragraphs[1]
		for _, line := range strings.Split(got, "\n") {
			if footer.MatchString(line) || strings.HasPrefix(line, "#") || len(line) > BodyWidth {
				t.Errorf("the body has the line %q:\n%s", line, got)
			}
		}
		if strings.Join(strings.Fields(got), " ") != body {
			t.Errorf("the body's words are not the given ones, in order:\n%s", got)
		}
	}
}

// A config over the fakes whose header lint is itos's own and whose footers
// are the ones the tests' bodies could be read as.
func lintConfig(t *testing.T) *config.Loaded {
	t.Helper()
	trees := treefake.Trees{Files: map[string]map[string]string{"worktree": nil}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(&vcsfake.VCS{}, trees, src, "itos.yaml",
		"version: 1\ncommits:\n  header_lint: { use: builtin }\n  footers:\n    Item: { source: text }\nwork: { registry: tasks/work-items.yaml }\n", false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// The registry of a text, as Load gives it.
func registryOf(t *testing.T, text string) work.Registry {
	t.Helper()
	raw, err := value.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	r := work.Registry{Phases: value.Prop(raw, "phases").(*value.Map)}
	for _, item := range value.Prop(raw, "items").([]any) {
		m := value.Copy(item).(*value.Map)
		if m.At("depends_on") == value.Undefined {
			m.Set("depends_on", []any{})
		}
		r.Items = append(r.Items, m)
	}
	return r
}

// The body of a commit itos makes of a text the person gave never reads as a
// footer and never runs over the line, whichever command writes it, where
// the text opens with "Word:" or holds a word longer than the line (bug
// 132): each of work drop, defer, give-back, add and edit, run through the
// body writer, is a message the built-in header lint and the footer rules
// let pass, whole.
func TestSelfBodyOfGivenTexts(t *testing.T) {
	cfg := lintConfig(t)
	path := "internal/slices/work/registry/commit/body/a-path-deep-enough-that-it-runs-past-the-line/of-the-header-lint/selfbody.go"
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: i, title: " + path + ", phase: 1, status: todo, owner: null, kind: idea, why: a gap }\n" +
		"  - { id: s, title: " + path + ", phase: 1, status: doing, owner: p, kind: slice }\n"
	r := registryOf(t, text)
	title := "Split " + path + " in two"
	for _, reason := range []string{
		"Superseded: slice-9 does what this idea asked for, and its scenarios pin it.",
		"Item: bug-1 " + strings.Repeat("and more ", 20),
		"See " + path + " for why.",
		path,
	} {
		for _, c := range []struct {
			run func() (work.Change, []out.Problem, error)
			// said is whether the body carries the text given.
			said bool
		}{
			{func() (work.Change, []out.Problem, error) { return work.Drop(cfg, r, text, "i", reason) }, true},
			{func() (work.Change, []out.Problem, error) { return work.Defer(cfg, r, text, "i", reason) }, true},
			{func() (work.Change, []out.Problem, error) {
				return work.GiveBack(cfg, r, text, "s", "p", false, reason)
			}, true},
			{func() (work.Change, []out.Problem, error) {
				return work.Add(cfg, r, text, work.New{ID: "p1-new", Title: title, Why: reason, Kind: "idea"}, nil)
			}, false},
			{func() (work.Change, []out.Problem, error) {
				return work.Edit(cfg, r, text, "i", work.Edits{Title: &title, Note: reason}, nil)
			}, false},
		} {
			change, found, err := c.run()
			if err != nil || len(found) > 0 {
				t.Fatalf("%q: %v, %v", reason, found, err)
			}
			body := selfBody(cfg, change.Header, change.Body)
			whole := change.Header + "\n\n" + body
			problems, err := message.HeaderProblems(cfg, whole, "")
			footers, ferr := message.FooterProblems(cfg, whole, message.Reading{})
			if err != nil || ferr != nil || len(problems) > 0 || len(footers) > 0 {
				t.Errorf("%q:\n%s\nheader %v (%v), footers %v (%v)", reason, whole, problems, err, footers, ferr)
			}
			for _, line := range strings.Split(body, "\n") {
				if strings.HasPrefix(line, "Superseded:") || strings.HasPrefix(line, "Item:") {
					t.Errorf("a line of the body opens as a footer: %q", line)
				}
			}
			if joined := strings.ReplaceAll(strings.ReplaceAll(body, "\n", ""), " ", ""); c.said &&
				!strings.Contains(joined, strings.ReplaceAll(reason, " ", "")) {
				t.Errorf("the body lost the text %q:\n%s", reason, body)
			}
		}
	}
}

// Ready is Write's refusals before anything is written: the hooks not ready
// an error, the first file with changes no commit holds a problem, nothing
// else nothing.
func TestReady(t *testing.T) {
	tracker, cfg := project(t)
	if p, err := Ready(t.Context(), tracker, cfg, files()); p != nil || err != nil {
		t.Errorf("Ready of clean files: %v, %v", p, err)
	}
	tracker.Edit("new.yaml", "there\n")
	tracker.Edit("ledger.yaml", "edited\n")
	changes := append(files(), File{Path: "ledger.yaml", Rule: "ledger-rule"})
	if p, err := Ready(t.Context(), tracker, cfg, changes); err != nil || p == nil || p.Rule != "ledger-file-uncommitted" || !strings.HasPrefix(p.Message, "new.yaml ") {
		t.Errorf("Ready of dirty files: %v, %v", p, err)
	}
	unready := errors.New("not ready")
	tracker.Unready = unready
	if p, err := Ready(t.Context(), tracker, cfg, changes); p != nil || err != unready {
		t.Errorf("Ready with the hooks not ready: %v, %v", p, err)
	}
}

// A config over the fakes, in a project whose working tree holds files,
// its registry tasks/work-items.yaml.
func configOver(t *testing.T, files map[string]string) *config.Loaded {
	t.Helper()
	trees := treefake.Trees{Files: map[string]map[string]string{"worktree": files}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(&vcsfake.VCS{}, trees, src, "itos.yaml", "version: 1\n", false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// A sound registry is Sound's answer, with the text the tracker holds.
func TestSoundRegistry(t *testing.T) {
	text := "phases:\n  1: nobody\nitems:\n  - id: p1-one\n    title: One\n    phase: 1\n    owner: null\n    status: todo\n    depends_on: []\n"
	cfg := configOver(t, map[string]string{"tasks/work-items.yaml": text})
	tracker := &porttest.Tracker{}
	tracker.Edit("tasks/work-items.yaml", text)
	registry, got, found, err := Sound(t.Context(), tracker, cfg)
	if err != nil || found != nil || got != text || len(registry.Items) != 1 {
		t.Fatalf("got %v, %q, %v, %v", registry, got, found, err)
	}
}

// A registry with problems is them, and nothing read.
func TestSoundRegistryProblems(t *testing.T) {
	cfg := configOver(t, nil)
	registry, text, found, err := Sound(t.Context(), &porttest.Tracker{}, cfg)
	if err != nil || len(found) != 1 || found[0].Rule != "work-registry-missing" || text != "" || registry.Items != nil {
		t.Fatalf("got %v, %q, %v, %v", registry, text, found, err)
	}
}

// A registry the tree reader finds but the tracker no longer has is an
// error naming it.
func TestSoundRegistryGone(t *testing.T) {
	cfg := configOver(t, map[string]string{"tasks/work-items.yaml": "items: []\n"})
	_, _, found, err := Sound(t.Context(), &porttest.Tracker{}, cfg)
	var path *fs.PathError
	if found != nil || !errors.As(err, &path) || path.Path != "tasks/work-items.yaml" || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("got %v, %v", found, err)
	}
}

// A registry that cannot be read as YAML is the error, with no problem.
func TestSoundRegistryUnreadable(t *testing.T) {
	cfg := configOver(t, map[string]string{"tasks/work-items.yaml": "items: [\n"})
	if _, _, found, err := Sound(t.Context(), &porttest.Tracker{}, cfg); found != nil || err == nil {
		t.Fatalf("got %v, %v", found, err)
	}
}

// changed is a source whose files read as text after the first read, as
// when another writer changes them between two reads.
type changed struct {
	treeport.Source
	text  string
	reads int
}

func (c *changed) Read(ctx context.Context, path string) (string, error) {
	if c.reads++; c.reads > 1 {
		return c.text, nil
	}
	return c.Source.Read(ctx, path)
}

// A registry judged sound but unreadable when it is loaded is the load's
// error.
func TestSoundRegistryChangedSinceJudged(t *testing.T) {
	cfg := configOver(t, map[string]string{"tasks/work-items.yaml": "items: []\n"})
	cfg.Source = &changed{Source: cfg.Source, text: "items: [\n"}
	tracker := &porttest.Tracker{}
	tracker.Edit("tasks/work-items.yaml", "items: []\n")
	if _, text, found, err := Sound(t.Context(), tracker, cfg); found != nil || err == nil || text != "" {
		t.Fatalf("got %q, %v, %v", text, found, err)
	}
}
