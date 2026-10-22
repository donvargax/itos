package ledger

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	trackeradapter "github.com/donvargax/itos/v8/internal/store/tracker/adapter"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// disk is the tracker's Load from the disk, as internal/cli binds it.
var disk = trackeradapter.Files{}.Load

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
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, "itos.yaml")
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
			edited, ok, err := WithoutChecks(cfg, c.id, disk)
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
		if edited, ok, err := WithoutChecks(cfg, id, disk); ok || err != nil || edited != (Edited{}) {
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
	if _, ok, err := WithoutChecks(cfg, "T-001", disk); ok || err == nil {
		t.Errorf("ok %v, err %v", ok, err)
	}
}

// A file load cannot read is load's error, and one gone since the ledger
// was listed an error naming it as not there, nothing to write.
func TestWithoutChecksUnread(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{
		"tasks/phase-1.yaml": "- { id: T-001, type: docs, title: One, done_when: [{ run: \"true\" }] }\n",
	})
	refused := errors.New("refused")
	failing := func(context.Context, []string) ([]trackerport.Doc, error) { return nil, refused }
	if edited, ok, err := WithoutChecks(cfg, "T-001", failing); ok || !errors.Is(err, refused) || edited != (Edited{}) {
		t.Errorf("a failing load: %+v, %v, %v", edited, ok, err)
	}
	gone := func(_ context.Context, names []string) ([]trackerport.Doc, error) {
		return []trackerport.Doc{{Name: names[0]}}, nil
	}
	edited, ok, err := WithoutChecks(cfg, "T-001", gone)
	if ok || !errors.Is(err, fs.ErrNotExist) || !strings.Contains(fmt.Sprint(err), "tasks/phase-1.yaml") || edited != (Edited{}) {
		t.Errorf("a file gone: %+v, %v, %v", edited, ok, err)
	}
}
