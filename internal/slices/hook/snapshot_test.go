package hook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/message"
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

// The snapshot fake supplies the caller's raw environment. Only the real
// tasksRule may remove commit-specific footer state before executing the check.
func TestTasksRuleScrubsHandedFootersFromExecutedCheck(t *testing.T) {
	p := staging(t, nil, "docs: x\n\nTask: T-2\n")
	t.Setenv(message.FootersEnv, "Task: T-9")
	t.Setenv(message.AmendEnv, "1")
	t.Setenv("ITOS_KEPT", "ordinary-value")
	observed := filepath.Join(t.TempDir(), "environment-checked")
	t.Setenv("ITOS_ENV_OBSERVATION", filepath.ToSlash(observed))
	command := `test "${ITOS_FOOTERS+x}" != x && test "${ITOS_AMEND+x}" != x && test "$ITOS_KEPT" = ordinary-value && printf checked > "$ITOS_ENV_OBSERVATION"`
	p.trees.Files["index"]["tasks/phase-1.yaml"] = "- { id: T-2, type: docs, title: Done, done_when: [{ run: '" + command + "', cost: static }] }\n"
	var output strings.Builder
	answer, err := NewHandler(p).tasksRule(t.Context(), p.cfg, "docs: x\n\nTask: T-2\n", Out{Stderr: &output})
	if err != nil || answer.Code != 0 || output.Len() != 0 {
		t.Fatalf("executed environment check = %+v, %v, output %q", answer, err, output.String())
	}
	data, err := os.ReadFile(observed)
	if err != nil || string(data) != "checked" {
		t.Fatalf("check did not record its environment assertion: %q, %v", data, err)
	}
}
