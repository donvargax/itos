package ids

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
	"github.com/donvargax/itos/v8/internal/work"
)

// The config every test here reads: tasks/phase-{group}.yaml the ledger,
// T-<n> its ids, work-items.yaml the registry.
const configText = "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\nwork: { registry: work-items.yaml }\n"

// A config over the tree reader's fake, whose working tree holds files, in
// a scratch folder the test stands in, so the feature files it writes are
// the only ones Mint finds.
func configOver(t *testing.T, files map[string]string) *config.Loaded {
	t.Helper()
	t.Chdir(t.TempDir())
	trees := treefake.Trees{Files: map[string]map[string]string{"worktree": files}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(&vcsfake.VCS{}, trees, src, "itos.yaml", configText, false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// writeFeature writes a feature file under features, in the folder the
// test stands in.
func writeFeature(t *testing.T, name, text string) {
	t.Helper()
	path := filepath.Join("features", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMintUsesTheLedgerAndCounterFloor(t *testing.T) {
	cfg := configOver(t, map[string]string{"tasks/phase-1.yaml": "- { id: T-009, type: chore, title: Nine }\n"})
	tracker := &porttest.Tracker{}
	registryIDs := []string{"T-010", "not-a-task-id"}
	first, err := Mint(t.Context(), tracker, cfg, "task", registryIDs)
	if err != nil || first != "T-011" {
		t.Fatalf("first task mint = %q, %v, want T-011", first, err)
	}
	second, err := Mint(t.Context(), tracker, cfg, "task", registryIDs)
	if err != nil || second != "T-012" {
		t.Fatalf("second task mint = %q, %v, want T-012", second, err)
	}
	if _, err := Mint(t.Context(), tracker, cfg, "idea", registryIDs); err == nil {
		t.Fatal("Mint accepted idea, which keeps its caller-provided slug")
	}
}

// A ledger that cannot be read stops a task's mint before anything is
// reserved: here its folder is not there.
func TestMintRefusesALedgerItCannotRead(t *testing.T) {
	cfg := configOver(t, nil)
	tracker := &porttest.Tracker{}
	if got, err := Mint(t.Context(), tracker, cfg, "task", nil); err == nil || !strings.Contains(err.Error(), "ledger") {
		t.Fatalf("Mint = %q, %v, want the ledger's error", got, err)
	}
	if first, err := tracker.Reserve(t.Context(), "task", 0, 1); err != nil || first != 1 {
		t.Fatalf("the counter after a refused mint gives %d, %v, want 1", first, err)
	}
}

func TestTaggedIDsReadsFeatureTagsAndIgnoresOtherFiles(t *testing.T) {
	cfg := configOver(t, nil)
	writeFeature(t, "one.feature", "@phase-1\nFeature: One\n  @ID-ONE-01 @slice-007 @slice-bad\n  Scenario: One\n")
	writeFeature(t, "nested/two.feature", "Feature: Two\n  @slice-010\n  Scenario: Two\n")
	writeFeature(t, "notes.txt", "@slice-999\n")
	ids, err := taggedIDs(cfg, "slice", []string{"slice-009"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"slice-007", "slice-bad", "slice-010", "slice-009"} {
		if !slices.Contains(ids, want) {
			t.Errorf("taggedIDs = %v, missing %q", ids, want)
		}
	}
	if slices.Contains(ids, "slice-999") {
		t.Fatalf("taggedIDs read a non-feature file: %v", ids)
	}
	got, err := Mint(t.Context(), &porttest.Tracker{}, cfg, "slice", []string{"slice-009"})
	if err != nil || got != "slice-011" {
		t.Fatalf("slice mint = %q, %v, want slice-011", got, err)
	}
}

// A bug is minted as a slice is (slice 104), past the registry's bugs and
// the feature files' @bug tags, its own series: no slice counts.
func TestMintMintsABugPastTheRegistrysBugsAndTheBugTags(t *testing.T) {
	cfg := configOver(t, nil)
	writeFeature(t, "one.feature", "Feature: One\n  @ID-ONE-01 @bug-07 @slice-30\n  Scenario: One\n")
	got, err := Mint(t.Context(), &porttest.Tracker{}, cfg, "bug", []string{"bug-5", "slice-40"})
	if err != nil || got != "bug-08" {
		t.Fatalf("bug mint = %q, %v, want bug-08", got, err)
	}
	ids, err := taggedIDs(cfg, "bug", nil)
	if err != nil || !slices.Equal(ids, []string{"bug-07"}) {
		t.Fatalf("taggedIDs(bug) = %v, %v, want [bug-07]", ids, err)
	}
}

// A feature file that cannot be read stops the mint before anything is
// reserved: here a link to itself, which no read resolves.
func TestMintRefusesAFeatureFileItCannotRead(t *testing.T) {
	cfg := configOver(t, nil)
	if err := os.MkdirAll("features", 0o755); err != nil {
		t.Fatal(err)
	}
	loop := filepath.Join("features", "loop.feature")
	if err := os.Symlink("loop.feature", loop); err != nil {
		t.Skipf("cannot make a symbolic link here: %v", err)
	}
	if _, err := os.ReadFile(loop); err == nil || os.IsNotExist(err) {
		t.Skipf("a link to itself reads here as %v, not as a failed read", err)
	}
	tracker := &porttest.Tracker{}
	if got, err := Mint(t.Context(), tracker, cfg, "bug", nil); err == nil {
		t.Fatalf("Mint = %q, want the unreadable feature file's error", got)
	}
	if first, err := tracker.Reserve(t.Context(), "bug", 0, 1); err != nil || first != 1 {
		t.Fatalf("the counter after a refused mint gives %d, %v, want 1", first, err)
	}
}

// A number past the largest int cannot be read back, and nothing is minted.
func TestMintRefusesANumberPastTheLargestInt(t *testing.T) {
	cfg := configOver(t, nil)
	got, err := Mint(t.Context(), &porttest.Tracker{}, cfg, "bug", []string{"bug-" + strconv.Itoa(math.MaxInt)})
	if err == nil || !strings.Contains(err.Error(), "cannot read the number from next bug id") {
		t.Fatalf("Mint = %q, %v, want the number refused", got, err)
	}
}

// A counter the tracker cannot raise is the mint's error: here one already
// at the largest int.
func TestMintGivesTheTrackersRefusal(t *testing.T) {
	cfg := configOver(t, nil)
	tracker := &porttest.Tracker{}
	if _, err := tracker.Reserve(t.Context(), "slice", math.MaxInt-1, 1); err != nil {
		t.Fatal(err)
	}
	if got, err := Mint(t.Context(), tracker, cfg, "slice", nil); err == nil || got != "" {
		t.Fatalf("Mint past a full counter = %q, %v, want the tracker's error", got, err)
	}
}

// The scenario kind's root is where the tags are read, features then left
// out.
func TestTaggedIDsReadsTheScenarioRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFeature(t, "one.feature", "Feature: One\n  @slice-006\n  Scenario: One\n")
	if err := os.MkdirAll("specs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("specs", "two.feature"), []byte("Feature: Two\n  @slice-008\n  Scenario: Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := "specs"
	cfg := &config.Loaded{}
	cfg.Tests.Values = map[string]config.Kind{"scenario": {Root: &root}}
	ids, err := taggedIDs(cfg, "slice", nil)
	if err != nil || !slices.Equal(ids, []string{"slice-008"}) {
		t.Fatalf("taggedIDs = %v, %v, want the specs root's slice-008 alone", ids, err)
	}
}

func TestTaggedIDsUsesDefaultRootWhenConfiguredRootIsEmpty(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFeature(t, "one.feature", "Feature: One\n  @slice-006\n  Scenario: One\n")
	root := ""
	cfg := &config.Loaded{}
	cfg.Tests.Values = map[string]config.Kind{"scenario": {Root: &root}}
	ids, err := taggedIDs(cfg, "slice", nil)
	if err != nil || !slices.Contains(ids, "slice-006") {
		t.Fatalf("taggedIDs = %v, %v, want the default features root", ids, err)
	}
}

func TestTaggedIDsUsesDefaultRootWhenScenarioRootIsNil(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFeature(t, "one.feature", "Feature: One\n  @slice-006\n  Scenario: One\n")
	cfg := &config.Loaded{}
	cfg.Tests.Values = map[string]config.Kind{"scenario": {}}
	ids, err := taggedIDs(cfg, "slice", nil)
	if err != nil || !slices.Contains(ids, "slice-006") {
		t.Fatalf("taggedIDs = %v, %v, want the default features root", ids, err)
	}
}

func TestMintSliceWidthUsesOnlyValidNumericSeriesIDs(t *testing.T) {
	t.Run("keeps zero padding of the series", func(t *testing.T) {
		cfg := configOver(t, nil)
		got, err := Mint(t.Context(), &porttest.Tracker{}, cfg, "slice", []string{"slice-009"})
		if err != nil || got != "slice-010" {
			t.Fatalf("Mint = %q, %v, want slice-010", got, err)
		}
	})
	t.Run("ignores non-series digits and malformed suffixes", func(t *testing.T) {
		cfg := configOver(t, nil)
		got, err := Mint(t.Context(), &porttest.Tracker{}, cfg, "slice", []string{"slice-1", "slice-000x", "123"})
		if err != nil || got != "slice-2" {
			t.Fatalf("Mint = %q, %v, want slice-2", got, err)
		}
	})
	for _, malformed := range []string{"slice-0/", "slice-0:"} {
		t.Run(malformed, func(t *testing.T) {
			cfg := configOver(t, nil)
			got, err := Mint(t.Context(), &porttest.Tracker{}, cfg, "slice", []string{"slice-1", malformed})
			if err != nil || got != "slice-2" {
				t.Fatalf("Mint = %q, %v, want slice-2", got, err)
			}
		})
	}
}

// The counter is the floor past the files: a slice reserved before is
// passed over.
func TestMintPassesTheCounter(t *testing.T) {
	cfg := configOver(t, nil)
	tracker := &porttest.Tracker{}
	if _, err := tracker.Reserve(t.Context(), "slice", 0, 12); err != nil {
		t.Fatal(err)
	}
	if got, err := Mint(t.Context(), tracker, cfg, "slice", []string{"slice-009"}); err != nil || got != "slice-013" {
		t.Fatalf("Mint past the counter = %q, %v, want slice-013", got, err)
	}
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

// Claim gives a run of the stem's tags as wide as next, past the counter,
// and raises the counter past the run.
func TestClaimGivesARunAsWideAsNext(t *testing.T) {
	tracker := &porttest.Tracker{}
	tags, err := Claim(t.Context(), tracker, "ID-A", "@ID-A-02", 2)
	if err != nil || !slices.Equal(tags, []string{"@ID-A-02", "@ID-A-03"}) {
		t.Fatalf("Claim = %v, %v, want @ID-A-02 and @ID-A-03", tags, err)
	}
	tags, err = Claim(t.Context(), tracker, "ID-A", "@ID-A-02", 1)
	if err != nil || !slices.Equal(tags, []string{"@ID-A-04"}) {
		t.Fatalf("Claim after a run = %v, %v, want @ID-A-04", tags, err)
	}
}

// A next with no number, or past the largest int, claims nothing, and a
// run the tracker refuses is its error.
func TestClaimRefusesWhatItCannotClaim(t *testing.T) {
	tracker := &porttest.Tracker{}
	for _, next := range []string{"@ID-A-", "@ID-A-" + strconv.Itoa(math.MaxInt) + "0"} {
		if tags, err := Claim(t.Context(), tracker, "ID-A", next, 1); err == nil || tags != nil ||
			!strings.Contains(err.Error(), "cannot read the number of the next ID-A id") {
			t.Errorf("Claim of %q = %v, %v, want the number refused", next, tags, err)
		}
	}
	if tags, err := Claim(t.Context(), tracker, "ID-A", "@ID-A-"+strconv.Itoa(math.MaxInt), 2); err == nil || tags != nil {
		t.Errorf("Claim of a run past the largest int = %v, %v, want the tracker's error", tags, err)
	}
	if first, err := tracker.Reserve(t.Context(), "ID-A", 0, 1); err != nil || first != 1 {
		t.Fatalf("the counter after refused claims gives %d, %v, want 1", first, err)
	}
}

// Registry is the registry's ids; none when it is not there, and its
// error when it cannot be read.
func TestRegistry(t *testing.T) {
	registry := "phases:\n  1: nobody\nitems:\n  - id: p1-one\n    title: One\n    phase: 1\n    owner: null\n    status: todo\n    depends_on: []\n  - id: T-2\n    title: Two\n    phase: 1\n    owner: null\n    status: todo\n    depends_on: []\n"
	cfg := configOver(t, map[string]string{"work-items.yaml": registry})
	if ids, err := Registry(t.Context(), cfg); err != nil || !slices.Equal(ids, []string{"p1-one", "T-2"}) {
		t.Errorf("Registry = %v, %v, want p1-one and T-2", ids, err)
	}
	if ids, err := Registry(t.Context(), configOver(t, nil)); err != nil || ids != nil {
		t.Errorf("Registry with none there = %v, %v, want none", ids, err)
	}
	if ids, err := Registry(t.Context(), configOver(t, map[string]string{"work-items.yaml": "items: [\n"})); err == nil || ids != nil {
		t.Errorf("Registry of a registry that is no YAML = %v, %v, want its error", ids, err)
	}
}

// Items is every item's id, an item with none left out.
func TestItems(t *testing.T) {
	cfg := configOver(t, map[string]string{"work-items.yaml": "items:\n  - id: T-1\n  - title: no id\n  - id: slice-2\n"})
	registry, err := work.Load(cfg, "work-items.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if ids := Items(registry); !slices.Equal(ids, []string{"T-1", "slice-2"}) {
		t.Errorf("Items = %v, want T-1 and slice-2", ids)
	}
	if ids := Items(work.Registry{}); ids == nil || len(ids) != 0 {
		t.Errorf("Items of no registry = %#v, want an empty list", ids)
	}
}
