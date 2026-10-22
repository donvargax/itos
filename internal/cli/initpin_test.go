package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

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
