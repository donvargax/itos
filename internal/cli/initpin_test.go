package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/release"
)

// init notes a pin strictly older than the newest release, naming itos
// upgrade (slice 115); a pin at the newest or past it, no pin, or a release
// server with no answer is no note.
func TestPinBehind(t *testing.T) {
	releaseServer(t, map[string]string{
		"/latest/download/checksums.txt": "aa  itos-9.2.0-linux-amd64.tar.gz\n",
	})
	pinning := func(v string) *config.Loaded {
		return &config.Loaded{Config: config.Config{Pin: &config.Pin{Version: v}}}
	}
	for _, c := range []struct {
		name string
		cfg  *config.Loaded
	}{
		{"no pin", &config.Loaded{}},
		{"a pin with no version", pinning("")},
		{"the newest", pinning("9.2.0")},
		{"one past the newest", pinning("9.3.0")},
	} {
		if p := pinBehind(c.cfg); p != nil {
			t.Errorf("%s: noted %+v", c.name, *p)
		}
	}
	p := pinBehind(pinning("9.1.0"))
	if p == nil {
		t.Fatal("a pin behind the newest release was not noted")
	}
	if p.Rule != "pin-behind" || !strings.Contains(p.Message, "itos 9.1.0") || !strings.Contains(p.Message, "itos 9.2.0") {
		t.Errorf("the note is %+v", *p)
	}
	if !strings.Contains(p.Fix, "itos upgrade") || strings.Contains(p.Fix, "itos pin") {
		t.Errorf("the note's fix names %q, not itos upgrade alone", p.Fix)
	}
	t.Setenv(release.Env, "http://127.0.0.1:1")
	if p := pinBehind(pinning("9.1.0")); p != nil {
		t.Errorf("with no answer from the release server, noted %+v", *p)
	}
}

// init over a config whose pin is behind lists it among its notes, in the
// area pin, never as missing.
func TestInitNotesAPinBehind(t *testing.T) {
	scratchFolder(t)
	gitIn(t, "init", "-q", "-b", "main")
	releaseServer(t, map[string]string{
		"/latest/download/checksums.txt": "aa  itos-9.2.0-linux-amd64.tar.gz\n",
	})
	writeFiles(t, map[string]string{
		"itos.yaml": "version: 1\npin: { version: 9.1.0, checksums: " + strings.Repeat("a", 64) + " }\n" +
			"ledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { people: { source: yaml, file: people.yaml } }\n",
		"tasks/phase-1.yaml":    "[]\n",
		"tasks/work-items.yaml": "phases: {}\nitems: []\n",
		"people.yaml":           "- a\n",
	})
	_, stdout, stderr := run("init", "--json")
	var report struct {
		Notes []Found `json:"notes"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("%v: stdout %q, stderr %q", err, stdout, stderr)
	}
	if len(report.Notes) != 1 || report.Notes[0].Area != "pin" || report.Notes[0].Rule != "pin-behind" {
		t.Errorf("notes %+v", report.Notes)
	}
}
