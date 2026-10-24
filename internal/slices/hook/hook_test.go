package hook

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/shell"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// rules is a project's config: docs may touch Markdown alone, chore
// anything; a Task footer names the ledger's tasks, whose checks the hook
// runs.
const rules = "version: 1\ncommits:\n  types: [docs, chore]\n  header_lint: { use: builtin }\n" +
	"  scopes: { docs: { only: [\"**/*.md\"] }, chore: {} }\n  footers:\n    Task: { source: ledger }\n    Notes: { source: text }\n" +
	"ledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\nwork: { registry: tasks/work-items.yaml }\n"

// tasksLedger is the tasks: T-1 passes, T-2 fails and its item is done, T-3
// fails and its item is in progress, T-4 fails with no item.
const tasksLedger = "- { id: T-1, type: docs, title: Passes, done_when: [{ run: \"true\", cost: static }] }\n" +
	"- { id: T-2, type: docs, title: Done, done_when: [{ run: \"exit 1\", cost: static }] }\n" +
	"- { id: T-3, type: docs, title: Doing, done_when: [{ run: \"exit 1\", cost: static }] }\n" +
	"- { id: T-4, type: docs, title: Lost, done_when: [{ run: \"exit 1\", cost: static }] }\n"

// registry holds T-2's and T-3's items.
const registry = "phases: { 1: null }\nitems:\n" +
	"  - { id: T-2, title: Done, phase: 1, owner: null, status: done, depends_on: [] }\n" +
	"  - { id: T-3, title: Doing, phase: 1, owner: null, status: doing, depends_on: [] }\n"

// ports are hook's ports over the fakes: the repository, its trees, the
// config the working tree holds or the error loading it gives, the git
// config and an old git, and the shell, which records each command and
// answers as run says, exit 0 by default.
type ports struct {
	*vcsfake.VCS
	trees   treefake.Trees
	cfg     *config.Loaded
	loadErr error
	git     map[string][]string
	setErr  error
	oldGit  bool
	ran     []string
	// hookFiles are the hooks folder's files, by their paths.
	hookFiles map[string]string
	run       func(command string, o shell.Options) shell.Result
	shows     int
	engine    *evidence.Engine
}

// Show is the repository's, refused once shows, when set, runs out.
func (p *ports) Show(ctx context.Context, rev string) (vcsport.Entry, error) {
	if p.shows > 0 {
		if p.shows--; p.shows == 0 {
			return vcsport.Entry{}, errors.New("show refused")
		}
	}
	return p.VCS.Show(ctx, rev)
}

func (p *ports) Trees(context.Context) treeport.Trees { return p.trees }

func (p *ports) CaptureIndex(ctx context.Context) (treeport.Source, error) {
	return p.trees.At(ctx, "index")
}

func (p *ports) TaskSnapshot(context.Context, string) (Snapshot, error) {
	return Snapshot{Env: os.Environ(), Remove: func() error { return nil }}, nil
}

// Evidence is the engine the test gave, none by default.
func (p *ports) Evidence(context.Context, *config.Loaded, string) *evidence.Engine { return p.engine }

func (p *ports) LoadConfig(context.Context) (*config.Loaded, error) { return p.cfg, p.loadErr }

func (p *ports) Read(ctx context.Context, path string) (string, error) {
	src, _ := p.trees.At(ctx, "worktree")
	return src.Read(ctx, filepath.ToSlash(path))
}

func (p *ports) TaskFlags(context.Context) []string { return []string{"--root"} }

func (p *ports) Shell(_ context.Context, _ *config.Loaded, command string, o shell.Options) shell.Result {
	p.ran = append(p.ran, command)
	if p.run == nil {
		return shell.Result{}
	}
	return p.run(command, o)
}

func (p *ports) Config(_ context.Context, key string, _ bool) []string { return p.git[key] }

func (p *ports) SetConfig(_ context.Context, key, value string) error {
	if p.setErr != nil {
		return p.setErr
	}
	p.git[key] = []string{value}
	return nil
}

func (p *ports) ConfigHooksRun(context.Context) bool { return !p.oldGit }

func (p *ports) Version(context.Context) string { return "2.53.0" }

// The hooks folder is .git/hooks, its files hookFiles.
func (p *ports) HooksPath(context.Context) (string, error) {
	return filepath.Join(".git", "hooks"), nil
}

func (p *ports) HookFile(_ context.Context, path string) (string, error) {
	text, ok := p.hookFiles[filepath.ToSlash(path)]
	if !ok {
		return "", os.ErrNotExist
	}
	return text, nil
}

func (p *ports) WriteHookFile(_ context.Context, path, text string) error {
	if p.hookFiles == nil {
		p.hookFiles = map[string]string{}
	}
	p.hookFiles[filepath.ToSlash(path)] = text
	return nil
}

func (p *ports) MoveHookFile(_ context.Context, from, to string) error {
	p.hookFiles[filepath.ToSlash(to)] = p.hookFiles[filepath.ToSlash(from)]
	delete(p.hookFiles, filepath.ToSlash(from))
	return nil
}

// world is a repository whose HEAD, root, holds head, the index index and
// the working tree work; the working tree's config is text, none when "".
// It stands in a new folder, with no ITOS_CONFIG and none of the variables
// the hook reads.
type world struct {
	head, index, work map[string]string
	text              string
}

// clean unsets the variables the hook reads, and moves to a new folder.
func clean(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ITOS_CONFIG", "ITOS_AT", message.FootersEnv, message.AmendEnv, "GIT_AUTHOR_NAME",
		"GIT_AUTHOR_EMAIL", "GIT_AUTHOR_DATE", "PRE_COMMIT_TO_REF", "PRE_COMMIT_FROM_REF", "PRE_COMMIT_LOCAL_BRANCH",
		"PRE_COMMIT_REMOTE_BRANCH"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Chdir(t.TempDir())
}

// with is the files with the changes given, a "" text taking the file out.
func with(files, changes map[string]string) map[string]string {
	tree := map[string]string{}
	for p, text := range files {
		tree[p] = text
	}
	for p, text := range changes {
		if text == "" {
			delete(tree, p)
		} else {
			tree[p] = text
		}
	}
	return tree
}

// project is the files of a project under rules.
var project = map[string]string{
	"itos.yaml":             rules,
	"a.md":                  "a\n",
	"tasks/phase-1.yaml":    tasksLedger,
	"tasks/work-items.yaml": registry,
}

// over is the ports over w.
func over(t *testing.T, w world) *ports {
	t.Helper()
	clean(t)
	repo := &vcsfake.VCS{
		History: []vcsfake.Commit{{Name: "root", Message: "docs: start\n", Tree: w.head, Author: "A", Email: "a@x", Date: "100"}},
		Named:   map[string]string{"HEAD": "root"},
		Index:   w.index,
		Located: map[string]vcsport.Location{"": {Top: "/r", GitDir: ".git", Common: ".git"}},
	}
	trees := treefake.Trees{Common: ".git", Files: map[string]map[string]string{
		"worktree": w.work, "index": w.index, "HEAD": w.head, "root": w.head}}
	p := &ports{VCS: repo, trees: trees, git: map[string][]string{}}
	if w.text != "" {
		src, err := trees.At(t.Context(), "worktree")
		if err != nil {
			t.Fatal(err)
		}
		if p.cfg, err = config.FromText(repo, trees, src, "itos.yaml", w.text, false); err != nil {
			t.Fatal(err)
		}
	} else {
		p.loadErr = errors.New("no config")
	}
	return p
}

// staging is a world under rules whose index changes the project as given
// and whose working tree holds the index and the message msg.
func staging(t *testing.T, changes map[string]string, msg string) *ports {
	t.Helper()
	index := with(project, changes)
	return over(t, world{head: project, index: index, work: with(index, map[string]string{"msg": msg}), text: rules})
}

// hook runs `hook <args>` over p: its exit, error and streams.
func hook(t *testing.T, p *ports, args ...string) (int, string, string, error) {
	t.Helper()
	c, err := Parse(args, false, "", &tests.Warnings{})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

func isUsage(err error, text string) bool {
	var u out.UsageError
	return errors.As(err, &u) && err.Error() == text
}

// Parse takes commit-msg's file from where the person stood, install's
// switches, and pre-push; anything else is a usage error.
func TestParse(t *testing.T) {
	warnings := &tests.Warnings{}
	c, err := Parse([]string{"commit-msg", "msg"}, false, "sub", warnings)
	if err != nil || c.Sub != "commit-msg" || c.File != filepath.Join("sub", "msg") || c.Tests != warnings {
		t.Errorf("commit-msg: %+v, %v", c, err)
	}
	abs := filepath.Join(t.TempDir(), "msg")
	if c, _ := Parse([]string{"commit-msg", abs}, false, "sub", nil); c.File != abs {
		t.Errorf("an absolute file: %q", c.File)
	}
	if c, _ := Parse([]string{"commit-msg", "-"}, false, "sub", nil); c.File != "-" {
		t.Errorf("stdin: %q", c.File)
	}
	if _, err := Parse([]string{"commit-msg"}, false, "", nil); !isUsage(err, "hook commit-msg needs <file>") {
		t.Errorf("no file: %v", err)
	}
	if c, err := Parse([]string{"pre-push", "origin", "url"}, false, "", nil); err != nil || c.Sub != "pre-push" {
		t.Errorf("pre-push: %+v, %v", c, err)
	}
	c, err = Parse([]string{"install", "--force", "--print"}, true, "", nil)
	if err != nil || !c.Print || !c.Force || !c.JSON {
		t.Errorf("install: %+v, %v", c, err)
	}
	if _, err := Parse([]string{"bogus"}, false, "", nil); !isUsage(err, "unknown command: hook bogus") {
		t.Errorf("bogus: %v", err)
	}
	if _, err := Parse(nil, false, "", nil); !errors.As(err, new(out.UsageError)) {
		t.Errorf("none: %v", err)
	}
}

// A commit that passes every rule exits 0, saying nothing; one whose
// message names a task whose checks pass too.
func TestCommitMsgPasses(t *testing.T) {
	p := staging(t, map[string]string{"b.md": "b\n"}, "docs: add b\n\nTask: T-1\n")
	if code, stdout, stderr, err := hook(t, p, "commit-msg", "msg"); code != 0 || err != nil || stdout+stderr != "" {
		t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	if len(p.ran) != 0 {
		t.Errorf("ran %q", p.ran)
	}
}

// The first rule: staged data that config check refuses rejects the commit
// under the staged reject message; data that cannot be read is one
// data-unreadable problem, under the default message when the staged
// config does not load.
func TestCommitMsgStagedData(t *testing.T) {
	p := staging(t, map[string]string{"tasks/work-items.yaml": "phases: [\n"}, "docs: x\n")
	code, _, stderr, err := hook(t, p, "commit-msg", "msg")
	if code != policy || err != nil || !strings.HasPrefix(stderr, "Commit rejected:\n  - ") {
		t.Errorf("a registry that cannot be read: exit %d, %v\n%s", code, err, stderr)
	}
	p = staging(t, map[string]string{"itos.yaml": "version: [\n"}, "docs: x\n")
	code, _, stderr, err = hook(t, p, "commit-msg", "msg")
	if code != policy || err != nil || !strings.HasPrefix(stderr, "Commit rejected:\n  - itos.yaml") {
		t.Errorf("a config that cannot be read: exit %d, %v\n%s", code, err, stderr)
	}
	p = staging(t, map[string]string{
		"itos.yaml":             strings.Replace(rules, "commits:\n", "commits:\n  reject_message: \"Refused:\"\n", 1),
		"tasks/work-items.yaml": "phases: [\n",
	}, "docs: x\n")
	code, _, stderr, err = hook(t, p, "commit-msg", "msg")
	if code != policy || err != nil || !strings.HasPrefix(stderr, "Refused:\n  - ") {
		t.Errorf("a staged reject message: exit %d, %v\n%s", code, err, stderr)
	}
}

// Outside a repository, or where the index cannot be read, the hook stops
// with the error.
func TestCommitMsgUnreadableIndex(t *testing.T) {
	p := staging(t, nil, "docs: x\n")
	p.VCS.Named = map[string]string{}
	if _, _, _, err := hook(t, p, "commit-msg", "msg"); err == nil {
		t.Error("staged paths with no HEAD: no error")
	}
	p = staging(t, nil, "docs: x\n")
	p.trees.Common = ""
	if _, _, _, err := hook(t, p, "commit-msg", "msg"); err == nil {
		t.Error("an index that cannot be read: no error")
	}
}

// smoked is a config whose scenario kind keeps a smoke set.
const smoked = "version: 1\ncommits: { types: [test], reject_message: \"Refused:\" }\n" +
	"tests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\", tag_prefix: \"@\", smoke: { file: features/smoke.yaml, every_file: true } }\n"

// A staged feature file with no smoke scenario is the smoke rule's problem,
// read from the index (slice 117); a staged tree whose config does not load
// leaves it to the other rules.
func TestCommitMsgSmokeRule(t *testing.T) {
	head := map[string]string{
		"itos.yaml":           smoked,
		"features/a.feature":  "Feature: A\n\n  @ID-A-01\n  Scenario: One\n    When it runs\n",
		"features/smoke.yaml": "- file: a.feature\n  scenarios: [{ id: \"@ID-A-01\", why: listed }]\n",
	}
	index := with(head, map[string]string{"features/b.feature": "Feature: B\n\n  @ID-B-01\n  Scenario: One\n    When it runs\n"})
	p := over(t, world{head: head, index: index, work: with(index, map[string]string{"msg": "test: b\n"}), text: smoked})
	for f, text := range index {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, _, stderr, err := hook(t, p, "commit-msg", "msg")
	if code != policy || err != nil || !strings.HasPrefix(stderr, "Refused:\n  - b.feature ") {
		t.Errorf("exit %d, %v\n%s", code, err, stderr)
	}
	h := NewHandler(p)
	idx, _ := p.trees.At(t.Context(), "index")
	p.trees.Files["index"] = with(index, map[string]string{"itos.yaml": "version: [\n"})
	if found := h.stagedSmokeIssues(t.Context(), idx, []string{"features/b.feature"}, "", nil); found != nil {
		t.Errorf("a staged tree with no config: %v", found)
	}
}

// stagesData: the config, a ledger file, the registry, the questions' file,
// a smoke set, a decision record or the decisions' index, as the staged
// config names them; a file beside them is not, nor is anything when the
// staged config does not load.
func TestStagesData(t *testing.T) {
	text := "version: 1\ncommits: { types: [docs] }\nledger: { files: \"tasks/phase-{group}.yaml\" }\n" +
		"work: { registry: work-items.yaml }\ntests: { scenario: { root: features, smoke: { file: features/smoke.yaml } } }\n"
	p := over(t, world{head: map[string]string{}, index: map[string]string{"itos.yaml": text}, work: map[string]string{}})
	h := NewHandler(p)
	src, _ := p.trees.At(t.Context(), "index")
	for staged, want := range map[string]bool{
		"itos.yaml":                         true,
		"./itos.yaml":                       true,
		"tasks/phase-1.yaml":                true,
		"tasks/notes.md":                    false,
		"docs/phase-1.yaml":                 false,
		"work-items.yaml":                   true,
		"asks.yaml":                         true,
		"features/smoke.yaml":               true,
		"notes.yaml":                        false,
		"docs/decisions/0001-use-go.md":     true,
		"docs/decisions/README.md":          true,
		"docs/decisions/notes.txt":          false,
		"docs/decisions/old/0001-use-go.md": false,
	} {
		if got := h.stagesData(t.Context(), src, []string{staged}); got != want {
			t.Errorf("stagesData(%q) = %v", staged, got)
		}
	}
	p.trees.Files["index"]["itos.yaml"] = "version: [\n"
	if h.stagesData(t.Context(), src, []string{"work-items.yaml"}) {
		t.Error("a staged config that does not load names data")
	}
}

// The second rule: the paths a docs commit may not touch reject it, with
// the path rules' rejection; a kind's staged range command runs for every
// type with path rules but its except_types, each `  - ` line it prints a
// problem, a failure without one a problem too.
func TestCommitMsgStagedRule(t *testing.T) {
	p := staging(t, map[string]string{"src.go": "package src\n"}, "docs: add code\n")
	code, _, stderr, err := hook(t, p, "commit-msg", "msg")
	if code != policy || err != nil || !strings.Contains(stderr, "docs commits may not touch src.go") {
		t.Errorf("a docs commit touching code: exit %d, %v\n%s", code, err, stderr)
	}

	text := rules + "tests:\n  scenario:\n    root: features\n    id: \"ID-[A-Z]+-\\\\d+\"\n    range_checks:\n" +
		"      - { name: says, staged: \"say {type}\", except_types: [chore] }\n"
	index := with(project, map[string]string{"b.md": "b\n"})
	p = over(t, world{head: project, index: index, work: with(index, map[string]string{"msg": "docs: b\n", "chore": "chore: b\n"}), text: text})
	p.run = func(command string, o shell.Options) shell.Result {
		o.Stderr.Write([]byte("  - judged " + strings.TrimPrefix(command, "say ") + "  \nnoise\n"))
		return shell.Result{Code: 1}
	}
	code, _, stderr, err = hook(t, p, "commit-msg", "msg")
	if code != policy || err != nil || !strings.Contains(stderr, "judged 'docs'") || !slices.Equal(p.ran, []string{"say 'docs'"}) {
		t.Errorf("a staged command that fails: exit %d, %v, ran %q\n%s", code, err, p.ran, stderr)
	}
	p.ran = nil
	if code, _, _, err := hook(t, p, "commit-msg", "chore"); code != 0 || err != nil || len(p.ran) != 0 {
		t.Errorf("an excepted type: exit %d, %v, ran %q", code, err, p.ran)
	}
	p.run = func(string, shell.Options) shell.Result { return shell.Result{Code: 1} }
	code, _, stderr, _ = hook(t, p, "commit-msg", "msg")
	if code != policy || !strings.Contains(stderr, "says failed: ") {
		t.Errorf("a staged command that fails saying nothing: exit %d\n%s", code, stderr)
	}
	p.run = nil
	if code, _, _, err := hook(t, p, "commit-msg", "msg"); code != 0 || err != nil {
		t.Errorf("a staged command that passes: exit %d, %v", code, err)
	}
}

// A working-tree config with no commits section, or one that does not
// load, stops the hook with its error; so does a message file it cannot
// read.
func TestCommitMsgErrors(t *testing.T) {
	p := staging(t, nil, "docs: x\n")
	if _, _, _, err := hook(t, p, "commit-msg", "missing"); err == nil {
		t.Error("a message that cannot be read: no error")
	}
	p.loadErr = errors.New("broken")
	if _, _, _, err := hook(t, p, "commit-msg", "msg"); err == nil || err.Error() != "broken" {
		t.Errorf("a config that does not load: %v", err)
	}
	index := with(project, map[string]string{"itos.yaml": "version: 1\n"})
	p = over(t, world{head: index, index: index, work: with(index, map[string]string{"msg": "docs: x\n"}), text: "version: 1\n"})
	if _, _, _, err := hook(t, p, "commit-msg", "msg"); err == nil {
		t.Error("a config with no commits section: no error")
	}
}

// The header lint and the footer rules: a header with no type is refused
// with the lint's own exit.
func TestCommitMsgHeaderLint(t *testing.T) {
	p := staging(t, nil, "no type here\n")
	if code, _, stderr, err := hook(t, p, "commit-msg", "msg"); code == 0 || err != nil || stderr == "" {
		t.Errorf("exit %d, %v\n%s", code, err, stderr)
	}
}

// The last rule: a named task whose check fails rejects the commit when its
// item is done, and is printed with its status otherwise; under
// hooks.commit_msg.task_checks: false nothing runs.
func TestCommitMsgTasksRule(t *testing.T) {
	p := staging(t, nil, "docs: x\n\nTask: T-2, T-3, T-4, T-9\n")
	code, _, stderr, err := hook(t, p, "commit-msg", "msg")
	if code != policy || err != nil {
		t.Fatalf("exit %d, %v\n%s", code, err, stderr)
	}
	for _, want := range []string{
		"T-2 Done\n  $ exit 1\n",
		"failing T-3: `exit 1` exited 1; T-3 is doing in tasks/work-items.yaml, so the commit goes through",
		"failing T-4: `exit 1` exited 1; T-4 has no item in tasks/work-items.yaml",
		"Commit rejected:\n  - failing T-2: `exit 1` exited 1, and tasks/work-items.yaml says T-2 is done\n",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	p = staging(t, nil, "docs: x\n\nTask: T-3\n")
	if code, _, stderr, err := hook(t, p, "commit-msg", "msg"); code != 0 || err != nil || !strings.Contains(stderr, "failing T-3") {
		t.Errorf("a failing task not done: exit %d, %v\n%s", code, err, stderr)
	}
	text := strings.Replace(rules, "version: 1\n", "version: 1\nhooks: { commit_msg: { task_checks: false } }\n", 1)
	index := with(project, nil)
	p = over(t, world{head: index, index: index, work: with(index, map[string]string{"msg": "docs: x\n\nTask: T-2\n"}), text: text})
	if code, _, stderr, err := hook(t, p, "commit-msg", "msg"); code != 0 || err != nil || stderr != "" {
		t.Errorf("task_checks false: exit %d, %v\n%s", code, err, stderr)
	}
	text = strings.Replace(rules, "version: 1\n", "version: 1\nhooks: { commit_msg: { check_timeout: 30 } }\n", 1)
	p = over(t, world{head: index, index: index, work: with(index, map[string]string{"msg": "docs: x\n\nTask: T-1\n"}), text: text})
	if code, _, stderr, err := hook(t, p, "commit-msg", "msg"); code != 0 || err != nil || stderr != "" {
		t.Errorf("a timeout: exit %d, %v\n%s", code, err, stderr)
	}
}

// A task whose checks cannot run stops the hook with its error; a staged
// tree whose config or ledger does not load names no task, and an index
// that cannot be read is an error.
func TestTasksRuleErrors(t *testing.T) {
	broken := strings.Replace(tasksLedger, "done_when: [{ run: \"true\", cost: static }]", "done_when: 3", 1)
	p := staging(t, nil, "docs: x\n\nTask: T-1\n")
	p.trees.Files["index"]["tasks/phase-1.yaml"] = broken
	h := NewHandler(p)
	if _, err := h.tasksRule(t.Context(), p.cfg, "docs: x\n\nTask: T-1\n", Out{}); err == nil || !strings.Contains(err.Error(), "done_when") {
		t.Errorf("a task whose checks cannot run: %v", err)
	}
	p.trees.Files["index"]["tasks/phase-1.yaml"] = "a: 1\n"
	index, err := p.CaptureIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if named := h.tasksAt(t.Context(), []string{"T-1"}, index); named != nil {
		t.Errorf("a staged ledger that is no list: %v", named)
	}
	p.trees.Files["index"]["itos.yaml"] = "version: [\n"
	if named := h.tasksAt(t.Context(), []string{"T-1"}, index); named != nil {
		t.Errorf("a broken staged config: %v", named)
	}
	p.trees.Common = ""
	if _, err := h.tasksRule(t.Context(), p.cfg, "docs: x\n\nTask: T-1\n", Out{Stderr: &strings.Builder{}}); err == nil {
		t.Error("the tasks rule over an index that cannot be read: no error")
	}
}

// An amend is what itos commit says it is, else a commit by HEAD's author
// in HEAD's second; it is judged against HEAD's parent.
func TestAmending(t *testing.T) {
	p := staging(t, nil, "docs: x\n")
	h := NewHandler(p)
	if h.amending(t.Context()) {
		t.Error("no word and no author: an amend")
	}
	t.Setenv("GIT_AUTHOR_NAME", "A")
	t.Setenv("GIT_AUTHOR_EMAIL", "a@x")
	t.Setenv("GIT_AUTHOR_DATE", "@100")
	if !h.amending(t.Context()) || h.judgedBase(t.Context()) != vcsport.EmptyTree {
		t.Error("HEAD's author in HEAD's second: not an amend of the root")
	}
	t.Setenv("GIT_AUTHOR_DATE", "@101")
	if h.amending(t.Context()) {
		t.Error("another second: an amend")
	}
	t.Setenv("GIT_AUTHOR_DATE", "@100")
	p.VCS.Named = map[string]string{}
	if h.amending(t.Context()) {
		t.Error("no HEAD: an amend")
	}
	t.Setenv(message.AmendEnv, "0")
	if h.amending(t.Context()) {
		t.Error("itos commit's 0: an amend")
	}
	t.Setenv(message.AmendEnv, "1")
	if !h.amending(t.Context()) {
		t.Error("itos commit's 1: no amend")
	}
}

// Under a stealth config the footers judged are those itos commit hands
// over, else an amend's HEAD note, else none; a note that cannot be read
// stops the hook.
func TestHandedFooters(t *testing.T) {
	p := staging(t, nil, "docs: x\n")
	h := NewHandler(p)
	p.VCS.History[0].Note = "Task: T-1\n"
	if got, err := h.handedFooters(t.Context()); got != "" || err != nil {
		t.Errorf("no amend: %q, %v", got, err)
	}
	t.Setenv(message.AmendEnv, "1")
	if got, err := h.handedFooters(t.Context()); got != "Task: T-1\n" || err != nil {
		t.Errorf("an amend: %q, %v", got, err)
	}
	p.VCS.Named = map[string]string{}
	if _, err := h.handedFooters(t.Context()); err == nil {
		t.Error("an amend with no HEAD: no error")
	}
	t.Setenv(message.FootersEnv, "Task: T-2")
	if got, err := h.handedFooters(t.Context()); got != "Task: T-2" || err != nil {
		t.Errorf("handed: %q, %v", got, err)
	}
}

// A stealth commit's tasks are those its note will hold: here T-2, which
// fails and is done, though the message names none; a note that cannot be
// read stops the hook.
func TestCommitMsgStealth(t *testing.T) {
	p := staging(t, nil, "docs: x\n")
	p.cfg.Stealth = true
	t.Setenv(message.FootersEnv, "Task: T-2")
	code, _, stderr, err := hook(t, p, "commit-msg", "msg")
	if code != policy || err != nil || !strings.Contains(stderr, "failing T-2") {
		t.Errorf("exit %d, %v\n%s", code, err, stderr)
	}
	os.Unsetenv(message.FootersEnv)
	t.Setenv(message.AmendEnv, "1")
	p.VCS.History = append(p.VCS.History, vcsfake.Commit{Name: "next", Parents: []string{"root"}, Message: "docs: next\n", Tree: project})
	p.VCS.Named["HEAD"] = "next"
	p.trees.Files["next"] = project
	p.VCS.History[0].Tree = project
	if code, _, stderr, err := hook(t, p, "commit-msg", "msg"); code != 0 || err != nil {
		t.Errorf("an amend with no note: exit %d, %v\n%s", code, err, stderr)
	}
	p.shows = 2
	if _, _, _, err := hook(t, p, "commit-msg", "msg"); err == nil || err.Error() != "show refused" {
		t.Errorf("a note the lint cannot read: %v", err)
	}
}

// A moves check whose code glob cannot be read stops the hook.
func TestCommitMsgMovesError(t *testing.T) {
	text := rules + "tests:\n  scenario:\n    root: features\n    id: \"ID-[A-Z]+-\\\\d+\"\n" +
		"    range_checks: [{ name: moves, builtin: moves, code: [\"?a\"] }]\n"
	index := with(project, map[string]string{"b.md": "b\n"})
	p := over(t, world{head: project, index: index, work: with(index, map[string]string{"msg": "docs: b\n"}), text: text})
	if _, _, _, err := hook(t, p, "commit-msg", "msg"); err == nil || !strings.Contains(err.Error(), "Nothing to repeat") {
		t.Errorf("a moves glob that cannot be read: %v", err)
	}
}

// stagedChange: the moves rule's change, the footers' tests and the
// staged paths, a merge's own; an error when the staged paths, or under a
// stealth config the note an amend carries, cannot be read.
func TestStagedChange(t *testing.T) {
	p := staging(t, map[string]string{"b.md": "b\n"}, "docs: x\n")
	h := NewHandler(p)
	got, err := h.stagedChange(t.Context(), p.cfg, "docs: x\n", "", nil)
	if err != nil || !slices.Equal(got.Paths, []string{"b.md"}) {
		t.Errorf("staged: %+v, %v", got, err)
	}
	if _, err := h.stagedChange(t.Context(), p.cfg, "docs: x\n", "no-such-commit", nil); err == nil {
		t.Error("staged paths against no commit: no error")
	}
	if _, err := h.stagedRule(t.Context(), p.cfg, "docs: x\n", "no-such-commit", nil, Out{}); err == nil {
		t.Error("the rule over staged paths against no commit: no error")
	}
	if _, err := h.stagedRule(t.Context(), p.cfg, "fix: x\n", "no-such-commit", nil, Out{}); err == nil {
		t.Error("the moves rule over staged paths against no commit: no error")
	}
	bad, err := config.FromText(p.VCS, p.trees, nil, "itos.yaml", strings.Replace(rules, `"**/*.md"`, `"?a"`, 1), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.stagedRule(t.Context(), bad, "docs: x\n", "", nil, Out{}); err == nil {
		t.Error("a glob that cannot be read: no error")
	}
	p.cfg.Stealth = true
	t.Setenv(message.AmendEnv, "1")
	p.VCS.Named = map[string]string{}
	if _, err := h.stagedChange(t.Context(), p.cfg, "docs: x\n", "", nil); err == nil {
		t.Error("an amend's note that cannot be read: no error")
	}
}

// mergeWorld is a merge being made of topic into main, with the index
// holding both and, when own is set, a change of the merge's own.
func mergeWorld(t *testing.T, own bool, msg string) *ports {
	t.Helper()
	text := "version: 1\ncommits:\n  types: [docs, chore]\n  scopes:\n    docs: { only: [\"**/*.md\"] }\n    chore: {}\n"
	base := map[string]string{"itos.yaml": text, "a.md": "a\n"}
	main := with(base, map[string]string{"main.md": "main\n"})
	topic := with(base, map[string]string{"topic.md": "topic\n"})
	index := with(main, map[string]string{"topic.md": "topic\n"})
	if own {
		index["src.js"] = "src\n"
	}
	p := over(t, world{head: main, index: index, work: with(index, map[string]string{"msg": msg}), text: text})
	p.VCS.History = []vcsfake.Commit{
		{Name: "base", Message: "docs: start\n", Tree: base},
		{Name: "topic", Parents: []string{"base"}, Message: "docs: topic\n", Tree: topic},
		{Name: "main", Parents: []string{"base"}, Message: "docs: main\n", Tree: main},
	}
	p.VCS.Named = map[string]string{"HEAD": "main"}
	p.VCS.MergeHead = []string{"topic"}
	p.trees.Files["main"] = main
	return p
}

// The hook judges a merge being made as verify judges it made (bug 30): by
// its own changes, none passing it whatever its message, and some judging
// it as its first line's type, refused when that is none of the commit
// types.
func TestCommitMsgMerge(t *testing.T) {
	p := mergeWorld(t, false, "update things\n")
	if code, _, stderr, err := hook(t, p, "commit-msg", "msg"); code != 0 || err != nil {
		t.Errorf("a merge with no change of its own: exit %d, %v\n%s", code, err, stderr)
	}
	p = mergeWorld(t, true, "Merge branch 'topic'\n")
	if code, _, stderr, _ := hook(t, p, "commit-msg", "msg"); code != policy ||
		!strings.Contains(stderr, "a merge commit with changes of its own (src.js)") {
		t.Errorf("an untyped merge with a change of its own: exit %d\n%s", code, stderr)
	}
	p = mergeWorld(t, true, "docs: merge the topic\n")
	if code, _, stderr, _ := hook(t, p, "commit-msg", "msg"); code != policy ||
		!strings.Contains(stderr, "docs commits may not touch src.js") || strings.Contains(stderr, "topic.md") {
		t.Errorf("a docs merge with a change of its own: exit %d\n%s", code, stderr)
	}
	p = mergeWorld(t, true, "chore: merge the topic\n")
	if code, _, stderr, err := hook(t, p, "commit-msg", "msg"); code != 0 || err != nil {
		t.Errorf("a chore merge with a change of its own: exit %d, %v\n%s", code, err, stderr)
	}
}

// A merge's parents: HEAD and MERGE_HEAD, or an amend's HEAD's; an error
// when HEAD or a parent names no commit; none for a commit of one parent.
func TestStagedMerge(t *testing.T) {
	p := mergeWorld(t, false, "x\n")
	h := NewHandler(p)
	p.VCS.Named = map[string]string{}
	if _, err := h.stagedMerge(t.Context()); err == nil {
		t.Error("a merge with no HEAD: no error")
	}
	p.VCS.Named = map[string]string{"HEAD": "main"}
	p.VCS.MergeHead = []string{"gone"}
	if _, err := h.stagedMerge(t.Context()); err == nil {
		t.Error("a merge of no commit: no error")
	}
	if _, _, _, err := hook(t, p, "commit-msg", "msg"); err == nil {
		t.Error("the hook over a merge of no commit: no error")
	}
	p.VCS.MergeHead = nil
	t.Setenv(message.AmendEnv, "1")
	if m, err := h.stagedMerge(t.Context()); m != nil || err != nil {
		t.Errorf("an amend of one parent: %+v, %v", m, err)
	}
	p.VCS.History = append(p.VCS.History, vcsfake.Commit{Name: "merged", Parents: []string{"main", "topic"}, Message: "x\n", Tree: p.VCS.Index})
	p.VCS.Named["HEAD"] = "merged"
	if m, err := h.stagedMerge(t.Context()); m == nil || err != nil || m.Parent != "main" {
		t.Errorf("an amend of a merge: %+v, %v", m, err)
	}
	p.VCS.Named = map[string]string{}
	if _, err := h.stagedMerge(t.Context()); err == nil {
		t.Error("an amend with no HEAD: no error")
	}
	p.VCS.Named = map[string]string{"HEAD": "main"}
	if _, _, _, err := hook(t, p, "commit-msg", "msg"); err != nil {
		t.Errorf("an amend of HEAD: %v", err)
	}
}

// A staged range command's problem lines are its `  - …` lines, read as
// JavaScript's /^\s+- (.*\S)/ reads them.
func TestProblemLine(t *testing.T) {
	var got []string
	for _, l := range []string{"  - one  ", "- not indented", "\t- two\r", "  -   ", "    - three - more"} {
		if m := problemLine.FindStringSubmatch(l); m != nil {
			got = append(got, m[1])
		}
	}
	if want := []string{"one", "two", "three - more"}; !slices.Equal(got, want) {
		t.Errorf("problem lines: %q", got)
	}
}

// Install declares both hooks, printing each, its lines on stderr under
// --json; an entry it cannot write and a config that does not load are
// errors; an old git exits 3.
func TestInstall(t *testing.T) {
	p := over(t, world{text: "version: 1\n"})
	code, stdout, stderr, err := hook(t, p, "install")
	if code != 0 || err != nil || stdout != "wrote hook.itos-commit-msg\nwrote hook.itos-pre-push\n" || stderr != "" {
		t.Errorf("install: exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	c, _ := Parse([]string{"install", "--force"}, true, "", nil)
	var out, log strings.Builder
	code, err = c.Run(t.Context(), &out, &log, NewHandler(p))
	var got struct {
		Hooks []struct{ Action string } `json:"hooks"`
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(out.String()), &got) != nil || len(got.Hooks) != 2 ||
		log.String() != "unchanged hook.itos-commit-msg\nunchanged hook.itos-pre-push\n" {
		t.Errorf("install --json: exit %d, %v\n%s%s", code, err, out.String(), log.String())
	}
	p.git = map[string][]string{}
	p.setErr = errors.New("locked")
	if _, _, _, err := hook(t, p, "install"); err == nil {
		t.Error("an entry that cannot be written: no error")
	}
	p.oldGit = true
	if code, stdout, stderr, _ := hook(t, p, "install"); code != 0 || len(p.hookFiles) != 2 ||
		!strings.Contains(p.hookFiles[".git/hooks/pre-push"], `itos hook pre-push "$@"`) ||
		stdout != "written .git/hooks/commit-msg\nwritten .git/hooks/pre-push\n" {
		t.Errorf("an old git: exit %d, files %q\n%s%s", code, p.hookFiles, stdout, stderr)
	}
	p.cfg, p.loadErr = nil, errors.New("broken")
	if _, _, _, err := hook(t, p, "install"); err == nil {
		t.Error("a config that does not load: no error")
	}
}

func TestShort(t *testing.T) {
	if short("abcdef0123") != "abcdef0" || short("abc") != "abc" {
		t.Error("short")
	}
}
