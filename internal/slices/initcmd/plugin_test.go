package initcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledPlugin(t *testing.T) {
	for _, c := range []struct {
		list      string
		installed bool
		scope     string
		error     bool
	}{
		{`[]`, false, "", false},
		{`[{"id":"itos@itos","scope":"project","enabled":false}]`, false, "", false},
		{`[{"id":"other@itos","scope":"user","enabled":true},{"id":"itos@itos","scope":"local","enabled":true}]`, true, "local", false},
		{`not json`, false, "", true},
	} {
		e, ok, err := installedPlugin([]byte(c.list))
		if ok != c.installed || e.Scope != c.scope || (err != nil) != c.error {
			t.Errorf("%s: %v %q %v", c.list, ok, e.Scope, err)
		}
	}
}

func TestPluginQuestion(t *testing.T) {
	for _, c := range []struct {
		stealth bool
		input   string
		answer  string
	}{
		{false, "\n", "project"},
		{true, "\n", "local"},
		{false, "User\n", "user"},
		{true, "project\nno\n", "no"},
		{false, "maybe\nlocal", "local"},
		{false, "maybe", ""},
		{false, "", ""},
	} {
		var log strings.Builder
		p := pluginOffer{stealth: c.stealth, log: &log, answers: strings.NewReader(c.input)}
		if got := p.question(); got != c.answer {
			t.Errorf("%q (stealth %v): %q, want %q\n%s", c.input, c.stealth, got, c.answer, log.String())
		}
	}
}

// pluginRun makes the offer over f with the flag, the mode and, when
// answers is not "", a terminal answering them, and gives its outcome, its
// exit code and what it said.
func pluginRun(f *fake, flag PluginFlag, stealth bool, answers string) (pluginOutcome, int, string) {
	var log strings.Builder
	p := pluginOffer{flag: flag, stealth: stealth, ask: answers != "", log: &log, answers: strings.NewReader(answers), ports: f}
	got, code := p.run(context.Background())
	return got, code, log.String()
}

// The offer, each way it ends: declined by the flag or the terminal; with
// no claude on the PATH, said only for --plugin; with claude plugin list
// failing, or not a list, a failure for --plugin and otherwise unknown;
// installed already; said where nobody is asked; installed at the flag's
// scope or the terminal's answer; and the install failing, the
// marketplace's failure said with it.
func TestPluginOfferOutcomes(t *testing.T) {
	given, bare := PluginFlag{Given: true, Scope: "user"}, PluginFlag{Given: true}
	listFails := map[string]error{"plugin list --json": exitErr(2)}
	for _, c := range []struct {
		name     string
		claude   bool
		outputs  map[string]string
		failures map[string]error
		flag     PluginFlag
		stealth  bool
		answers  string
		action   string
		scope    any
		code     int
		says     string
	}{
		{"--plugin no", true, nil, nil, PluginFlag{Given: true, Scope: "no"}, false, "", "declined", nil, 0, ""},
		{"no claude, unasked", false, nil, nil, PluginFlag{}, false, "", "no_claude", nil, 0, ""},
		{"no claude, --plugin", false, nil, nil, bare, true, "", "no_claude", "local", 0,
			"Claude Code was not found (no claude on the PATH), so the itos plugin for it is not installed; " +
				"with Claude Code installed, itos init --plugin local installs it.\n"},
		{"the list failing, --plugin", true, map[string]string{"plugin list --json": " oops \n"}, listFails, given, false, "",
			"failed", "user", exitPolicy, "claude plugin list --json failed (exit 2):\noops\n"},
		{"the list failing, unasked", true, map[string]string{"plugin list --json": "oops"}, listFails, PluginFlag{}, false, "",
			"unknown", nil, 0, "claude plugin list --json failed (exit 2), so init cannot say whether the itos plugin for " +
				"Claude Code is installed; itos init --plugin installs it.\n"},
		{"the list no JSON", true, map[string]string{"plugin list --json": "no"}, nil, given, false, "", "failed", "user",
			exitPolicy, "claude plugin list --json failed (its output is no JSON list of plugins: "},
		{"installed already", true, map[string]string{"plugin list --json": `[{"id":"itos@itos","scope":"user","enabled":true}]`},
			nil, given, false, "", "already", "user", 0,
			"The itos plugin for Claude Code, itos@itos, is installed already (its user scope).\n"},
		{"unasked", true, nil, nil, PluginFlag{}, false, "", "offered", nil, 0,
			"The itos plugin for Claude Code is not installed: itos init --plugin installs it for this repository " +
				"(.claude/settings.json, to commit), --plugin user for every repository of yours, --plugin local for you alone in this one.\n"},
		{"unasked, stealth", true, nil, nil, PluginFlag{}, true, "", "offered", nil, 0,
			"The itos plugin for Claude Code is not installed: itos init --plugin installs it for you alone in this repository " +
				"(.claude/settings.local.json, kept out of git status), --plugin user for every repository of yours.\n"},
		{"a no at the terminal", true, nil, nil, PluginFlag{}, false, "no\n", "declined", nil, 0, "or no [project]: "},
		{"a scope at the terminal", true, nil, nil, PluginFlag{}, false, "user\n", "installed", "user", 0, pluginInstalled("user")},
		{"--plugin", true, nil, nil, bare, false, "", "installed", "project", 0, pluginInstalled("project")},
		{"the install failing", true, map[string]string{"plugin marketplace add donvargax/itos --scope user": " gone \n",
			"plugin install itos@itos --scope user": "not found"},
			map[string]error{"plugin marketplace add donvargax/itos --scope user": errors.New("no such program"),
				"plugin install itos@itos --scope user": exitErr(1)},
			given, false, "", "failed", "user", exitPolicy,
			"claude plugin install itos@itos --scope user failed (exit 1):\nclaude plugin marketplace add donvargax/itos " +
				"--scope user failed (no such program): gone\nnot found\n"},
		{"the marketplace failing alone", true, nil,
			map[string]error{"plugin marketplace add donvargax/itos --scope user": exitErr(1)},
			given, false, "", "installed", "user", 0, pluginInstalled("user")},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := scratch(t, "commit", nil)
			if c.claude {
				withClaude(f)
				for k, v := range c.outputs {
					f.outputs[k] = v
				}
				f.failures = c.failures
			}
			got, code, said := pluginRun(f, c.flag, c.stealth, c.answers)
			if got.Action != c.action || got.Scope != c.scope || code != c.code || !strings.Contains(said, c.says) ||
				(c.says == "" && said != "") || got.Excluded {
				t.Errorf("%+v, exit %d:\n%s", got, code, said)
			}
			if got.Action == "failed" && !strings.HasPrefix(said, got.Problem) {
				t.Errorf("the problem %q, said %q", got.Problem, said)
			}
		})
	}
}

// Installed locally under --stealth, .claude/settings.local.json is listed
// in the git folder's info/exclude while git shows it untracked, its
// last line ended first; one git does not show is left alone; and an
// exclude that cannot be written is said.
func TestPluginOfferExcludesTheLocalSettings(t *testing.T) {
	local := PluginFlag{Given: true, Scope: "local"}
	f := withClaude(scratch(t, "commit", map[string]string{".git/info/exclude": "*.log"}))
	f.Work = map[string]string{localSettings: "{}\n"}
	got, code, said := pluginRun(f, local, true, "")
	if got.Action != "installed" || !got.Excluded || code != 0 || read(filepath.Join(".git", "info", "exclude")) != "*.log\n/"+localSettings+"\n" ||
		!strings.HasSuffix(said, "Listed .claude/settings.local.json in the git folder's info/exclude, so git status stays clean.\n") {
		t.Errorf("%+v, exit %d:\n%s", got, code, said)
	}

	f = withClaude(scratch(t, "commit", nil))
	if got, _, said := pluginRun(f, local, true, ""); got.Excluded || said != pluginInstalled("local") || exists(".git/info") {
		t.Errorf("not shown: %+v:\n%s", got, said)
	}
	if got, _, _ := pluginRun(f, local, false, ""); got.Excluded {
		t.Errorf("not stealth: %+v", got)
	}

	for name, files := range map[string]map[string]string{
		"info a file":      {".git/info": "mine\n"},
		"exclude a folder": {".git/info/exclude/x": ""},
	} {
		t.Run(name, func(t *testing.T) {
			f := withClaude(scratch(t, "commit", files))
			f.Work = map[string]string{localSettings: "{}\n"}
			got, code, said := pluginRun(f, local, true, "")
			if got.Action != "installed" || got.Excluded || code != 0 ||
				!strings.Contains(said, "Could not list .claude/settings.local.json in the git folder's info/exclude: ") {
				t.Errorf("%+v, exit %d:\n%s", got, code, said)
			}
		})
	}

	f = withClaude(scratch(t, "commit", nil))
	f.Work, f.Located = map[string]string{localSettings: "{}\n"}, nil
	if got, _, said := pluginRun(f, local, true, ""); got.Excluded || !strings.Contains(said, "Could not list") {
		t.Errorf("no repository: %+v:\n%s", got, said)
	}
}

// A command's failure is its exit code, or why it did not run.
func TestFailureOf(t *testing.T) {
	if got := failureOf(exitErr(3)); got != "exit 3" {
		t.Errorf("an exit: %q", got)
	}
	if got := failureOf(os.ErrPermission); got != "permission denied" {
		t.Errorf("no run: %q", got)
	}
}
