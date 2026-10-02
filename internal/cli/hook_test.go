package cli

import (
	"slices"
	"testing"
)

// A hook file is a shim when its one line, besides comments and a shebang,
// calls itos's hook.
func TestIsShim(t *testing.T) {
	for text, want := range map[string]bool{
		"exec tools/bin/itos hook commit-msg \"$1\"\n":             true,
		"#!/bin/sh\n# by hand\n\nexec itos hook pre-push \"$@\"\n": true,
		"exec tools/bin/itos hook commit-msg \"$1\"\necho more\n":  false,
		"vp exec commitlint --edit \"$1\"\n":                       false,
		"exec tools/bin/notitos hook commit-msg\n":                 false,
		"exec tools/bin/itos hook commit-msgs \"$1\"\n":            false,
		"   \n# nothing\n": false,
		"exec tools/bin/itos hook pre-push \"$@\" # itos hook commit-msg": true,
	} {
		if got := isShim(text); got != want {
			t.Errorf("isShim(%q) = %v", text, got)
		}
	}
}

// A deleted branch runs nothing, a new one the whole run; neither asks git.
func TestPushBases(t *testing.T) {
	zero := "0000000000000000000000000000000000000000"
	bases, whole := pushBases("(delete) " + zero + " refs/heads/old abc\n\n")
	if len(bases) != 0 || whole {
		t.Errorf("deleted branch: %q %v", bases, whole)
	}
	bases, whole = pushBases("refs/heads/new abc refs/heads/new " + zero + "\n")
	if len(bases) != 0 || !whole {
		t.Errorf("new branch: %q %v", bases, whole)
	}
}

// A staged range command's problem lines are its `  - …` lines, read as
// JavaScript's /^\s+- (.*\S)/ reads them.
func TestProblemLine(t *testing.T) {
	var got []string
	for _, l := range []string{"  - one  ", "- not indented", "\t- two\r", "  -   ", "    - three - more"} {
		if m := problemLine.FindStringSubmatch(l); m != nil {
			got = append(got, m[1])
		}
	}
	if want := []string{"one", "two", "three - more"}; !slices.Equal(got, want) {
		t.Errorf("problem lines: %q", got)
	}
}
