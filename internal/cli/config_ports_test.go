package cli

import (
	"slices"
	"strings"
	"testing"

	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
)

// configPorts bind the config slice to the real code: the trees over git,
// the working tree and the flags of itos task that name no group; runConfig
// reads the line and runs the slice over them.
func TestConfigPortsBindTheRealCode(t *testing.T) {
	gitConfigRepo(t, "version: 1\nhooks: { bin: go run ./cmd/itos }\n")
	p := configPorts{}
	if p.Worktree(t.Context()) != trees.Worktree {
		t.Error("Worktree is not the working tree's files")
	}
	src, err := p.Trees(t.Context()).At(t.Context(), "index")
	if err != nil || src.Tree(t.Context()) != "index" {
		t.Errorf("Trees().At(index) = %v, %v, want the index git holds", src, err)
	}
	if got := p.TaskFlags(t.Context()); !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(ungroupedTaskFlags()))) {
		t.Errorf("TaskFlags = %q, want ungroupedTaskFlags", got)
	}
	var stdout strings.Builder
	if code, err := runConfig([]string{"get", "hooks.bin"}, Out{Stdout: &stdout}); code != 0 || err != nil || stdout.String() != "go run ./cmd/itos\n" {
		t.Errorf("runConfig get hooks.bin = %d, %v, stdout %q", code, err, stdout.String())
	}
	if code, err := runConfig([]string{"nope"}, Out{}); code != 0 || err == nil || err.Error() != "unknown command: config nope" {
		t.Errorf("runConfig nope = %d, %v, want 0 and the usage error", code, err)
	}
}
