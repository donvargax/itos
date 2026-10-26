package adapter

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// A path a tree lacks is told by git's words, as git show says them for a
// commit and for the index; any other failure is no missing path.
func TestMissingPath(t *testing.T) {
	for words, want := range map[string]bool{
		"fatal: path 'counters/slice' does not exist in 'abc'\n": true,
		"fatal: path 'x' exists on disk, but not in 'HEAD'\n":    true,
		"object does not exist in tree":                          true,
		"fatal: invalid object name 'nope'.\n":                   false,
		"permission denied":                                      false,
	} {
		if got := missing(words); got != want {
			t.Errorf("missing(%q) = %v", words, got)
		}
	}
}

// A failure of a fetch, a push or an ls-remote is read from git's words: a
// refusal before a remote out of reach, then one that is no repository.
func TestFailureOfGitsWords(t *testing.T) {
	for words, want := range map[string]port.Failure{
		" ! [rejected] main -> main (fetch first)\nerror: failed to push some refs to 'o'\n": port.Refused,
		"fatal: unable to access 'x': Could not resolve host: x\n":                           port.Unreachable,
		"fatal: 'nowhere' does not appear to be a git repository\n":                          port.NoRepository,
		"fatal: the remote end hung up unexpectedly\n":                                       port.Other,
	} {
		if got := failure(words); got != want {
			t.Errorf("failure(%q) = %v; want %v", words, got, want)
		}
	}
}

// A failed command's error says the command, its exit code and its words.
func TestFailedSaysItAll(t *testing.T) {
	err := error(&port.Failed{Command: "git push origin x", Code: 1, Words: "refused\n"})
	if got := err.Error(); got != "git push origin x: exit status 1: refused" {
		t.Errorf("Error() = %q", got)
	}
	var failed *port.Failed
	if !errors.As(err, &failed) || !strings.Contains(failed.Words, "refused") {
		t.Errorf("errors.As: %v", failed)
	}
}

// A talk whose one writer takes both streams still lets output read git's
// stdout; the streams share a pipe only for a run with no out to fill.
func TestOutputIsKeptBesideASharedWriter(t *testing.T) {
	var shared bytes.Buffer
	talk := port.Talk{Stdout: &shared, Stderr: &shared}
	got, err := Git{}.output(context.Background(), talk, false, "--version")
	if err != nil || !strings.HasPrefix(got, "git version") {
		t.Fatalf("output = %q, %v", got, err)
	}
	if shared.String() != got {
		t.Errorf("the shared writer got %q, want %q", shared.String(), got)
	}
}
