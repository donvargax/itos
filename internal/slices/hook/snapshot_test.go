package hook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type snapshotPorts struct {
	*ports
	snapshot    Snapshot
	snapshotErr error
	snapshots   int
}

func (p *snapshotPorts) TaskSnapshot(context.Context, string) (Snapshot, error) {
	p.snapshots++
	return p.snapshot, p.snapshotErr
}

func TestTasksRuleDoesNotSetUpSnapshotWithoutRunnableChecks(t *testing.T) {
	for _, checks := range []string{"[]", "[{ run: 'true', cost: late }]", "[{ run: 'true', cost: static, after: push }]"} {
		t.Run(checks, func(t *testing.T) {
			p := staging(t, nil, "docs: x\n\nTask: T-1\n")
			p.trees.Files["index"]["tasks/phase-1.yaml"] = "- { id: T-1, type: docs, title: One, done_when: " + checks + " }\n"
			s := &snapshotPorts{ports: p, snapshotErr: errors.New("snapshot must not be attempted")}
			answer, err := NewHandler(s).tasksRule(t.Context(), p.cfg, "docs: x\n\nTask: T-1\n", Out{})
			if err != nil || answer.Code != 0 || s.snapshots != 0 {
				t.Fatalf("no runnable check = %+v, %v, %d snapshots", answer, err, s.snapshots)
			}
		})
	}
}

func TestTasksRuleReturnsSnapshotSetupFailure(t *testing.T) {
	p := staging(t, nil, "docs: x\n\nTask: T-1\n")
	refused := errors.New("checkout setup refused")
	s := &snapshotPorts{ports: p, snapshotErr: refused}
	var output strings.Builder
	answer, err := NewHandler(s).tasksRule(t.Context(), p.cfg, "docs: x\n\nTask: T-1\n", Out{Stderr: &output})
	if !errors.Is(err, refused) || answer.Code != 0 || s.snapshots != 1 || output.Len() != 0 {
		t.Fatalf("setup error = %+v, %v, %d snapshots, output %q", answer, err, s.snapshots, output.String())
	}
}

func TestTasksRuleReleasesSnapshotWhenOutputCaptureFails(t *testing.T) {
	p := staging(t, nil, "docs: x\n\nTask: T-1\n")
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	removed := 0
	s := &snapshotPorts{ports: p, snapshot: Snapshot{Dir: t.TempDir(), TempDir: blocker, Remove: func() error { removed++; return nil }}}
	var output strings.Builder
	answer, err := NewHandler(s).tasksRule(t.Context(), p.cfg, "docs: x\n\nTask: T-1\n", Out{Stderr: &output})
	if err == nil || answer.Code != 0 || removed != 1 || s.snapshots != 1 || output.Len() != 0 {
		t.Fatalf("capture error = %+v, %v, %d releases, %d snapshots, output %q", answer, err, removed, s.snapshots, output.String())
	}
}
