package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/out"
)

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
	if err := os.MkdirAll(filepath.Dir(installScriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(installScriptPath, []byte(text), 0o755); err != nil {
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
	if err := os.Remove(installScriptPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(installScriptPath, 0o755); err != nil {
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
	t.Setenv("CI", "")
	t.Setenv("ITOS_NO_UPDATE", "")
	latest := func(v string) {
		releaseServer(t, map[string]string{"/latest/download/checksums.txt": "aa  itos-" + v + "-linux-amd64.tar.gz\n"})
	}
	latest("9.2.0")
	if got := newestItos("9.1.0"); got == nil || *got != (itosStanding{Runs: "9.1.0", Newest: "9.2.0"}) {
		t.Errorf("within the major: %+v", got)
	}
	if got := newestItos(""); got != nil {
		t.Errorf("running no version named: %+v", got)
	}
	for _, name := range []string{"CI", "ITOS_NO_UPDATE"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "1")
			if got := newestItos("9.1.0"); got != nil {
				t.Errorf("with %s set: %+v", name, got)
			}
		})
	}
	latest("10.0.0")
	if got := newestItos("9.1.0"); got == nil || *got != (itosStanding{Runs: "9.1.0", Newest: "10.0.0", NewMajor: true}) {
		t.Errorf("a newer major: %+v", got)
	}
	if got := newestItos("10.0.0-rc.1"); got == nil || got.NewMajor {
		t.Errorf("a pre-release of the newest's major: %+v", got)
	}
	latest("11.0.0-rc.1")
	if got := newestItos("9.1.0"); got != nil {
		t.Errorf("a pre-release as the newest: %+v", got)
	}
	releaseServer(t, map[string]string{"/latest/download/checksums.txt": "aa  itos.schema.json\n"})
	if got := newestItos("9.1.0"); got != nil {
		t.Errorf("an answer naming no archive: %+v", got)
	}
	releaseServer(t, map[string]string{"/latest/download/checksums.txt": "fail"})
	if got := newestItos("9.1.0"); got != nil {
		t.Errorf("a server that fails: %+v", got)
	}
}

// The itos line names both versions, and itos upgrade with the notes only
// for a newer major; --json gives it as itos, null without one.
func TestItosLine(t *testing.T) {
	same := &itosStanding{Runs: "9.1.0", Newest: "9.2.0"}
	if line := same.line(); !strings.Contains(line, "9.1.0") || !strings.Contains(line, "9.2.0") ||
		strings.Contains(line, "itos upgrade") {
		t.Errorf("within the major: %q", line)
	}
	major := &itosStanding{Runs: "9.1.0", Newest: "10.0.0", NewMajor: true}
	if line := major.line(); !strings.Contains(line, "10.0.0") || !strings.Contains(line, "itos upgrade") ||
		!strings.Contains(line, "/tag/v10.0.0") {
		t.Errorf("a newer major: %q", line)
	}
	st := &standing{Itos: major}
	var text strings.Builder
	st.print(&text)
	if !strings.Contains(text.String(), "\n"+major.line()+"\n") {
		t.Errorf("the status does not print the itos line:\n%s", text.String())
	}
	for _, want := range []*itosStanding{major, nil} {
		var b strings.Builder
		if err := out.Emit(&b, (&standing{Itos: want}).fields()...); err != nil {
			t.Fatal(err)
		}
		var got struct {
			Itos *struct {
				Runs     *string `json:"runs"`
				Newest   *string `json:"newest"`
				NewMajor *bool   `json:"new_major"`
			} `json:"itos"`
		}
		if err := json.Unmarshal([]byte(b.String()), &got); err != nil {
			t.Fatal(err)
		}
		switch {
		case want == nil && got.Itos != nil:
			t.Errorf("itos is %+v, not null", *got.Itos)
		case want != nil && (got.Itos == nil || got.Itos.Runs == nil || *got.Itos.Runs != want.Runs ||
			got.Itos.Newest == nil || *got.Itos.Newest != want.Newest ||
			got.Itos.NewMajor == nil || *got.Itos.NewMajor != want.NewMajor):
			t.Errorf("itos is not %+v:\n%s", *want, b.String())
		}
	}
}
