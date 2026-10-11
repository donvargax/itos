package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stealthOf is the stealth rules of an AGENTS.override.md's text ("" and
// absent for none) and a project's AGENTS.md (absent for none), as parsed.
func stealthOf(t *testing.T, override string, overrideThere bool, agents string, agentsThere bool) *stealthRules {
	t.Helper()
	s := &stealthRules{importPath: ".git/itos/AGENTS.md", agents: agents, hasAgents: agentsThere}
	s.rules = stealthTarget{path: s.importPath}
	s.claude = stealthTarget{path: claudeLocalFile, top: true}
	s.override = stealthTarget{path: overrideFile, text: override, exists: overrideThere, top: true}
	if err := s.parse(); err != nil {
		t.Fatal(err)
	}
	return s
}

const testBlock = rulesBegin + "\n\nnew\n\n" + rulesEnd + "\n"

func TestStealthOverrideIsWrittenWithTheCopyThenTheRules(t *testing.T) {
	s := stealthOf(t, "", false, "Be kind.\n", true)
	s.plan(testBlock)
	want := copyBegin + "\n\nBe kind.\n\n" + copyEnd + "\n\n" + testBlock
	if s.override.next != want {
		t.Errorf("AGENTS.override.md:\n%q\nwant\n%q", s.override.next, want)
	}
	if want := rulesBegin + "\n\n@AGENTS.md\n\n@.git/itos/AGENTS.md\n\n" + rulesEnd + "\n"; s.claude.next != want {
		t.Errorf("CLAUDE.local.md:\n%q\nwant\n%q", s.claude.next, want)
	}

	// No AGENTS.md: the rules alone, and no import of it.
	s = stealthOf(t, "", false, "", false)
	s.plan(testBlock)
	if s.override.next != testBlock || s.claude.next != rulesBegin+"\n\n@.git/itos/AGENTS.md\n\n"+rulesEnd+"\n" {
		t.Errorf("with no AGENTS.md: %q, %q", s.override.next, s.claude.next)
	}
}

// A file of the person's gains the rules at its end, never the copy; one
// that holds only itos's rules is itos's, and gains the copy once the project
// has an AGENTS.md.
func TestStealthOverrideOfThePersonsGainsTheRulesAlone(t *testing.T) {
	s := stealthOf(t, "Mine.\n", true, "Be kind.\n", true)
	s.plan(testBlock)
	if s.override.next != "Mine.\n\n"+testBlock {
		t.Errorf("the person's: %q", s.override.next)
	}
	s = stealthOf(t, "\n"+rulesBegin+"\nold\n"+rulesEnd+"\n", true, "Be kind.\n", true)
	s.plan(testBlock)
	if want := copyBegin + "\n\nBe kind.\n\n" + copyEnd + "\n\n" + testBlock; s.override.next != want {
		t.Errorf("itos's alone:\n%q\nwant\n%q", s.override.next, want)
	}
}

// A rerun rewrites only between the markers, the copy and the rules each in
// place, the text around them kept; the project's AGENTS.md gone, the copy
// goes too. Markers of the rules inside the copy are the copy's text.
func TestStealthOverrideRewritesBetweenTheMarkers(t *testing.T) {
	old := copyBegin + "\n\nOld.\n" + rulesBegin + "\nNot mine.\n" + rulesEnd + "\n\n" + copyEnd + "\n\nBetween.\n\n" +
		rulesBegin + "\nold\n" + rulesEnd + "\nAfter.\n"
	s := stealthOf(t, old, true, "New.\n", true)
	s.plan(testBlock)
	want := copyBegin + "\n\nNew.\n\n" + copyEnd + "\n\nBetween.\n\n" + testBlock + "After.\n"
	if s.override.next != want {
		t.Errorf("rewritten:\n%q\nwant\n%q", s.override.next, want)
	}
	s = stealthOf(t, copyBegin+"\n\nOld.\n\n"+copyEnd+"\n\n"+rulesBegin+"\nold\n"+rulesEnd+"\n", true, "", false)
	s.plan(testBlock)
	if s.override.next != testBlock {
		t.Errorf("with AGENTS.md gone: %q", s.override.next)
	}
}

func TestStealthMarkersOutOfOrderAreRefused(t *testing.T) {
	s := &stealthRules{override: stealthTarget{path: overrideFile, text: copyBegin + "\nMine.\n", exists: true}}
	s.rules.path, s.claude.path = ".git/itos/AGENTS.md", claudeLocalFile
	if err := s.parse(); err == nil {
		t.Error("a copy with no end marker: no error")
	}
}

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
