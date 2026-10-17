package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

// A folder's place on the PATH, and the real git's, are each the first
// folder that is it: a folder named twice keeps its first place. With no
// git on the PATH there is no real git's place, even for the folder the
// test stands in.
func TestPathStanding(t *testing.T) {
	real, err := git.Real()
	if err != nil {
		t.Skip("no git on the PATH")
	}
	gitDir, link := filepath.Dir(real), t.TempDir()
	t.Setenv("PATH", strings.Join([]string{gitDir, link, gitDir, link}, string(os.PathListSeparator)))
	if at, realGit, realAt := pathStanding(link); at != 1 || realGit != real || realAt != 0 {
		t.Errorf("pathStanding = %d, %q, %d", at, realGit, realAt)
	}
	t.Chdir(link)
	t.Setenv("PATH", link)
	if at, realGit, realAt := pathStanding(link); at != 0 || realGit != "" || realAt != -1 {
		t.Errorf("with no git: pathStanding = %d, %q, %d", at, realGit, realAt)
	}
}
