package initcmd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// A folder init cannot make stops it with the error: the ledger's, and
// under --stealth the config's and the smoke set's.
func TestInitWriteStopsAtAFolderItCannotMake(t *testing.T) {
	for _, c := range []struct {
		name  string
		files map[string]string
		args  []string
		want  string
	}{
		{"the ledger's", map[string]string{"tasks": "mine\n"}, nil, "tasks"},
		{"the stealth config's", map[string]string{".git/itos": "mine\n"}, []string{"--stealth"}, "itos"},
		{"the stealth smoke set's", map[string]string{".git/itos/features": "mine\n", "features/a.feature": "Feature: A\n"},
			[]string{"--stealth"}, "features"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := scratch(t, "commit", c.files)
			code, _, _, err := call(t, f, c.args...)
			if code != 0 || err == nil || !strings.Contains(err.Error(), c.want) || len(f.installs) != 0 {
				t.Errorf("exit %d, %v", code, err)
			}
		})
	}
}

// Under --json init prints one object on stdout, what it says on stderr:
// its config, the files, HEAD as since, the pin, hook install's object
// without its schema, and the offers', whose words are discarded; with no
// release server no pin, and why.
func TestInitWriteUnderJSON(t *testing.T) {
	type report struct {
		Config     string
		Action     string
		GitInit    bool `json:"git_init"`
		Since      *string
		Files      []writtenFile
		Pin        *struct{ Version, Checksums string }
		PinProblem string `json:"pin_problem"`
		Hooks      map[string]any
		Plugin     string
		GitShim    string `json:"git_shim"`
		AgentRules string `json:"agent_rules"`
	}
	for _, served := range []bool{true, false} {
		f := scratch(t, "commit", nil)
		f.hooksJSON = "{\n  \"schema\": 1,\n  \"hooks\": [1]\n}\n"
		if !served {
			f.releaseErr = errors.New("no server")
		}
		code, stdout, stderr, err := call(t, f, "--json")
		var got report
		if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || stderr != "" {
			t.Fatalf("served %v: exit %d, %v:\n%s%s", served, code, err, stdout, stderr)
		}
		if got.Config != "itos.yaml" || got.Action != "initialized" || got.GitInit || got.Since == nil || *got.Since != head ||
			len(got.Files) != 3 || got.Files[2] != (writtenFile{"tasks/work-items.yaml", "wrote"}) ||
			(got.Pin != nil) != served || (got.PinProblem == "") != served || len(got.Hooks) != 1 ||
			got.Plugin != "plugin" || got.GitShim != "shim" || got.AgentRules != "rules" {
			t.Errorf("served %v:\n%s", served, stdout)
		}
		if served && (got.Pin.Version != "9.2.0" || got.Pin.Checksums != release.SHA256([]byte(sums))) {
			t.Errorf("the pin %+v", got.Pin)
		}
		if !served && got.PinProblem != "no server" {
			t.Errorf("the pin's problem %q", got.PinProblem)
		}
		if len(f.installs) != 1 || !f.installs[0] || f.offers[0].Ask {
			t.Errorf("hooks %v, offers %+v", f.installs, f.offers)
		}
	}
	// In an unborn repository, since is null.
	f := scratch(t, "unborn", nil)
	f.hooksJSON = ""
	_, stdout, _, err := call(t, f, "--json")
	if err != nil || !strings.Contains(stdout, `"since": null`) || !strings.Contains(stdout, `"hooks": null`) {
		t.Errorf("unborn: %v:\n%s", err, stdout)
	}
}

// hook install's code is init's, the offers' only when it is 0, and with
// it not 0 init names nothing to commit; hook install failing, or writing
// what is no JSON under --json, stops init.
func TestInitWriteTakesHookInstallsCode(t *testing.T) {
	f := scratch(t, "commit", nil)
	f.hooksCode, f.offered.Code = 3, 1
	code, stdout, _, err := call(t, f)
	if code != 3 || err != nil || strings.Contains(stdout, "Commit what init wrote") || !strings.HasSuffix(stdout, "offered\n") {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout)
	}
	f = scratch(t, "commit", nil)
	f.offered.Code = 1
	if code, _, _, err := call(t, f); code != 1 || err != nil {
		t.Errorf("an offer failing: exit %d, %v", code, err)
	}
	f = scratch(t, "commit", nil)
	f.offered.Code = 1
	if code, _, _, err := call(t, f, "--json"); code != 1 || err != nil {
		t.Errorf("an offer failing under --json: exit %d, %v", code, err)
	}
	f = scratch(t, "commit", nil)
	f.hooksErr = errors.New("git config failed")
	if _, _, _, err := call(t, f); err != f.hooksErr || len(f.offers) != 0 {
		t.Errorf("hook install failing: %v", err)
	}
	f = scratch(t, "commit", nil)
	f.hooksErr = errors.New("git config failed")
	if _, _, _, err := call(t, f, "--json"); err != f.hooksErr {
		t.Errorf("hook install failing under --json: %v", err)
	}
	f = scratch(t, "commit", nil)
	f.hooksJSON = "not json"
	if _, _, _, err := call(t, f, "--json"); err == nil || err.Error() != "hook install wrote no JSON object: not json" {
		t.Errorf("hook install writing no JSON: %v", err)
	}
}

// ITOS_CONFIG naming a config that is not there is a usage error, nothing
// written; so is a run in the git folder.
func TestInitUsageErrorsWhereItRuns(t *testing.T) {
	f := scratch(t, "commit", nil)
	t.Setenv("ITOS_CONFIG", "missing.yaml")
	if _, _, _, err := call(t, f); !isUsage(err, "not the missing missing.yaml that --config or ITOS_CONFIG names") ||
		exists("itos.yaml") {
		t.Errorf("ITOS_CONFIG: %v", err)
	}
	f = scratch(t, "commit", nil)
	f.Located[""] = vcsport.Location{GitDir: "."}
	if _, _, _, err := call(t, f); !isUsage(err, "init runs in a repository's working tree, not in its git folder") {
		t.Errorf("in the git folder: %v", err)
	}
}

// hasFeatureFiles finds a feature file at any depth, and nothing in a
// folder without one or a folder that is not there.
func TestHasFeatureFiles(t *testing.T) {
	scratch(t, "", map[string]string{"a/b/c.feature": "", "d/e.txt": "", "f/g.feature/h": ""})
	for dir, want := range map[string]bool{"a": true, "d": false, "f": false, "missing": false} {
		if got := hasFeatureFiles(dir); got != want {
			t.Errorf("%s: %v", dir, got)
		}
	}
}

// The smoke set names each file's first live test after the tag prefix, and
// is an empty list when no file has one.
func TestSmokeSetOf(t *testing.T) {
	text, n := smokeSetOf(tests.List{Tests: []tests.Test{{ID: "U-1", File: "a_test.go", Live: true}}}, "")
	if n != 1 || text != starterSmokeHeader+"- file: a_test.go\n  scenarios:\n    - id: \"U-1\"\n      why: picked by itos init, "+
		"the file's first live scenario\n" {
		t.Errorf("%d:\n%s", n, text)
	}
	if count(1, "scenario") != "1 scenario" || count(0, "scenario") != "0 scenarios" || count(2, "file") != "2 files" {
		t.Error("count")
	}
}
