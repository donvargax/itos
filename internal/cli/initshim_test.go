package cli

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

// A shim linked in a folder the PATH names after the real git's, the
// first, is on the PATH but not before git.
func TestShimOfferLinkedAfterTheFirstGit(t *testing.T) {
	real, err := git.Real()
	if err != nil {
		t.Skip("no git on the PATH")
	}
	dir := t.TempDir()
	t.Setenv("PATH", strings.Join([]string{filepath.Dir(real), dir}, string(os.PathListSeparator)))
	s := shimOffer{flag: shimFlag{given: true, install: true, dir: dir}, log: io.Discard, goos: runtime.GOOS}
	got, code := s.run()
	if code != 0 || got.Action != "linked" || !got.OnPath || got.BeforeGit {
		t.Errorf("exit %d, %+v", code, got)
	}
}
