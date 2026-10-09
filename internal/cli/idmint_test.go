package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
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
	cfg, err := config.Load(filepath.Join(dir, "itos.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg, dir
}

func TestMintItemIDUsesTheLedgerAndCounterFloor(t *testing.T) {
	cfg, _ := idMintTestConfig(t, "- { id: T-009, type: chore, title: Nine }\n")
	registryIDs := []string{"T-010", "not-a-task-id"}
	first, err := mintItemID(cfg, "task", registryIDs)
	if err != nil || first != "T-011" {
		t.Fatalf("first task mint = %q, %v, want T-011", first, err)
	}
	second, err := mintItemID(cfg, "task", registryIDs)
	if err != nil || second != "T-012" {
		t.Fatalf("second task mint = %q, %v, want T-012", second, err)
	}
	if _, err := mintItemID(cfg, "idea", registryIDs); err == nil {
		t.Fatal("mintItemID accepted idea, which keeps its caller-provided slug")
	}
}

func TestSliceIDsReadsFeatureTagsAndIgnoresOtherFiles(t *testing.T) {
	cfg, dir := idMintTestConfig(t, "[]\n")
	features := filepath.Join(dir, "features")
	if err := os.MkdirAll(filepath.Join(features, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(features, "one.feature"), []byte("@phase-1\nFeature: One\n  @ID-ONE-01 @slice-007 @slice-bad\n  Scenario: One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(features, "nested/two.feature"), []byte("Feature: Two\n  @slice-010\n  Scenario: Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(features, "notes.txt"), []byte("@slice-999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ids, err := sliceIDs(cfg, []string{"slice-009"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"slice-007", "slice-bad", "slice-010", "slice-009"} {
		if !slices.Contains(ids, want) {
			t.Errorf("sliceIDs = %v, missing %q", ids, want)
		}
	}
	if slices.Contains(ids, "slice-999") {
		t.Fatalf("sliceIDs read a non-feature file: %v", ids)
	}
	got, err := mintItemID(cfg, "slice", []string{"slice-009"})
	if err != nil || got != "slice-011" {
		t.Fatalf("slice mint = %q, %v, want slice-011", got, err)
	}
}

func TestReserveItemNumberUsesLocalFallbackAndPropagatesGitErrors(t *testing.T) {
	t.Run("no remote uses common-dir counter", func(t *testing.T) {
		cfg, dir := idMintTestConfig(t, "[]\n")
		first, err := reserveItemNumber(cfg, "slice", 9)
		if err != nil || first != 10 {
			t.Fatalf("reserveItemNumber = (%d, %v), want (10, nil)", first, err)
		}
		counter := filepath.Join(dir, ".git", "itos", "ids.yaml")
		text, err := os.ReadFile(counter)
		if err != nil || !strings.Contains(string(text), "slice: 10") {
			t.Fatalf("local counter = %q (%v), want slice 10", text, err)
		}
	})
	t.Run("git discovery error is returned", func(t *testing.T) {
		gitConfigRepo(t, "version: 1\n")
		t.Chdir(t.TempDir())
		if n, err := reserveItemNumber(&config.Loaded{}, "slice", 0); err == nil || n != 0 {
			t.Fatalf("reserveItemNumber outside a repository = (%d, %v), want (0, error)", n, err)
		}
	})
}

func TestTrailingNumberRejectsMissingAndOverflowingSuffixes(t *testing.T) {
	for _, test := range []struct {
		id   string
		want int
	}{
		{id: "slice-009", want: 9},
		{id: "T-000", want: 0},
	} {
		if got, err := trailingNumber(test.id); err != nil || got != test.want {
			t.Errorf("trailingNumber(%q) = (%d, %v), want %d", test.id, got, err, test.want)
		}
	}
	for _, id := range []string{"slice", "T-"} {
		if n, err := trailingNumber(id); err == nil || n != 0 {
			t.Errorf("trailingNumber(%q) = (%d, %v), want (0, error)", id, n, err)
		}
	}
	if _, err := trailingNumber("T-999999999999999999999999999999999999"); err == nil {
		t.Error("trailingNumber accepted an overflowing suffix")
	}
}
