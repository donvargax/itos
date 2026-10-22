package status

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	"github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/release"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// releaseServer serves each path's body, "fail" a 500 and any other path
// a 404, as the release server for the test.
func releaseServer(t *testing.T, files map[string]string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		switch {
		case !ok:
			http.NotFound(w, r)
		case body == "fail":
			http.Error(w, "down", http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv(release.Env, server.URL)
}

// The version of itos a repository runs is its pin, else its install
// script's version= line, as itos upgrade reads the one it moves from; with
// neither, or one naming no version, it runs none status can name (slice 116).
func TestItosRuns(t *testing.T) {
	t.Chdir(t.TempDir())
	pinning := func(v string) *config.Loaded {
		return &config.Loaded{Config: config.Config{Pin: &config.Pin{Version: v}}}
	}
	if got := itosRuns(pinning("9.1.0")); got != "9.1.0" {
		t.Errorf("a pin of 9.1.0 runs %q", got)
	}
	for name, cfg := range map[string]*config.Loaded{"no pin": {}, "a pin that is no version": pinning("nine")} {
		if got := itosRuns(cfg); got != "" {
			t.Errorf("%s and no install script: runs %q", name, got)
		}
	}
	if err := os.MkdirAll(filepath.Dir(pin.InstallScript), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(pin.InstallScript, []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("#!/bin/sh\nversion=8.4.0\n")
	if got := itosRuns(&config.Loaded{}); got != "8.4.0" {
		t.Errorf("no pin and an install script of 8.4.0: runs %q", got)
	}
	if got := itosRuns(pinning("9.1.0")); got != "9.1.0" {
		t.Errorf("a pin and an install script: runs %q, not the pin's", got)
	}
	write("#!/bin/sh\nversion=latest\n")
	if got := itosRuns(&config.Loaded{}); got != "" {
		t.Errorf("an install script naming no version: runs %q", got)
	}
	if err := os.Remove(pin.InstallScript); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(pin.InstallScript, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := itosRuns(&config.Loaded{}); got != "" {
		t.Errorf("an install script that cannot be read: runs %q", got)
	}
}

// The newest release is asked of the server once, and a newer major alone
// is one; nothing is asked where the launcher asks nothing, and no answer,
// an answer naming no archive or a pre-release is no itos line.
func TestNewestItos(t *testing.T) {
	none := envOf(nil)
	latest := func(v string) {
		releaseServer(t, map[string]string{"/latest/download/checksums.txt": "aa  itos-" + v + "-linux-amd64.tar.gz\n"})
	}
	latest("9.2.0")
	if got := newestItos("9.1.0", none); got == nil || *got != (Itos{Runs: "9.1.0", Newest: "9.2.0"}) {
		t.Errorf("within the major: %+v", got)
	}
	if got := newestItos("", none); got != nil {
		t.Errorf("running no version named: %+v", got)
	}
	for _, name := range []string{"CI", "ITOS_NO_UPDATE"} {
		if got := newestItos("9.1.0", envOf(map[string]string{name: "1"})); got != nil {
			t.Errorf("with %s set: %+v", name, got)
		}
	}
	latest("10.0.0")
	if got := newestItos("9.1.0", none); got == nil || *got != (Itos{Runs: "9.1.0", Newest: "10.0.0", NewMajor: true}) {
		t.Errorf("a newer major: %+v", got)
	}
	if got := newestItos("10.0.0-rc.1", none); got == nil || got.NewMajor {
		t.Errorf("a pre-release of the newest's major: %+v", got)
	}
	latest("11.0.0-rc.1")
	if got := newestItos("9.1.0", none); got != nil {
		t.Errorf("a pre-release as the newest: %+v", got)
	}
	releaseServer(t, map[string]string{"/latest/download/checksums.txt": "aa  itos.schema.json\n"})
	if got := newestItos("9.1.0", none); got != nil {
		t.Errorf("an answer naming no archive: %+v", got)
	}
	releaseServer(t, map[string]string{"/latest/download/checksums.txt": "fail"})
	if got := newestItos("9.1.0", none); got != nil {
		t.Errorf("a server that fails: %+v", got)
	}
}

// The itos line names both versions, and itos upgrade with the notes only
// for a newer major; --json gives it as itos, null without one.
func TestItosLine(t *testing.T) {
	same := &Itos{Runs: "9.1.0", Newest: "9.2.0"}
	if line := same.line(); line != "itos: this repository runs 9.1.0; the newest release is 9.2.0." {
		t.Errorf("within the major: %q", line)
	}
	major := &Itos{Runs: "9.1.0", Newest: "10.0.0", NewMajor: true}
	if line := major.line(); !strings.Contains(line, "10.0.0, a new major: itos upgrade moves it there (notes: ") ||
		!strings.HasSuffix(line, "/tag/v10.0.0)") {
		t.Errorf("a newer major: %q", line)
	}
	if got := printed(&Standing{Itos: major}); !strings.Contains(got, "\n\n"+major.line()+"\n\n") {
		t.Errorf("the status does not print the itos line:\n%s", got)
	}
	for _, want := range []*Itos{major, nil} {
		var got struct {
			Itos *struct {
				Runs     *string `json:"runs"`
				Newest   *string `json:"newest"`
				NewMajor *bool   `json:"new_major"`
			} `json:"itos"`
		}
		if err := json.Unmarshal([]byte(emitted(t, &Standing{Itos: want})), &got); err != nil {
			t.Fatal(err)
		}
		switch {
		case want == nil && got.Itos != nil:
			t.Errorf("itos is %+v, not null", *got.Itos)
		case want != nil && (got.Itos == nil || got.Itos.Runs == nil || *got.Itos.Runs != want.Runs ||
			got.Itos.Newest == nil || *got.Itos.Newest != want.Newest ||
			got.Itos.NewMajor == nil || *got.Itos.NewMajor != want.NewMajor):
			t.Errorf("itos is not %+v", *want)
		}
	}
}

// A stale block of rules for agents is one line naming itos init
// --agent-rules, in one paragraph with the itos line, and --json's
// agent_rules names its files, null with none (slice 124).
func TestAgentRules(t *testing.T) {
	t.Chdir(t.TempDir())
	v := &vcsfake.VCS{}
	if got := agentRulesOf(t.Context(), v, false); got != nil {
		t.Errorf("no AGENTS.md: %+v", got)
	}
	older := agentrules.Begin + "\n\n## The rules itos holds this repository to\n\n" + agentrules.End + "\n"
	if err := os.WriteFile(agentrules.AgentsFile, []byte("Mine.\n\n"+older), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := agentRulesOf(t.Context(), v, false); got == nil || len(got.Stale) != 1 || got.Stale[0] != agentrules.AgentsFile || got.stealth {
		t.Errorf("a stale AGENTS.md: %+v", got)
	}
	if got := agentRulesOf(t.Context(), v, true); got != nil {
		t.Errorf("stealth with no rules written: %+v", got)
	}
	st := &Standing{Itos: &Itos{Runs: "9.1.0", Newest: "9.2.0"}, AgentRules: &AgentRules{Stale: []string{"x"}, stealth: true}}
	if want := "\n\n" + st.Itos.line() + "\n" + agentrules.StaleLine(true) + "\n\n"; !strings.Contains(printed(st), want) {
		t.Errorf("the advice lines are not one paragraph:\n%s", printed(st))
	}
	if got := emitted(t, st); !strings.Contains(got, `"agent_rules": {`) || !strings.Contains(got, `"stale": [`) {
		t.Errorf("agent_rules is not given:\n%s", got)
	}
	if got := emitted(t, &Standing{}); !strings.Contains(got, `"agent_rules": null`) {
		t.Errorf("agent_rules is not null:\n%s", got)
	}
	if got := printed(&Standing{AgentRules: &AgentRules{Stale: []string{"x"}}}); !strings.Contains(got, "\n\n"+agentrules.StaleLine(false)+"\n\n") {
		t.Errorf("the line alone:\n%s", got)
	}
	if got := printed(&Standing{}); strings.Contains(got, "itos init --agent-rules") {
		t.Errorf("a status with no stale block names itos init --agent-rules:\n%s", got)
	}
}
