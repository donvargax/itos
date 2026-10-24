package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

func TestHookCaptureIndexOutsideRepository(t *testing.T) {
	dir := scratchFolder(t)
	source, err := (hookPorts{Git: vcs.Git{Dir: dir}}).CaptureIndex(t.Context())
	if source != nil || ExitCode(err) != ExitMissing {
		t.Fatalf("capture outside repository = %v, %v", source, err)
	}
}

func TestHookCaptureAndCheckoutUseCapturedTree(t *testing.T) {
	dir := scratchFolder(t)
	gitIn(t, "init", "-q")
	writeFiles(t, map[string]string{"input": "staged"})
	gitIn(t, "add", "--", "input")
	p := hookPorts{Git: vcs.Git{Dir: dir}}
	source, err := p.CaptureIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, map[string]string{"input": "later"})
	gitIn(t, "add", "--", "input")
	text, err := source.Read(t.Context(), "input")
	if err != nil || text != "staged" {
		t.Fatalf("captured input = %q, %v", text, err)
	}
	snapshot, err := p.TaskSnapshot(t.Context(), source.Tree(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Remove()
	data, err := os.ReadFile(filepath.Join(snapshot.Dir, "input"))
	if err != nil || string(data) != "staged" {
		t.Fatalf("checkout input = %q, %v", data, err)
	}
	if snapshot.TempDir != filepath.Dir(snapshot.Dir) || snapshot.Env == nil {
		t.Errorf("checkout binding = %+v", snapshot)
	}
	if err := snapshot.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(snapshot.TempDir); !os.IsNotExist(err) {
		t.Errorf("checkout resources remain: %v", err)
	}
}

// A check that runs in the working folder gets the hook's environment less
// what belongs to this commit alone: the index git made it from, the footers
// and the amend itos commit handed the hook.
func TestHookCheckEnvIsTheHooksLessTheCommitsOwn(t *testing.T) {
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "index"))
	t.Setenv(AmendEnv, "1")
	t.Setenv("ITOS_CHECK_ENV_KEPT", "kept")
	env := (hookPorts{}).CheckEnv(t.Context())
	got := map[string]bool{}
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		got[name] = true
	}
	if got["GIT_INDEX_FILE"] || got[AmendEnv] || !got["ITOS_CHECK_ENV_KEPT"] {
		t.Errorf("the check's environment: %v", got)
	}
}
