package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// task next-id takes no word: one is a usage error before the config is
// read.
func TestTaskNextIDTakesNoArguments(t *testing.T) {
	gitConfigRepo(t, "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\n")
	putFile(t, filepath.Join("tasks", "phase-1.yaml"), "- id: T-1\n  title: One\n")
	var stdout strings.Builder
	for _, args := range [][]string{{"T-1"}, {"--json", "a", "b"}} {
		if _, err := taskNextID(args, Out{Stdout: &stdout, Stderr: &stdout}); ExitCode(err) != ExitUsage ||
			err.Error() != "task next-id takes no arguments" || stdout.Len() != 0 {
			t.Errorf("task next-id %q: %v, printed %q", args, err, stdout.String())
		}
	}
	if _, err := taskNextID(nil, Out{Stdout: &stdout, Stderr: &stdout}); err != nil || stdout.String() != "T-2\n" {
		t.Errorf("task next-id: %v, printed %q", err, stdout.String())
	}
}
