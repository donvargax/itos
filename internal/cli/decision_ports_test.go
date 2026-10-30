package cli

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/store/tracker/adapter"
)

// decisionPorts bind the decision slice to the real code: the config the
// working tree has, the tracker of the config on the OS itos runs on, and
// the clock; runDecision reads the line and runs the slice over them.
func TestDecisionPortsBindTheRealCode(t *testing.T) {
	gitConfigRepo(t, "version: 1\nwork: { registry: work-items.yaml }\n")
	p := decisionPorts{Out{OS: runtime.GOOS}}
	cfg, err := p.Config(t.Context())
	if err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Fatalf("Config = %+v, %v, want the working tree's config", cfg, err)
	}
	files, ok := p.Tracker(t.Context(), cfg).(adapter.Files)
	if !ok || files.OS != runtime.GOOS || files.Lock != "work-items.yaml" || files.Stealth {
		t.Errorf("Tracker = %#v, want the files adapter of the config on this OS", files)
	}
	if now := p.Now(t.Context()); time.Since(now).Abs() > time.Minute {
		t.Errorf("Now = %v, want the clock's", now)
	}
	var stdout strings.Builder
	if code, err := runDecision(nil, Out{Stdout: &stdout, OS: runtime.GOOS}); code != 0 || err != nil || stdout.String() != "No open questions.\n" {
		t.Errorf("runDecision = %d, %v, stdout %q", code, err, stdout.String())
	}
	if _, err := runDecision([]string{"nope"}, Out{}); err == nil ||
		err.Error() != "decision has no subcommand nope: it takes add, answer, ratify, overturn, record or show, else --all" {
		t.Errorf("runDecision nope = %v, want the usage error", err)
	}
}
