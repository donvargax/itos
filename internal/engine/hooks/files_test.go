package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
)

const (
	commitMsg = ".git/hooks/commit-msg"
	prePush   = ".git/hooks/pre-push"
)

// itos's hook file runs the kept hook first with the input saved, then
// itos's hook with git's arguments; its lines end in LF alone.
func TestHookFile(t *testing.T) {
	text := hookFile("tools/bin/itos", "pre-push")
	for _, want := range []string{"#!/bin/sh\n" + marker + " (a git older than 2.54.0 ", "pre-push.before-itos, and runs first",
		`kept="$0.before-itos"`, `"$kept" "$@" < "$input"`, `tools/bin/itos hook pre-push "$@" < "$input"`,
		"\nexec tools/bin/itos hook pre-push \"$@\"\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "\r") || strings.Contains(text, "{") || !callsHook(text, "pre-push") || callsHook(text, "commit-msg") {
		t.Errorf("the file:\n%s", text)
	}
}

func TestCallsHook(t *testing.T) {
	for text, want := range map[string]bool{"itos hook commit-msg \"$1\"": true, "/bin/itos hook commit-msg": true,
		"notitos hook commit-msg": false, "itos hook commit-msgs": false, "itos hook pre-push": false, "": false} {
		if got := callsHook(text, "commit-msg"); got != want {
			t.Errorf("callsHook(%q) = %v", text, got)
		}
	}
}

// On a git that runs no hook its config declares, Ready asks each event's
// hook file in the hooks folder to call itos's hook for it.
func TestReadyReadsTheHookFiles(t *testing.T) {
	ctx := context.Background()
	g := &fakeGit{old: true}
	err := Ready(ctx, g, Events...)
	if kind.Of(err) != kind.Missing ||
		!strings.Contains(err.Error(), "no hook file in .git/hooks calls itos's commit-msg and pre-push hooks, so") {
		t.Errorf("none: %v", err)
	}
	g.files = map[string]string{commitMsg: "lefthook run commit-msg\n", prePush: "itos hook commit-msg\n"}
	if err := Ready(ctx, g, "commit-msg"); kind.Of(err) != kind.Missing {
		t.Errorf("a file that does not call itos: %v", err)
	}
	if err := Ready(ctx, g, "pre-push"); kind.Of(err) != kind.Missing {
		t.Errorf("a file calling the other event's hook: %v", err)
	}
	g.files = map[string]string{commitMsg: "npx lint\nitos hook commit-msg \"$1\"\n", prePush: hookFile("x/itos", "pre-push")}
	if err := Ready(ctx, g, Events...); err != nil {
		t.Errorf("both: %v", err)
	}
	g.folder = errors.New("not a git repository")
	if err := Ready(ctx, g, "commit-msg"); kind.Of(err) != kind.Missing || err.Error() != "not a git repository" {
		t.Errorf("no folder: %v", err)
	}
}

// installFiles is declare on an old git over g.
func installFiles(t *testing.T, g *fakeGit, print, json bool) (int, string, string, []string) {
	t.Helper()
	g.old = true
	return declare(t, g, print, false, json)
}

func TestWriteFilesWritesThenLeavesAlone(t *testing.T) {
	g := &fakeGit{}
	code, stdout, stderr, said := installFiles(t, g, false, false)
	if code != 0 || stdout != "" || stderr != "" || len(g.local) != 0 ||
		!slices.Equal(said, []string{"written " + commitMsg, "written " + prePush}) ||
		g.files[commitMsg] != hookFile("itos", "commit-msg") || g.files[prePush] != hookFile("itos", "pre-push") {
		t.Fatalf("first: exit %d, said %q, files %q\n%s%s", code, said, g.files, stdout, stderr)
	}
	g.sets = nil
	code, _, _, said = installFiles(t, g, false, false)
	if code != 0 || len(g.sets) != 0 || !slices.Equal(said, []string{"unchanged " + commitMsg, "unchanged " + prePush}) {
		t.Errorf("second: exit %d, said %q, set %q", code, said, g.sets)
	}
}

// A hook the folder held that does not call itos is kept beside itos's
// file, which runs it first; itos's own file is rewritten, and one that
// calls itos already is left as it is.
func TestWriteFilesKeepsTheFoldersHook(t *testing.T) {
	g := &fakeGit{files: map[string]string{commitMsg: "#!/bin/sh\nlint \"$1\"\n", prePush: hookFile("/old/itos", "pre-push")}}
	code, _, stderr, said := installFiles(t, g, false, false)
	if code != 0 || stderr != "" || !slices.Equal(said, []string{"replaced " + commitMsg + ", keeping the hook it held as " +
		commitMsg + ".before-itos, which runs first", "wrote " + prePush}) ||
		g.files[commitMsg+".before-itos"] != "#!/bin/sh\nlint \"$1\"\n" || g.files[commitMsg] != hookFile("itos", "commit-msg") ||
		g.files[prePush] != hookFile("itos", "pre-push") ||
		!slices.Equal(g.sets, []string{"moved to " + commitMsg + ".before-itos", commitMsg, prePush}) {
		t.Errorf("exit %d, said %q, set %q, files %q\n%s", code, said, g.sets, g.files, stderr)
	}
	g = &fakeGit{files: map[string]string{commitMsg: "itos hook commit-msg \"$@\"\n", prePush: hookFile("itos", "pre-push")}}
	if code, _, _, said := installFiles(t, g, false, false); code != 0 || len(g.sets) != 0 ||
		!slices.Equal(said, []string{"unchanged " + commitMsg, "unchanged " + prePush}) {
		t.Errorf("a file calling itos: exit %d, said %q, set %q", code, said, g.sets)
	}
}

// A hooks folder in the work tree is the project's, whether git tracks its
// files or not: each hook file that does not call itos refuses the
// install, naming the line to add, and no file is written.
func TestWriteFilesRefusesAFolderInTheWorkTree(t *testing.T) {
	g := &fakeGit{hooks: ".husky", files: map[string]string{".husky/commit-msg": "#!/bin/sh\nlint\n"}}
	code, stdout, stderr, said := installFiles(t, g, false, true)
	var got struct{ Hooks []configHook }
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	refusal := func(name, event string) string {
		return "itos: hook install: " + name + " is in the work tree, the project's, so itos writes no file there: " +
			"add the line itos hook " + event + " \"$@\" to it, a sh script made if there is none, then run itos " +
			"hook install again\n"
	}
	if code != exitPolicy || len(g.sets) != 0 || len(said) != 0 || !slices.Equal(got.Hooks, []configHook{
		{Name: ".husky/commit-msg", Event: "commit-msg", Command: "itos hook commit-msg", Action: "refused"},
		{Name: ".husky/pre-push", Event: "pre-push", Command: "itos hook pre-push", Action: "refused"},
	}) || stderr != refusal(".husky/commit-msg", "commit-msg")+refusal(".husky/pre-push", "pre-push") {
		t.Errorf("exit %d, said %q, set %q\n%s%s", code, said, g.sets, stdout, stderr)
	}
	g = &fakeGit{hooks: "githooks", files: map[string]string{"githooks/commit-msg": "itos hook commit-msg \"$1\"\n",
		"githooks/pre-push": hookFile("itos", "pre-push")}}
	if code, _, _, said := installFiles(t, g, false, false); code != 0 || len(g.sets) != 0 ||
		!slices.Equal(said, []string{"unchanged githooks/commit-msg", "unchanged githooks/pre-push"}) {
		t.Errorf("files calling itos: exit %d, said %q", code, said)
	}
	g = &fakeGit{hooks: "/elsewhere/hooks"}
	if code, _, _, said := installFiles(t, g, false, false); code != 0 ||
		!slices.Equal(said, []string{"written /elsewhere/hooks/commit-msg", "written /elsewhere/hooks/pre-push"}) {
		t.Errorf("a folder outside the work tree: exit %d, said %q", code, said)
	}
}

// fromTop names a path from the top when it is under it, and only a path
// outside the git folder there is in the work tree.
func TestFromTop(t *testing.T) {
	for _, c := range []struct {
		top, path, name string
		inTree          bool
	}{
		{"/repo", "/repo/.husky/commit-msg", ".husky/commit-msg", true},
		{"/repo", "/repo", ".", true},
		{"/repo", "/repo/.git/hooks/commit-msg", ".git/hooks/commit-msg", false},
		{"/repo", "/repo/.gitx/commit-msg", ".gitx/commit-msg", true},
		{"/repo", "/other/hooks", "/other/hooks", false},
		{"/repo", "/repo/../hooks", "/repo/../hooks", false},
		{"", "/repo/.husky", "/repo/.husky", false},
		{"/repo", "hooks", "hooks", false},
	} {
		name, inTree := fromTop(filepath.FromSlash(c.top), filepath.FromSlash(c.path))
		if name != c.name || inTree != c.inTree {
			t.Errorf("fromTop(%q, %q) = %q, %v", c.top, c.path, name, inTree)
		}
	}
}

// A hook that does not call itos where one is kept already is refused:
// keeping it would lose the other.
func TestWriteFilesRefusesASecondKeptHook(t *testing.T) {
	g := &fakeGit{files: map[string]string{commitMsg: "lint\n", commitMsg + ".before-itos": "older\n"}}
	code, _, stderr, said := installFiles(t, g, false, false)
	if code != exitPolicy || len(g.sets) != 0 || len(said) != 0 || stderr != "itos: hook install: "+commitMsg+
		" does not call itos, and "+commitMsg+".before-itos holds the hook kept before it: move one of them away, "+
		"then run itos hook install again\n" {
		t.Errorf("exit %d, said %q, set %q\n%s", code, said, g.sets, stderr)
	}
}

func TestWriteFilesPrintsAndWritesNothing(t *testing.T) {
	g := &fakeGit{}
	code, stdout, stderr, said := installFiles(t, g, true, false)
	if code != 0 || stderr != "" || len(said) != 0 || len(g.sets) != 0 || stdout != "==> "+commitMsg+" <==\n"+
		hookFile("itos", "commit-msg")+"==> "+prePush+" <==\n"+hookFile("itos", "pre-push") {
		t.Errorf("exit %d, said %q, set %q\n%s%s", code, said, g.sets, stdout, stderr)
	}
	_, stdout, _, _ = installFiles(t, g, true, true)
	if strings.Count(stdout, `"action": "printed"`) != 2 || !strings.Contains(stdout, `"name": "`+prePush+`"`) {
		t.Errorf("--json:\n%s", stdout)
	}
}

// What the folder cannot give or take is Declare's error, and the JSON is
// not emitted.
func TestWriteFilesErrors(t *testing.T) {
	for name, g := range map[string]*fakeGit{
		"no folder":            {folder: errors.New("not a git repository")},
		"an unreadable file":   {failing: commitMsg + "?"},
		"an unreadable kept":   {files: map[string]string{commitMsg: "lint\n"}, failing: commitMsg + ".before-itos?"},
		"a file not written":   {failing: prePush},
		"a hook that not kept": {files: map[string]string{commitMsg: "lint\n"}, failing: commitMsg + ".before-itos"},
	} {
		g.old = true
		var stdout strings.Builder
		code, err := Declare(context.Background(), g, "itos", false, false, Out{Stdout: &stdout, Stderr: &stdout, JSON: true},
			func(string) {})
		if err == nil || stdout.Len() != 0 {
			t.Errorf("%s: exit %d, %v\n%s", name, code, err, stdout.String())
		} else if code != 0 {
			t.Errorf("%s: error path returned exit %d, want zero", name, code)
		}
	}
	if _, err := Declare(context.Background(), &fakeGit{old: true}, "itos", true, false,
		Out{Stdout: failingWriter{}, Stderr: failingWriter{}, JSON: true}, func(string) {}); err == nil {
		t.Error("no error from a failed emit")
	}
}
