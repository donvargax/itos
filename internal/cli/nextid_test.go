package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// nextIDRepo is a scratch repository, never this checkout, whose scenario
// kind's ID pattern is an ID- one and whose feature file holds @ID-A-01 and
// @slice-3: tests next-id claims from it through the counter in its git
// folder, as it has no remote.
func nextIDRepo(t *testing.T) string {
	t.Helper()
	dir := gitConfigRepo(t, "version: 1\ntests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\", tag_prefix: \"@\" }\n")
	if err := os.MkdirAll(filepath.Join(dir, "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "features", "a.feature"), "Feature: A\n  @ID-A-01 @slice-3\n  Scenario: One\n")
	return dir
}

func nextID(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var stdout strings.Builder
	o := Out{Stdout: &stdout, Stderr: &stdout}
	if len(args) > 0 && args[0] == "--json" {
		o.JSON, args = true, args[1:]
	}
	_, err := testsNextID(args, o)
	return stdout.String(), err
}

// An area's tag is claimed, so the next run gives the one after; --count
// claims a run; slice only reads, so a repeat gives the same tag.
func TestTestsNextIDClaimsAnAreasTagsAndOnlyReadsAnyOtherStem(t *testing.T) {
	dir := nextIDRepo(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"scenario", "ID-A"}, "@ID-A-02\n"},
		{[]string{"scenario", "ID-A"}, "@ID-A-03\n"},
		{[]string{"scenario", "ID-A", "--count", "2"}, "@ID-A-04\n@ID-A-05\n"},
		{[]string{"scenario", "ID-NEW"}, "@ID-NEW-01\n"},
		{[]string{"scenario", "slice"}, "@slice-4\n"},
		{[]string{"scenario", "slice"}, "@slice-4\n"},
		{[]string{"--json", "scenario", "ID-A", "--count", "2"},
			`{"schema":1,"kind":"scenario","stem":"ID-A","id":"@ID-A-06","ids":["@ID-A-06","@ID-A-07"],"claimed":true}`},
		{[]string{"--json", "scenario", "slice"},
			`{"schema":1,"kind":"scenario","stem":"slice","id":"@slice-4","ids":["@slice-4"],"claimed":false}`},
	} {
		got, err := nextID(t, c.args...)
		if c.args[0] == "--json" {
			var compact bytes.Buffer
			if json.Compact(&compact, []byte(got)) == nil {
				got = compact.String()
			}
		}
		if err != nil || got != c.want {
			t.Fatalf("tests next-id %q = %q, %v, want %q", c.args, got, err, c.want)
		}
	}
	counters := fileText(t, filepath.Join(dir, ".git", "itos", "ids.yaml"))
	if counters != "counters:\n    ID-A: 7\n    ID-NEW: 1\n" {
		t.Fatalf("the counters = %q, want ID-A at 7 and ID-NEW at 1, and no slice", counters)
	}
}

// Each refusal is a usage error and claims nothing.
func TestTestsNextIDRefusesAWrongLineAndClaimsNothing(t *testing.T) {
	dir := nextIDRepo(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"scenario"}, "needs <kind> <stem>"},
		{[]string{"scenario", "ID-A", "--count", "0"}, `not "0"`},
		{[]string{"scenario", "ID-A", "--count", "-1"}, `not "-1"`},
		{[]string{"scenario", "ID-A", "--count", "two"}, `not "two"`},
		{[]string{"unit", "ID-A"}, "no tests kind unit"},
		{[]string{"scenario", "slice", "--count", "1"}, "slice is no area of tests.scenario's id"},
	} {
		got, err := nextID(t, c.args...)
		var u usageError
		if !errors.As(err, &u) || !strings.Contains(err.Error(), c.want) || got != "" {
			t.Errorf("tests next-id %q = %q, %v, want a usage error saying %q", c.args, got, err, c.want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "itos", "ids.yaml")); !os.IsNotExist(err) {
		t.Fatalf("a refused next-id wrote the counter: %v", err)
	}
}

// A tag past the largest int has no number to claim after, a run past it is
// refused by the counter, and a counter that cannot be located (no git
// repository) claims nothing either.
func TestTestsNextIDRefusesWhatItCannotClaim(t *testing.T) {
	dir := nextIDRepo(t)
	writeFile(t, filepath.Join(dir, "features", "a.feature"), "Feature: A\n  @ID-A-"+strconv.Itoa(math.MaxInt)+"\n  Scenario: One\n")
	if got, err := nextID(t, "scenario", "ID-A"); err == nil || !strings.Contains(err.Error(), "cannot read the number") {
		t.Fatalf("tests next-id past the largest int = %q, %v, want the number refused", got, err)
	}
	writeFile(t, filepath.Join(dir, "features", "a.feature"), "Feature: A\n  @ID-A-"+strconv.Itoa(math.MaxInt-1)+"\n  Scenario: One\n")
	if got, err := nextID(t, "scenario", "ID-A", "--count", "2"); err == nil || got != "" || !strings.Contains(err.Error(), "largest number") {
		t.Fatalf("tests next-id --count 2 up to past the largest int = %q, %v, want the run refused", got, err)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	writeFile(t, filepath.Join(dir, "features", "a.feature"), "Feature: A\n  @ID-A-01\n  Scenario: One\n")
	if got, err := nextID(t, "scenario", "ID-A"); err == nil || got != "" {
		t.Fatalf("tests next-id outside a repository = %q, %v, want the counter's error", got, err)
	}
}

// A config that does not load, a registry that does not, and a kind whose
// listing fails each stop the command before anything is claimed.
func TestTestsNextIDStopsAtWhatItCannotRead(t *testing.T) {
	dir := nextIDRepo(t)
	for _, c := range []struct {
		config, registry string
		args             []string
	}{
		{"version: [\n", "", []string{"scenario", "ID-A"}},
		{"version: 1\nwork: { registry: work-items.yaml }\ntests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\n",
			"items: [\n", []string{"scenario", "ID-A"}},
		{"version: 1\ntests:\n  listed: { adapter: { command: \"exit 3\", supports_at: false }, id: \"ID-[A-Z]+-\\\\d+\" }\n",
			"", []string{"listed", "ID-A"}},
	} {
		writeFile(t, filepath.Join(dir, "itos.yaml"), c.config)
		if c.registry != "" {
			writeFile(t, filepath.Join(dir, "work-items.yaml"), c.registry)
		}
		if got, err := nextID(t, c.args...); err == nil || got != "" {
			t.Errorf("tests next-id %q under %q = %q, %v, want an error", c.args, c.config, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "itos", "ids.yaml")); !os.IsNotExist(err) {
		t.Fatalf("a next-id that could not read wrote the counter: %v", err)
	}
}
