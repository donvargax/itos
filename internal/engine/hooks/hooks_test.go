package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// fakeGit is a git in memory: a repository unless outside, whose config is
// local, the repository's own, after global, every other level's; old runs
// no hook its config declares; failing makes setting that key, or writing
// or moving to that file, fail, and reading it with a "?" after it. Its
// top is /repo and its hooks folder hooks under it, .git/hooks when that is
// "", or folder's error when set; files are the folder's, by their paths
// from the top (key).
type fakeGit struct {
	outside, old  bool
	local, global map[string][]string
	failing       string
	sets          []string
	hooks         string
	folder        error
	files         map[string]string
}

// key is a file's path as fakeGit's files hold it: from its top, with
// forward slashes.
func key(path string) string { return strings.TrimPrefix(filepath.ToSlash(path), "/repo/") }

func (g *fakeGit) Locate(context.Context, string) (vcsport.Location, error) {
	if g.outside {
		return vcsport.Location{}, errors.New("not a git repository")
	}
	return vcsport.Location{Top: "/repo"}, nil
}

func (g *fakeGit) Config(_ context.Context, key string, local bool) []string {
	if local {
		return g.local[key]
	}
	return append(slices.Clone(g.global[key]), g.local[key]...)
}

func (g *fakeGit) SetConfig(_ context.Context, key, value string) error {
	if key == g.failing {
		return errors.New("git config: could not lock config file")
	}
	if g.local == nil {
		g.local = map[string][]string{}
	}
	g.local[key] = []string{value}
	g.sets = append(g.sets, key)
	return nil
}

func (g *fakeGit) ConfigHooksRun(context.Context) bool { return !g.old }

func (g *fakeGit) Version(context.Context) string { return "2.30.0" }

func (g *fakeGit) HooksPath(context.Context) (string, error) {
	hooks := g.hooks
	if hooks == "" {
		hooks = ".git/hooks"
	}
	if strings.HasPrefix(hooks, "/") {
		return filepath.FromSlash(hooks), g.folder
	}
	return filepath.Join("/repo", filepath.FromSlash(hooks)), g.folder
}

func (g *fakeGit) HookFile(_ context.Context, path string) (string, error) {
	if key(path)+"?" == g.failing {
		return "", errors.New("permission denied")
	}
	text, ok := g.files[key(path)]
	if !ok {
		return "", fs.ErrNotExist
	}
	return text, nil
}

func (g *fakeGit) WriteHookFile(_ context.Context, path, text string) error {
	if key(path) == g.failing {
		return errors.New("read-only file system")
	}
	if g.files == nil {
		g.files = map[string]string{}
	}
	g.files[key(path)] = text
	g.sets = append(g.sets, key(path))
	return nil
}

func (g *fakeGit) MoveHookFile(_ context.Context, from, to string) error {
	if key(to) == g.failing {
		return errors.New("read-only file system")
	}
	g.files[key(to)] = g.files[key(from)]
	delete(g.files, key(from))
	g.sets = append(g.sets, "moved to "+key(to))
	return nil
}

// declared is a config with itos's entry for each event, running bin.
func declared(bin string, events ...string) map[string][]string {
	config := map[string][]string{}
	for _, event := range events {
		config["hook.itos-"+event+".command"] = []string{bin + " hook " + event}
		config["hook.itos-"+event+".event"] = []string{event}
	}
	return config
}

func TestEntryNamesItosHook(t *testing.T) {
	if got := Entry("pre-push"); got != "itos-pre-push" {
		t.Errorf("Entry = %q", got)
	}
	if !slices.Equal(Events, []string{"commit-msg", "pre-push"}) {
		t.Errorf("Events = %q", Events)
	}
}

func TestAndListsAsASentence(t *testing.T) {
	for names, want := range map[string]string{"": "", "a": "a", "a b": "a and b", "a b c": "a, b and c"} {
		if got := and(strings.Fields(names)); got != want {
			t.Errorf("and(%q) = %q, want %q", names, got, want)
		}
	}
}

// An entry is declared at any level when its last command calls itos and
// one of its events is the event.
func TestDeclared(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name          string
		local, global map[string][]string
		want          bool
	}{
		{"local", declared("itos", "commit-msg"), nil, true},
		{"global", nil, declared("/bin/itos", "commit-msg"), true},
		{"none", nil, nil, false},
		{"the last command does not call itos", map[string][]string{
			"hook.itos-commit-msg.command": {"itos hook commit-msg", "lefthook run"},
			"hook.itos-commit-msg.event":   {"commit-msg"},
		}, nil, false},
		{"the last command calls itos", map[string][]string{
			"hook.itos-commit-msg.command": {"lefthook run", "itos hook commit-msg"},
			"hook.itos-commit-msg.event":   {"commit-msg"},
		}, nil, true},
		{"another event", map[string][]string{
			"hook.itos-commit-msg.command": {"itos hook commit-msg"},
			"hook.itos-commit-msg.event":   {"pre-push"},
		}, nil, false},
		{"one of its events", map[string][]string{
			"hook.itos-commit-msg.command": {"itos hook commit-msg"},
			"hook.itos-commit-msg.event":   {"pre-push", "commit-msg"},
		}, nil, true},
		{"no event", map[string][]string{"hook.itos-commit-msg.command": {"itos hook commit-msg"}}, nil, false},
		{"a word containing itos", map[string][]string{
			"hook.itos-commit-msg.command": {"notitos hook commit-msg"},
			"hook.itos-commit-msg.event":   {"commit-msg"},
		}, nil, false},
	} {
		if got := Declared(ctx, &fakeGit{local: c.local, global: c.global}, "commit-msg"); got != c.want {
			t.Errorf("%s: Declared = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReady(t *testing.T) {
	ctx := context.Background()
	if err := Ready(ctx, &fakeGit{outside: true, old: true}, "commit-msg"); err != nil {
		t.Errorf("outside a repository: %v", err)
	}
	err := Ready(ctx, &fakeGit{old: true, local: declared("itos", Events...)}, "commit-msg")
	if kind.Of(err) != kind.Missing || err.Error() != "git 2.30.0 runs no hook its config declares, and no hook "+
		"file in .git/hooks calls itos's commit-msg hook, so git would run none of its checks: run itos hook "+
		"install once in the clone, then run this again" {
		t.Errorf("an old git: %v", err)
	}
	if err := Ready(ctx, &fakeGit{local: declared("itos", Events...)}, Events...); err != nil {
		t.Errorf("both declared: %v", err)
	}
	if err := Ready(ctx, &fakeGit{}); err != nil {
		t.Errorf("no event asked: %v", err)
	}
	err = Ready(ctx, &fakeGit{local: declared("itos", "pre-push")}, Events...)
	if kind.Of(err) != kind.Missing || err.Error() != "itos's commit-msg hook is not declared in this clone's git "+
		"config, so git would run none of its checks: run itos hook install once in the clone (its worktrees "+
		"share it), then run this again" {
		t.Errorf("commit-msg missing: %v", err)
	}
	err = Ready(ctx, &fakeGit{}, Events...)
	if kind.Of(err) != kind.Missing ||
		!strings.HasPrefix(err.Error(), "itos's commit-msg and pre-push hooks are not declared in") {
		t.Errorf("both missing: %v", err)
	}
}

// declare runs Declare over g, returning its exit, stdout, stderr and the
// lines it said.
func declare(t *testing.T, g *fakeGit, print, force, json bool) (int, string, string, []string) {
	t.Helper()
	var stdout, stderr strings.Builder
	var said []string
	code, err := Declare(context.Background(), g, "itos", print, force,
		Out{Stdout: &stdout, Stderr: &stderr, JSON: json}, func(line string) { said = append(said, line) })
	if err != nil {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String(), said
}

func TestDeclareWritesThenLeavesAlone(t *testing.T) {
	g := &fakeGit{}
	code, stdout, stderr, said := declare(t, g, false, false, false)
	if code != 0 || stdout != "" || stderr != "" ||
		!slices.Equal(said, []string{"wrote hook.itos-commit-msg", "wrote hook.itos-pre-push"}) {
		t.Fatalf("first: exit %d, said %q\n%s%s", code, said, stdout, stderr)
	}
	want := []string{"hook.itos-commit-msg.command", "hook.itos-commit-msg.event",
		"hook.itos-pre-push.command", "hook.itos-pre-push.event"}
	if !slices.Equal(g.sets, want) || !Declared(context.Background(), g, "pre-push") {
		t.Errorf("set %q, config %q", g.sets, g.local)
	}
	g.sets = nil
	code, _, _, said = declare(t, g, false, false, false)
	if code != 0 || len(g.sets) != 0 ||
		!slices.Equal(said, []string{"unchanged hook.itos-commit-msg", "unchanged hook.itos-pre-push"}) {
		t.Errorf("second: exit %d, said %q, set %q", code, said, g.sets)
	}
}

// An entry whose command is itos's but whose events are more than its one
// is written again.
func TestDeclareRewritesAnEntryWithMoreEvents(t *testing.T) {
	config := declared("itos", Events...)
	config["hook.itos-pre-push.event"] = []string{"pre-push", "commit-msg"}
	g := &fakeGit{local: config}
	_, _, _, said := declare(t, g, false, false, false)
	if !slices.Equal(said, []string{"unchanged hook.itos-commit-msg", "wrote hook.itos-pre-push"}) ||
		!slices.Equal(g.local["hook.itos-pre-push.event"], []string{"pre-push"}) {
		t.Errorf("said %q, config %q", said, g.local)
	}
}

// An entry that runs another bin's itos hook is itos's, and rewritten
// without --force; the last of several commands is the one compared.
func TestDeclareRewritesAnotherItos(t *testing.T) {
	config := declared("/old/itos", Events...)
	config["hook.itos-commit-msg.command"] = []string{"lefthook run", "/old/itos hook commit-msg"}
	g := &fakeGit{local: config}
	code, _, stderr, said := declare(t, g, false, false, false)
	if code != 0 || stderr != "" || !slices.Equal(said, []string{"wrote hook.itos-commit-msg", "wrote hook.itos-pre-push"}) {
		t.Errorf("exit %d, said %q\n%s", code, said, stderr)
	}
}

func TestDeclareRefusesAForeignEntryUnlessForced(t *testing.T) {
	foreign := func() *fakeGit {
		config := declared("itos", Events...)
		config["hook.itos-pre-push.command"] = []string{"itos hook pre-push", "lefthook run pre-push"}
		return &fakeGit{local: config}
	}
	g := foreign()
	code, stdout, stderr, said := declare(t, g, false, false, false)
	if code != exitPolicy || stdout != "" || stderr != "hook.itos-pre-push does not call itos; pass --force to replace it\n" ||
		!slices.Equal(said, []string{"unchanged hook.itos-commit-msg"}) || len(g.sets) != 0 {
		t.Errorf("unforced: exit %d, said %q, set %q\n%s%s", code, said, g.sets, stdout, stderr)
	}
	g = foreign()
	code, _, stderr, said = declare(t, g, false, true, false)
	if code != 0 || stderr != "" || !slices.Equal(said, []string{"unchanged hook.itos-commit-msg", "replaced hook.itos-pre-push"}) ||
		!Declared(context.Background(), g, "pre-push") {
		t.Errorf("forced: exit %d, said %q, config %q\n%s", code, said, g.local, stderr)
	}
}

func TestDeclarePrintsAndWritesNothing(t *testing.T) {
	g := &fakeGit{}
	code, stdout, stderr, said := declare(t, g, true, false, false)
	want := "[hook \"itos-commit-msg\"]\n\tevent = commit-msg\n\tcommand = itos hook commit-msg\n" +
		"[hook \"itos-pre-push\"]\n\tevent = pre-push\n\tcommand = itos hook pre-push\n"
	if code != 0 || stdout != want || stderr != "" || len(said) != 0 || len(g.sets) != 0 {
		t.Errorf("exit %d, said %q, set %q\n%s%s", code, said, g.sets, stdout, stderr)
	}
	code, stdout, _, said = declare(t, g, true, false, true)
	var got struct {
		Schema int
		Hooks  []configHook
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if code != 0 || len(said) != 0 || got.Schema != 1 || !slices.Equal(got.Hooks, []configHook{
		{Name: "itos-commit-msg", Event: "commit-msg", Command: "itos hook commit-msg", Action: "printed"},
		{Name: "itos-pre-push", Event: "pre-push", Command: "itos hook pre-push", Action: "printed"},
	}) {
		t.Errorf("--json: exit %d, said %q\n%s", code, said, stdout)
	}
}

func TestDeclareJSONSaysWhatItDid(t *testing.T) {
	_, stdout, _, said := declare(t, &fakeGit{}, false, false, true)
	if strings.Count(stdout, `"action": "wrote"`) != 2 || !strings.Contains(stdout, `"hooks": [`) ||
		!slices.Equal(said, []string{"wrote hook.itos-commit-msg", "wrote hook.itos-pre-push"}) {
		t.Errorf("said %q\n%s", said, stdout)
	}
}

func TestDeclareStopsAtAFailedWrite(t *testing.T) {
	for _, key := range []string{"hook.itos-commit-msg.command", "hook.itos-commit-msg.event"} {
		g := &fakeGit{failing: key}
		var stdout strings.Builder
		code, err := Declare(context.Background(), g, "itos", false, false, Out{Stdout: &stdout, Stderr: &stdout},
			func(line string) { t.Errorf("%s: said %q", key, line) })
		if code != 0 || err == nil || !strings.Contains(err.Error(), "could not lock") || stdout.Len() != 0 {
			t.Errorf("%s: exit %d, %v\n%s", key, code, err, stdout.String())
		}
	}
}

// A write that fails under --json emits nothing.
func TestDeclareJSONEmitsNothingOnAFailedWrite(t *testing.T) {
	var stdout strings.Builder
	_, err := Declare(context.Background(), &fakeGit{failing: "hook.itos-pre-push.event"}, "itos", false, false,
		Out{Stdout: &stdout, Stderr: &stdout, JSON: true}, func(string) {})
	if err == nil || stdout.Len() != 0 {
		t.Errorf("%v\n%s", err, stdout.String())
	}
}

// An emit that fails is Declare's error.
func TestDeclareJSONEmitFails(t *testing.T) {
	_, err := Declare(context.Background(), &fakeGit{}, "itos", true, false,
		Out{Stdout: failingWriter{}, Stderr: failingWriter{}, JSON: true}, func(string) {})
	if err == nil {
		t.Error("no error from a failed emit")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestPathSet(t *testing.T) {
	t.Setenv("GIT_CONFIG_PARAMETERS", "")
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_CONFIG_KEY_1", "Core.HooksPath")
	if !PathSet() {
		t.Error("core.hooksPath among the shim's keys")
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	if PathSet() {
		t.Error("core.hooksPath past GIT_CONFIG_COUNT")
	}
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	if !PathSet() {
		t.Error("core.hooksPath the first key")
	}
	t.Setenv("GIT_CONFIG_COUNT", "")
	for params, want := range map[string]bool{"'core.hooksPath'='/x'": true, "'core.hookspath=/x'": true,
		"'user.name'='core.hooksPath'": false, "'a'='x' 'CORE.HOOKSPATH'='y'": true,
		"'a'='x '\\''core.hooksPath='": false, "": false, "'core.hooksPathx'='y'": false} {
		t.Setenv("GIT_CONFIG_PARAMETERS", params)
		if got := PathSet(); got != want {
			t.Errorf("GIT_CONFIG_PARAMETERS %q: PathSet = %v", params, got)
		}
	}
}
