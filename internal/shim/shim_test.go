package shim

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/launch"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// Main finds the real git by the OS the tests run on, as cmd/itos has it.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// The name git, and on windows git.exe in any case.
func TestNamed(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		windows := goos == "windows"
		for arg0, want := range map[string]bool{
			"git": true, "/usr/local/bin/git": true, "itos": false, "/x/git/itos": false, "git-shim": false,
			"git.exe": windows, "/x/Git.EXE": windows, "GIT": windows, "itos.exe": false,
		} {
			if got := Named(arg0, goos); got != want {
				t.Errorf("Named(%q, %s) = %v, want %v", arg0, goos, got, want)
			}
		}
	}
}

// git's options before the command: -C folders joined as git joins them (an
// empty one changing nothing), each -c kept, --no-pager and -P passed over;
// any other option, a missing value or no command is not the shim's to read.
func TestParse(t *testing.T) {
	l, ok := parse([]string{"-C", "a", "-C", "", "-c", "user.name=x", "--no-pager", "-C", "b", "-P", "commit", "-m", "-C"})
	if !ok || l.dir != filepath.Join("a", "b") || !slices.Equal(l.configs, []string{"user.name=x"}) ||
		l.name != "commit" || !slices.Equal(l.rest, []string{"-m", "-C"}) {
		t.Errorf("parse = %+v, %v", l, ok)
	}
	abs := map[bool]string{false: "/abs", true: `C:\abs`}[runtime.GOOS == "windows"]
	if l, ok := parse([]string{"-C", abs, "push"}); !ok || l.dir != abs || l.name != "push" {
		t.Errorf("an absolute -C: %+v, %v", l, ok)
	}
	for _, args := range [][]string{
		{"--git-dir=x", "commit"}, {"--work-tree", "x", "commit"}, {"-C"}, {"-c"}, {}, {"--no-pager"},
	} {
		if l, ok := parse(args); ok {
			t.Errorf("parse(%q) = %+v, want not the shim's", args, l)
		}
	}
}

// git add's arguments that stage more than the files they name (slice 128),
// read as git reads them: -A, -u and their long forms and abbreviations, in a
// cluster too, and a folder, . among them, after "--" too; never a valued
// option's value, a file, a path that is not there, or --no-all.
func TestSweeping(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for args, want := range map[string]string{
		"-A":                        "-A",
		"-vA notes.md":              "-A",
		"--all":                     "--all",
		"--al":                      "--al",
		"--no-ignore-removal":       "--no-ignore-removal",
		"-u":                        "-u",
		"-nu":                       "-u",
		"--update":                  "--update",
		"--up":                      "--up",
		".":                         ".",
		"notes.md sub":              "sub",
		"-- sub":                    "sub",
		"notes.md":                  "",
		"-- -A":                     "",
		"gone.md":                   "",
		"--no-all notes.md":         "",
		"-v -f notes.md":            "",
		"-U sub notes.md":           "",
		"-U3 notes.md":              "",
		"--chmod +x notes.md":       "",
		"--chmod=+x notes.md":       "",
		"--pathspec-from-file sub":  "",
		"--inter-hunk-context 3 -p": "",
		"--a":                       "",
	} {
		arg, what := sweeping(dir, strings.Fields(args))
		if arg != want || (want == "") != (what == "") {
			t.Errorf("sweeping(%q) = %q, %q; want %q", args, arg, what, want)
		}
	}
	if _, what := sweeping(dir, []string{"sub"}); what != "everything in the folder sub" {
		t.Errorf("a folder stages %q", what)
	}
}

// In a repository itos manages, git add -A and git stage of a folder are
// refused, exit 2, naming git add <path>, and no git runs.
func TestMainRefusesSweepingStages(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{git.EnvGit, "GIT_DIR", "GIT_WORK_TREE", "ITOS_CONFIG"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	// A git folder of the repository's, for -C sub to find its top by.
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "itos.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	for _, args := range [][]string{{"add", "-A"}, {"-C", "sub", "stage", "."}} {
		var stderr strings.Builder
		itos, code, done := Main(args, &stderr, vcs.Git{Platform: runtime.GOOS}, launch.CLI{})
		if itos != nil || code != 2 || !done || !strings.Contains(stderr.String(), "git add <path>") ||
			!strings.Contains(stderr.String(), "git "+args[len(args)-2]+" "+args[len(args)-1]+" would stage") {
			t.Errorf("Main(%q) = %q, %d, %v:\n%s", args, itos, code, done, stderr.String())
		}
	}
}

// When the real git cannot start, Main reports the launch failure rather
// than returning the zero exit code from run.
func TestMainReportsRealGitThatCannotStart(t *testing.T) {
	t.Setenv(git.EnvGit, t.TempDir())
	var stderr strings.Builder
	itos, code, done := Main([]string{"status"}, &stderr, vcs.Git{Platform: runtime.GOOS}, launch.CLI{})
	if itos != nil || code != 3 || !done || !strings.HasPrefix(stderr.String(), "itos: cannot run git (") {
		t.Errorf("Main with an unstartable git = %q, %d, %v, %q", itos, code, done, stderr.String())
	}
}
