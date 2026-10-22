package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stealthOffer makes the offer under a stealth config in a scratch
// repository holding the files, those named in tracked committed, the
// terminal answering answers when there are any, and gives its outcome, its
// exit code and what it said.
func stealthOffer(t *testing.T, write bool, answers string, files map[string]string, tracked ...string) (rulesOutcome, int, string) {
	t.Helper()
	initScratch(t, "commit", files)
	if len(tracked) > 0 {
		gitIn(t, append([]string{"add", "--"}, tracked...)...)
		gitIn(t, "-c", "user.name=itos", "-c", "user.email=itos@example.com", "commit", "-q", "-m", "docs: add")
	}
	return stealthRun(t, write, answers)
}

// stealthRun makes the offer under a stealth config where the test stands.
func stealthRun(t *testing.T, write bool, answers string) (rulesOutcome, int, string) {
	t.Helper()
	var log strings.Builder
	r := rulesOffer{flag: rulesFlag{given: write, write: write}, stealth: true, rerun: !write && answers == "", log: &log,
		answers: strings.NewReader(answers), ask: answers != ""}
	got, code := r.run()
	return got, code, log.String()
}

// The offer under a stealth config, each way it ends.
func TestStealthRulesOfferOutcomes(t *testing.T) {
	stale := map[string]string{".git/itos/AGENTS.md": rulesBegin + "\nOld.\n" + rulesEnd + "\n"}
	misplaced := map[string]string{overrideFile: rulesEnd + "\n"}
	for _, c := range []struct {
		name    string
		write   bool
		answers string
		files   map[string]string
		tracked []string
		action  string
		code    int
		says    string
	}{
		{"misplaced markers, rerun", false, "", misplaced, nil, "failed", 0, "The itos rules for agents cannot be checked: "},
		{"misplaced markers, asked", true, "", misplaced, nil, "failed", ExitPolicy, "The itos rules for agents are not written: "},
		{"a no at the terminal", false, "n\n", nil, nil, "declined", 0, "Write itos's rules for agents into .git/itos/AGENTS.md"},
		{"an older itos's block", false, "", stale, nil, "stale", 0, "are not the ones this itos writes"},
		{"only CLAUDE.local.md", false, "", map[string]string{claudeLocalFile: rulesBegin + "\n" + rulesEnd + "\n"}, nil,
			"stale", 0, "are not all in place here (.git/itos/AGENTS.md, CLAUDE.local.md, AGENTS.override.md)"},
		{"a tracked CLAUDE.local.md", true, "", map[string]string{claudeLocalFile: "Ours.\n"}, []string{claudeLocalFile},
			"failed", ExitPolicy, "CLAUDE.local.md is tracked by the project"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, code, log := stealthOffer(t, c.write, c.answers, c.files, c.tracked...)
			if got.Action != c.action || code != c.code || !strings.Contains(log, c.says) {
				t.Errorf("%s, exit %d:\n%s", got.Action, code, log)
			}
		})
	}
}

// With no rules yet and no --agent-rules or --no-agent-rules, the offer
// under a stealth config is said on init's first run when nobody is asked,
// and left unsaid on a rerun.
func TestStealthRulesOfferWithNoRulesYet(t *testing.T) {
	initScratch(t, "commit", nil)
	for _, rerun := range []bool{false, true} {
		var log strings.Builder
		r := rulesOffer{stealth: true, rerun: rerun, log: &log, answers: strings.NewReader("")}
		got, code := r.run()
		says := strings.Contains(log.String(), "itos init --agent-rules writes them into .git/itos/AGENTS.md")
		if got.Action != "offered" || code != 0 || says == rerun {
			t.Errorf("rerun %v: %s, exit %d:\n%s", rerun, got.Action, code, log.String())
		}
	}
}

// Written once, the rules are current on a rerun with or without
// --agent-rules.
func TestStealthRulesOfferCurrent(t *testing.T) {
	stealthOffer(t, true, "", nil)
	if got, code, log := stealthRun(t, false, ""); got.Action != "current" || code != 0 || log != "" {
		t.Errorf("rerun: %s, exit %d:\n%s", got.Action, code, log)
	}
	got, code, log := stealthRun(t, true, "")
	if got.Action != "current" || code != 0 || !strings.Contains(log, "The itos rules for agents are in place already, in ") {
		t.Errorf("--agent-rules: %s, exit %d:\n%s", got.Action, code, log)
	}
}

// A folder it cannot make, a file it cannot write, and an exclude it cannot
// list fail the write, or are said.
func TestStealthRulesOfferWhereItCannotWrite(t *testing.T) {
	t.Run("the git folder locked", func(t *testing.T) {
		dir := initScratch(t, "commit", nil)
		readOnly(t, filepath.Join(dir, ".git"))
		if got, code, log := stealthRun(t, true, ""); got.Action != "failed" || code != ExitPolicy || !strings.Contains(log, "not written") {
			t.Errorf("%s, exit %d:\n%s", got.Action, code, log)
		}
	})
	t.Run("itos's folder locked", func(t *testing.T) {
		dir := initScratch(t, "commit", nil)
		if err := os.MkdirAll(filepath.Join(dir, ".git", "itos"), 0o755); err != nil {
			t.Fatal(err)
		}
		readOnly(t, filepath.Join(dir, ".git", "itos"))
		if got, code, log := stealthRun(t, true, ""); got.Action != "failed" || code != ExitPolicy || !strings.Contains(log, "not written") {
			t.Errorf("%s, exit %d:\n%s", got.Action, code, log)
		}
	})
	t.Run("git status failing", func(t *testing.T) {
		initScratch(t, "commit", nil)
		useWrappedGit(t, "status", "", "")
		got, code, log := stealthRun(t, true, "")
		if got.Action != "written" || code != 0 || !strings.Contains(log, "Could not list CLAUDE.local.md in the git folder's info/exclude") {
			t.Errorf("%s, exit %d:\n%s", got.Action, code, log)
		}
	})
}
