package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// configIn is the config of the text, loaded in a fresh folder holding the
// files.
func configIn(t *testing.T, text string, files ...string) *config.Loaded {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, path := range append(files, "itos.yaml") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, "itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// A ledger layout that reads nothing, or reads the registry, is a problem
// naming its key (slice 130).
func TestLayoutProblems(t *testing.T) {
	cases := map[string][]string{
		"version: 1\n": nil,
		"version: 1\nledger: { files: 'tasks/phase-{group}.yaml' }\n":                               nil,
		"version: 1\nledger: { files: 'tasks/phase-{group}.yaml', group: { pattern: '\\d+|x' } }\n": nil,
		"version: 1\nledger: { files: 'tasks/phase-{group}.yaml', group: { pattern: '^\\d+$' } }\n": {
			"config-ledger-group-anchors: ledger.group.pattern has anchors of its own: ^\\d+$; it is matched inside " +
				"a file's name, between what ledger.files writes around {group}, so it matches no file",
		},
		"version: 1\nledger: { files: 'tasks/phase-{group}.yaml', group: { pattern: '(?:a|\\d+\\z)' } }\n": {
			"config-ledger-group-anchors: ledger.group.pattern has anchors of its own: (?:a|\\d+\\z); it is matched " +
				"inside a file's name, between what ledger.files writes around {group}, so it matches no file",
		},
		"version: 1\nledger: { files: 'tasks/{group}/tasks.yaml' }\n": {
			"config-ledger-files-folder: ledger.files has {group} in its folder: tasks/{group}/tasks.yaml; the ledger " +
				"is one folder, and a group is a part of a file's name",
		},
		"version: 1\nledger: { files: 'tasks/{group}.yaml', group: { pattern: '[a-z-]+' } }\n": {
			"config-ledger-registry: ledger.files (tasks/{group}.yaml) matches work.registry (tasks/work-items.yaml), " +
				"so the registry would be read as a ledger file",
		},
		"version: 1\nledger: { files: 'tasks/{group}.yaml', group: { pattern: '[a-z-]+' } }\nwork: { registry: work.yaml }\n": nil,
	}
	for text, want := range cases {
		var got []string
		for _, p := range LayoutProblems(configIn(t, text)) {
			got = append(got, p.Rule+": "+p.Message)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

func TestLayoutProblemsSayWhatToWrite(t *testing.T) {
	cfg := configIn(t, "version: 1\nledger: { files: 'tasks/{group}/{group}.yaml', group: { pattern: '^[a-z-]+' } }\n")
	cfg.Work.Registry = "tasks/{group}/work-items.yaml"
	var got []string
	for _, p := range LayoutProblems(cfg) {
		got = append(got, p.Fix)
	}
	want := []string{
		"remove the anchors (^, $, \\A, \\z) from ledger.group.pattern",
		"write {group} in the file's name, as in tasks/phase-{group}.yaml",
		"change ledger.files or ledger.group.pattern so that no ledger file is work.registry, or move work.registry",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A pattern that does not compile has no anchors to name; the loader
// refuses it.
func TestAnchoredReadsOnlyAPatternThatCompiles(t *testing.T) {
	for pattern, want := range map[string]bool{"(": false, `\d+`: false, `x$`: true, `(?m)^x`: true, `\^x\$`: false} {
		if got := anchored(pattern); got != want {
			t.Errorf("%q: got %t, want %t", pattern, got, want)
		}
	}
}

// Numeric groups are read in the order of their numbers, a group that is no
// number after them (slice 130); other groups by name.
func TestFilesSortsNumericGroupsByNumber(t *testing.T) {
	files := []string{"tasks/phase-10.yaml", "tasks/phase-2.yaml", "tasks/phase-1.yaml", "tasks/phase-x.yaml", "tasks/phase-b.yaml",
		"tasks/phase-01.yaml"}
	for text, want := range map[string][]string{
		// 01 and 1 are one number, kept in the order of their names.
		"version: 1\nledger: { files: 'tasks/phase-{group}.yaml', group: { pattern: '\\w+', numeric: true } }\n": {
			"01", "1", "2", "10", "b", "x",
		},
		"version: 1\nledger: { files: 'tasks/phase-{group}.yaml', group: { pattern: '\\w+' } }\n": {
			"01", "1", "10", "2", "b", "x",
		},
	} {
		cfg := configIn(t, text, files...)
		listed, err := Files(cfg)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range listed {
			got = append(got, f.Group)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

// ledger.id is anchored as a whole, so each branch of an alternation is
// (slice 130); with none, any ID is one.
func TestIDPatternAnchorsEveryBranch(t *testing.T) {
	cfg := configIn(t, "version: 1\nledger: { files: 'tasks/phase-{group}.yaml', id: 'T-\\d+|OPS-\\d+' }\n")
	pattern := IDPattern(cfg)
	for id, want := range map[string]bool{"T-1": true, "OPS-2": true, "T-1x": false, "xOPS-2": false} {
		if got := pattern.MatchString(id); got != want {
			t.Errorf("%q: got %t, want %t", id, got, want)
		}
	}
	cfg.Ledger.ID = nil
	if !IDPattern(cfg).MatchString("anything at all") {
		t.Error("with no ledger.id, an ID did not match")
	}
}

// A working tree's ledger folder that is there and cannot be listed keeps
// its own error; one that is not there is the config error naming it.
func TestFilesTellsAFolderMissingFromOneUnlisted(t *testing.T) {
	const text = "version: 1\nledger: { files: 'tasks/phase-{group}.yaml' }\n"
	var invalid *config.Error
	if _, err := Files(configIn(t, text, "tasks")); err == nil || errors.As(err, &invalid) {
		t.Errorf("a folder that is a file: got %v, want its own error", err)
	}
	if _, err := Files(configIn(t, text)); !errors.As(err, &invalid) || invalid.Problems[0].Rule != "ledger-folder-missing" {
		t.Errorf("no folder: got %v, want ledger-folder-missing", err)
	}
}
