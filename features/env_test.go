package features

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScenarioEnvironmentKeepsPathFirstPrecedence(t *testing.T) {
	w := &world{support: t.TempDir(), dir: t.TempDir()}
	w.vars = []string{"PATH=" + filepath.Join(w.support, "overridden"), "ITOS_KEPT=once"}
	if err := os.MkdirAll(w.extensionsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	var path string
	kept := 0
	for _, kv := range w.env() {
		if value, ok := strings.CutPrefix(kv, "PATH="); ok {
			path = value
		}
		if kv == "ITOS_KEPT=once" {
			kept++
		}
	}
	if !strings.HasPrefix(path, w.extensionsDir()+string(os.PathListSeparator)) {
		t.Errorf("scenario PATH = %q, want extensions first", path)
	}
	if kept != 1 {
		t.Errorf("scenario variable occurs %d times, want once", kept)
	}
}
