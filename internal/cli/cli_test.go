package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestParseGlobals(t *testing.T) {
	g := ParseGlobals([]string{"version", "--json", "--config", "c.yaml", "-q", "--root", "sub", "--check"})
	if !g.JSON || !g.Quiet || g.Config != "c.yaml" || g.Root != "sub" {
		t.Errorf("flags: %+v", g)
	}
	if !slices.Equal(g.Rest, []string{"version", "--check"}) {
		t.Errorf("rest: %q", g.Rest)
	}
	// After "--" every argument is the command's.
	g = ParseGlobals([]string{"tests", "smoke", "run", "scenario", "--", "--json", "-q"})
	if g.JSON || g.Quiet || !slices.Equal(g.Rest, []string{"tests", "smoke", "run", "scenario", "--", "--json", "-q"}) {
		t.Errorf("after --: %+v", g)
	}
	// A valued flag at the end takes nothing.
	if g = ParseGlobals([]string{"version", "--config"}); g.Config != "" || !slices.Equal(g.Rest, []string{"version"}) {
		t.Errorf("trailing --config: %+v", g)
	}
}

func run(args ...string) (int, string, string) {
	var stdout, stderr strings.Builder
	code := Main(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// The usage errors the corpus leaves out read as the TypeScript's.
func TestUsageErrors(t *testing.T) {
	for args, want := range map[string]string{
		"tests moves":         "tests moves needs <kind>",
		"tests smoke":         "tests smoke check|ids|run needs <kind>",
		"tests smoke bogus x": "unknown command: tests smoke bogus",
		"tests":               "unknown command: tests",
		"hook commit-msg":     "hook commit-msg needs <file>",
		"hooks":               "unknown command: hooks",
		"ci range --head":     "ci range --head needs a value",
		"ci":                  "unknown command: ci",
	} {
		code, _, stderr := run(strings.Fields(args)...)
		if code != ExitUsage || stderr != "itos: "+want+" (itos --help)\n" {
			t.Errorf("itos %s: exit %d, stderr %q", args, code, stderr)
		}
	}
}

// The help is the longest command path it knows, whatever else the
// arguments hold; a bare itos prints itos's own and exits 2.
func TestHelp(t *testing.T) {
	ownPath(t, "git")
	for _, c := range []struct {
		args []string
		code int
		want string
	}{
		{nil, ExitUsage, mainHelp},
		{[]string{"--help"}, 0, mainHelp},
		{[]string{"bogus", "--help"}, 0, mainHelp},
		{[]string{"help", "ci", "plan"}, 0, helpTexts["ci plan"]},
		{[]string{"verify", "-h"}, 0, helpTexts["verify"]},
		{[]string{"tests", "smoke", "check", "scenario", "--help"}, 0, helpTexts["tests smoke"]},
		{[]string{"task", "T-007", "--help"}, 0, helpTexts["task"]},
		{[]string{"--json", "commit", "--help"}, 0, helpTexts["commit"]},
	} {
		code, stdout, stderr := run(c.args...)
		if code != c.code || stdout != c.want+"\n" || stderr != "" {
			t.Errorf("itos %s: exit %d, stdout %q, stderr %q", strings.Join(c.args, " "), code, stdout, stderr)
		}
	}
}

// ownPath gives the test a PATH of its own, so that no extension the
// caller has installed reaches the help it reads: one folder holding a link
// to each program named that the caller's PATH has, and nothing beside it.
// git is what finds the repository's top, where itos runs.
func ownPath(t *testing.T, programs ...string) {
	t.Helper()
	dir := t.TempDir()
	path := dir
	for _, name := range programs {
		program, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if err := os.Symlink(program, filepath.Join(dir, filepath.Base(program))); err != nil {
			// Where links cannot be made, the program's own folder.
			path += string(os.PathListSeparator) + filepath.Dir(program)
		}
	}
	t.Setenv("PATH", path)
}

// pathWith puts a folder holding an executable itos-<name> for each name
// first on the PATH, and gives the folder.
func pathWith(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, "itos-"+name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// An extension's arguments are its own: only the global flags before its
// name are read, while a built-in's are read wherever they stand.
func TestParseExtension(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the extensions here are shell scripts")
	}
	dir := pathWith(t, "hello", "work")
	g := Parse([]string{"--root", "sub", "--json", "hello", "--json", "--config", "c.yaml", "a"})
	if g.Extension != filepath.Join(dir, "itos-hello") || !g.JSON || g.Root != "sub" || g.Config != "" ||
		!slices.Equal(g.Rest, []string{"hello", "--json", "--config", "c.yaml", "a"}) {
		t.Errorf("extension: %+v", g)
	}
	g = Parse([]string{"work", "check", "--json"})
	if g.Extension != "" || !g.JSON || !slices.Equal(g.Rest, []string{"work", "check"}) {
		t.Errorf("a built-in wins: %+v", g)
	}
	for _, args := range [][]string{{"frob", "--json"}, {"--", "hello"}, {"--frob", "hello"}} {
		if g := Parse(args); g.Extension != "" {
			t.Errorf("itos %s: extension %s", strings.Join(args, " "), g.Extension)
		}
	}
	if path := extensionPath("../hello"); path != "" {
		t.Errorf("a path names no extension: %s", path)
	}
}

// The main help lists the extensions on the PATH, the first folder's of a
// name several have and none a built-in shadows; a command's help does not.
func TestHelpListsExtensions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the extensions here are shell scripts")
	}
	ownPath(t, "git")
	pathWith(t, "hello") // after the folder below on the PATH, so not listed
	first := pathWith(t, "zap", "hello", "work", "a-name-longer-than-the-command-column")
	if err := os.WriteFile(filepath.Join(first, "itos-plain"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := run("--help")
	want := mainCommands + "\n\nExtensions (itos-<command> on the PATH):" +
		"\n  a-name-longer-than-the-command-column\n" + strings.Repeat(" ", 35) +
		filepath.Join(first, "itos-a-name-longer-than-the-command-column") +
		"\n  hello                            " + filepath.Join(first, "itos-hello") +
		"\n  zap                              " + filepath.Join(first, "itos-zap") +
		"\n\n" + mainHelpTail + "\n"
	if code != 0 || stdout != want {
		t.Errorf("exit %d, stdout:\n%s\nnot:\n%s", code, stdout, want)
	}
	if _, stdout, _ := run("verify", "--help"); stdout != helpTexts["verify"]+"\n" {
		t.Errorf("verify --help: %q", stdout)
	}
}

// From a subfolder, itos runs at the repository's top, where its config is,
// and reads a path the person typed from the folder they stood in, as git
// does; --root keeps reading it from the root.
func TestSubfolderPaths(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\ncommits: { types: [docs], scopes: { docs: { only: [\"**/*.md\"] } } }\n")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(dir, "sub"))
	code, stdout, stderr := run("commit", "check-paths", "--type", "docs", "a.md", "../b.go", "--json")
	if code != ExitPolicy || !strings.Contains(stdout, `"sub/a.md"`) || !strings.Contains(stdout, `"b.go"`) {
		t.Errorf("from sub: exit %d, stdout %s, stderr %s", code, stdout, stderr)
	}
	if here, _ := os.Getwd(); !sameDir(here, dir) {
		t.Errorf("from sub, itos ran in %s, not the top %s", here, dir)
	}
	t.Chdir(filepath.Join(dir, "sub"))
	code, stdout, _ = run("--root", dir, "commit", "check-paths", "--type", "docs", "a.md", "--json")
	if code != 0 || !strings.Contains(stdout, `"a.md"`) || strings.Contains(stdout, "sub/") {
		t.Errorf("under --root: exit %d, stdout %s", code, stdout)
	}
}

func sameDir(a, b string) bool {
	x, errX := os.Stat(a)
	y, errY := os.Stat(b)
	return errX == nil && errY == nil && os.SameFile(x, y)
}
