package commit

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
	"github.com/donvargax/itos/v8/internal/message"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// rules is a project's config: docs may touch Markdown alone; a feat needs
// a Task and an Upgrading; Breaking, a footer of free text whose flag
// --breaking takes, has none.
const rules = "version: 1\ncommits:\n  types: [docs, feat]\n  header_lint: { use: builtin }\n" +
	"  scopes: { docs: { only: [\"**/*.md\"] } }\n  footers:\n" +
	"    Task: { source: ledger, required_for: [feat] }\n" +
	"    Upgrading: { source: text, required_for: [feat] }\n" +
	"    Breaking: { source: text, required_for: [feat] }\n"

// fake is the ports over the VCS fake: the configs the two loads give, or
// their errors; the stealth check's answer; the working tree's files; the
// git config, an addition to it refused with addErr, and an old git that
// runs no hook its config declares. A commit is recorded in made, with how
// git talks, and fails with failure, else moves HEAD to a new commit when
// moves.
type fake struct {
	*vcsfake.VCS
	cfg       *config.Loaded
	loadErr   error
	absent    error
	commitErr error
	stealth   bool
	files     map[string]string
	git       map[string][]string
	addErr    error
	oldGit    bool
	failure   error
	moves     bool
	made      []vcsport.Commit
	talks     []vcsport.Talk
}

func (f *fake) Stealth(context.Context) bool { return f.stealth }

func (f *fake) LoadConfig(context.Context) (*config.Loaded, error) { return f.cfg, f.loadErr }

func (f *fake) CommitConfig(context.Context) (*config.Loaded, error, error) {
	if f.commitErr != nil {
		return nil, nil, f.commitErr
	}
	return f.cfg, f.absent, nil
}

func (f *fake) Read(_ context.Context, path string) (string, error) {
	text, ok := f.files[filepath.ToSlash(path)]
	if !ok {
		return "", errors.New(path + ": no such file")
	}
	return text, nil
}

func (f *fake) Config(_ context.Context, key string, _ bool) []string { return f.git[key] }

func (f *fake) SetConfig(_ context.Context, key, value string) error {
	f.git[key] = []string{value}
	return nil
}

func (f *fake) AddConfig(_ context.Context, key, value string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.git[key] = append(f.git[key], value)
	return nil
}

func (f *fake) ConfigHooksRun(context.Context) bool { return !f.oldGit }

func (f *fake) Version(context.Context) string { return "2.53.0" }

// The hooks folder is .git/hooks, its files the working tree's.
func (f *fake) HooksPath(context.Context) (string, error) { return filepath.Join(".git", "hooks"), nil }

func (f *fake) HookFile(_ context.Context, path string) (string, error) {
	text, ok := f.files[filepath.ToSlash(path)]
	if !ok {
		return "", os.ErrNotExist
	}
	return text, nil
}

func (f *fake) WriteHookFile(context.Context, string, string) error {
	return errors.New("no hook file is written")
}

func (f *fake) MoveHookFile(context.Context, string, string) error {
	return errors.New("no hook file is moved")
}

func (f *fake) Commit(_ context.Context, c vcsport.Commit, talk vcsport.Talk) error {
	f.made, f.talks = append(f.made, c), append(f.talks, talk)
	if f.failure != nil {
		return f.failure
	}
	if f.moves {
		head, _ := f.VCS.Resolve(context.Background(), "HEAD")
		f.VCS.History = append(f.VCS.History, vcsfake.Commit{Name: "made", Parents: []string{head}, Message: "made\n"})
		f.VCS.Named["HEAD"] = "made"
	}
	return nil
}

// repository is one commit, HEAD, whose message is a docs commit's and
// whose note links T-1, inside a repository when located.
func repository(located bool) *vcsfake.VCS {
	v := &vcsfake.VCS{
		History: []vcsfake.Commit{{Name: "root", Message: "docs: start\n\nUpgrading: none\n", Note: "Task: T-1\n"}},
		Named:   map[string]string{"HEAD": "root"},
	}
	if located {
		v.Located = map[string]vcsport.Location{"": {Top: "/r", GitDir: ".git", Common: ".git"}}
	}
	return v
}

// over is the ports over repo with the config text, none when text is "",
// and itos's commit-msg hook declared in the git config.
func over(t *testing.T, repo *vcsfake.VCS, text string) *fake {
	t.Helper()
	return loaded(t, repo, text, false)
}

// stealthOver is over under the stealth config, the config's file in the
// git folder.
func stealthOver(t *testing.T, repo *vcsfake.VCS, text string) *fake {
	t.Helper()
	f := loaded(t, repo, text, true)
	f.stealth = true
	return f
}

// loaded is over, the config the stealth one when stealth.
func loaded(t *testing.T, repo *vcsfake.VCS, text string, stealth bool) *fake {
	t.Helper()
	f := &fake{VCS: repo, files: map[string]string{}, git: map[string][]string{
		"hook.itos-commit-msg.command": {"itos hook commit-msg"},
		"hook.itos-commit-msg.event":   {"commit-msg"},
	}}
	if text == "" {
		f.absent = errors.New("itos.yaml: cannot be read")
		return f
	}
	trees := treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": {}}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	if f.cfg, err = config.FromText(repo, trees, src, "itos.yaml", text, stealth); err != nil {
		t.Fatal(err)
	}
	return f
}

// run is commit's command line over f, from origin, its exit, streams and
// error.
func run(t *testing.T, f *fake, origin string, json bool, args ...string) (int, string, string, error) {
	t.Helper()
	c, err := Parse(args, json, false, origin, &tests.Warnings{})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(f))
	return code, stdout.String(), stderr.String(), err
}

// stdin is the process's stdin reading text until the test ends.
func stdin(t *testing.T, text string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	was := os.Stdin
	os.Stdin = in
	t.Cleanup(func() {
		os.Stdin = was
		in.Close()
	})
}

// A first word that is no flag is a subcommand, each reading its own
// arguments, paths from where the person stood; anything else is a commit.
func TestParse(t *testing.T) {
	warnings := &tests.Warnings{}
	c, err := Parse(nil, true, true, "sub", warnings)
	if err != nil || c.Sub != "" || c.Args != nil || !c.JSON || !c.Quiet || c.Tests != warnings || c.Origin != "sub" {
		t.Errorf("Parse() = %+v, %v", c, err)
	}
	c, err = Parse([]string{"-m", "x", "a.md"}, false, false, "", nil)
	if err != nil || c.Sub != "" || !slices.Equal(c.Args, []string{"-m", "x", "a.md"}) {
		t.Errorf("Parse(-m x a.md) = %+v, %v", c, err)
	}
	if c := Git([]string{"a.md"}, true, false, "sub", warnings); c.Sub != "" || !slices.Equal(c.Args, []string{"a.md"}) ||
		!c.JSON || c.Origin != "sub" || c.Tests != warnings {
		t.Errorf("Git(a.md) = %+v", c)
	}
	c, err = Parse([]string{"check-message", "m.txt", "--at", "HEAD"}, false, false, "sub", nil)
	if err != nil || c.Sub != "check-message" || c.File != filepath.Join("sub", "m.txt") || c.At != "HEAD" {
		t.Errorf("Parse(check-message m.txt --at HEAD) = %+v, %v", c, err)
	}
	c, err = Parse([]string{"check-message", "-"}, false, false, "sub", nil)
	if err != nil || c.File != "-" || c.At != "" {
		t.Errorf("Parse(check-message -) = %+v, %v", c, err)
	}
	abs, err := filepath.Abs("x.md")
	if err != nil {
		t.Fatal(err)
	}
	c, err = Parse([]string{"check-paths", "--type", "docs", "a.md", abs}, false, false, "sub", nil)
	if err != nil || c.Type != "docs" || !slices.Equal(c.Files, []string{"sub/a.md", filepath.ToSlash(abs)}) {
		t.Errorf("Parse(check-paths) = %+v, %v", c, err)
	}
	c, err = Parse([]string{"footers", "Upgrading", "a", "b"}, false, false, "", nil)
	if err != nil || c.Footer != "Upgrading" || c.From != "a" || c.To != "b" {
		t.Errorf("Parse(footers) = %+v, %v", c, err)
	}
	for args, want := range map[string]string{
		"check-message":           "commit check-message needs <file|->",
		"check-message --at HEAD": "commit check-message needs <file|->",
		"check-paths a.md":        "commit check-paths needs --type <type>",
		"footers Upgrading a":     "commit footers needs <name> <from> <to>",
		"bogus -m x":              "unknown command: commit bogus",
	} {
		if _, err := Parse(strings.Fields(args), false, false, "", nil); !isUsage(err, want) {
			t.Errorf("Parse(%s) = %v", args, err)
		}
	}
}

// check-message judges one message, from a file read where the person stood
// or from stdin, its --at a commit of the repository; the config, the file
// and the ref each refuse when they cannot be had.
func TestCheckMessage(t *testing.T) {
	t.Setenv("ITOS_AT", "")
	t.Setenv(message.FootersEnv, "")
	f := over(t, repository(true), rules)
	f.files["sub/m.txt"] = "docs: add a\n"
	f.files["bad.txt"] = "nope\n"
	if code, _, stderr, err := run(t, f, "sub", false, "check-message", "m.txt", "--at", "HEAD"); code != 0 || err != nil {
		t.Errorf("a good message: exit %d, %v\n%s", code, err, stderr)
	}
	if code, _, stderr, err := run(t, f, "", false, "check-message", "bad.txt"); code != 1 || err != nil || stderr == "" {
		t.Errorf("a bad message: exit %d, %v\n%s", code, err, stderr)
	}
	stdin(t, "nope\n")
	if code, stdout, _, err := run(t, f, "", true, "check-message", "-"); code != 1 || err != nil ||
		!strings.Contains(stdout, `"ok": false`) {
		t.Errorf("a bad message on stdin: exit %d, %v\n%s", code, err, stdout)
	}
	if _, _, _, err := run(t, f, "", false, "check-message", "m.txt", "--at", "gone"); !isUsage(err,
		"commit check-message: gone is not a commit of this repository") {
		t.Errorf("an --at naming no commit: %v", err)
	}
	if _, _, _, err := run(t, f, "", false, "check-message", "none.txt"); err == nil {
		t.Error("a file that is not there: no error")
	}
	f.loadErr = errors.New("no config")
	if _, _, _, err := run(t, f, "", false, "check-message", "bad.txt"); err != f.loadErr {
		t.Errorf("a config that does not load: %v", err)
	}
}

// stdin that cannot be read is the error.
func TestCheckMessageUnreadableStdin(t *testing.T) {
	in, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	was := os.Stdin
	os.Stdin = in
	t.Cleanup(func() {
		os.Stdin = was
		in.Close()
	})
	if _, _, _, err := run(t, over(t, repository(false), rules), "", false, "check-message", "-"); err == nil {
		t.Error("a folder as stdin: no error")
	}
}

// check-paths says what the type's path rules reject, exit 1, on stderr or
// under --json with the type and the paths; a config with no commits
// section, or none, is the error.
func TestCheckPaths(t *testing.T) {
	f := over(t, repository(false), rules)
	if code, stdout, stderr, err := run(t, f, "", false, "check-paths", "--type", "docs", "a.md"); code != 0 || err != nil ||
		stdout != "" || stderr != "" {
		t.Errorf("a Markdown path: exit %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err := run(t, f, "", false, "check-paths", "--type", "docs", "a.go")
	if code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "a.go") {
		t.Errorf("a Go path: exit %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = run(t, f, "", true, "check-paths", "--type", "docs")
	var got struct {
		Type     string   `json:"type"`
		Files    []string `json:"files"`
		OK       bool     `json:"ok"`
		Problems []struct {
			Rule string `json:"rule"`
		} `json:"problems"`
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Type != "docs" || got.Files == nil ||
		len(got.Files) != 0 || !got.OK {
		t.Errorf("no path under --json: exit %d, %v, %s", code, err, stdout)
	}
	code, stdout, _, err = run(t, f, "sub", true, "check-paths", "--type", "docs", "a.go")
	if code != refused || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.OK ||
		!slices.Equal(got.Files, []string{"sub/a.go"}) || len(got.Problems) != 1 || got.Problems[0].Rule != "scope-only" {
		t.Errorf("a Go path under --json: exit %d, %v, %s", code, err, stdout)
	}
	if _, _, _, err := run(t, over(t, repository(false), "version: 1\n"), "", false, "check-paths", "--type", "docs"); err == nil {
		t.Error("a config with no commits section: no error")
	}
	badGlob := over(t, repository(false), "version: 1\ncommits:\n  types: [bad]\n  scopes: { bad: { only: [\"*.md\", \"?x\"] } }\n")
	if _, _, _, err := run(t, badGlob, "", false, "check-paths", "--type", "bad", "a.ts"); err == nil {
		t.Error("a glob JavaScript refuses: no error")
	}
	f.loadErr = errors.New("no config")
	if _, _, _, err := run(t, f, "", false, "check-paths", "--type", "docs"); err != f.loadErr {
		t.Errorf("a config that does not load: %v", err)
	}
}

// footers lists a range's footers of free text, leaving out none, a line
// each or under --json with the full SHA and the subject; a footer of IDs,
// a ref naming no commit, a range that cannot be read and a config that
// does not load are refused.
func TestFooters(t *testing.T) {
	repo := repository(true)
	repo.History = append(repo.History,
		vcsfake.Commit{Name: "0123456789abcdef", Parents: []string{"root"}, Message: "feat: a\n\nUpgrading: run it again\n"},
		vcsfake.Commit{Name: "merge", Parents: []string{"0123456789abcdef", "root"}, Message: "Merge\n\nUpgrading: no\n"})
	repo.Named["HEAD"] = "merge"
	f := over(t, repo, rules)
	code, stdout, _, err := run(t, f, "", false, "footers", "Upgrading", "", "HEAD")
	if code != 0 || err != nil || stdout != "0123456 run it again\n" {
		t.Errorf("footers: exit %d, %v, %q", code, err, stdout)
	}
	code, stdout, _, err = run(t, f, "", true, "footers", "Upgrading", "root", "HEAD")
	var got struct {
		Footer  string `json:"footer"`
		Range   span   `json:"range"`
		Footers []struct {
			SHA     string `json:"sha"`
			Subject string `json:"subject"`
			Text    string `json:"text"`
		} `json:"footers"`
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Footer != "Upgrading" ||
		got.Range != (span{"root", "HEAD"}) || len(got.Footers) != 1 || got.Footers[0].SHA != "0123456789abcdef" ||
		got.Footers[0].Subject != "feat: a" {
		t.Errorf("footers --json: exit %d, %v, %s", code, err, stdout)
	}
	if _, _, _, err := run(t, f, "", false, "footers", "Task", "", "HEAD"); !isUsage(err,
		"commit footers needs a footer of free text: commits.footers.Task is not one (source: text)") {
		t.Errorf("a footer of IDs: %v", err)
	}
	if _, _, _, err := run(t, f, "", false, "footers", "Upgrading", "gone", "HEAD"); !isUsage(err,
		"commit footers: gone is not a commit of this repository") {
		t.Errorf("a ref naming no commit: %v", err)
	}
	unreadable := over(t, repository(false), rules)
	if _, _, _, err := run(t, unreadable, "", false, "footers", "Upgrading", "gone", "HEAD"); err == nil {
		t.Error("a range that cannot be read: no error")
	}
	f.loadErr = errors.New("no config")
	if _, _, _, err := run(t, f, "", false, "footers", "Upgrading", "", "HEAD"); err != f.loadErr {
		t.Errorf("a config that does not load: %v", err)
	}
}

// A commit runs git with the footers as trailers, git's arguments after
// them, from where the person stood, ITOS_AMEND saying whether it amends;
// its exit is git's, and under --json HEAD once it made one, git's stdout
// on stderr.
func TestCommitRunsGit(t *testing.T) {
	f := over(t, repository(true), rules)
	f.moves = true
	var stdout, stderr strings.Builder
	c, err := Parse([]string{"--task", "T-1", "--upgrading", "none", "-m", "feat: x", "--amend"}, true, true, "sub", nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(f))
	var got struct {
		OK     bool    `json:"ok"`
		Commit *string `json:"commit"`
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout.String()), &got) != nil || !got.OK || got.Commit == nil ||
		*got.Commit != "made" {
		t.Fatalf("exit %d, %v, %q", code, err, stdout.String())
	}
	want := []string{"--trailer", "Task: T-1", "--trailer", "Upgrading: none", "-m", "feat: x", "--amend"}
	if len(f.made) != 1 || !slices.Equal(f.made[0].Args, want) || !f.made[0].Quiet {
		t.Errorf("git commit %+v", f.made)
	}
	talk := f.talks[0]
	if talk.Dir != "sub" || talk.Stdout != &stderr || talk.Stderr != &stderr || !talk.Owns ||
		!slices.Contains(talk.Env, message.AmendEnv+"=1") {
		t.Errorf("git talks %+v", talk)
	}
	f.failure = &vcsport.Failed{Command: "git commit", Code: 1}
	code, stdout2, _, err := run(t, f, "", true, "-m", "docs: x")
	got.Commit = nil
	if code != 1 || err != nil || json.Unmarshal([]byte(stdout2), &got) != nil || got.OK || got.Commit != nil ||
		!slices.Contains(f.talks[1].Env, message.AmendEnv+"=0") {
		t.Errorf("git refusing: exit %d, %v, %q", code, err, stdout2)
	}
	f.failure = errors.New("no git")
	code, _, stderr2, err := run(t, f, "", false, "-m", "docs: x")
	if code != missing || err != nil || stderr2 != "itos: cannot run git: no git\n" {
		t.Errorf("no git: exit %d, %v, %q", code, err, stderr2)
	}
}

// With no config git commit still runs, its message wrapped around git's
// comment character (bug 120) by no limit; a link flag is the error the
// config's load gave.
func TestCommitWithoutAConfig(t *testing.T) {
	f := over(t, repository(false), "")
	code, stdout, _, err := run(t, f, "", false, "-m", "docs: "+strings.Repeat("word ", 40))
	if code != 0 || err != nil || stdout != "" || len(f.made) != 1 || f.made[0].Args[1] != "docs: "+strings.Repeat("word ", 40) {
		t.Errorf("exit %d, %v, %+v", code, err, f.made)
	}
	if _, _, _, err := run(t, f, "", false, "--task", "T-1", "-m", "docs: x"); err != f.absent {
		t.Errorf("--task with no config: %v", err)
	}
	f.commitErr = errors.New("broken config")
	if _, _, _, err := run(t, f, "", false, "-m", "docs: x"); err != f.commitErr {
		t.Errorf("a config that does not load: %v", err)
	}
}

// What would commit past the gates, a flag with no value and a link flag
// the config has no footer for are refused before git runs.
func TestCommitRefusesItsArguments(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "")
	t.Setenv("GIT_CONFIG_PARAMETERS", "")
	f := over(t, repository(false), rules)
	for args, want := range map[string]string{
		"-am x":               "commit refuses -a:",
		"--task":              "commit --task needs <id>",
		"--item a -m docs:x":  "commit --item needs a footer whose source is the work registry",
		"--breaking":          "commit --breaking needs <text>",
		"--upgrading -- -m x": "",
	} {
		_, _, _, err := run(t, f, "", false, strings.Fields(args)...)
		if want == "" {
			if err != nil {
				t.Errorf("%s: %v", args, err)
			}
			continue
		}
		if !isUsage(err, want) {
			t.Errorf("%s: %v", args, err)
		}
	}
	if len(f.made) != 1 {
		t.Errorf("git ran %d times", len(f.made))
	}
}

// A commit whose type needs a footer neither the flags nor the message
// give is refused, naming each flag, as the hook reports it or under
// --json; a footer no flag writes is left to the hook. Its type is read
// from -m, -F's file where the person stood, or HEAD's message for an
// amend with --no-edit; the editor's is the hook's.
func TestCommitRefusesAMissingFooter(t *testing.T) {
	f := over(t, repository(false), rules)
	code, _, stderr, err := run(t, f, "", false, "-m", "feat: x")
	if code != refused || err != nil || !strings.Contains(stderr, "  - feat commits need --task <id>\n") ||
		!strings.Contains(stderr, "  - feat commits need --upgrading <text|none>\n") || strings.Contains(stderr, "Breaking") {
		t.Errorf("exit %d, %v\n%s", code, err, stderr)
	}
	code, stdout, _, err := run(t, f, "", true, "--task", "T-1", "-m", "feat: x")
	if code != refused || err != nil || !strings.Contains(stdout, `"ok": false`) || !strings.Contains(stdout, "upgrading-footer") ||
		strings.Contains(stdout, "task-footer") {
		t.Errorf("--json: exit %d, %v, %s", code, err, stdout)
	}
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("sub", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("sub", "m.txt"), []byte("feat: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _, err := run(t, f, "sub", false, "--task", "T-1", "-F", "m.txt"); code != refused || err != nil {
		t.Errorf("-F: exit %d, %v", code, err)
	}
	f.VCS.History[0].Message = "feat: start\n"
	if code, _, _, err := run(t, f, "", false, "--amend", "--no-edit"); code != refused || err != nil {
		t.Errorf("--amend --no-edit: exit %d, %v", code, err)
	}
	if code, _, _, err := run(t, f, "", false, "--task", "T-1"); code != 0 || err != nil || len(f.made) != 1 {
		t.Errorf("the editor's: exit %d, %v", code, err)
	}
}

// Where itos is set up, a git that would run none of its hooks refuses the
// commit before git runs, exit 3.
func TestCommitNeedsTheHook(t *testing.T) {
	f := over(t, repository(true), rules)
	f.oldGit = true
	if _, _, _, err := run(t, f, "", false, "-m", "docs: x"); err == nil || !strings.Contains(err.Error(), "itos hook install") ||
		len(f.made) != 0 {
		t.Errorf("an old git: %v", err)
	}
	if f.files == nil {
		f.files = map[string]string{}
	}
	f.files[".git/hooks/commit-msg"] = "#!/bin/sh\nitos hook commit-msg \"$@\"\n"
	if code, _, _, err := run(t, f, "", false, "-m", "docs: x"); code != 0 || err != nil || len(f.made) != 1 {
		t.Errorf("an old git with itos's hook file: exit %d, %v", code, err)
	}
}

// Under a stealth config the links are handed to the hook and written as
// the new commit's note, never trailers, notes.rewriteRef set first; an
// amend giving none is judged by HEAD's note. A rewriteRef git refuses
// stops before git runs, exit 3; a note that cannot be written is exit 1.
func TestCommitUnderAStealthConfig(t *testing.T) {
	t.Setenv(message.FootersEnv, "Task: T-9")
	f := stealthOver(t, repository(false), rules)
	f.moves = true
	code, _, stderr, err := run(t, f, "", false, "--task", "T-1", "--upgrading", "none", "-m", "feat: x")
	if code != 0 || err != nil {
		t.Fatalf("exit %d, %v\n%s", code, err, stderr)
	}
	if args := f.made[0].Args; !slices.Equal(args, []string{"--trailer", "Upgrading: none", "-m", "feat: x"}) {
		t.Errorf("git commit %q", args)
	}
	env := f.talks[0].Env
	if !slices.Contains(env, message.FootersEnv+"=Task: T-1") || slices.Contains(env, message.FootersEnv+"=Task: T-9") {
		t.Errorf("the hook is handed %q", env)
	}
	if note := f.VCS.History[1].Note; note != "Task: T-1\n" {
		t.Errorf("the note is %q", note)
	}
	if got := f.git["notes.rewriteRef"]; !slices.Equal(got, []string{message.NotesRef}) {
		t.Errorf("notes.rewriteRef is %q", got)
	}
	f.VCS.History[1].Message = "feat: x\n\nUpgrading: none\n"
	f.VCS.History[1].Note = ""
	if code, _, _, err := run(t, f, "", false, "--amend", "--no-edit"); code != refused || err != nil {
		t.Errorf("an amend of a commit with no note: exit %d, %v", code, err)
	}
	f.VCS.History[1].Note = "Task: T-1\n"
	f.moves = false
	if code, _, _, err := run(t, f, "", false, "--amend", "--no-edit"); code != 0 || err != nil {
		t.Errorf("an amend keeping its note: exit %d, %v", code, err)
	}

	unnoted := stealthOver(t, &vcsfake.VCS{Named: map[string]string{}}, rules)
	code, _, stderr, err = run(t, unnoted, "", false, "--task", "T-1", "-m", "docs: x")
	if code != refused || err != nil || !strings.HasPrefix(stderr, "itos: the commit is made, but its note is not: ") {
		t.Errorf("no note: exit %d, %v, %q", code, err, stderr)
	}
	unnoted.addErr = errors.New("locked")
	unnoted.git = map[string][]string{}
	code, _, stderr, err = run(t, unnoted, "", false, "-m", "docs: x")
	if code != missing || err != nil || stderr != "itos: cannot set notes.rewriteRef: locked\n" || len(unnoted.made) != 1 {
		t.Errorf("no rewriteRef: exit %d, %v, %q", code, err, stderr)
	}
}

// A body line longer than the built-in lint's limit is wrapped, each -m in
// its place, and -F's text in a file of its own, removed once git ran; a
// file for it that cannot be made is the error.
func TestCommitWrapsTheBody(t *testing.T) {
	long := "docs: x\n\n" + strings.Repeat("word ", 30)
	f := over(t, repository(false), rules)
	if code, _, _, err := run(t, f, "", false, "-m", long); code != 0 || err != nil ||
		strings.Contains(f.made[0].Args[1], strings.Repeat("word ", 30)) {
		t.Errorf("-m: exit %d, %v, %+v", code, err, f.made)
	}
	file := filepath.Join(t.TempDir(), "m.txt")
	if err := os.WriteFile(file, []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _, err := run(t, f, "", false, "-F", file); code != 0 || err != nil || f.made[1].Args[1] == file {
		t.Fatalf("-F: exit %d, %v, %+v", code, err, f.made)
	}
	if _, err := os.Stat(f.made[1].Args[1]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the wrapped message is still there: %v", err)
	}
	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, notDir)
	}
	if _, _, _, err := run(t, f, "", false, "-F", file); err == nil || len(f.made) != 2 {
		t.Errorf("no file for the message: %v", err)
	}
}

// -F - wraps stdin's text into a file of its own, even when nothing needed
// wrapping; an -F file already wrapped, or that cannot be read, is given
// to git as it is, which reports the latter.
func TestCommitWrapsAFile(t *testing.T) {
	f := over(t, repository(false), rules)
	stdin(t, "docs: x\n")
	if code, _, _, err := run(t, f, "", false, "-F", "-"); code != 0 || err != nil || f.made[0].Args[1] == "-" {
		t.Errorf("-F -: exit %d, %v, %+v", code, err, f.made)
	}
	file := filepath.Join(t.TempDir(), "m.txt")
	if err := os.WriteFile(file, []byte("docs: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	for _, args := range [][]string{{"-F", file}, {"--file=" + folder}, {"-F"}} {
		before := len(f.made)
		if code, _, _, err := run(t, f, "", false, args...); code != 0 || err != nil || len(f.made) != before+1 ||
			!slices.Equal(f.made[before].Args, args) {
			t.Errorf("%q: exit %d, %v, %+v", args, code, err, f.made[before:])
		}
	}
}

// The flags take their values as git takes an option's, a "--" ending what
// itos reads; --breaking writes BREAKING-CHANGE; git's own options, -e among
// a cluster's, and a valued option with nothing after it are read as git
// reads them.
func TestCommitReadsItsFlags(t *testing.T) {
	f := over(t, repository(false), rules)
	args := []string{"--breaking", "it goes", "-em", "docs: x", "--", "--task", "a.md"}
	if code, _, _, err := run(t, f, "", false, args...); code != 0 || err != nil {
		t.Fatalf("exit %d, %v", code, err)
	}
	want := []string{"--trailer", "BREAKING-CHANGE: it goes", "-em", "docs: x", "--", "--task", "a.md"}
	if !slices.Equal(f.made[0].Args, want) {
		t.Errorf("git commit %q", f.made[0].Args)
	}
	if g := readGitArgs([]string{"--no-edit", "-e", "--message"}); g.noEdit || len(g.messages) != 1 || g.messagesAt[0].index != -1 {
		t.Errorf("readGitArgs = %+v", g)
	}
}

// broken is a writer that takes nothing.
type broken struct{}

func (broken) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// --json's object that cannot be written is the error.
func TestRunCannotWrite(t *testing.T) {
	f := over(t, repository(false), rules)
	f.moves = true
	for _, args := range [][]string{{"-m", "docs: x"}, {"check-paths", "--type", "docs"}} {
		c, err := Parse(args, true, false, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Run(t.Context(), broken{}, broken{}, NewHandler(f)); err == nil {
			t.Errorf("%q: no error", args)
		}
	}
}

// A commit's abbreviation is its first 7 characters, all of a shorter one.
func TestShort(t *testing.T) {
	if short("0123456789") != "0123456" || short("abc") != "abc" {
		t.Errorf("short = %q, %q", short("0123456789"), short("abc"))
	}
}

// A footer a flag would write is compared with the message's where itos
// reads it (-m, -F <file>, HEAD's for --amend --no-edit): the same values
// write nothing of it, the others' are still written, other values are
// refused before git runs, the differing footers first, as problems under
// --json; the editor's message is not read, and under a stealth config only
// the content is compared, the links never being in the message.
func TestCommitComparesItsFootersWithTheMessage(t *testing.T) {
	f := over(t, repository(false), rules)
	given := "feat: x\n\nTask: T-1\nUpgrading: none"
	if code, _, _, err := run(t, f, "", false, "--task", "T-1", "--upgrading", "none", "-m", given); code != 0 || err != nil ||
		!slices.Equal(f.made[0].Args, []string{"-m", given}) {
		t.Errorf("the same footers: exit %d, %v, %+v", code, err, f.made)
	}
	if code, _, _, err := run(t, f, "", false, "--task", "T-1", "--breaking", "y", "-m", given); code != 0 || err != nil ||
		!slices.Equal(f.made[1].Args, []string{"--trailer", "BREAKING-CHANGE: y", "-m", given}) {
		t.Errorf("one footer the message lacks: exit %d, %v, %+v", code, err, f.made)
	}

	if code, _, _, err := run(t, f, "", false, "--task", "T-1", "--upgrading", "none", "--breaking", "x", "-m", given+"\nBREAKING CHANGE: x"); code != 0 || err != nil ||
		!slices.Equal(f.made[2].Args, []string{"--trailer", "BREAKING-CHANGE: x", "-m", given + "\nBREAKING CHANGE: x"}) {
		t.Errorf("the spaced form is written: exit %d, %v, %+v", code, err, f.made)
	}

	code, _, stderr, err := run(t, f, "", false, "--task", "T-2", "--upgrading", "none", "--breaking", "x", "-m", given+"\nBREAKING CHANGE: y")
	if code != refused || err != nil || len(f.made) != 3 || !strings.HasPrefix(stderr, rejected(f)+"\n  - Task: the message gives T-1, --task gives T-2\n"+
		"  - BREAKING-CHANGE: the message gives \"y\", --breaking gives \"x\"\n") || strings.Count(stderr, "\n  - ") != 2 {
		t.Errorf("other values: exit %d, %v, %d commits\n%s", code, err, len(f.made), stderr)
	}
	code, stdout, _, err := run(t, f, "", true, "--task", "T-2", "--upgrading", "x", "-m", given)
	if code != refused || err != nil || !strings.Contains(stdout, `"rule": "task-footer-differs"`) ||
		!strings.Contains(stdout, `"fix": "give Upgrading: once, in the message or with --upgrading"`) {
		t.Errorf("--json: exit %d, %v\n%s", code, err, stdout)
	}

	code, _, stderr, err = run(t, f, "", false, "--task", "T-2", "-m", "feat: x\n\nTask: T-1")
	if code != refused || err != nil || !strings.Contains(stderr, "  - Task: the message gives T-1, --task gives T-2\n  - feat commits need --upgrading <text|none>\n") {
		t.Errorf("differing and missing: exit %d, %v\n%s", code, err, stderr)
	}

	if code, _, _, err := run(t, f, "", false, "--task", "T-2", "--upgrading", "x"); code != 0 || err != nil ||
		!slices.Equal(f.made[3].Args, []string{"--trailer", "Task: T-2", "--trailer", "Upgrading: x"}) {
		t.Errorf("the editor's message: exit %d, %v, %+v", code, err, f.made)
	}

	f.VCS.History[0].Message = "docs: start\n\nUpgrading: none\n"
	if code, _, _, err := run(t, f, "", false, "--amend", "--no-edit", "--upgrading", "none"); code != 0 || err != nil ||
		!slices.Equal(f.made[4].Args, []string{"--amend", "--no-edit"}) {
		t.Errorf("an amend keeping a footer: exit %d, %v, %+v", code, err, f.made)
	}
	if code, _, _, err := run(t, f, "", false, "--amend", "--no-edit", "--upgrading", "x"); code != refused || err != nil || len(f.made) != 5 {
		t.Errorf("an amend changing a footer: exit %d, %v, %d commits", code, err, len(f.made))
	}

	words := over(t, repository(false), rules)
	if code, _, _, err := run(t, words, "", false, "--upgrading", "run x", "-m", "docs: x\n\nUpgrading: run x"); code != 0 || err != nil ||
		!slices.Equal(words.made[0].Args, []string{"-m", "docs: x\n\nUpgrading: run x"}) {
		t.Errorf("a text of several words: exit %d, %v, %+v", code, err, words.made)
	}

	s := stealthOver(t, repository(false), rules)
	if code, _, _, err := run(t, s, "", false, "--task", "T-2", "--upgrading", "none", "-m", "feat: x\n\nTask: T-1\nUpgrading: none"); code != 0 || err != nil ||
		!slices.Equal(s.made[0].Args, []string{"-m", "feat: x\n\nTask: T-1\nUpgrading: none"}) ||
		!slices.Contains(s.talks[0].Env, message.FootersEnv+"=Task: T-2") {
		t.Errorf("a stealth link: exit %d, %v, %+v", code, err, s.made)
	}
	if code, _, _, err := run(t, s, "", false, "--task", "T-1", "--upgrading", "x", "-m", "feat: x\n\nUpgrading: none"); code != refused || err != nil || len(s.made) != 1 {
		t.Errorf("a stealth content: exit %d, %v, %d commits", code, err, len(s.made))
	}
}

// rejected is the config's reject message, the first line of a refusal.
func rejected(f *fake) string { return f.cfg.Commits.RejectMessage }

// An amend with --no-edit whose message comes from stdin (-F -) is not read
// as HEAD's: its footers are neither compared with HEAD's nor dropped.
func TestCommitDoesNotReadHeadForAStdinMessage(t *testing.T) {
	f := over(t, repository(false), rules)
	f.VCS.History[0].Message = "docs: start\n\nUpgrading: none\n"
	stdin(t, "docs: again\n\nUpgrading: x\n")
	if code, _, _, err := run(t, f, "", false, "--amend", "--no-edit", "-F", "-", "--upgrading", "x"); code != 0 || err != nil ||
		len(f.made) != 1 || !slices.Contains(f.made[0].Args, "Upgrading: x") {
		t.Errorf("exit %d, %v, %+v", code, err, f.made)
	}
}
