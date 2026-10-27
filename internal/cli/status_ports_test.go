package cli

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/store/tracker/adapter"
)

// statusPorts bind the status slice to the real code: the config the
// working tree has, work.identity's provider, the environment, the tracker
// of the config on the OS itos runs on; runStatus reads the line and runs
// the slice over them.
func TestStatusPortsBindTheRealCode(t *testing.T) {
	statusRepo(t, map[string]string{"work-items.yaml": items("one"), "people.yaml": "- a\n",
		"itos.yaml": "version: 1\nwork:\n  registry: work-items.yaml\n  people: { source: yaml, file: people.yaml }\n" +
			"  identity: { provider: none, hint: say --as }\n"})
	p := statusPorts{o: Out{OS: runtime.GOOS}}
	cfg, err := p.Config(t.Context())
	if err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Fatalf("Config = %+v, %v, want the working tree's config", cfg, err)
	}
	if answer := p.Identity(t.Context(), cfg)(); !strings.Contains(answer.Problem, "say --as") {
		t.Errorf("Identity() = %+v, want work.identity none's", answer)
	}
	t.Setenv("T263_STATUS", "set")
	if setup := p.Setup(t.Context()); setup.Env("T263_STATUS") != "set" || setup.GhToken == nil {
		t.Errorf("Setup = %+v, want the environment and gh's token", setup)
	}
	files, ok := p.Tracker(t.Context(), cfg).(adapter.Files)
	if !ok || files.OS != runtime.GOOS || files.Lock != "work-items.yaml" || files.Stealth {
		t.Errorf("Tracker = %#v, want the files adapter of the config on this OS", files)
	}
	if l, err := p.Locate(t.Context(), ""); err != nil || l.Common == "" {
		t.Errorf("Locate = %+v, %v, want git's", l, err)
	}
	var stdout, stderr strings.Builder
	if code, err := runStatus([]string{"--as", "a"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != 0 || err != nil ||
		!strings.HasPrefix(stdout.String(), "# Where things stand") {
		t.Errorf("runStatus --as a = %d, %v, stdout %.60q, stderr %q", code, err, stdout.String(), stderr.String())
	}
	if code, err := runStatus([]string{"x"}, Out{}); code != 0 || err == nil || err.Error() != "status takes no arguments: x" {
		t.Errorf("runStatus x = %d, %v, want the usage error alone", code, err)
	}
	t.Setenv("PATH", t.TempDir())
	if out, err := p.Gh(t.Context(), "api", "graphql"); out != "" || !errors.Is(err, providers.ErrNoGh) {
		t.Errorf("Gh with no gh = %q, %v, want providers.ErrNoGh", out, err)
	}
}
