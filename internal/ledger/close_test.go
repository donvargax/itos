package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
)

// A ledger in a scratch folder, its files written, and its config loaded.
func scratchLedger(t *testing.T, files map[string]string) *config.Loaded {
	t.Helper()
	t.Chdir(t.TempDir())
	files["itos.yaml"] = "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n"
	for path, text := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Load("itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// The task's done_when goes, in a block ledger and a flow one, with CRLF
// line breaks kept, the rest of the text as it was; its why stays.
func TestWithoutChecks(t *testing.T) {
	block := "# the first phase\n- id: T-001\n  type: docs\n  title: One\n  done_when:\n    - run: \"true\" # passes\n    - fails: grep -q x y\n      timeout: 5\n\n" +
		"- id: T-002\n  type: docs\n  title: Two\n  why: >\n    Because.\n  done_when:\n    - run: echo two\n  # a note after\n- id: T-003\n  type: docs\n  title: Three\n"
	cases := []struct {
		name, id, file, text, want string
	}{
		{"a block task, a blank line after it", "T-001", "tasks/phase-1.yaml", block,
			"# the first phase\n- id: T-001\n  type: docs\n  title: One\n\n" +
				"- id: T-002\n  type: docs\n  title: Two\n  why: >\n    Because.\n  done_when:\n    - run: echo two\n  # a note after\n- id: T-003\n  type: docs\n  title: Three\n"},
		{"a block task's last key, its why kept", "T-002", "tasks/phase-1.yaml", block,
			"# the first phase\n- id: T-001\n  type: docs\n  title: One\n  done_when:\n    - run: \"true\" # passes\n    - fails: grep -q x y\n      timeout: 5\n\n" +
				"- id: T-002\n  type: docs\n  title: Two\n  why: >\n    Because.\n  # a note after\n- id: T-003\n  type: docs\n  title: Three\n"},
		{"a flow task", "T-001", "tasks/phase-1.yaml",
			"- { id: T-001, type: chore, title: Tidy, done_when: [{ run: \"true\" }], why: \"The ledger says why.\" }\n- { id: T-002, type: chore, title: Sweep }\n",
			"- { id: T-001, type: chore, title: Tidy, why: \"The ledger says why.\" }\n- { id: T-002, type: chore, title: Sweep }\n"},
		{"CRLF line breaks", "T-001", "tasks/phase-1.yaml",
			"- id: T-001\r\n  type: docs\r\n  title: One\r\n  done_when:\r\n    - run: \"true\"\r\n  why: Kept.\r\n- id: T-002\r\n  type: docs\r\n  title: Two\r\n",
			"- id: T-001\r\n  type: docs\r\n  title: One\r\n  why: Kept.\r\n- id: T-002\r\n  type: docs\r\n  title: Two\r\n"},
		{"a task in a later file", "T-009", "tasks/phase-2.yaml",
			"- id: T-009\n  type: docs\n  title: Nine\n  done_when:\n    - run: \"true\"\n",
			"- id: T-009\n  type: docs\n  title: Nine\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{c.file: c.text}
			if c.file != "tasks/phase-1.yaml" {
				files["tasks/phase-1.yaml"] = "- { id: T-001, type: docs, title: One, done_when: [{ run: \"true\" }] }\n"
			}
			cfg := scratchLedger(t, files)
			edited, ok, err := WithoutChecks(cfg, c.id)
			if err != nil || !ok {
				t.Fatalf("ok %v, err %v", ok, err)
			}
			if edited.Path != c.file || edited.Old != c.text {
				t.Errorf("edited %s from %q, not %s from %q", edited.Path, edited.Old, c.file, c.text)
			}
			if edited.Text != c.want {
				t.Errorf("got  %q\nwant %q", edited.Text, c.want)
			}
		})
	}
}

// A task with no checks, an empty done_when, or none in the ledger leaves
// nothing to write.
func TestWithoutChecksNone(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{
		"tasks/phase-1.yaml": "- { id: T-001, type: docs, title: One }\n- { id: T-002, type: docs, title: Two, done_when: [] }\n",
	})
	for _, id := range []string{"T-001", "T-002", "T-404"} {
		if edited, ok, err := WithoutChecks(cfg, id); ok || err != nil || edited != (Edited{}) {
			t.Errorf("%s: %+v, %v, %v", id, edited, ok, err)
		}
	}
}

// A done_when the edit cannot cut in place (on its item's dash line) is an
// error naming the file, nothing to write.
func TestWithoutChecksUneditable(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{
		"tasks/phase-1.yaml": "- done_when: [{ run: \"true\" }]\n  id: T-001\n  type: docs\n  title: One\n",
	})
	if _, ok, err := WithoutChecks(cfg, "T-001"); ok || err == nil {
		t.Errorf("ok %v, err %v", ok, err)
	}
}
