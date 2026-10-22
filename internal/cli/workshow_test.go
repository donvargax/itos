package cli

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
	"github.com/donvargax/itos/v8/internal/work"
)

// work show --json --patch of a commit whose patch cannot be read is that
// error, and prints nothing: no item is given with a commit's patch missing.
func TestEmitShownPatchFails(t *testing.T) {
	v := &porttest.VCS{History: []porttest.Commit{{Name: "c-a", Tree: map[string]string{"a.md": "a\n"}}}}
	shown := work.Shown{Item: value.NewMap("id", "T-1")}
	commits := []message.Logged{{SHA: "c-a", Header: "docs: add a"}, {SHA: "gone", Header: "docs: gone"}}
	var stdout strings.Builder
	err := emitShown(v, shown, false, nil, nil, commits, true, Out{Stdout: &stdout, JSON: true})
	if err == nil || err.Error() != "gone names no commit" || stdout.Len() > 0 {
		t.Errorf("emitShown = %v\n%s", err, stdout.String())
	}
}
