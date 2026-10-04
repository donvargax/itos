package cli

import (
	"slices"
	"testing"

	"github.com/donvargax/itos/v3/internal/git"
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

// A deleted branch sends nothing, so it runs nothing; a new one is judged by
// the commits on no remote branch and runs the whole run; neither asks git.
func TestPushedRefs(t *testing.T) {
	zero := "0000000000000000000000000000000000000000"
	refs := pushedRefs("(delete) " + zero + " refs/heads/old abc\n\n")
	bases, whole := pushBases(refs)
	if len(refs) != 0 || len(bases) != 0 || whole {
		t.Errorf("deleted branch: %v %q %v", refs, bases, whole)
	}
	refs = pushedRefs("refs/heads/new abc refs/heads/new " + zero + "\n")
	bases, whole = pushBases(refs)
	if len(bases) != 0 || !whole {
		t.Errorf("new branch: %q %v", bases, whole)
	}
	if len(refs) != 1 || refs[0].pushedRange() != (span{git.Unpushed, "abc"}) {
		t.Errorf("new branch's range: %v", refs)
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

// An amend is --amend among git commit's options, never a value of one, nor
// a path after --.
func TestAmends(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"--amend", "--no-edit"}, true},
		{[]string{"-m", "x", "--amend"}, true},
		{[]string{"-am", "x", "--amend"}, true},
		{[]string{"-m", "--amend"}, false},
		{[]string{"-m--amend"}, false},
		{[]string{"--message", "--amend"}, false},
		{[]string{"--message=--amend"}, false},
		{[]string{"-C", "--amend"}, false},
		{[]string{"-q", "--", "--amend"}, false},
		{[]string{"--amend", "--no-amend"}, false},
		{[]string{"-m", "x"}, false},
	} {
		if got := readGitArgs(c.args).amend; got != c.want {
			t.Errorf("readGitArgs(%q).amend = %v", c.args, got)
		}
	}
}

// A decision record or the index, in the folder work.decisions names, is
// itos's data the hook checks when staged (slice 74); a file beside them
// that is neither, or one in a folder below, is not.
func TestStagesDecisions(t *testing.T) {
	notesRepo(t)
	writeFile(t, "itos.yaml", "version: 1\ncommits: { types: [docs] }\nwork: { registry: work-items.yaml }\n")
	for staged, want := range map[string]bool{
		"docs/decisions/0001-use-go.md":     true,
		"docs/decisions/README.md":          true,
		"docs/decisions/notes.txt":          false,
		"docs/decisions/old/0001-use-go.md": false,
		"docs/0001-use-go.md":               false,
	} {
		if got := stagesData([]string{staged}); got != want {
			t.Errorf("stagesData(%q) = %v", staged, got)
		}
	}
}
