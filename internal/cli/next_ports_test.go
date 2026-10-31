package cli

import (
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tracker/adapter"
)

// nextPorts bind the next slice to the real code: the config the working
// tree has and the tracker of the config on the OS itos runs on; runNext
// reads the line and runs the slice over them.
func TestNextPortsBindTheRealCode(t *testing.T) {
	statusRepo(t, map[string]string{"work-items.yaml": items("one"), "people.yaml": "- a\n"})
	p := nextPorts{o: Out{OS: runtime.GOOS}}
	cfg, err := p.Config(t.Context())
	if err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Fatalf("Config = %+v, %v, want the working tree's config", cfg, err)
	}
	files, ok := p.Tracker(t.Context(), cfg).(adapter.Files)
	if !ok || files.OS != runtime.GOOS || files.Lock != "work-items.yaml" || files.Stealth {
		t.Errorf("Tracker = %#v, want the files adapter of the config on this OS", files)
	}
	var stdout, stderr strings.Builder
	if code, err := runNext([]string{"one"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != 0 || err != nil ||
		stdout.String() != "one: idea; next: person promote\n" {
		t.Errorf("runNext one = %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
	if code, err := runNext(nil, Out{}); code != 0 || err == nil || err.Error() != "next needs one <id>" {
		t.Errorf("runNext = %d, %v, want the usage error alone", code, err)
	}
}
