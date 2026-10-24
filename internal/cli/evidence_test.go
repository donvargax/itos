package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/evidence"
)

// The engine of a hand-run itos task names --working-tree when it leaves
// changes out and applies no ci.env; ci run's applies ci.env, always a map.
func TestNewEvidenceForTaskAndCI(t *testing.T) {
	gitConfigRepo(t, "version: 1\nci: { steps: [], env: { A: \"1\" } }\n")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	task := newEvidence(cfg, "task")
	if task.CIEnv != nil || task.Mode != "task" || !strings.Contains(task.Hint, "--working-tree") || task.Cfg != cfg || task.Binding != evidence.ThisBinary() {
		t.Errorf("task: %+v", task)
	}
	ci := newEvidence(cfg, "ci")
	if ci.CIEnv["A"] != "1" || ci.Mode != "ci" || ci.Hint != "" {
		t.Errorf("ci: %+v", ci)
	}
	bare := *cfg
	bare.CI.Env = nil
	if got := newEvidence(&bare, "ci"); got.CIEnv == nil || len(got.CIEnv) != 0 {
		t.Errorf("ci with no ci.env: %#v", got.CIEnv)
	}
}

// The ports bind the engine to the repository: an opted-in check is looked
// up by the index tree, run in a checkout of it and recorded in the common
// dir's store, then answered from it.
func TestEvidenceRunsOnTheIndexAndRecordsInTheCommonDir(t *testing.T) {
	scratchFolder(t)
	gitIn(t, "init", "-q")
	writeFiles(t, map[string]string{"input": "staged\n", "itos.yaml": "version: 1\n"})
	gitIn(t, "add", "--", "input", "itos.yaml")
	writeFiles(t, map[string]string{"input": "edited\n", "untracked": "x\n"})
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	e := newEvidence(cfg, "task")
	var log strings.Builder
	req := evidence.Request{Command: "cat input; test ! -e untracked", Log: &log}
	first := e.Run(t.Context(), req)
	if !first.OK || !first.Info.Recorded || first.Info.Hit || first.Info.Tree == "" {
		t.Fatalf("first: %+v\n%s", first, log.String())
	}
	if n, counted := e.Unstaged(); n != 1 || !counted || !strings.Contains(log.String(), "1 changed path not staged") {
		t.Errorf("unstaged %d %v: %s", n, counted, log.String())
	}
	records, err := filepath.Glob(filepath.Join(".git", "itos", "evidence", first.Info.Key, "complete"))
	if err != nil || len(records) != 1 {
		t.Errorf("records %v, %v", records, err)
	}
	again := newEvidence(cfg, "task").Run(t.Context(), req)
	if !again.Info.Hit || again.Info.Key != first.Info.Key {
		t.Errorf("again: %+v", again)
	}
	if left, _ := os.ReadDir(filepath.Join(".git", "itos", "snapshots")); len(left) != 0 {
		t.Errorf("checkouts left: %v", left)
	}
}
