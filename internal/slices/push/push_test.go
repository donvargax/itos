package push

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// The repository's commits: base, the start; up, origin's main, which adds
// up.md; mine, the local main, which adds mine.md.
const (
	base = "base0"
	up   = "up000"
	mine = "mine0"
)

// fake is the ports over the VCS fake: whether a config file is there, the
// config its load gives, or its error; the stealth check's answer; the git
// config, an addition to it refused with addErr, and an old git that runs no
// hook its config declares; ci.watch's provider, ok and its error. fetchErr,
// refsErr and pushErr fail the fetch, the remote's listing and the push,
// rebase stands in for the fake's rebase when set; each push's refspecs and
// how git talks are recorded.
type fake struct {
	*vcsfake.VCS
	found     bool
	cfg       *config.Loaded
	loadErr   error
	stealth   bool
	git       map[string][]string
	addErr    error
	oldGit    bool
	hookFiles map[string]string
	watcher   providers.Watcher
	watchOK   bool
	watchErr  error
	fetchErr  error
	refsErr   error
	pushErr   error
	rebase    func() error
	pushed    [][]string
	talk      vcsport.Talk

	// push.mode pr (pr_test.go): the branch the remote's HEAD names and the
	// error asking it, gh's answer to the arguments of each call and the
	// calls, the GitHub repository the calls name, and the error listing a
	// range's commits.
	remoteHead string
	headErr    error
	gh         func(args []string) (string, error)
	ghCalls    [][]string
	repository string
	commitsErr error
	touchedErr error            // the error listing the paths a range touches
	showErr    error            // the error showing a commit
	fetchErrs  map[string]error // by refspec, a fetch that fails whatever fetchErr says
	atomic     []bool           // whether each push was atomic
	// The quiet fetches (no prompt) by refspec, and what runs when one is made:
	// a head the clone lacked arriving.
	quiet   []string
	onQuiet func()
}

func (f *fake) ConfigFound(context.Context) bool { return f.found }

func (f *fake) Stealth(context.Context) bool { return f.stealth }

func (f *fake) LoadConfig(context.Context) (*config.Loaded, error) { return f.cfg, f.loadErr }

func (f *fake) Watcher(context.Context, *config.Loaded, string) (providers.Watcher, bool, error) {
	return f.watcher, f.watchOK, f.watchErr
}

func (f *fake) RemoteHead(context.Context, string, vcsport.Talk) (string, error) {
	return f.remoteHead, f.headErr
}

func (f *fake) Gh(_ context.Context, args ...string) (string, error) {
	f.ghCalls = append(f.ghCalls, args)
	if f.gh == nil {
		return "", nil
	}
	return f.gh(args)
}

func (f *fake) GitHubRepository(context.Context, *config.Loaded, string) string { return f.repository }

func (f *fake) Commits(ctx context.Context, r vcsport.Range) ([]string, error) {
	if f.commitsErr != nil {
		return nil, f.commitsErr
	}
	return f.VCS.Commits(ctx, r)
}

func (f *fake) Touched(ctx context.Context, r vcsport.Range, merges bool) ([]string, error) {
	if f.touchedErr != nil {
		return nil, f.touchedErr
	}
	return f.VCS.Touched(ctx, r, merges)
}

func (f *fake) Show(ctx context.Context, rev string) (vcsport.Entry, error) {
	if f.showErr != nil {
		return vcsport.Entry{}, f.showErr
	}
	return f.VCS.Show(ctx, rev)
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

// The hooks folder is .git/hooks, holding hookFiles.
func (f *fake) HooksPath(context.Context) (string, error) { return filepath.Join(".git", "hooks"), nil }

func (f *fake) HookFile(_ context.Context, path string) (string, error) {
	text, ok := f.hookFiles[filepath.ToSlash(path)]
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

func (f *fake) Fetch(ctx context.Context, remote, refspec string, talk vcsport.Talk) (string, error) {
	if talk.NoPrompt {
		f.quiet = append(f.quiet, refspec)
		if f.onQuiet != nil {
			f.onQuiet()
		}
	}
	if f.fetchErr != nil {
		return "", f.fetchErr
	}
	if err := f.fetchErrs[refspec]; err != nil {
		return "", err
	}
	return f.VCS.Fetch(ctx, remote, refspec, talk)
}

func (f *fake) RemoteRefs(ctx context.Context, remote string, refs []string, talk vcsport.Talk) (map[string]string, error) {
	if f.refsErr != nil {
		return nil, f.refsErr
	}
	return f.VCS.RemoteRefs(ctx, remote, refs, talk)
}

func (f *fake) Rebase(ctx context.Context, onto string, quiet bool, talk vcsport.Talk) error {
	if f.rebase != nil {
		return f.rebase()
	}
	return f.VCS.Rebase(ctx, onto, quiet, talk)
}

func (f *fake) Push(ctx context.Context, remote string, specs []string, p vcsport.Pushing, talk vcsport.Talk) error {
	f.pushed, f.talk, f.atomic = append(f.pushed, specs), talk, append(f.atomic, p.Atomic)
	if f.pushErr != nil {
		return f.pushErr
	}
	return f.VCS.Push(ctx, remote, specs, p, talk)
}

// loaded is the config of the text.
func loaded(t *testing.T, text string) *config.Loaded { return loadedWith(t, text, nil) }

// loadedWith is the config of the text over a working tree holding the files.
func loadedWith(t *testing.T, text string, files map[string]string) *config.Loaded {
	t.Helper()
	tree := treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": files}}
	src, err := tree.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(&vcsfake.VCS{}, tree, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// repo is a clone on main at mine, whose upstream, origin's main, is at up:
// both grow from base. A config file is there, with no CI provider, and the
// git config declares itos's pre-push hook.
func repo(t *testing.T) *fake {
	t.Helper()
	start := map[string]string{"README.md": "start\n"}
	with := func(path string) map[string]string {
		tree := maps.Clone(start)
		tree[path] = path + "\n"
		return tree
	}
	v := &vcsfake.VCS{
		Root:    "/r",
		Located: map[string]vcsport.Location{"": {Top: "/r", GitDir: ".git", Common: ".git"}},
		History: []vcsfake.Commit{
			{Name: base, Message: "chore: start\n", Tree: start},
			{Name: up, Parents: []string{base}, Message: "docs: theirs\n", Tree: with("up.md")},
			{Name: mine, Parents: []string{base}, Message: "docs: mine\n", Tree: with("mine.md")},
		},
		Named:    map[string]string{"HEAD": mine, "refs/heads/main": mine},
		OnBranch: "main",
		Remote:   map[string]*vcsfake.Remote{"origin": {URL: "https://example.com/r.git", Refs: map[string]string{"refs/heads/main": up}}},
		Tracking: map[string][2]string{"main": {"origin", "refs/heads/main"}},
	}
	return &fake{VCS: v, found: true, cfg: loaded(t, "version: 1\n"), git: map[string][]string{
		"hook.itos-pre-push.command": {"itos hook pre-push"},
		"hook.itos-pre-push.event":   {"pre-push"},
	}}
}

// call is push's command line run over f, from the folder sub: its exit,
// stdout, stderr and error; a --json or -q first sets the global flag.
func call(t *testing.T, f *fake, args ...string) (int, string, string, error) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", "0")
	t.Setenv("GIT_CONFIG_PARAMETERS", "")
	asJSON, quiet := false, false
	for len(args) > 0 && (args[0] == "--json" || args[0] == "-q") {
		asJSON, quiet = asJSON || args[0] == "--json", quiet || args[0] == "-q"
		args = args[1:]
	}
	c, err := Parse(args, asJSON, quiet, "sub")
	if err != nil {
		return 0, "", "", err
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(f))
	return code, stdout.String(), stderr.String(), err
}

// report is what --json printed.
type report struct {
	OK      bool     `json:"ok"`
	Outcome string   `json:"outcome"`
	Remote  string   `json:"remote"`
	Branch  string   `json:"branch"`
	Tags    []string `json:"tags"`
	Refs    []string `json:"refs"`
	Commit  string   `json:"commit"`
	CI      string   `json:"ci"`
	PR      *struct {
		URL       string `json:"url"`
		Number    int    `json:"number"`
		Created   bool   `json:"created"`
		Updated   bool   `json:"updated"`
		Draft     bool   `json:"draft"`
		AutoMerge bool   `json:"auto_merge"`
	} `json:"pull_request"`
	Run *struct {
		URL string `json:"url"`
	} `json:"run"`
}

// reported is stdout read as --json's object.
func reported(t *testing.T, stdout string) report {
	t.Helper()
	var r report
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatalf("%s: %q", err, stdout)
	}
	return r
}

// origin is where origin's main is.
func (f *fake) origin() string { return f.Remote["origin"].Refs["refs/heads/main"] }

// head is the commit HEAD names.
func (f *fake) head(t *testing.T) string {
	t.Helper()
	sha, err := f.Resolve(t.Context(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

// --tag and --ref are read as often as given, as --tag=<tag> too, beside
// --no-wait, with the folder git runs in; a name that is missing, an
// option, a forcing refspec or a refspec is a usage error, as is any other
// argument, a forcing one and --no-verify saying why.
func TestParse(t *testing.T) {
	c, err := Parse([]string{"--tag", "v1", "--no-wait", "--tag=cli/v1", "--ref", "ci/windows"}, true, true, "sub")
	want := CLI{Query: Query{NoWait: true, Tags: []string{"v1", "cli/v1"}, Refs: []string{"ci/windows"}, Origin: "sub"}, JSON: true, Quiet: true}
	if err != nil || !reflect.DeepEqual(c, want) {
		t.Errorf("Parse = %+v, %v", c, err)
	}
	if c, err := Parse(nil, false, false, ""); err != nil || !reflect.DeepEqual(c, CLI{}) {
		t.Errorf("Parse() = %+v, %v", c, err)
	}
	for args, want := range map[string]string{
		"--tag":                 "push --tag needs a tag's name",
		"--tag=":                "push --tag needs a tag's name",
		"--ref --no-wait":       "push --ref needs a branch's name",
		"--ref +main":           "push never forces: +main is a forcing refspec; push --ref takes a branch's name",
		"--tag v1:v2":           "push --tag takes a tag's name, not a refspec (v1:v2): it pushes to the remote's of the same name",
		"--tag v1 origin":       "push takes no arguments but --no-wait, --ready, --tag <tag> and --ref <branch>: it pushes HEAD to the branch's upstream (origin)",
		"--ref=b --force":       "push never forces: --force would overwrite the remote's commits; itos push rebases onto them instead",
		"--tag v1 --tag -":      "push --tag needs a tag's name",
		"--no-verify":           "push refuses --no-verify: it skips the pre-push hook, and the checks it runs; push without it, and fix what the hook reports",
		"--no-veri":             "push refuses --no-veri: it skips the pre-push hook",
		"-f":                    "push never forces: -f would overwrite",
		"--force-with-lease":    "push never forces: --force-with-lease would overwrite",
		"-uf":                   "push never forces: -uf would overwrite",
		"+main":                 "push never forces: +main is a forcing refspec; itos push rebases onto the remote's commits instead",
		"--follow-tags":         "push takes no arguments",
		"-n":                    "push takes no arguments",
		"--no-wait --no-wait x": "push takes no arguments but --no-wait, --ready, --tag <tag> and --ref <branch>: it pushes HEAD to the branch's upstream (x)",
	} {
		_, err := Parse(strings.Fields(args), false, false, "")
		var u out.UsageError
		if !errors.As(err, &u) || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("Parse(%s) = %v, want %q", args, err, want)
		}
	}
}

// A core.hooksPath git -c sets is a usage error before anything is looked
// at, in the shim's GIT_CONFIG_KEY_<n> as in GIT_CONFIG_PARAMETERS.
func TestPushRefusesAHooksPathSetByGit(t *testing.T) {
	f := repo(t)
	f.Located = nil
	c, err := Parse(nil, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	var stderr strings.Builder
	code, err := c.Run(t.Context(), io.Discard, &stderr, NewHandler(f))
	var u out.UsageError
	if code != 0 || !errors.As(err, &u) || !strings.HasPrefix(err.Error(), "push refuses core.hooksPath set by git -c: it moves the hooks") ||
		stderr.String() != "" {
		t.Errorf("push under git -c core.hooksPath = %d, %v, %q", code, err, stderr.String())
	}
}

// Outside a repository the push says so, exit 3, and looks at nothing more.
func TestPushOutsideARepository(t *testing.T) {
	f := repo(t)
	f.Located = nil
	code, stdout, stderr, err := call(t, f, "--json")
	if code != missing || err != nil || stdout != "" || stderr != "itos: push runs in a git repository, and this is none\n" {
		t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
}

// Where a config is there, a push git would make without itos's pre-push
// hook is refused, exit 3, before anything is fetched: under --json the
// reason goes to stderr too. Where none is, the hook is not asked for.
func TestPushRefusesWhereTheHookWouldNotRun(t *testing.T) {
	f := repo(t)
	f.git = map[string][]string{}
	code, stdout, stderr, err := call(t, f)
	if code != missing || err != nil || stdout != "" ||
		!strings.HasPrefix(stderr, "itos: itos's pre-push hook is not declared in this clone's git config") || f.pushed != nil {
		t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	code, stdout, stderr, _ = call(t, f, "--json", "--tag", "v1")
	if r := reported(t, stdout); code != missing || r.OK || r.Outcome != "hooks-not-run" || r.Tags != nil ||
		!strings.HasPrefix(stderr, "itos: itos's pre-push hook is not declared") {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
	f = repo(t)
	f.oldGit = true
	if code, _, stderr, _ := call(t, f, "--no-wait"); code != missing || !strings.Contains(stderr, "itos hook install") {
		t.Errorf("an old git: exit %d\n%s", code, stderr)
	}
	f.found = false
	if code, stdout, stderr, _ := call(t, f, "--no-wait"); code != 0 || !strings.HasPrefix(stdout, "Pushed ") {
		t.Errorf("no config: exit %d\n%s%s", code, stdout, stderr)
	}
	f = repo(t)
	f.oldGit = true
	f.hookFiles = map[string]string{".git/hooks/pre-push": "#!/bin/sh\nitos hook pre-push \"$@\"\n"}
	if code, stdout, stderr, _ := call(t, f, "--no-wait"); code != 0 || !strings.HasPrefix(stdout, "Pushed ") {
		t.Errorf("an old git with itos's hook file: exit %d\n%s%s", code, stdout, stderr)
	}
}

// The routine fetches the upstream, rebases onto it and pushes the commit
// the rebase made to origin's main, from the folder the person stood in;
// with no provider to wait with it says so and exits 0. Under --json the
// remote, the branch and the commit pushed are given, git's output on
// stderr; under -q nothing is said.
func TestPushRebasesAndPushes(t *testing.T) {
	f := repo(t)
	code, stdout, stderr, err := call(t, f)
	head := f.head(t)
	if code != 0 || err != nil || stderr != "" || stdout != "Pushed "+head[:len(head)-1]+" to origin/main.\n" {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	if parents, _ := f.Parents(t.Context(), head); f.origin() != head || !reflect.DeepEqual(parents, []string{up}) {
		t.Errorf("origin's main is %s, HEAD %s on %v", f.origin(), head, parents)
	}
	if !reflect.DeepEqual(f.pushed, [][]string{{head + ":refs/heads/main"}}) || f.talk.Dir != "sub" || f.talk.Stdout == f.talk.Stderr {
		t.Errorf("pushed %v, talking %+v", f.pushed, f.talk)
	}
	f = repo(t)
	code, stdout, stderr, _ = call(t, f, "--json", "--no-wait")
	r := reported(t, stdout)
	if code != 0 || !r.OK || r.Outcome != "pushed" || r.Remote != "origin" || r.Branch != "main" || r.Commit != f.head(t) || stderr != "" {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
	if f.talk.Stdout != f.talk.Stderr {
		t.Errorf("git's output under --json is not on stderr: %+v", f.talk)
	}
	if code, stdout, stderr, _ := call(t, repo(t), "-q"); code != 0 || stdout != "" || stderr != "" {
		t.Errorf("-q: exit %d\n%s%s", code, stdout, stderr)
	}
}

// A rebase in progress, a conflict left and an uncommitted change to a
// tracked file are refused, exit 1, before anything is fetched; so is a
// detached HEAD.
func TestPushRefusesBeforeThePull(t *testing.T) {
	for name, c := range map[string]struct {
		set     func(*vcsfake.VCS)
		outcome string
		says    []string
	}{
		"a rebase": {func(v *vcsfake.VCS) { v.InRebase = true }, "rebase-in-progress", []string{
			"itos push: a rebase is in progress or a conflict is left; nothing was pulled or pushed.\n" +
				"Resolve it and run git rebase --continue, or run git rebase --abort, then itos push again.\n"}},
		"a conflict": {func(v *vcsfake.VCS) { v.Unmerged = []string{"README.md"} }, "rebase-in-progress", []string{"a conflict is left"}},
		"a change": {func(v *vcsfake.VCS) {
			v.Work = map[string]string{"README.md": "changed\n", "mine.md": "mine.md\n", "new.md": "new\n"}
		}, "uncommitted", []string{
			"itos push: tracked files have uncommitted changes; commit or stash them first, then run itos push again:\n   M README.md\n"}},
		"a detached HEAD": {func(v *vcsfake.VCS) { v.OnBranch = "" }, "detached", []string{
			"itos push: HEAD is not on a branch; check out the branch to push first\n"}},
	} {
		t.Run(name, func(t *testing.T) {
			f := repo(t)
			c.set(f.VCS)
			code, stdout, stderr, err := call(t, f)
			if code != policy || err != nil || stdout != "" || f.pushed != nil || f.Named["refs/remotes/origin/main"] != "" {
				t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
			}
			for _, text := range c.says {
				if !strings.Contains(stderr, text) {
					t.Errorf("no %q in %q", text, stderr)
				}
			}
			f = repo(t)
			c.set(f.VCS)
			_, stdout, _, _ = call(t, f, "--json")
			if r := reported(t, stdout); r.OK || r.Outcome != c.outcome {
				t.Errorf("--json: %s", stdout)
			}
		})
	}
}

// A branch with no commit is nothing to push; with no upstream and no
// remote origin there is nowhere to push, exit 3; and a branch at its
// upstream's commit is nothing to push.
func TestPushRoutineEdges(t *testing.T) {
	f := repo(t)
	f.History, f.Named = nil, map[string]string{}
	if code, stdout, stderr, _ := call(t, f); code != 0 || stdout != "Nothing to push: main has no commit yet.\n" || stderr != "" {
		t.Errorf("no commit: exit %d\n%s%s", code, stdout, stderr)
	}
	f = repo(t)
	f.Remote, f.Tracking = nil, nil
	if code, stdout, stderr, _ := call(t, f); code != missing || stdout != "" ||
		stderr != "itos push: main has no upstream and there is no remote origin to push to\n" {
		t.Errorf("no remote: exit %d\n%s%s", code, stdout, stderr)
	}
	f = repo(t)
	f.Remote["origin"].Refs["refs/heads/main"] = mine
	if code, stdout, stderr, _ := call(t, f); code != 0 || stdout != "Nothing to push: main is at origin/main.\n" || f.pushed != nil {
		t.Errorf("at origin: exit %d\n%s%s", code, stdout, stderr)
	}
	// An upstream the config sets is fetched whatever the remotes are.
	f = repo(t)
	f.Tracking = map[string][2]string{"main": {"upstream", "refs/heads/trunk"}}
	code, stdout, stderr, _ := call(t, f)
	if code != missing || stdout != "" || stderr != "fatal: 'upstream' does not appear to be a git repository\n"+
		"itos push: fetching upstream/trunk failed: upstream is no repository git can find (git's message above); nothing was pushed.\n"+
		"Check the remote's URL (git remote get-url upstream), then run itos push again.\n" {
		t.Errorf("an upstream with no remote: exit %d\n%s%s", code, stdout, stderr)
	}
	code, stdout, stderr, _ = call(t, f, "--json")
	if r := reported(t, stdout); code != missing || r.Outcome != "fetch-failed" || r.Remote != "upstream" || r.Branch != "trunk" ||
		stderr != "fatal: 'upstream' does not appear to be a git repository\n" {
		t.Errorf("--json, an upstream with no remote: exit %d\n%s%s", code, stdout, stderr)
	}
}

// A remote without the branch is pushed to with no rebase, the push making
// it; a fetch that fails otherwise prints git's words and exits by their
// kind: 75 for a remote out of reach, 70 for no kind, whether or not the
// remote can then be listed.
func TestPushFetch(t *testing.T) {
	f := repo(t)
	f.Remote["origin"].Refs = map[string]string{}
	f.cfg = loaded(t, "version: 1\nci: { watch: { provider: github } }\n")
	if code, stdout, stderr, _ := call(t, f); code != 0 || stdout != "Pushed mine to origin/main.\n" || f.origin() != mine || stderr != "" {
		t.Errorf("a new branch: exit %d\n%s%s", code, stdout, stderr)
	}
	f = repo(t)
	f.Remote["origin"].Down = true
	code, stdout, stderr, _ := call(t, f)
	if code != temporary || stdout != "" || stderr != "fatal: unable to access 'https://example.com/r.git': Could not resolve host\n"+
		"itos push: fetching origin/main failed: origin cannot be reached (git's message above); nothing was pushed.\n"+
		"Check the network, then run itos push again.\n" {
		t.Errorf("out of reach: exit %d\n%s%s", code, stdout, stderr)
	}
	for name, refsErr := range map[string]error{"listed": nil, "not listed": errors.New("no")} {
		f = repo(t)
		f.fetchErr, f.refsErr = &vcsport.Failed{Code: 128, Failure: vcsport.Other, Words: "fatal: odd\n"}, refsErr
		code, stdout, stderr, _ = call(t, f)
		if code != software || stdout != "" || stderr != "fatal: odd\nitos push: fetching origin/main failed (git's message above); nothing was pushed\n" {
			t.Errorf("%s: exit %d\n%s%s", name, code, stdout, stderr)
		}
	}
}

// Under a stealth config the rebase carries the notes, notes.rewriteRef set
// first; one that cannot be set stops the push before the rebase, exit 3.
func TestPushRebaseUnderAStealthConfig(t *testing.T) {
	f := repo(t)
	f.stealth = true
	if code, _, stderr, _ := call(t, f, "--no-wait"); code != 0 || !reflect.DeepEqual(f.git["notes.rewriteRef"], []string{"refs/notes/itos"}) {
		t.Errorf("exit %d, notes.rewriteRef %v\n%s", code, f.git["notes.rewriteRef"], stderr)
	}
	f = repo(t)
	f.stealth, f.addErr = true, errors.New("locked")
	code, stdout, stderr, _ := call(t, f, "--json")
	if code != missing || stdout != "" || stderr != "itos: cannot set notes.rewriteRef: locked\n" || f.head(t) != mine {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	f = repo(t)
	f.addErr = errors.New("locked")
	if code, _, _, _ := call(t, f, "--no-wait"); code != 0 {
		t.Errorf("no stealth config: exit %d", code)
	}
}

// A rebase that stops, on a conflict or with a rebase in progress, or that
// leaves a conflict, exits 1 saying how to go on; one that failed with no
// stop exits 1 too; a git that cannot start exits 3; and a HEAD the rebase
// left that names no commit exits 3.
func TestPushRebaseStops(t *testing.T) {
	stopped := "itos push: the rebase onto origin/main stopped; nothing was pushed.\n" +
		"Resolve the conflicts, git add the files and run git rebase --continue, then itos push again;\n" +
		"or run git rebase --abort to go back to where the branch was.\n"
	f := repo(t)
	f.History[1].Tree["mine.md"] = "theirs\n"
	code, stdout, stderr, _ := call(t, f)
	if code != policy || stdout != "" || stderr != stopped || !f.InRebase || f.pushed != nil {
		t.Errorf("a conflict: exit %d\n%s%s", code, stdout, stderr)
	}
	for name, c := range map[string]struct {
		rebase  func(*fake) error
		code    int
		outcome string
		says    string
	}{
		"a conflict left": {func(f *fake) error { f.Unmerged = []string{"README.md"}; return nil }, policy, "rebase-stopped", stopped},
		"in progress":     {func(f *fake) error { f.InRebase = true; return nil }, policy, "rebase-stopped", stopped},
		"failed": {func(*fake) error { return &vcsport.Failed{Code: 1} }, policy, "rebase-failed",
			"itos push: the rebase onto origin/main did not run (git's message above); nothing was pushed\n"},
		"no git": {func(*fake) error { return errors.New("no git") }, missing, "", "itos: cannot run git: no git\n"},
		"no HEAD": {func(f *fake) error { f.Named["HEAD"] = "gone"; return nil }, missing, "",
			"itos: cannot resolve HEAD after the rebase: HEAD names no commit\n"},
	} {
		t.Run(name, func(t *testing.T) {
			f := repo(t)
			f.rebase = func() error { return c.rebase(f) }
			code, stdout, stderr, _ := call(t, f)
			if code != c.code || stdout != "" || stderr != c.says || f.pushed != nil {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
			f = repo(t)
			f.rebase = func() error { return c.rebase(f) }
			code, stdout, _, _ = call(t, f, "--json")
			if c.outcome == "" && stdout != "" || c.outcome != "" && reported(t, stdout).Outcome != c.outcome {
				t.Errorf("--json: exit %d, %s", code, stdout)
			}
		})
	}
}

// A rebase that stops on a conflict in the work registry says what
// happened there, beside git: an item taken on both sides names the
// upstream's owner and git rebase --skip (slice 66); one both sides changed
// says so; items added on each side name none and say to keep both (bug
// 118). Nothing is said with no config file, one that cannot be read, or a
// conflict elsewhere.
func TestPushRebaseStopsOnTheRegistry(t *testing.T) {
	item := func(id, title, owner, status string) string {
		return "  - { id: " + id + ", title: " + title + ", phase: 1, owner: " + owner + ", status: " + status + ", depends_on: [] }\n"
	}
	head := "phases: { 1: null }\nitems:\n"
	start := head + item("slice-9", "nine", "null", "todo")
	const registry = "tasks/work-items.yaml"
	stopped := func(f *fake, theirs, mine string) func() error {
		return func() error {
			f.History = append(f.History,
				vcsfake.Commit{Name: "rb000", Tree: map[string]string{registry: start}},
				vcsfake.Commit{Name: "rt000", Tree: map[string]string{registry: theirs}},
				vcsfake.Commit{Name: "rm000", Tree: map[string]string{registry: mine}})
			f.Named["REBASE_HEAD^"], f.Named["HEAD"], f.Named["REBASE_HEAD"] = "rb000", "rt000", "rm000"
			f.InRebase, f.Unmerged = true, []string{"./" + registry}
			return &vcsport.Failed{Code: 1}
		}
	}
	for name, c := range map[string]struct {
		theirs, mine string
		says         string
	}{
		"taken": {head + item("slice-9", "nine", "ana", "doing"), head + item("slice-9", "nine", "bo", "doing"),
			"\nThe conflict is in the work registry, tasks/work-items.yaml: a commit on origin/main changed\n" +
				"the same item as yours, so it was taken, or changed, there first, and the remote keeps that.\n" +
				"  slice-9 is taken: a commit on origin/main made ana its owner first.\n" +
				"To leave it to them, run git rebase --skip, which drops your commit of the registry, then itos\n" +
				"push again and itos work for another item; or agree with them who keeps it, and resolve the\n" +
				"conflict as above.\n"},
		"taken by nobody": {head + item("slice-9", "nine", "null", "doing"), head + item("slice-9", "nine", "bo", "doing"),
			"  slice-9 was changed on origin/main too.\n"},
		"changed": {head + item("slice-9", "ana's", "null", "todo"), head + item("slice-9", "bo's", "null", "todo"),
			"  slice-9 was changed on origin/main too.\n"},
		"added on each side": {start + item("p1-ana", "a", "null", "todo"), start + item("p1-bo", "b", "null", "todo"),
			"\nThe conflict is in the work registry, tasks/work-items.yaml: a commit on origin/main and yours added or\n" +
				"changed different items, so nothing was taken twice. Keep both sides' items in the file, git add it\n" +
				"and run git rebase --continue, then itos push again; or run git rebase --abort to leave it.\n"},
	} {
		t.Run(name, func(t *testing.T) {
			f := repo(t)
			f.rebase = stopped(f, c.theirs, c.mine)
			code, _, stderr, _ := call(t, f)
			if code != policy || !strings.HasPrefix(stderr, "itos push: the rebase onto origin/main stopped") || !strings.Contains(stderr, c.says) {
				t.Errorf("exit %d\n%s", code, stderr)
			}
		})
	}
	taken := head + item("slice-9", "nine", "ana", "doing")
	for name, set := range map[string]func(*fake){
		"no config":           func(f *fake) { f.found = false },
		"a config unreadable": func(f *fake) { f.loadErr = errors.New("bad") },
		"another registry":    func(f *fake) { f.cfg = loaded(t, "version: 1\nwork: { registry: elsewhere.yaml }\n") },
	} {
		t.Run(name, func(t *testing.T) {
			f := repo(t)
			f.rebase = stopped(f, taken, head+item("slice-9", "nine", "bo", "doing"))
			set(f)
			if code, _, stderr, _ := call(t, f); code != policy || strings.Contains(stderr, "work registry") {
				t.Errorf("exit %d\n%s", code, stderr)
			}
		})
	}
}

// A push that fails exits by the kind git's words give it: 75 out of
// reach, 3 no repository, 1 refused, by the remote or the hook, 70 any
// other; a git that cannot start exits 3.
func TestPushFailures(t *testing.T) {
	refused := "itos push: the push to origin/main failed (git's message above); nothing was forced.\n" +
		"If the remote moved, run itos push again to rebase onto it; if a hook refused it, fix what it reported first.\n"
	for name, c := range map[string]struct {
		err  error
		code int
		says string
	}{
		"out of reach": {&vcsport.Failed{Code: 128, Failure: vcsport.Unreachable}, temporary,
			"itos push: the push to origin/main failed: origin cannot be reached (git's message above); nothing was pushed.\n" +
				"Check the network, then run itos push again.\n"},
		"no repository": {&vcsport.Failed{Code: 128, Failure: vcsport.NoRepository}, missing,
			"itos push: the push to origin/main failed: origin is no repository git can find (git's message above); nothing was pushed.\n"},
		"refused": {&vcsport.Failed{Code: 1, Failure: vcsport.Refused}, policy, refused},
		"other":   {&vcsport.Failed{Code: 128, Failure: vcsport.Other}, software, refused},
		"signal":  {&vcsport.Failed{Code: -1, Failure: vcsport.Refused}, policy, refused},
		"no git":  {errors.New("no git"), missing, "itos: cannot run git: no git\n"},
	} {
		t.Run(name, func(t *testing.T) {
			f := repo(t)
			f.pushErr = c.err
			code, stdout, stderr, _ := call(t, f)
			if code != c.code || stdout != "" || !strings.HasPrefix(stderr, c.says) {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
			f = repo(t)
			f.pushErr = c.err
			_, stdout, _, _ = call(t, f, "--json")
			if c.code != missing || name == "no repository" {
				if r := reported(t, stdout); r.OK || r.Outcome != "push-failed" || r.Commit != "" {
					t.Errorf("--json: %s", stdout)
				}
			}
		})
	}
}

// After the push, with ci.watch's provider, push waits for the commit's
// run and exits with its result; with no config file, one that cannot be
// read (saying so), or the provider none it does not; nor when the commits
// pushed touch only the work registry (slice 56), naming itos ci watch.
func TestPushWaits(t *testing.T) {
	github := "version: 1\nci: { watch: { provider: github } }\n"
	ran := func(conclusion string) providers.Watcher {
		return providers.Watcher{Look: func(string) (providers.Run, bool, error) {
			return providers.Run{URL: "https://ci/1", Status: "completed", Conclusion: conclusion}, true, nil
		}}
	}
	over := func(set func(*fake)) *fake {
		f := repo(t)
		f.cfg, f.watcher, f.watchOK = loaded(t, github), ran("success"), true
		set(f)
		return f
	}
	pushed := func(f *fake) string {
		head := f.head(t)
		return "Pushed " + head[:len(head)-1] + " to origin/main.\n"
	}
	f := over(func(*fake) {})
	code, stdout, stderr, _ := call(t, f)
	if code != 0 || stdout != pushed(f)+"CI run: https://ci/1\nCI passed: https://ci/1\n" || stderr != "" {
		t.Errorf("a run that passed: exit %d\n%q%q", code, stdout, stderr)
	}
	f = over(func(f *fake) { f.watcher = ran("failure") })
	if code, _, _, _ := call(t, f); code != policy {
		t.Errorf("a run that failed: exit %d", code)
	}
	f = over(func(*fake) {})
	code, stdout, stderr, _ = call(t, f, "--json")
	if r := reported(t, stdout); code != 0 || !r.OK || r.Outcome != "pushed" || r.CI != "success" || r.Run == nil || r.Run.URL != "https://ci/1" ||
		r.Commit != f.head(t) || !strings.Contains(stderr, "CI run: https://ci/1") {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
	f = over(func(*fake) {})
	if code, stdout, _, _ := call(t, f, "-q"); code != 0 || stdout != "" {
		t.Errorf("-q: exit %d, %q", code, stdout)
	}
	f = over(func(f *fake) { f.watchErr = errors.New("no token") })
	code, stdout, stderr, _ = call(t, f)
	if code != missing || stdout != pushed(f) || stderr != "itos: no token; the commits are pushed, and itos ci watch "+f.head(t)+" waits for their run\n" {
		t.Errorf("a provider that cannot look: exit %d\n%s%s", code, stdout, stderr)
	}
	f = over(func(f *fake) { f.watchErr = errors.New("no token") })
	code, stdout, _, _ = call(t, f, "--json")
	if r := reported(t, stdout); code != missing || r.OK || r.Outcome != "pushed" || r.CI != "error" {
		t.Errorf("--json, a provider that cannot look: exit %d\n%s", code, stdout)
	}
	for name, set := range map[string]func(*fake){
		"--no-wait":     func(*fake) {},
		"no config":     func(f *fake) { f.found = false },
		"provider none": func(f *fake) { f.watchOK = false },
	} {
		f := over(set)
		args := []string{}
		if name == "--no-wait" {
			args = append(args, name)
		}
		if code, stdout, stderr, _ := call(t, f, args...); code != 0 || stdout != pushed(f) || stderr != "" {
			t.Errorf("%s: exit %d\n%s%s", name, code, stdout, stderr)
		}
	}
	f = over(func(f *fake) { f.loadErr = errors.New("bad") })
	if code, stdout, stderr, _ := call(t, f); code != 0 || stdout != pushed(f) || stderr != "itos: no CI run watched, the config cannot be read: bad\n" {
		t.Errorf("a config unreadable: exit %d\n%s%s", code, stdout, stderr)
	}
	registryOnly := func(f *fake) {
		f.History[2].Tree = map[string]string{"README.md": "start\n", "tasks/work-items.yaml": "items: []\n"}
	}
	f = over(registryOnly)
	code, stdout, stderr, _ = call(t, f)
	if code != 0 || stderr != "" || stdout != pushed(f)+"Its commits touch only tasks/work-items.yaml, so its CI run is not waited for; itos ci watch "+
		f.head(t)+" waits for it.\n" {
		t.Errorf("registry only: exit %d\n%s%s", code, stdout, stderr)
	}
	// A push that makes the branch is waited for: what it adds is not known.
	f = over(func(f *fake) { registryOnly(f); f.Remote["origin"].Refs = map[string]string{} })
	if code, stdout, _, _ := call(t, f); code != 0 || !strings.Contains(stdout, "CI passed") {
		t.Errorf("a new branch of the registry alone: exit %d\n%s", code, stdout)
	}
	// With the provider none, the registry's commits are not looked at.
	f = over(func(f *fake) { registryOnly(f); f.cfg = loaded(t, "version: 1\n"); f.watchOK = false })
	if code, stdout, _, _ := call(t, f); code != 0 || stdout != pushed(f) {
		t.Errorf("registry only, provider none: exit %d\n%s", code, stdout)
	}
}

// --tag and --ref push what they name, alone and atomically, to the current
// branch's upstream's remote, else origin: a tag the remote has as the
// clone does is left out, one it has as another object refused, and so is a
// name the clone lacks; nothing is fetched or rebased.
func TestPushNamed(t *testing.T) {
	named := func() *fake {
		f := repo(t)
		f.Named["refs/tags/v1"], f.Named["refs/tags/v0"], f.Named["refs/heads/b"] = mine, base, mine
		return f
	}
	f := named()
	code, stdout, stderr, _ := call(t, f, "--tag", "v1", "--ref", "b")
	if code != 0 || stdout != "Pushed tag v1, branch b to origin.\n" || stderr != "" ||
		!reflect.DeepEqual(f.pushed, [][]string{{"refs/tags/v1:refs/tags/v1", "refs/heads/b:refs/heads/b"}}) ||
		f.Remote["origin"].Refs["refs/tags/v1"] != mine || f.origin() != up || f.head(t) != mine || f.talk.Dir != "sub" {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	code, stdout, _, _ = call(t, f, "--json", "--tag", "v1")
	if r := reported(t, stdout); code != 0 || r.Outcome != "nothing-to-push" || r.Remote != "origin" || r.Branch != "" ||
		!reflect.DeepEqual(r.Tags, []string{"v1"}) || r.Refs != nil {
		t.Errorf("--json of a tag origin has: exit %d\n%s", code, stdout)
	}
	if code, stdout, _, _ := call(t, f, "--tag", "v1"); code != 0 || stdout != "Nothing to push: origin has every tag named.\n" {
		t.Errorf("a tag origin has: exit %d\n%s", code, stdout)
	}
	code, stdout, _, _ = call(t, f, "--json", "--ref", "b")
	if r := reported(t, stdout); code != 0 || r.Outcome != "pushed" || !reflect.DeepEqual(r.Refs, []string{"b"}) || r.Tags != nil || r.Commit != "" {
		t.Errorf("--json --ref b: exit %d\n%s", code, stdout)
	}
	f = named()
	f.Remote["origin"].Refs["refs/tags/v0"] = up
	code, stdout, stderr, _ = call(t, f, "--tag", "v1", "--tag", "v0")
	if code != policy || stdout != "" || f.pushed != nil || stderr != "itos push: origin already has the tag v0, at up000, not this clone's base0; "+
		"a tag is never moved, so nothing was pushed.\nRelease under a new tag, or delete this clone's and fetch the remote's.\n" {
		t.Errorf("a tag origin has as another: exit %d\n%s%s", code, stdout, stderr)
	}
	for _, args := range [][]string{{"--tag", "v9"}, {"--ref", "nope"}} {
		f = named()
		code, _, stderr, _ := call(t, f, args...)
		what := map[string]string{"--tag": "tags/", "--ref": "heads/"}[args[0]] + args[1]
		if code != policy || f.pushed != nil || stderr != "itos push: this clone has no "+what+"; nothing was pushed\n" {
			t.Errorf("%v: exit %d\n%s", args, code, stderr)
		}
	}
	// The remote is the current branch's upstream's, origin on a detached HEAD.
	f = named()
	f.Tracking = map[string][2]string{"main": {"upstream", "refs/heads/main"}}
	f.Remote["upstream"] = &vcsfake.Remote{URL: "u", Refs: map[string]string{}}
	if code, stdout, _, _ := call(t, f, "--ref", "b"); code != 0 || stdout != "Pushed branch b to upstream.\n" {
		t.Errorf("an upstream: exit %d\n%s", code, stdout)
	}
	f.OnBranch = ""
	if code, stdout, _, _ := call(t, f, "--ref", "b"); code != 0 || stdout != "Pushed branch b to origin.\n" {
		t.Errorf("a detached HEAD: exit %d\n%s", code, stdout)
	}
}

// A named push whose remote's tags cannot be read, or whose push fails,
// exits by the failure's kind, as the routine's; a branch the remote has
// commits of that the clone's lacks is refused, exit 1, nothing landing.
func TestPushNamedFailures(t *testing.T) {
	f := repo(t)
	f.Named["refs/tags/v1"] = mine
	f.Remote["origin"].Down = true
	code, stdout, stderr, _ := call(t, f, "--json", "--tag", "v1")
	if r := reported(t, stdout); code != temporary || r.Outcome != "fetch-failed" ||
		!strings.HasSuffix(stderr, "Could not resolve host\n") || f.pushed != nil {
		t.Errorf("tags out of reach: exit %d\n%s%s", code, stdout, stderr)
	}
	if code, _, stderr, _ := call(t, f, "--tag", "v1"); code != temporary ||
		!strings.Contains(stderr, "itos push: reading origin's tags failed: origin cannot be reached") {
		t.Errorf("tags out of reach: exit %d\n%s", code, stderr)
	}
	code, _, stderr, _ = call(t, f, "--ref", "main")
	if code != temporary || !strings.HasPrefix(stderr, "itos push: the push of branch main failed: origin cannot be reached") {
		t.Errorf("a branch out of reach: exit %d\n%s", code, stderr)
	}
	f = repo(t)
	f.Named["refs/tags/v1"] = mine
	code, stdout, stderr, _ = call(t, f, "--tag", "v1", "--ref", "main")
	if code != policy || stdout != "" || f.Remote["origin"].Refs["refs/tags/v1"] != "" ||
		stderr != "itos push: the push of tag v1, branch main to origin failed (git's message above); nothing was forced, "+
			"and none of them landed.\nIf the remote has commits a branch lacks, rebase the branch onto them first; "+
			"if a hook refused the push, fix what it reported.\n" {
		t.Errorf("a branch behind: exit %d\n%s%s", code, stdout, stderr)
	}
	for err, want := range map[error]int{errors.New("no git"): missing, &vcsport.Failed{Code: 128, Failure: vcsport.Other}: software} {
		f = repo(t)
		f.pushErr = err
		code, _, stderr, _ := call(t, f, "--ref", "main")
		if code != want || stderr == "" {
			t.Errorf("%v: exit %d\n%s", err, code, stderr)
		}
	}
}

// A remote's failure is its kind, and its words what git said; an error
// that is no git's failure has none of either. Its exit is the kind's.
func TestRemoteKindWordsAndExit(t *testing.T) {
	for failure, want := range map[vcsport.Failure]kind.Kind{
		vcsport.Unreachable: kind.Temporary, vcsport.NoRepository: kind.Missing, vcsport.Refused: kind.Unknown, vcsport.Other: kind.Unknown,
	} {
		if got := remoteKind(&vcsport.Failed{Failure: failure}); got != want {
			t.Errorf("remoteKind(%v) = %v", failure, got)
		}
	}
	plain := errors.New("no git")
	if remoteKind(plain) != kind.Unknown || words(plain) != "" || words(&vcsport.Failed{Words: "said\n"}) != "said\n" {
		t.Errorf("remoteKind or words of an error git did not give")
	}
	for k, want := range map[kind.Kind]int{kind.Temporary: temporary, kind.Missing: missing, kind.Unknown: software} {
		if got := remoteExit(k); got != want {
			t.Errorf("remoteExit(%v) = %d", k, got)
		}
	}
	if refusalExit(&vcsport.Failed{Failure: vcsport.Refused}) != policy || refusalExit(&vcsport.Failed{Failure: vcsport.Other}) != software ||
		refusalExit(plain) != software {
		t.Errorf("refusalExit")
	}
}

// The registry alone, as a commit's paths name it, is registry-only; a
// path beside it, no path, or paths that cannot be read is not.
func TestTouchesOnlyRegistry(t *testing.T) {
	for _, c := range []struct {
		files []string
		err   error
		want  bool
	}{
		{[]string{"tasks/work-items.yaml", "./tasks/work-items.yaml"}, nil, true},
		{[]string{"tasks/work-items.yaml", "a.md"}, nil, false},
		{nil, nil, false},
		{[]string{"tasks/work-items.yaml"}, errors.New("unreadable"), false},
	} {
		if got := touchesOnlyRegistry("tasks//work-items.yaml", c.files, c.err); got != c.want {
			t.Errorf("touchesOnlyRegistry(%v, %v) = %v", c.files, c.err, got)
		}
	}
	if short("abcdefghij") != "abcdefg" || short("abc") != "abc" {
		t.Errorf("short")
	}
}

// onNewBranch is repo with the clone on a new branch, tidy, at mine, which
// has no upstream, and a remote whose HEAD names main.
func onNewBranch(t *testing.T) *fake {
	t.Helper()
	f := repo(t)
	f.Tracking, f.remoteHead = nil, "main"
	f.on("tidy", mine)
	return f
}

// The line a push from a branch with no upstream adds to its report (bug 136).
const noUpstreamLine = "tidy has no upstream, so itos push used origin/main; itos push --ref tidy pushes it to its own name.\n"

// A trunk push of a branch with no upstream is rebased onto the remote's
// default branch, the one its HEAD names, and pushed to it, never making a
// branch of the local name, and says so beside the Pushed line: not under
// -q, not under --json, whose branch is the default branch; a branch of the
// local name the remote has changes nothing. A remote with no branch, though
// tags and notes it may have, has no default branch to name, and the push
// makes the branch of the local name, saying nothing of an upstream. A remote
// whose HEAD names no branch it has is refused before anything is fetched,
// a remote that cannot be asked is not pushed to, and git's config is never
// written.
func TestPushWithNoUpstream(t *testing.T) {
	f := onNewBranch(t)
	code, stdout, stderr, _ := call(t, f)
	head := f.head(t)
	if code != 0 || stderr != "" || stdout != "Pushed "+head[:len(head)-1]+" to origin/main.\n"+noUpstreamLine {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if parents, _ := f.Parents(t.Context(), head); f.origin() != head || !reflect.DeepEqual(parents, []string{up}) ||
		!reflect.DeepEqual(f.pushed, [][]string{{head + ":refs/heads/main"}}) || len(f.Remote["origin"].Refs) != 1 || len(f.git) != 2 {
		t.Errorf("origin's main is %s, HEAD %s on %v, pushed %v, refs %v, git config %v", f.origin(), head, parents, f.pushed, f.Remote["origin"].Refs, f.git)
	}
	f = onNewBranch(t)
	if code, stdout, stderr, _ := call(t, f, "-q"); code != 0 || stdout != "" || stderr != "" || len(f.pushed) != 1 {
		t.Errorf("-q: exit %d\n%s%s", code, stdout, stderr)
	}
	f = onNewBranch(t)
	code, stdout, stderr, _ = call(t, f, "--json", "--no-wait")
	if r := reported(t, stdout); code != 0 || r.Outcome != "pushed" || r.Remote != "origin" || r.Branch != "main" || r.Commit != f.head(t) ||
		strings.Contains(stdout, "upstream") || stderr != "" {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}

	f = onNewBranch(t)
	f.remoteHead = "develop"
	f.Remote["origin"].Refs["refs/heads/develop"] = up
	code, stdout, _, _ = call(t, f, "--json")
	if r := reported(t, stdout); code != 0 || r.Branch != "develop" || f.Remote["origin"].Refs["refs/heads/develop"] != f.head(t) || f.origin() != up {
		t.Errorf("the HEAD names develop: exit %d\n%s", code, stdout)
	}
	f = onNewBranch(t)
	f.Remote["origin"].Refs["refs/heads/tidy"] = base
	if code, _, _, _ := call(t, f); code != 0 || f.origin() != f.head(t) || f.Remote["origin"].Refs["refs/heads/tidy"] != base {
		t.Errorf("a branch of its name on the remote: exit %d, origin's main %s", code, f.origin())
	}

	f = onNewBranch(t)
	f.Remote["origin"].Refs = map[string]string{"refs/tags/v1": up, "refs/notes/itos": up}
	f.remoteHead = ""
	code, stdout, stderr, _ = call(t, f)
	head = f.head(t)
	if code != 0 || stderr != "" || stdout != "Pushed "+head[:len(head)-1]+" to origin/tidy.\n" || f.Remote["origin"].Refs["refs/heads/tidy"] != head {
		t.Errorf("a remote with no branch: exit %d\n%s%s", code, stdout, stderr)
	}

	for name, c := range map[string]struct {
		set     func(*fake)
		code    int
		outcome string
	}{
		"a HEAD naming no branch the remote has": {func(f *fake) { f.remoteHead = "trunk" }, missing, "no-default-branch"},
		"no HEAD but branches":                   {func(f *fake) { f.remoteHead = "" }, missing, "no-default-branch"},
		"a HEAD that cannot be read": {func(f *fake) { f.headErr = &vcsport.Failed{Failure: vcsport.Unreachable} }, temporary,
			"fetch-failed"},
		"branches that cannot be listed": {func(f *fake) { f.remoteHead, f.refsErr = "", &vcsport.Failed{} }, software, "fetch-failed"},
	} {
		t.Run(name, func(t *testing.T) {
			f := onNewBranch(t)
			c.set(f)
			code, stdout, stderr, _ := call(t, f, "--json")
			if r := reported(t, stdout); code != c.code || r.Outcome != c.outcome || r.Branch != "tidy" || f.pushed != nil || f.origin() != up {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
			if c.outcome == "no-default-branch" {
				f = onNewBranch(t)
				c.set(f)
				code, stdout, stderr, _ = call(t, f)
				if code != missing || stdout != "" || !strings.Contains(stderr, "tidy has no upstream, and origin's HEAD names no branch origin has") ||
					!strings.Contains(stderr, "itos push --ref tidy") {
					t.Errorf("exit %d\n%s%s", code, stdout, stderr)
				}
			}
		})
	}
}

// The line saying the branch has no upstream goes where the push's other
// lines go: beside a stopped rebase on stderr, and, when a CI run is waited
// for, on stdout after the Pushed line and before the run's, not under -q.
func TestPushWithNoUpstreamSaysSoWhereverItReports(t *testing.T) {
	f := onNewBranch(t)
	f.History[1].Tree["mine.md"] = "theirs\n"
	code, stdout, stderr, _ := call(t, f)
	if code != policy || stdout != "" || !strings.HasSuffix(stderr, noUpstreamLine) || !strings.Contains(stderr, "the rebase onto origin/main stopped") {
		t.Errorf("a rebase that stops: exit %d\n%s%s", code, stdout, stderr)
	}
	watching := func() *fake {
		f := onNewBranch(t)
		f.cfg = loaded(t, "version: 1\nci: { watch: { provider: github } }\n")
		f.watchOK = true
		f.watcher = providers.Watcher{Look: func(string) (providers.Run, bool, error) {
			return providers.Run{URL: "https://ci/1", Status: "completed", Conclusion: "success"}, true, nil
		}}
		return f
	}
	f = watching()
	code, stdout, stderr, _ = call(t, f)
	head := f.head(t)
	if code != 0 || stderr != "" || stdout != "Pushed "+head[:len(head)-1]+" to origin/main.\n"+noUpstreamLine+"CI run: https://ci/1\nCI passed: https://ci/1\n" {
		t.Errorf("a run waited for: exit %d\n%q%q", code, stdout, stderr)
	}
	code, stdout, _, _ = call(t, watching(), "-q")
	if code != 0 || strings.Contains(stdout, "upstream") {
		t.Errorf("-q: exit %d\n%q", code, stdout)
	}
}
