package cli

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/guide"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
)

// guidePorts bind the guide slice to the real code: git, the trees over it
// and the working tree; runGuide reads the line and runs the slice over
// them.
func TestGuidePortsBindTheRealCode(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	p := guidePorts{}
	if p.Worktree(t.Context()) != trees.Worktree {
		t.Error("Worktree is not the working tree's files")
	}
	src, err := p.Trees(t.Context()).At(t.Context(), "index")
	if err != nil || src.Tree(t.Context()) != "index" {
		t.Errorf("Trees().At(index) = %v, %v, want the index git holds", src, err)
	}
	if l, err := p.Locate(t.Context(), ""); err != nil || l.Common == "" {
		t.Errorf("Locate = %+v, %v, want the repository git locates", l, err)
	}
	var stdout strings.Builder
	want, _ := guide.Text("land")
	if code, err := runGuide([]string{"land"}, Out{Stdout: &stdout}); code != 0 || err != nil || stdout.String() != want {
		t.Errorf("runGuide land = %d, %v, stdout %.60q", code, err, stdout.String())
	}
	if _, err := runGuide([]string{"land", "x"}, Out{}); err == nil || err.Error() != "guide land takes no argument: x" {
		t.Errorf("runGuide land x = %v, want the usage error", err)
	}
}
