package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/vcs/port"
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
	if code, _, stderr := run("draft", "add", "x", "-m", "docs: x", "a"); code != ExitSoftware || !strings.Contains(stderr, "no work tree") {
		t.Errorf("draft add in the git folder: exit %d, stderr %q", code, stderr)
	}
}
