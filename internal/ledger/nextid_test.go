package ledger

import "testing"

func TestNextIDAfterUsesCounterFloorAndPreservesSeriesWidth(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{
		"tasks/phase-1.yaml": "- { id: T-007, type: chore, title: Seven }\n",
	})
	got, err := NextIDAfter(cfg, []string{"T-009", "OTHER-999", "T-invalid"}, 12)
	if err != nil || got != "T-013" {
		t.Fatalf("NextIDAfter = %q, %v, want T-013", got, err)
	}
}

func TestNextIDAfterInfersPrefixAndWidthFromHighestMatchingID(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{
		"tasks/phase-1.yaml": "- { id: TEAM-004, type: chore, title: Four }\n",
	})
	pattern := `.*-\d{3}`
	cfg.Ledger.ID = &pattern
	got, err := NextIDAfter(cfg, []string{"OPS-009", "TEAM-010", "not-numbered"}, 0)
	if err != nil || got != "TEAM-011" {
		t.Fatalf("NextIDAfter = %q, %v, want TEAM-011", got, err)
	}
}

func TestNextIDAfterStartsKnownSeriesAndRejectsUnknownSeries(t *testing.T) {
	t.Run("known prefix starts at one", func(t *testing.T) {
		cfg := scratchLedger(t, map[string]string{"tasks/phase-1.yaml": "[]\n"})
		got, err := NextIDAfter(cfg, nil, 0)
		if err != nil || got != "T-1" {
			t.Fatalf("NextIDAfter = %q, %v, want T-1", got, err)
		}
	})
	t.Run("no literal prefix and no numbered IDs", func(t *testing.T) {
		cfg := scratchLedger(t, map[string]string{"tasks/phase-1.yaml": "[]\n"})
		pattern := `.*-\d+`
		cfg.Ledger.ID = &pattern
		if got, err := NextIDAfter(cfg, []string{"idea"}, 12); err == nil || got != "" {
			t.Fatalf("NextIDAfter = %q, %v, want empty ID and error", got, err)
		}
	})
}
