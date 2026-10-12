package cli

import (
	"io"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/tests"
)

// The commit being made as the moves rule reads it beside its trees (slice
// 134): the tests its message's footers name, by kind, or under a stealth
// config those itos commit hands the hook, and the staged paths, a merge's
// own; an error, which stops the hook, when the staged paths or the note an
// amend carries cannot be read.
func TestStagedChange(t *testing.T) {
	notesRepo(t)
	if err := os.Mkdir("src", 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "src/app.go", "package app\n")
	gitIn(t, "add", "src/app.go")
	cfg, err := config.FromText("itos.yaml", `version: 1
commits:
  types: [test]
  footers:
    Scenarios: { source: { tests: scenario }, strip_prefix: "@" }
tests:
  scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
`, false)
	if err != nil {
		t.Fatal(err)
	}
	msg := "test: describe it\n\nScenarios: @ID-A-01\n"
	got, err := stagedChange(cfg, msg, "", nil)
	want := tests.Change{Named: map[string][]string{"scenario": {"ID-A-01"}}, Paths: []string{"src/app.go"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("staged: %+v, %v, want %+v", got, err, want)
	}
	got, err = stagedChange(cfg, msg, "", &judged{files: []string{"own.go"}})
	want.Paths = []string{"own.go"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("a merge: %+v, %v, want %+v", got, err, want)
	}
	if _, err := stagedChange(cfg, msg, "no-such-commit", nil); err == nil {
		t.Error("staged paths against a commit that is none gave no error")
	}
	if code, err := stagedRule(cfg, "docs: x\n", "no-such-commit", nil, Out{Stdout: io.Discard, Stderr: io.Discard}); err == nil {
		t.Errorf("the hook judged the moves of paths it could not read: exit %d", code)
	}
	cfg.Stealth = true
	t.Setenv(message.FootersEnv, "Scenarios: @ID-B-01")
	got, err = stagedChange(cfg, msg, "", nil)
	want = tests.Change{Named: map[string][]string{"scenario": {"ID-B-01"}}, Paths: []string{"src/app.go"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("stealth: %+v, %v, want %+v", got, err, want)
	}
	os.Unsetenv(message.FootersEnv)
	t.Setenv(AmendEnv, "1")
	unborn := t.TempDir()
	if out, err := exec.Command(git.Bin(), "init", "-q", unborn).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	t.Chdir(unborn)
	if _, err := stagedChange(cfg, msg, "", nil); err == nil {
		t.Error("an amend's note that cannot be read gave no error")
	}
}
