package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A --config the environment refuses (a NUL in it) stops the run before any
// command.
func TestApplyGlobalsRefusesAConfigTheEnvironmentRefuses(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	if _, _, err := applyGlobals(Globals{Config: "a\x00b"}, t.TempDir(), runtime.GOOS); err == nil {
		t.Error("a --config holding a NUL was set")
	}
}

// --root names the folder the command runs in, read from the run's folder
// when relative; one that is missing or no folder is a usage error naming
// it, before any command.
func TestApplyGlobalsRoot(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	dir := t.TempDir()
	writeFiles(t, map[string]string{filepath.Join(dir, "sub", "file"): "a file\n"})
	at, origin, err := applyGlobals(Globals{Root: "sub"}, dir, runtime.GOOS)
	if err != nil || !sameDir(at, filepath.Join(dir, "sub")) || origin != "" {
		t.Errorf("--root sub = %q, %q, %v; want the run's sub, no origin", at, origin, err)
	}
	for _, root := range []string{"gone", filepath.Join("sub", "file"), "a\x00b"} {
		_, _, err := applyGlobals(Globals{Root: root}, dir, runtime.GOOS)
		if ExitCode(err) != ExitUsage || !strings.HasPrefix(err.Error(), "--root "+root+": ") {
			t.Errorf("--root %q: %v, want a usage error naming it", root, err)
		}
	}
}

// From a subfolder of a repository whose config is at its top, the command
// runs at the top, and gives where the person stood; at the top it stays.
func TestApplyGlobalsMovesToTheTop(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\n")
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	at, origin, err := applyGlobals(Globals{}, sub, runtime.GOOS)
	if err != nil || !sameDir(at, dir) || origin != filepath.Join("a", "b") {
		t.Errorf("from a/b = %q, %q, %v; want the top and a/b", at, origin, err)
	}
	if at, origin, err := applyGlobals(Globals{}, dir, runtime.GOOS); err != nil || at != dir || origin != "" {
		t.Errorf("at the top = %q, %q, %v; want it as given, no origin", at, origin, err)
	}
}

// A folder entered keeps the spelling of the one it was entered from when
// it is the same; another is given with its links resolved, but on windows,
// as it was entered.
func TestEntered(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("no symbolic link here: %v", err)
	}
	if got := entered("linux", link, real); got != link {
		t.Errorf("entered the same folder = %q, want %q", got, link)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	if got := entered("linux", dir, link); got != resolved {
		t.Errorf("entered a link = %q, want %q", got, resolved)
	}
	if got := entered("windows", dir, link); got != link {
		t.Errorf("entered a link on windows = %q, want %q", got, link)
	}
	gone := filepath.Join(dir, "gone")
	if got := entered("linux", dir, gone); got != gone {
		t.Errorf("entered a missing folder = %q, want %q", got, gone)
	}
}

// moveTo gives no origin for the folder the person stands in, nor for a
// top no path leads to from where they stood.
func TestMoveToGivesNoOrigin(t *testing.T) {
	dir := t.TempDir()
	if at, origin := moveTo(runtime.GOOS, dir, dir); at != dir || origin != "" {
		t.Errorf("moveTo the folder itself = %q, %q", at, origin)
	}
	if _, origin := moveTo(runtime.GOOS, "relative", dir); origin != "" {
		t.Errorf("moveTo from a relative folder = %q, want no origin", origin)
	}
}

// A path typed in a subfolder is read there, though itos runs at the top:
// commit check-message's file, tests smoke check's --features; tests list
// lists the kind at the working tree. The run's folder says where the
// person stood; the process stands elsewhere.
func TestPathsTypedInASubfolder(t *testing.T) {
	dir := gitConfigRepo(t, `version: 1
commits: { types: [docs], header_lint: { use: builtin } }
tests:
  scenario: { root: features, id: "ID-[A-Z]+-\\d+", tag_prefix: "@", smoke: { file: smoke.yaml } }
`)
	feature := "Feature: A\n\n  @ID-A-01\n  Scenario: One\n    When I go\n"
	writeFiles(t, map[string]string{
		filepath.Join(dir, "smoke.yaml"):                    "- file: a.feature\n  scenarios: [{ id: \"@ID-A-01\", why: it goes }]\n",
		filepath.Join(dir, "features", "a.feature"):         feature,
		filepath.Join(dir, "sub", "copy", "a.feature"):      feature,
		filepath.Join(dir, "sub", "copy", "orphan.feature"): "Feature: B\n\n  @ID-B-01\n  Scenario: Two\n    When I stay\n",
		filepath.Join(dir, "sub", "message.txt"):            "docs: say a thing\n",
		filepath.Join(dir, "message.txt"):                   "no type at all\n",
	})
	t.Chdir(t.TempDir())
	in := func(folder string, args ...string) (int, string, string) {
		t.Helper()
		var stdout, stderr strings.Builder
		code := Main(Run{Args: args, Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS, Dir: folder})
		return code, stdout.String(), stderr.String()
	}
	sub := filepath.Join(dir, "sub")
	if code, _, stderr := in(sub, "commit", "check-message", "message.txt"); code != 0 {
		t.Errorf("check-message of the subfolder's message.txt: exit %d, %s", code, stderr)
	}
	if code, _, _ := in(dir, "commit", "check-message", "message.txt"); code != ExitPolicy {
		t.Errorf("check-message of the top's message.txt: exit %d, want %d", code, ExitPolicy)
	}
	if code, _, stderr := in(sub, "tests", "smoke", "check", "scenario", "--features", "copy"); code != ExitPolicy || !strings.Contains(stderr, "orphan.feature") {
		t.Errorf("smoke check of the subfolder's copy: exit %d, %s, want its orphan named", code, stderr)
	}
	if code, stdout, stderr := in(sub, "tests", "list", "scenario"); code != 0 || !strings.Contains(stdout, "ID-A-01  a.feature") || strings.Contains(stdout, "ID-B-01") {
		t.Errorf("tests list from the subfolder: exit %d, %q %s, want the top's features alone", code, stdout, stderr)
	}
}
