package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A --config the environment refuses (a NUL in it) stops the run before any
// command.
func TestApplyGlobalsRefusesAConfigTheEnvironmentRefuses(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	if _, err := applyGlobals(Globals{Config: "a\x00b"}); err == nil {
		t.Error("a --config holding a NUL was set")
	}
}

// failingGetwd makes getwd fail from its nth call on, until the test ends.
func failingGetwd(t *testing.T, n int) {
	t.Helper()
	calls := 0
	t.Cleanup(func() { getwd = os.Getwd })
	getwd = func() (string, error) {
		calls++
		if calls >= n {
			return "", errors.New("the folder is gone")
		}
		return os.Getwd()
	}
}

// moveTo gives no origin when the person stands at the top, and fails when
// the top cannot be entered or either folder cannot be read.
func TestMoveTo(t *testing.T) {
	here := t.TempDir()
	t.Chdir(here)
	if origin, err := moveTo(here); err != nil || origin != "" {
		t.Errorf("moveTo the folder it stands in = %q, %v, want no origin", origin, err)
	}
	gone := filepath.Join(here, "gone")
	if _, err := moveTo(gone); err == nil || !strings.Contains(err.Error(), gone) {
		t.Errorf("moveTo a missing folder: %v, want an error naming it", err)
	}
	for n, where := range map[int]string{1: "where the person stood", 2: "the top"} {
		t.Run(where, func(t *testing.T) {
			t.Chdir(here)
			failingGetwd(t, n)
			if _, err := moveTo(here); err == nil {
				t.Errorf("moveTo when %s cannot be read gave no error", where)
			}
		})
	}
}

// From a subfolder of a repository whose config is at its top, a folder
// that cannot be read stops the run before any command.
func TestApplyGlobalsStopsWhenItCannotMoveToTheTop(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\n")
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	failingGetwd(t, 1)
	if _, err := applyGlobals(Globals{}); err == nil {
		t.Error("applyGlobals moved to the top though where it stood could not be read")
	}
}

// A path typed in a subfolder is read there, though itos runs at the top:
// commit check-message's file, tests smoke check's --features; tests list
// lists the kind at the working tree.
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
	sub := filepath.Join(dir, "sub")
	from := func(args ...string) (int, string, string) {
		t.Helper()
		t.Chdir(sub)
		return run(args...)
	}
	if code, _, stderr := from("commit", "check-message", "message.txt"); code != 0 {
		t.Errorf("check-message of the subfolder's message.txt: exit %d, %s", code, stderr)
	}
	t.Chdir(dir)
	if code, _, _ := run("commit", "check-message", "message.txt"); code != ExitPolicy {
		t.Errorf("check-message of the top's message.txt: exit %d, want %d", code, ExitPolicy)
	}
	if code, _, stderr := from("tests", "smoke", "check", "scenario", "--features", "copy"); code != ExitPolicy || !strings.Contains(stderr, "orphan.feature") {
		t.Errorf("smoke check of the subfolder's copy: exit %d, %s, want its orphan named", code, stderr)
	}
	if code, stdout, stderr := from("tests", "list", "scenario"); code != 0 || !strings.Contains(stdout, "ID-A-01  a.feature") || strings.Contains(stdout, "ID-B-01") {
		t.Errorf("tests list from the subfolder: exit %d, %q %s, want the top's features alone", code, stdout, stderr)
	}
}
