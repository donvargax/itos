package initcmd

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

// project is a config check finds nothing in, its people file among it.
func project() map[string]string {
	return map[string]string{
		"itos.yaml":             "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { people: { source: yaml, file: people.yaml } }\n",
		"tasks/phase-1.yaml":    "[]\n",
		"tasks/work-items.yaml": "phases: {}\nitems: []\n",
		"people.yaml":           "- a\n",
	}
}

// declared is f with itos's two hooks declared in its git config.
func declared(f *fake) *fake {
	for _, event := range []string{"commit-msg", "pre-push"} {
		f.git["hook.itos-"+event+".command"] = []string{"itos hook " + event}
		f.git["hook.itos-"+event+".event"] = []string{event}
	}
	return f
}

// Where a config is there and nothing is missing, init writes nothing,
// says so, and makes the offers as a rerun, asking nobody; their code is
// its own.
func TestInitReportsNothingMissing(t *testing.T) {
	f := declared(scratch(t, "commit", project()))
	code, stdout, _, err := call(t, f, "--git-shim")
	link := filepath.Join("bin", "git")
	want := "itos.yaml is there already; init kept it as it is.\nNothing is missing: the config, its ledger, registry and " +
		"smoke sets are sound, and the hooks call itos.\nLinked the git shim: " + link + " to itos.\nbin stands at -1,  at -1.\n"
	if code != 0 || err != nil || stdout != want {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout)
	}
	if f.asked != 0 || len(f.made) != 1 || f.made[0] != link || len(f.installs) != 0 {
		t.Errorf("terminal asked %d times, linked %q, hooks %v", f.asked, f.made, f.installs)
	}
	f = declared(scratch(t, "commit", project()))
	f.placeErr = errors.New("no itos to link")
	if code, _, _, err := call(t, f); code != 1 || err != nil {
		t.Errorf("an offer failing: exit %d, %v", code, err)
	}
}

// What is missing is listed, with what fixes it, exit 1: each hook the git
// config does not declare, or a git that runs none; then the notes, never
// counted: a people file the repository lacks, and a pin behind the newest
// release.
func TestInitReportsWhatIsMissing(t *testing.T) {
	files := project()
	files["itos.yaml"] = "version: 1\npin: { version: 9.1.0, checksums: \"" + strings.Repeat("a", 64) + "\" }\n" +
		"ledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { people: { source: yaml, file: people.yaml } }\n"
	delete(files, "people.yaml")
	f := scratch(t, "commit", files)
	code, stdout, _, err := call(t, f)
	for _, want := range []string{
		"Missing:\n  the commit-msg hook: hook.itos-commit-msg in the git config does not run itos; run itos hook install\n" +
			"  the pre-push hook: hook.itos-pre-push in the git config does not run itos; run itos hook install\n",
		"Noted, not counted as missing:\n  ",
		"people.yaml",
		"  the pin, itos 9.1.0, is behind the newest release, itos 9.2.0; itos upgrade moves it there, applying the config " +
			"changes each release between them declares and naming what is left by hand\n" + shimUnasked,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("no %q in:\n%s", want, stdout)
		}
	}
	if code != 1 || err != nil {
		t.Errorf("exit %d, %v", code, err)
	}
	f = scratch(t, "commit", project())
	f.oldGit = true
	_, stdout, _, _ = call(t, f)
	if want := "Missing:\n  the commit-msg hook: git 2.53.0 runs no hook its config declares; upgrade git to 2.54.0 or later, " +
		"then run itos hook install\n" + shimUnasked; !strings.HasSuffix(stdout, want) {
		t.Errorf("an old git:\n%s", stdout)
	}
}

// Under --json the report is one object: the config, what is missing and
// what is noted, each an empty list for none, and the offers', whose words
// are discarded.
func TestInitReportsUnderJSON(t *testing.T) {
	f := declared(scratch(t, "commit", project()))
	code, stdout, _, err := call(t, f, "--json")
	type offer struct{ Action string }
	var got struct {
		Config, Action string
		Missing, Notes []map[string]any
		Plugin         offer
		GitShim        offer `json:"git_shim"`
		AgentRules     offer `json:"agent_rules"`
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Config != "itos.yaml" ||
		got.Action != "checked" || got.Missing == nil || got.Notes == nil || got.Plugin.Action != "no_claude" ||
		got.GitShim.Action != "offered" || got.AgentRules.Action != "offered" || strings.Contains(stdout, "not linked") {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout)
	}
	f = scratch(t, "commit", project())
	f.placeErr = errors.New("no itos to link")
	code, stdout, _, err = call(t, f, "--json")
	if code != 1 || err != nil || !strings.Contains(stdout, `"rule": "hook-missing"`) {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout)
	}
}

// A config that does not load is what is missing, its hooks unjudged; data
// config check cannot read stops init with the error.
func TestInitReportsAConfigThatDoesNotLoad(t *testing.T) {
	f := scratch(t, "commit", map[string]string{"itos.yaml": "version: [\n"})
	code, stdout, _, err := call(t, f)
	if code != 1 || err != nil || !strings.Contains(stdout, "Missing:\n  itos.yaml") || strings.Contains(stdout, "hook") {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout)
	}
	code, stdout, _, err = call(t, f, "--json")
	if code != 1 || err != nil || !strings.Contains(stdout, `"notes": [],`) {
		t.Errorf("--json: exit %d, %v:\n%s", code, err, stdout)
	}
	files := project()
	delete(files, "tasks/work-items.yaml")
	files["tasks/work-items.yaml/x"] = ""
	f = scratch(t, "commit", files)
	if _, _, _, err := call(t, f); err == nil {
		t.Error("a registry that is a folder: no error")
	}
}

// --stealth where the project has its own config is a usage error; under a
// stealth config there, --plugin project is one, and the offers are made
// as under --stealth.
func TestInitReportsUnderAStealthConfig(t *testing.T) {
	f := scratch(t, "commit", project())
	if _, _, _, err := call(t, f, "--stealth"); !isUsage(err, "--stealth writes a config in the git folder for a clone that "+
		"cannot change the project, but this one has its own itos.yaml; run init without --stealth") {
		t.Errorf("--stealth: %v", err)
	}
	stealth := filepath.Join(".git", "itos", "itos.yaml")
	f = scratch(t, "commit", map[string]string{stealth: "version: 1\n"})
	if _, _, _, err := call(t, f, "--plugin", "project"); !isUsage(err, "under --stealth, --plugin project") {
		t.Errorf("--plugin project: %v", err)
	}
	for _, args := range [][]string{nil, {"--stealth"}} {
		f = withClaude(scratch(t, "commit", map[string]string{stealth: "version: 1\n"}))
		_, stdout, _, err := call(t, f, args...)
		if err != nil || !strings.HasPrefix(stdout, stealth+" is there already") ||
			!strings.Contains(stdout, "itos init --plugin installs it for you alone in this repository (.claude/settings.local.json, "+
				"kept out of git status)") {
			t.Errorf("init %v: %v:\n%s", args, err, stdout)
		}
	}
}

// init notes a pin strictly older than the newest release, naming itos
// upgrade (slice 115); a pin at the newest or past it, no pin, or a release
// server with no answer is no note.
func TestPinBehind(t *testing.T) {
	f := scratch(t, "", nil)
	h, ctx := NewHandler(f), context.Background()
	pinning := func(v string) *config.Loaded {
		return &config.Loaded{Config: config.Config{Pin: &config.Pin{Version: v}}}
	}
	for _, c := range []struct {
		name string
		cfg  *config.Loaded
	}{
		{"no pin", &config.Loaded{}},
		{"a pin with no version", pinning("")},
		{"the newest", pinning("9.2.0")},
		{"one past the newest", pinning("9.3.0")},
	} {
		if p := h.pinBehind(ctx, c.cfg); p != nil {
			t.Errorf("%s: noted %+v", c.name, *p)
		}
	}
	p := h.pinBehind(ctx, pinning("9.1.0"))
	if p == nil || p.Rule != "pin-behind" || p.Message != "the pin, itos 9.1.0, is behind the newest release, itos 9.2.0" {
		t.Fatalf("the note is %+v", p)
	}
	f.releaseErr = errors.New("no answer")
	if p := h.pinBehind(ctx, pinning("9.1.0")); p != nil {
		t.Errorf("with no answer from the release server, noted %+v", *p)
	}
}
