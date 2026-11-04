package ledger

import (
	"context"
	"errors"
	"testing"

	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// A group's file is the ledger's whose group it is, a numeric group by its
// number, else the one ledger.files names; none where it cannot name one.
func TestGroupFile(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{})
	files := []File{{Path: "tasks/phase-01.yaml", Group: "01"}}
	cfg.Ledger.Group.Numeric = true
	if got := GroupFile(cfg, files, "1"); got != "tasks/phase-01.yaml" {
		t.Errorf("a numeric group's file: %q", got)
	}
	if got := GroupFile(cfg, files, "2"); got != "tasks/phase-2.yaml" {
		t.Errorf("a group with no file: %q", got)
	}
	if got := GroupFile(cfg, files, ""); got != "" {
		t.Errorf("no group: %q", got)
	}
	cfg.Ledger.Files = "tasks/all.yaml"
	if got := GroupFile(cfg, nil, "2"); got != "" {
		t.Errorf("a ledger.files with no {group}: %q", got)
	}
}

// The new task goes after the last of its group's file as load reads it, or
// in a file made for a group that has none, its text before none; a load
// that fails is its error, nothing added.
func TestAddReadsTheFileThroughLoad(t *testing.T) {
	one := "- { id: T-001, type: docs, title: One }\n"
	cfg := scratchLedger(t, map[string]string{"tasks/phase-1.yaml": one})
	added, found, err := Add(cfg, NewTask{ID: "T-002", Type: "docs", Title: "Two", Group: "1"}, disk)
	if err != nil || len(found) > 0 {
		t.Fatalf("found %v, err %v", found, err)
	}
	if added.Path != "tasks/phase-1.yaml" || added.Old != one || added.Created || added.Text != one+"- id: T-002\n  type: docs\n  title: Two\n" {
		t.Errorf("added to its group's file: %+v", added)
	}
	added, found, err = Add(cfg, NewTask{ID: "T-002", Type: "docs", Title: "Two", Group: "2"}, disk)
	if err != nil || len(found) > 0 {
		t.Fatalf("found %v, err %v", found, err)
	}
	if added.Path != "tasks/phase-2.yaml" || added.Old != "" || !added.Created {
		t.Errorf("added to a new group's file: %+v", added)
	}
	refused := errors.New("refused")
	failing := func(context.Context, []string) ([]trackerport.Doc, error) { return nil, refused }
	if added, found, err := Add(cfg, NewTask{ID: "T-002", Type: "docs", Title: "Two", Group: "1"}, failing); !errors.Is(err, refused) || len(found) > 0 || added.Path != "" {
		t.Errorf("a failing load: %+v, %v, %v", added, found, err)
	}
}
