package cli

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/ci"
	"github.com/donvargax/itos/v8/internal/plan"
)

// ciPorts bind the ci slice to the real code: the config the working tree
// has, which is not the stealth one, the process's environment, internal/ci
// carrying a plan out, and ci.watch's provider, none here.
func TestCIPortsBindTheRealCode(t *testing.T) {
	t.Setenv("ITOS_FROM", "")
	t.Setenv("ITOS_TO", "")
	t.Setenv("ITOS_CI_PORTS", "set")
	gitConfigRepo(t, "version: 1\nci: { steps: [], nightly: { steps: [echo one] }, watch: { provider: none } }\n")
	p := ciPorts{}
	cfg, err := p.Config(t.Context())
	if err != nil || len(cfg.CI.Nightly.Steps) != 1 {
		t.Fatalf("Config = %+v, %v, want the working tree's config", cfg, err)
	}
	if p.Stealth(t.Context()) {
		t.Error("Stealth = true for a project's config")
	}
	if got := p.Env(t.Context(), "ITOS_CI_PORTS"); got != "set" {
		t.Errorf("Env = %q, want the process's", got)
	}
	var stdout, stderr strings.Builder
	code, err := p.RunPlan(t.Context(), cfg, plan.ForNightly(cfg), ci.Options{Nightly: true, Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err != nil || !strings.Contains(stdout.String(), "one\n") {
		t.Errorf("RunPlan = %d, %v, stdout %q, want the nightly run", code, err, stdout.String())
	}
	if _, ok, err := p.Watcher(t.Context(), cfg, "origin"); ok || err != nil {
		t.Errorf("Watcher = %v, %v, want none", ok, err)
	}
}
