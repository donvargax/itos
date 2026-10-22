package initcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// head is the scratch repository's one commit.
const head = "0123456789abcdef0123456789abcdef01234567"

// sums is the newest release's checksums.txt.
const sums = "aa  itos-9.2.0-linux-amd64.tar.gz\n"

// fake is init's ports over the VCS fake and the folder the test stands
// in: the git config, and an old git that runs no hook its config
// declares; git init failing with initErr; the newest release, or
// releaseErr; what hook install writes, and under json, its exit and its error, each call's
// json recorded; what the offers come to and say, each offer recorded; and
// a folder made absolute failing with absErr, a file's write with its
// error in writeErrs.
type fake struct {
	*vcsfake.VCS
	git        map[string][]string
	oldGit     bool
	initErr    error
	releaseErr error
	hooks      string
	hooksJSON  string
	hooksCode  int
	hooksErr   error
	installs   []bool
	offered    Offered
	said       string
	offers     []Offer
	absErr     error
	writeErrs  map[string]error
}

func (f *fake) Init(ctx context.Context) error {
	if f.initErr != nil {
		return f.initErr
	}
	return f.VCS.Init(ctx)
}

func (f *fake) Config(_ context.Context, key string, _ bool) []string { return f.git[key] }

func (f *fake) SetConfig(_ context.Context, key, value string) error {
	f.git[key] = []string{value}
	return nil
}

func (f *fake) ConfigHooksRun(context.Context) bool { return !f.oldGit }

func (f *fake) Version(context.Context) string { return "2.53.0" }

func (f *fake) Trees(context.Context) treeport.Trees { return disk{} }

func (f *fake) Worktree(context.Context) treeport.Source { return disk{} }

func (f *fake) TaskFlags(context.Context) []string { return []string{"--config", "--root", "--json"} }

func (f *fake) Release(_ context.Context, want string) (string, []byte, error) {
	if f.releaseErr != nil {
		return "", nil, f.releaseErr
	}
	return "9.2.0", []byte(sums), nil
}

func (f *fake) InstallHooks(_ context.Context, stdout io.Writer, json bool) (int, error) {
	f.installs = append(f.installs, json)
	if json {
		fmt.Fprint(stdout, f.hooksJSON)
	} else {
		fmt.Fprint(stdout, f.hooks)
	}
	return f.hooksCode, f.hooksErr
}

func (f *fake) Offers(_ context.Context, o Offer, log io.Writer) Offered {
	f.offers = append(f.offers, o)
	fmt.Fprint(log, f.said)
	return f.offered
}

func (f *fake) Abs(_ context.Context, path string) (string, error) {
	if f.absErr != nil {
		return "", f.absErr
	}
	return filepath.Abs(path)
}

func (f *fake) WriteFile(_ context.Context, name string, data []byte, perm os.FileMode) error {
	if err := f.writeErrs[name]; err != nil {
		return err
	}
	return os.WriteFile(name, data, perm)
}

// disk is the working tree, the files on disk where the test stands; the
// only tree it has.
type disk struct{}

func (disk) At(_ context.Context, tree string) (treeport.Source, error) {
	if tree != "worktree" {
		return nil, errors.New(tree + " cannot be read")
	}
	return disk{}, nil
}

func (disk) Texts(context.Context, string, string, func(string) bool) map[string]string {
	return map[string]string{}
}

func (disk) Tree(context.Context) string { return "worktree" }

func (disk) Has(_ context.Context, p string) bool { return exists(p) }

func (disk) Read(_ context.Context, p string) (string, error) {
	text, err := os.ReadFile(p)
	return string(text), err
}

func (disk) List(_ context.Context, dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, err
}

// scratch is the ports over an empty folder the test stands in, holding
// files, with no ITOS_CONFIG: a repository with one commit when repo is
// "commit", one with none when "unborn", none when "".
func scratch(t *testing.T, repo string, files map[string]string) *fake {
	t.Helper()
	t.Setenv("ITOS_CONFIG", "")
	os.Unsetenv("ITOS_CONFIG")
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	for p, text := range files {
		put(t, p, text)
	}
	f := &fake{VCS: &vcsfake.VCS{Root: dir}, git: map[string][]string{}, hooks: "hooks declared\n",
		hooksJSON: "{\n  \"schema\": 1,\n  \"hooks\": []\n}\n",
		offered:   Offered{Plugin: "plugin", Shim: "shim", Rules: "rules"}, said: "offered\n"}
	if repo != "" {
		f.Located = map[string]vcsport.Location{"": {Top: dir, GitDir: ".git", Common: ".git"}}
		put(t, ".git/HEAD", "ref: refs/heads/main\n")
	}
	if repo == "commit" {
		f.History = []vcsfake.Commit{{Name: head, Tree: map[string]string{"README.md": "# Scratch\n"}}}
		f.Named = map[string]string{"HEAD": head}
	}
	return f
}

// put writes a file, its folder made first.
func put(t *testing.T, p, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// read is a file's text, "" when it cannot be read.
func read(p string) string {
	text, _ := os.ReadFile(p)
	return string(text)
}

// call is init's line run over f: its exit, stdout, stderr and error; a
// --json first sets the global flag.
func call(t *testing.T, f *fake, args ...string) (int, string, string, error) {
	t.Helper()
	asJSON := len(args) > 0 && args[0] == "--json"
	if asJSON {
		args = args[1:]
	}
	c, err := Parse(context.Background(), args, "", asJSON, &tests.Warnings{}, f)
	if err != nil {
		return 0, "", "", err
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(context.Background(), &stdout, &stderr, NewHandler(f))
	return code, stdout.String(), stderr.String(), err
}

// isUsage is whether err is a usage error (out.Usage's, or of the kind)
// saying want.
func isUsage(err error, want string) bool {
	var u out.UsageError
	return err != nil && (errors.As(err, &u) || kind.Of(err) == kind.Usage) && strings.Contains(err.Error(), want)
}

// Parse refuses what init does not take, each a usage error, before it
// looks at any folder.
func TestParseRefuses(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--plugin", "everywhere"}, "--plugin takes project, user, local or no (everywhere)"},
		{[]string{"--plugin=everywhere"}, "--plugin takes project, user, local or no (everywhere)"},
		{[]string{"--bogus"}, "init takes only --stealth, --policy <file>, --plugin [<scope>], --git-shim, " +
			"--git-shim-dir <folder>, --no-git-shim, --agent-rules and --no-agent-rules (--bogus)"},
		{[]string{"--policy"}, "(--policy)"},
		{[]string{"--no-git-shim", "--git-shim-dir", "x"}, "--git-shim-dir names where --git-shim links itos as git, not --no-git-shim"},
		{[]string{"--git-shim", "--git-shim-dir"}, "--git-shim-dir needs <folder>"},
		{[]string{"--git-shim-dir="}, "--git-shim-dir needs <folder>"},
		{[]string{"--git-shim", "--no-git-shim"}, "init takes --git-shim or --no-git-shim, not both"},
		{[]string{"--no-git-shim", "--git-shim"}, "init takes --git-shim or --no-git-shim, not both"},
		{[]string{"--agent-rules", "--no-agent-rules"}, "init takes --agent-rules or --no-agent-rules, not both"},
		{[]string{"--stealth", "--plugin", "project"}, "under --stealth, --plugin project would write .claude/settings.json"},
	} {
		if _, err := Parse(context.Background(), c.args, "", false, nil, &fake{}); !isUsage(err, c.want) {
			t.Errorf("init %v: %v", c.args, err)
		}
	}
}

// Parse reads each flag in each of its forms, a folder typed relative to
// where itos was started.
func TestParseReadsItsFlags(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join("sub", "bin"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args []string
		want Query
	}{
		{[]string{"--stealth", "--policy", "p.yaml", "--plugin", "--git-shim", "--git-shim-dir=bin", "--agent-rules"},
			Query{Stealth: true, Policy: "p.yaml", Plugin: PluginFlag{Given: true}, Shim: ShimFlag{true, true, abs},
				Rules: RulesFlag{true, true}, Origin: "sub"}},
		{[]string{"--plugin", "user", "--no-git-shim", "--no-git-shim", "--no-agent-rules", "--no-agent-rules"},
			Query{Plugin: PluginFlag{true, "user"}, Shim: ShimFlag{Given: true}, Rules: RulesFlag{Given: true}, Origin: "sub"}},
		{[]string{"--plugin=local", "--git-shim-dir", "bin", "--plugin"},
			Query{Plugin: PluginFlag{Given: true}, Shim: ShimFlag{Dir: abs}, Origin: "sub"}},
		{[]string{"--stealth", "--plugin=local"}, Query{Stealth: true, Plugin: PluginFlag{true, "local"}, Origin: "sub"}},
	} {
		got, err := Parse(context.Background(), c.args, "sub", true, nil, &fake{})
		if err != nil || got.Query != c.want || !got.JSON {
			t.Errorf("init %v: %+v, %v", c.args, got, err)
		}
	}
	if got, _ := Parse(context.Background(), []string{"--git-shim-dir", abs}, "sub", false, nil, &fake{}); got.Shim.Dir != abs {
		t.Errorf("an absolute folder: %q", got.Shim.Dir)
	}
	for p, want := range map[string]string{"": "", "-": "-", "a": filepath.Join("sub", "a"), abs: abs} {
		if got := typed("sub", p); got != want {
			t.Errorf("typed %q: %q", p, got)
		}
	}
	if got := typed("", "a"); got != "a" {
		t.Errorf("typed with no origin: %q", got)
	}
}

// A --git-shim-dir folder that cannot be made absolute stops Parse with
// that error, in either form, before a later argument's usage error.
func TestParseStopsAtAFolderItCannotMakeAbsolute(t *testing.T) {
	f := &fake{absErr: errors.New("getwd: the working folder is gone")}
	for _, args := range [][]string{{"--git-shim-dir", "bin", "--bogus"}, {"--git-shim-dir=bin", "--no-git-shim"}} {
		if _, err := Parse(context.Background(), args, "sub", false, nil, f); err != f.absErr {
			t.Errorf("init %v: %v", args, err)
		}
	}
}

// The starter written in a repository with a commit: each file, what init
// says of it, hook install, the offers asked on a terminal, and what to
// commit, the project's plugin settings and the rules for agents among it.
func TestInitWritesTheStarter(t *testing.T) {
	f := scratch(t, "commit", nil)
	f.Work = map[string]string{"README.md": "# Scratch\n", projectSettings: "{}\n"}
	f.offered.ProjectPlugin, f.offered.RulesWritten = true, []string{"AGENTS.md", "CLAUDE.md"}
	code, stdout, stderr, err := call(t, f)
	want := "wrote itos.yaml\nwrote tasks/phase-1.yaml\nwrote tasks/work-items.yaml\n" +
		"commits.since is HEAD, 0123456: no commit before it is judged.\n" +
		"Pinned itos 9.2.0, the newest release.\nhooks declared\noffered\n" +
		"Commit what init wrote with the task that adopts itos: git add itos.yaml tasks/phase-1.yaml " +
		"tasks/work-items.yaml .claude/settings.json AGENTS.md CLAUDE.md, then itos commit --task T-1 -m 'chore: adopt itos'\n"
	if code != 0 || err != nil || stdout != want || stderr != "" {
		t.Fatalf("exit %d, %v:\n%s\nwant\n%s%s", code, err, stdout, want, stderr)
	}
	pin := release.SHA256([]byte(sums))
	if text := read("itos.yaml"); text != (starter{since: head, pin: &[2]string{"9.2.0", pin}}).config() {
		t.Errorf("itos.yaml:\n%s", text)
	}
	if read("tasks/phase-1.yaml") != (starter{}).ledger() || read("tasks/work-items.yaml") != starterRegistryText {
		t.Error("the ledger or the registry is not the starter's")
	}
	if len(f.installs) != 1 || f.installs[0] || len(f.offers) != 1 ||
		f.offers[0] != (Offer{Plugin: PluginFlag{}, Ask: true}) {
		t.Errorf("hooks %v, offers %+v", f.installs, f.offers)
	}
}

// Run again past the config check, files there are kept and none written:
// the next step names no file and no task; a plugin installed for the
// project whose settings git does not show is left out.
func TestInitWriteKeepsWhatIsThere(t *testing.T) {
	f := scratch(t, "commit", map[string]string{"itos.yaml": "version: 1\n", "tasks/phase-1.yaml": "[]\n",
		"tasks/work-items.yaml": "phases: {}\nitems: []\n"})
	f.offered.ProjectPlugin = true
	var stdout strings.Builder
	code, err := NewHandler(f).write(context.Background(), false, false, Offer{}, &stdout, Out{Stdout: &stdout})
	want := "kept itos.yaml\nkept tasks/phase-1.yaml\nkept tasks/work-items.yaml\n" +
		"commits.since is HEAD, 0123456: no commit before it is judged.\nPinned itos 9.2.0, the newest release.\n" +
		"hooks declared\noffered\nCommit what init wrote with the task that adopts itos: git add , then itos commit " +
		"--task <a task of the ledger> -m 'chore: adopt itos'\n"
	if code != 0 || err != nil || stdout.String() != want || read("itos.yaml") != "version: 1\n" {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout.String())
	}
}

// In a folder that is no repository init runs git init and says so; the
// config has no since, and with no release server pins nothing and says why.
func TestInitRunsGitInit(t *testing.T) {
	f := scratch(t, "", nil)
	f.releaseErr = errors.New("the release server cannot be reached")
	code, stdout, _, err := call(t, f)
	here, _ := os.Getwd()
	for _, want := range []string{
		"Ran git init: " + here + " is a git repository now.\n",
		"The repository has no commit, so the config has no commits.since: every commit is judged.\n",
		"Pinned no itos: the release server cannot be reached. Run itos pin when the release server can be reached.\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("no %q in:\n%s", want, stdout)
		}
	}
	if code != 0 || err != nil || read("itos.yaml") != (starter{}).config() {
		t.Errorf("exit %d, %v:\n%s", code, err, read("itos.yaml"))
	}
	f = scratch(t, "", nil)
	f.initErr = errors.New("git init: injected git failure")
	if _, _, _, err := call(t, f); err != f.initErr {
		t.Errorf("git init failing: %v", err)
	}
}

// Under --stealth the starter goes beside the stealth config, and the next
// step is to commit as ever; a git that cannot name its folder stops it.
func TestInitWritesTheStealthStarter(t *testing.T) {
	f := scratch(t, "unborn", nil)
	code, stdout, _, err := call(t, f, "--stealth")
	stealth := filepath.Join(".git", "itos")
	if code != 0 || err != nil || read(filepath.Join(stealth, "itos.yaml")) != (starter{stealth: true, pin: &[2]string{"9.2.0", release.SHA256([]byte(sums))}}).config() ||
		read(filepath.Join(stealth, "tasks", "phase-1.yaml")) != (starter{stealth: true}).ledger() ||
		!strings.HasSuffix(stdout, "offered\nNothing the project tracks changed: commit as ever, with itos commit, and push with itos push.\n") {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout)
	}
	if exists("itos.yaml") || !f.offers[0].Stealth {
		t.Errorf("itos.yaml written, or the offers %+v", f.offers)
	}
	f = scratch(t, "", nil)
	if _, err := NewHandler(f).write(context.Background(), true, false, Offer{}, io.Discard, Out{Stdout: io.Discard}); err == nil {
		t.Error("stealth, no repository: no error")
	}
}

// With feature files the starter has the scenario kind and a smoke set
// naming each file's first live scenario; with none tagged the footer is
// required of no type, and init says how to tag one.
func TestInitWritesTheSmokeSet(t *testing.T) {
	scenario := func(tag string) string { return "Feature: A\n\n  " + tag + "\n  Scenario: One\n    Given a\n" }
	for _, c := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"one", map[string]string{"features/a.feature": scenario("@ID-A-01")},
			"The smoke set names 1 scenario, one of each feature file with a live one; a feat or a fix names the ones it " +
				"turns green (itos commit --scenarios).\n"},
		{"two", map[string]string{"features/a.feature": scenario("@ID-A-01"), "features/b/b.feature": scenario("@ID-B-01")},
			"The smoke set names 2 scenarios,"},
		{"untagged", map[string]string{"features/a.feature": scenario("@slow")},
			"No scenario under features carries an ID tag, so the smoke set names none and no feat or fix needs a Scenarios " +
				"footer yet: tag a scenario with its ID, such as @ID-PAGE-01 on the line above it, then set " +
				"commits.footers.Scenarios.required_for to [feat, fix] in itos.yaml.\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := scratch(t, "commit", c.files)
			code, stdout, _, err := call(t, f)
			if code != 0 || err != nil || !strings.Contains(stdout, "wrote features/smoke.yaml\n") || !strings.Contains(stdout, c.want) {
				t.Errorf("exit %d, %v:\n%s", code, err, stdout)
			}
			untagged := c.name == "untagged"
			if text := read("itos.yaml"); strings.Contains(text, "required_for: []") != untagged {
				t.Errorf("itos.yaml:\n%s", text)
			}
			if text := read("features/smoke.yaml"); strings.Contains(text, `"@ID-A-01"`) == untagged {
				t.Errorf("the smoke set:\n%s", text)
			}
		})
	}
}

// The scenarios are listed before anything is written: a config there that
// does not load, or whose kind cannot list them, stops init with nothing
// written.
func TestInitWriteStopsBeforeWriting(t *testing.T) {
	for name, text := range map[string]string{
		"a config that does not load": "version: [\n",
		"a list that fails":           "version: 1\ntests: { scenario: { root: missing, smoke: { file: s.yaml } } }\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := scratch(t, "commit", map[string]string{"itos.yaml": text, "features/a.feature": "Feature: A\n"})
			if _, err := NewHandler(f).write(context.Background(), false, false, Offer{}, io.Discard, Out{Stdout: io.Discard}); err == nil {
				t.Error("no error")
			}
			if exists(starterLedger) {
				t.Errorf("%s was written", starterLedger)
			}
		})
	}
}
