package cli

import "testing"

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
