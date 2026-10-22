package cli

import (
	"io"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"

	vcsgit "github.com/donvargax/itos/v8/internal/vcs/adapter"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/message"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/tests"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
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
	cfg, err := config.FromText(vcs.Git{}, trees.Trees{VCS: vcsgit.Git{}}, trees.Worktree, "itos.yaml", `version: 1
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
	if out, err := exec.Command(git.Bin(runtime.GOOS), "init", "-q", unborn).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	t.Chdir(unborn)
	if _, err := stagedChange(cfg, msg, "", nil); err == nil {
		t.Error("an amend's note that cannot be read gave no error")
	}
}

// A kind's staged range command runs for a commit of every type with path
// rules but those its except_types names, each line it prints on stderr a
// problem of the commit.
func TestStagedRuleSkipsTheExceptTypes(t *testing.T) {
	notesRepo(t)
	writeFile(t, "a.md", "a\n")
	gitIn(t, "add", "a.md")
	cfg, err := config.FromText(vcs.Git{}, trees.Trees{VCS: vcsgit.Git{}}, trees.Worktree, "itos.yaml", `version: 1
commits:
  types: [feat, docs]
  scopes: { feat: {}, docs: {} }
tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    range_checks:
      - { name: staged-says, staged: "echo '  - judged {type}' >&2; exit 1", except_types: [docs] }
`, false)
	if err != nil {
		t.Fatal(err)
	}
	for typ, want := range map[string]int{"feat": ExitPolicy, "docs": 0} {
		var stderr strings.Builder
		code, err := stagedRule(cfg, typ+": x\n", "", nil, Out{Stdout: io.Discard, Stderr: &stderr})
		if err != nil || code != want || strings.Contains(stderr.String(), "judged "+typ) != (want != 0) {
			t.Errorf("%s: exit %d, %v\n%s", typ, code, err, stderr.String())
		}
	}
}
