package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func registryWriterRepo(t *testing.T, registry string) string {
	t.Helper()
	dir := gitConfigRepo(t, "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\nwork:\n  registry: registry.yaml\n")
	standInHooks(t)
	gitIn(t, "config", "user.name", "itos test")
	gitIn(t, "config", "user.email", "test@localhost")
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "add", "--", "itos.yaml", "registry.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: start")
	return dir
}

// work add's minting through the git the counter is kept by: a git that
// fails is the error, exit 70 (bug 119).
func TestWorkAddPropagatesIDReservationFailures(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems: []\n")
	useFailingGit(t)
	code, err := runWork([]string{"add", "--kind", "slice", "--phase", "1", "--title", "Slice", "--why", "Because."}, Out{Stderr: &strings.Builder{}})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "Command failed: git rev-parse") {
		t.Fatalf("work add = (%d, %v), want exit code 0 and the injected Git command failure", code, err)
	}
	if got := ExitCode(err); got != ExitSoftware {
		t.Fatalf("work add's reservation failure exits %d, want %d (bug 119)", got, ExitSoftware)
	}
}
