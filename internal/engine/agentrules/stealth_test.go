package agentrules

import (
	"os"
	"testing"
)

// stealthOf is the stealth rules of an AGENTS.override.md's text ("" and
// absent for none) and a project's AGENTS.md (absent for none), as parsed.
func stealthOf(t *testing.T, override string, overrideThere bool, agents string, agentsThere bool) *Stealth {
	t.Helper()
	s := &Stealth{ImportPath: ".git/itos/AGENTS.md", agents: agents, hasAgents: agentsThere}
	s.rules = Target{Path: s.ImportPath}
	s.claude = Target{Path: ClaudeLocalFile, Top: true}
	s.override = Target{Path: OverrideFile, Text: override, Exists: overrideThere, Top: true}
	if err := s.parse(); err != nil {
		t.Fatal(err)
	}
	return s
}

const testBlock = Begin + "\n\nnew\n\n" + End + "\n"

func TestStealthOverrideIsWrittenWithTheCopyThenTheRules(t *testing.T) {
	s := stealthOf(t, "", false, "Be kind.\n", true)
	s.Plan(testBlock)
	want := CopyBegin + "\n\nBe kind.\n\n" + CopyEnd + "\n\n" + testBlock
	if s.override.Next != want {
		t.Errorf("AGENTS.override.md:\n%q\nwant\n%q", s.override.Next, want)
	}
	if want := Begin + "\n\n@AGENTS.md\n\n@.git/itos/AGENTS.md\n\n" + End + "\n"; s.claude.Next != want {
		t.Errorf("CLAUDE.local.md:\n%q\nwant\n%q", s.claude.Next, want)
	}

	// No AGENTS.md: the rules alone, and no import of it.
	s = stealthOf(t, "", false, "", false)
	s.Plan(testBlock)
	if s.override.Next != testBlock || s.claude.Next != Begin+"\n\n@.git/itos/AGENTS.md\n\n"+End+"\n" {
		t.Errorf("with no AGENTS.md: %q, %q", s.override.Next, s.claude.Next)
	}
}

// A file of the person's gains the rules at its end, never the copy; one
// that holds only itos's rules is itos's, and gains the copy once the project
// has an AGENTS.md.
func TestStealthOverrideOfThePersonsGainsTheRulesAlone(t *testing.T) {
	s := stealthOf(t, "Mine.\n", true, "Be kind.\n", true)
	s.Plan(testBlock)
	if s.override.Next != "Mine.\n\n"+testBlock {
		t.Errorf("the person's: %q", s.override.Next)
	}
	s = stealthOf(t, "\n"+Begin+"\nold\n"+End+"\n", true, "Be kind.\n", true)
	s.Plan(testBlock)
	if want := CopyBegin + "\n\nBe kind.\n\n" + CopyEnd + "\n\n" + testBlock; s.override.Next != want {
		t.Errorf("itos's alone:\n%q\nwant\n%q", s.override.Next, want)
	}
}

// A rerun rewrites only between the markers, the copy and the rules each in
// place, the text around them kept; the project's AGENTS.md gone, the copy
// goes too. Markers of the rules inside the copy are the copy's text.
func TestStealthOverrideRewritesBetweenTheMarkers(t *testing.T) {
	old := CopyBegin + "\n\nOld.\n" + Begin + "\nNot mine.\n" + End + "\n\n" + CopyEnd + "\n\nBetween.\n\n" +
		Begin + "\nold\n" + End + "\nAfter.\n"
	s := stealthOf(t, old, true, "New.\n", true)
	s.Plan(testBlock)
	want := CopyBegin + "\n\nNew.\n\n" + CopyEnd + "\n\nBetween.\n\n" + testBlock + "After.\n"
	if s.override.Next != want {
		t.Errorf("rewritten:\n%q\nwant\n%q", s.override.Next, want)
	}
	s = stealthOf(t, CopyBegin+"\n\nOld.\n\n"+CopyEnd+"\n\n"+Begin+"\nold\n"+End+"\n", true, "", false)
	s.Plan(testBlock)
	if s.override.Next != testBlock {
		t.Errorf("with AGENTS.md gone: %q", s.override.Next)
	}
}

// An override of itos's holding the copy and no rules block gains the
// block after the copy.
func TestStealthOverrideWithTheCopyAloneGainsTheRules(t *testing.T) {
	copied := CopyBegin + "\n\nBe kind.\n\n" + CopyEnd + "\n"
	s := stealthOf(t, copied, true, "Be kind.\n", true)
	s.Plan(testBlock)
	if want := copied + "\n" + testBlock; s.override.Next != want {
		t.Errorf("AGENTS.override.md:\n%q\nwant\n%q", s.override.Next, want)
	}
}

// Any of the files that cannot be read fails the reading; a folder is a
// file no system reads.
func TestReadStealthFailsOnAFileItCannotRead(t *testing.T) {
	for _, folder := range []string{".git/itos/AGENTS.md", ClaudeLocalFile, OverrideFile} {
		t.Run(folder, func(t *testing.T) {
			v := inRepository(t)
			if err := os.MkdirAll(folder, 0o755); err != nil {
				t.Fatal(err)
			}
			if s, err := ReadStealth(t.Context(), v); err == nil {
				t.Errorf("read: %+v", s)
			}
		})
	}
}

func TestStealthMarkersOutOfOrderAreRefused(t *testing.T) {
	s := &Stealth{override: Target{Path: OverrideFile, Text: CopyBegin + "\nMine.\n", Exists: true}}
	s.rules.Path, s.claude.Path = ".git/itos/AGENTS.md", ClaudeLocalFile
	if err := s.parse(); err == nil {
		t.Error("a copy with no end marker: no error")
	}
}

// Found is whether any file holds a block of itos's, and CopyStale whether
// the override's copy of AGENTS.md no longer matches it.
func TestStealthFoundAndCopyStale(t *testing.T) {
	copied := CopyBegin + "\n\nBe kind.\n\n" + CopyEnd + "\n"
	for _, c := range []struct {
		name, override, agents string
		found, copyStale       bool
	}{
		{"nothing", "", "Be kind.\n", false, false},
		{"the rules alone", testBlock, "Be kind.\n", true, false},
		{"the copy current", copied, "Be kind.\n", true, false},
		{"the copy stale", copied, "Be fair.\n", true, true},
	} {
		s := stealthOf(t, c.override, c.override != "", c.agents, true)
		if s.Found() != c.found || s.CopyStale() != c.copyStale {
			t.Errorf("%s: found %v, copy stale %v", c.name, s.Found(), s.CopyStale())
		}
	}
	s := stealthOf(t, "", false, "", false)
	s.claude.found = true
	if !s.Found() {
		t.Error("a CLAUDE.local.md's block: not found")
	}
	s = stealthOf(t, "", false, "", false)
	s.rules.found = true
	if !s.Found() {
		t.Error("the rules file's block: not found")
	}
}
