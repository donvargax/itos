package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

func idMintTestConfig(t *testing.T, ledger string) (*config.Loaded, string) {
	t.Helper()
	dir := gitConfigRepo(t, "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n")
	path := filepath.Join(dir, "tasks/phase-1.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile("", filepath.Join(dir, "itos.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg, dir
}

// mintItemID reserves through the repository's counter, in its git folder
// when there is no remote; the engine's tests (internal/engine/ids) mint
// over the tracker's fake, the adapter's reserve in scratch repositories.
func TestMintItemIDUsesTheLedgerAndCounterFloor(t *testing.T) {
	cfg, dir := idMintTestConfig(t, "- { id: T-009, type: chore, title: Nine }\n")
	registryIDs := []string{"T-010", "not-a-task-id"}
	first, err := mintItemID(cfg, "task", registryIDs, runtime.GOOS)
	if err != nil || first != "T-011" {
		t.Fatalf("first task mint = %q, %v, want T-011", first, err)
	}
	second, err := mintItemID(cfg, "task", registryIDs, runtime.GOOS)
	if err != nil || second != "T-012" {
		t.Fatalf("second task mint = %q, %v, want T-012", second, err)
	}
	if _, err := mintItemID(cfg, "idea", registryIDs, runtime.GOOS); err == nil {
		t.Fatal("mintItemID accepted idea, which keeps its caller-provided slug")
	}
	text, err := os.ReadFile(filepath.Join(dir, ".git", "itos", "ids.yaml"))
	if err != nil || !strings.Contains(string(text), "task: 12") {
		t.Fatalf("local counter = %q (%v), want task 12", text, err)
	}
}
