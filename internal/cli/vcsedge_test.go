package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// A git that failed exits as it did, one a signal ended 1; one that did not
// start is the error, with no code.
func TestExitOf(t *testing.T) {
	plain := errors.New("no git")
	for _, c := range []struct {
		err  error
		code int
		want error
	}{
		{nil, 0, nil},
		{&port.Failed{Code: 2}, 2, nil},
		{&port.Failed{Code: -1}, ExitPolicy, nil},
		{plain, 0, plain},
	} {
		if code, err := exitOf(c.err); code != c.code || err != c.want {
			t.Errorf("exitOf(%v) = %d, %v", c.err, code, err)
		}
	}
}

// A remote's failure is its kind, and its words what git said; an error
// that is no git's failure has none of either.
func TestRemoteKindAndWords(t *testing.T) {
	for failure, want := range map[port.Failure]kind.Kind{
		port.Unreachable: kind.Temporary, port.NoRepository: kind.Missing, port.Refused: kind.Unknown, port.Other: kind.Unknown,
	} {
		if got := remoteKind(&port.Failed{Failure: failure}); got != want {
			t.Errorf("remoteKind(%v) = %v", failure, got)
		}
	}
	plain := errors.New("no git")
	if remoteKind(plain) != kind.Unknown || words(plain) != "" || words(&port.Failed{Words: "said\n"}) != "said\n" {
		t.Errorf("remoteKind or words of an error git did not give")
	}
}

// refusedNote is the fake, its notes refused.
type refusedNote struct{ *porttest.VCS }

func (refusedNote) Note(context.Context, string, string) error {
	return &port.Failed{Command: "git notes", Code: 1, Words: "refused\n"}
}

// The note of a commit just made that cannot be written says git's words;
// none is written when HEAD did not move.
func TestWriteNote(t *testing.T) {
	v := refusedNote{&porttest.VCS{History: []porttest.Commit{{Name: "c-a"}}, Named: map[string]string{"HEAD": "c-a"}}}
	if err := writeNote(v, "", []string{"Task: T-1"}); err == nil || err.Error() != "git notes add: refused" {
		t.Errorf("writeNote = %v", err)
	}
	if err := writeNote(v, "c-a", []string{"Task: T-1"}); err != nil {
		t.Errorf("writeNote with HEAD as it was = %v", err)
	}
	if err := writeNote(refusedNote{&porttest.VCS{}}, "", nil); err == nil {
		t.Error("writeNote with no HEAD: no error")
	}
}

// The way up to the top is a ../ a folder, none at the top or in the git
// folder; outside a repository there is none to give.
func TestUpToTop(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\n")
	for folder, want := range map[string]string{"a/b": "../../", ".git": "", ".": ""} {
		if err := os.MkdirAll(filepath.Join(dir, folder), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(filepath.Join(dir, folder))
		if got, ok := upToTop(); got != want || !ok {
			t.Errorf("upToTop from %s = %q, %v; want %q", folder, got, ok, want)
		}
	}
	t.Chdir(t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	if got, ok := upToTop(); ok {
		t.Errorf("upToTop outside a repository = %q", got)
	}
}

// Draft's paths need a work tree: in the git folder there is none.
func TestTopPathsInTheGitFolder(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\n")
	t.Chdir(filepath.Join(dir, ".git"))
	if _, err := topPaths("draft add", []string{"a"}, ""); err == nil || !strings.Contains(err.Error(), "no work tree") {
		t.Errorf("topPaths in the git folder: %v", err)
	}
}
