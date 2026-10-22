package cli

import (
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tracker/adapter"
)

// testsPorts bind the tests slice to the real code: the config the working
// tree has, the tracker of the config on the OS itos runs on, and the
// config's shell writing to itos's streams.
func TestTestsPortsBindTheRealCode(t *testing.T) {
	gitConfigRepo(t, "version: 1\nwork: { registry: work-items.yaml }\n")
	var stdout, stderr strings.Builder
	p := testsPorts{OS: runtime.GOOS, Stdout: &stdout, Stderr: &stderr}
	cfg, err := p.Config(t.Context())
	if err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Fatalf("Config = %+v, %v, want the working tree's config", cfg, err)
	}
	files, ok := p.Tracker(t.Context(), cfg).(adapter.Files)
	if !ok || files.OS != runtime.GOOS || files.Lock != "work-items.yaml" {
		t.Errorf("Tracker = %#v, want the files adapter of the config on this OS", files)
	}
	result := p.Shell(t.Context(), cfg, "echo out; echo err >&2; exit 3")
	if result.Code != 3 || stdout.String() != "out\n" || stderr.String() != "err\n" {
		t.Errorf("Shell = %+v, stdout %q, stderr %q, want exit 3 on itos's streams", result, stdout.String(), stderr.String())
	}
}
