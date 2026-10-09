package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
)

func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(git.Bin(), args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

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

func TestSliceIDsUsesDefaultRootWhenConfiguredRootIsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("features", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("features/one.feature", []byte("Feature: One\n  @slice-006\n  Scenario: One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := ""
	cfg := &config.Loaded{}
	cfg.Tests.Values = map[string]config.Kind{"scenario": {Root: &root}}
	ids, err := sliceIDs(cfg, nil)
	if err != nil || !slices.Contains(ids, "slice-006") {
		t.Fatalf("sliceIDs = %v, %v, want the default features root", ids, err)
	}
}

func TestSliceIDsUsesDefaultRootWhenScenarioRootIsNil(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("features", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("features/one.feature", []byte("Feature: One\n  @slice-006\n  Scenario: One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Loaded{}
	cfg.Tests.Values = map[string]config.Kind{"scenario": {}}
	ids, err := sliceIDs(cfg, nil)
	if err != nil || !slices.Contains(ids, "slice-006") {
		t.Fatalf("sliceIDs = %v, %v, want the default features root", ids, err)
	}
}

func TestMintItemIDSliceWidthUsesOnlyValidNumericSeriesIDs(t *testing.T) {
	t.Run("keeps zero padding of the series", func(t *testing.T) {
		cfg, _ := idMintTestConfig(t, "[]\n")
		got, err := mintItemID(cfg, "slice", []string{"slice-009"})
		if err != nil || got != "slice-010" {
			t.Fatalf("mintItemID = %q, %v, want slice-010", got, err)
		}
	})
	t.Run("ignores non-series digits and malformed suffixes", func(t *testing.T) {
		cfg, _ := idMintTestConfig(t, "[]\n")
		got, err := mintItemID(cfg, "slice", []string{"slice-1", "slice-000x", "123"})
		if err != nil || got != "slice-2" {
			t.Fatalf("mintItemID = %q, %v, want slice-2", got, err)
		}
	})
	for _, malformed := range []string{"slice-0/", "slice-0:"} {
		t.Run(malformed, func(t *testing.T) {
			cfg, _ := idMintTestConfig(t, "[]\n")
			got, err := mintItemID(cfg, "slice", []string{"slice-1", malformed})
			if err != nil || got != "slice-2" {
				t.Fatalf("mintItemID = %q, %v, want slice-2", got, err)
			}
		})
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
	t.Run("configured upstream is the counter remote", func(t *testing.T) {
		_, dir := idMintTestConfig(t, "[]\n")
		bare := filepath.Join(t.TempDir(), "upstream.git")
		gitIn(t, "init", "-q", "--bare", bare)
		branch := strings.TrimSpace(gitIn(t, "branch", "--show-current"))
		gitIn(t, "remote", "add", "upstream", bare)
		gitIn(t, "config", "branch."+branch+".remote", "upstream")
		gitIn(t, "config", "branch."+branch+".merge", "refs/heads/main")
		n, err := reserveItemNumber(&config.Loaded{}, "slice", 0)
		if err != nil || n != 1 {
			t.Fatalf("reserveItemNumber = (%d, %v), want (1, nil)", n, err)
		}
		ref := gitAt(t, bare, "rev-parse", "refs/itos/ids")
		if strings.TrimSpace(ref) == "" {
			t.Fatalf("upstream counter ref was not created (repo %s)", dir)
		}
	})
	t.Run("git discovery error is returned", func(t *testing.T) {
		gitConfigRepo(t, "version: 1\n")
		t.Chdir(t.TempDir())
		if n, err := reserveItemNumber(&config.Loaded{}, "slice", 0); err == nil || n != 0 {
			t.Fatalf("reserveItemNumber outside a repository = (%d, %v), want (0, error)", n, err)
		}
	})
	t.Run("working-directory failures are returned", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("removing the current directory is a Unix-specific test")
		}
		for _, test := range []struct {
			name, reported string
		}{
			{name: "relative common-dir path", reported: ".git"},
			{name: "absolute common-dir path", reported: t.TempDir()},
		} {
			t.Run(test.name, func(t *testing.T) {
				repo := t.TempDir()
				fakeGit := filepath.Join(t.TempDir(), "fake-git")
				script := "#!/bin/sh\nprintf '%s\\n' \"$T129_COMMON_DIR\"\nrm -rf \"$T129_REPO_DIR\" >/dev/null 2>&1 || true\n"
				if err := os.WriteFile(fakeGit, []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("ITOS_GIT", fakeGit)
				t.Setenv("T129_COMMON_DIR", test.reported)
				t.Setenv("T129_REPO_DIR", repo)
				t.Chdir(repo)
				if n, err := reserveItemNumber(&config.Loaded{}, "slice", 0); err == nil || n != 0 {
					t.Fatalf("reserveItemNumber = (%d, %v), want (0, error)", n, err)
				}
			})
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
		{id: "9", want: 9},
	} {
		if got, err := trailingNumber(test.id); err != nil || got != test.want {
			t.Errorf("trailingNumber(%q) = (%d, %v), want %d", test.id, got, err, test.want)
		}
	}
	for _, id := range []string{"", "slice", "T-"} {
		if n, err := trailingNumber(id); err == nil || n != 0 {
			t.Errorf("trailingNumber(%q) = (%d, %v), want (0, error)", id, n, err)
		}
	}
	if _, err := trailingNumber("T-999999999999999999999999999999999999"); err == nil {
		t.Error("trailingNumber accepted an overflowing suffix")
	}
}
