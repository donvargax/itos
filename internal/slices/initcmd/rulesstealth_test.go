package initcmd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// stealthOffer makes the offer under a stealth config in a scratch
// repository holding the files, those named in tracked in the index, git
// showing the rest untracked, the terminal answering answers when there
// are any, and gives its outcome, its exit code and what it said, and the
// ports.
func stealthOffer(t *testing.T, write bool, answers string, files map[string]string, tracked ...string) (rulesOutcome, int,
	string, *fake) {
	t.Helper()
	f := scratch(t, "commit", files)
	f.Work = map[string]string{claudeLocalFile: "", overrideFile: ""}
	if len(tracked) > 0 {
		f.Index = map[string]string{}
		for _, p := range tracked {
			f.Index[p] = files[p]
		}
	}
	got, code, said := stealthRun(f, write, answers)
	return got, code, said, f
}

// stealthRun makes the offer under a stealth config over f.
func stealthRun(f *fake, write bool, answers string) (rulesOutcome, int, string) {
	var log strings.Builder
	r := rulesOffer{flag: RulesFlag{Given: write, Write: write}, stealth: true, rerun: !write && answers == "", log: &log,
		answers: strings.NewReader(answers), ask: answers != "", ports: f}
	got, code := r.run(context.Background())
	return got, code, log.String()
}

// The offer under a stealth config, each way it ends.
func TestStealthRulesOfferOutcomes(t *testing.T) {
	stale := map[string]string{".git/itos/AGENTS.md": rulesBegin + "\nOld.\n" + rulesEnd + "\n"}
	misplaced := map[string]string{overrideFile: rulesEnd + "\n"}
	copied := map[string]string{agentsFile: "Ours.\n", overrideFile: copyBegin + "\n\nOld.\n\n" + copyEnd + "\n\n" + rulesBlock}
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
		{"misplaced markers, asked", true, "", misplaced, nil, "failed", exitPolicy, "The itos rules for agents are not written: "},
		{"a no at the terminal", false, "n\n", nil, nil, "declined", 0, "Write itos's rules for agents into .git/itos/AGENTS.md"},
		{"an older itos's block", false, "", stale, nil, "stale", 0, "are not the ones this itos writes"},
		{"only CLAUDE.local.md", false, "", map[string]string{claudeLocalFile: rulesBegin + "\n" + rulesEnd + "\n"}, nil,
			"stale", 0, "are not all in place here (.git/itos/AGENTS.md, CLAUDE.local.md, AGENTS.override.md)"},
		{"a stale copy", false, "", copied, nil, "stale", 0,
			"AGENTS.override.md's copy of AGENTS.md no longer matches it: itos init --agent-rules rewrites it, only between the markers.\n"},
		{"a tracked CLAUDE.local.md", true, "", map[string]string{claudeLocalFile: "Ours.\n"}, []string{claudeLocalFile},
			"failed", exitPolicy, "CLAUDE.local.md is tracked by the project"},
		{"the project's AGENTS.md", true, "", map[string]string{agentsFile: "Ours.\n"}, nil, "written", 0,
			", after a copy of AGENTS.md between <!-- itos:agents-md:begin --> and <!-- itos:agents-md:end -->.\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, code, log, _ := stealthOffer(t, c.write, c.answers, c.files, c.tracked...)
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
	f := scratch(t, "commit", nil)
	for _, rerun := range []bool{false, true} {
		var log strings.Builder
		r := rulesOffer{stealth: true, rerun: rerun, log: &log, answers: strings.NewReader(""), ports: f}
		got, code := r.run(context.Background())
		if got.Action != "offered" || code != 0 || (log.String() == stealthUnasked) == rerun {
			t.Errorf("rerun %v: %s, exit %d:\n%s", rerun, got.Action, code, log.String())
		}
	}
}

// Written once, each file listed in info/exclude, the rules are current on
// a rerun with or without --agent-rules.
func TestStealthRulesOfferCurrent(t *testing.T) {
	got, _, said, f := stealthOffer(t, true, "", nil)
	if got.Action != "written" || len(got.Files) != 3 || read(filepath.Join(".git", "info", "exclude")) !=
		"/"+claudeLocalFile+"\n/"+overrideFile+"\n" ||
		!strings.Contains(said, "Listed CLAUDE.local.md and AGENTS.override.md in the git folder's info/exclude") {
		t.Errorf("%+v:\n%s", got, said)
	}
	if got, code, log := stealthRun(f, false, ""); got.Action != "current" || code != 0 || log != "" {
		t.Errorf("rerun: %s, exit %d:\n%s", got.Action, code, log)
	}
	got, code, log := stealthRun(f, true, "")
	if got.Action != "current" || code != 0 || !strings.Contains(log, "The itos rules for agents are in place already, in ") {
		t.Errorf("--agent-rules: %s, exit %d:\n%s", got.Action, code, log)
	}
}

// A folder it cannot make, a file it cannot write, and an exclude it cannot
// list fail the write, or are said.
func TestStealthRulesOfferWhereItCannotWrite(t *testing.T) {
	failed := errors.New("injected failure")
	rules := filepath.Join(".git", "itos", "AGENTS.md")
	t.Run("itos's folder", func(t *testing.T) {
		f := scratch(t, "commit", nil)
		f.mkdirErrs = map[string]error{filepath.Dir(rules): failed}
		if got, code, log := stealthRun(f, true, ""); got.Action != "failed" || code != exitPolicy ||
			!strings.Contains(log, "not written: injected failure") {
			t.Errorf("%s, exit %d:\n%s", got.Action, code, log)
		}
	})
	t.Run("the rules file", func(t *testing.T) {
		f := scratch(t, "commit", nil)
		f.writeErrs = map[string]error{rules: failed}
		if got, code, log := stealthRun(f, true, ""); got.Action != "failed" || code != exitPolicy ||
			!strings.Contains(log, "not written: injected failure") {
			t.Errorf("%s, exit %d:\n%s", got.Action, code, log)
		}
	})
	t.Run("the exclude", func(t *testing.T) {
		got, code, log, _ := stealthOffer(t, true, "", map[string]string{".git/info/exclude/x": ""})
		if got.Action != "written" || code != 0 || !strings.Contains(log, "Could not list CLAUDE.local.md in the git folder's info/exclude") {
			t.Errorf("%s, exit %d:\n%s", got.Action, code, log)
		}
	})
}
