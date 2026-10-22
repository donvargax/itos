package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// Draft's paths need a work tree: in the git folder there is none.
func TestTopPathsInTheGitFolder(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\n")
	t.Chdir(filepath.Join(dir, ".git"))
	if code, _, stderr := run("draft", "add", "x", "-m", "docs: x", "a"); code != ExitSoftware || !strings.Contains(stderr, "no work tree") {
		t.Errorf("draft add in the git folder: exit %d, stderr %q", code, stderr)
	}
}
