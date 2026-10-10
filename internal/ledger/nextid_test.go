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

func TestNextIDAfterTreatsOneAsAnAlreadyClaimedFloor(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{"tasks/phase-1.yaml": "[]\n"})
	got, err := NextIDAfter(cfg, nil, 1)
	if err != nil || got != "T-2" {
		t.Fatalf("NextIDAfter = %q, %v, want T-2", got, err)
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

func TestNextIDAfterInfersPrefixWhenTheOnlyNumberIsZero(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{
		"tasks/phase-1.yaml": "- { id: TEAM-000, type: chore, title: Zero }\n",
	})
	pattern := `.*-\d+`
	cfg.Ledger.ID = &pattern
	got, err := NextIDAfter(cfg, nil, 0)
	if err != nil || got != "TEAM-001" {
		t.Fatalf("NextIDAfter = %q, %v, want TEAM-001", got, err)
	}
}

// With no fixed start, the series is the one whose number is highest: the
// first of them on a tie, and a number too large to read counts for none.
func TestNextIDAfterInfersPrefixFromTheFirstHighestReadableNumber(t *testing.T) {
	for _, test := range []struct {
		name   string
		others []string
		want   string
	}{
		{name: "tie", others: []string{"B-5"}, want: "A-6"},
		{name: "unreadable number", others: []string{"B-99999999999999999999"}, want: "A-6"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := scratchLedger(t, map[string]string{
				"tasks/phase-1.yaml": "- { id: A-5, type: chore, title: Five }\n",
			})
			pattern := `.*-\d+`
			cfg.Ledger.ID = &pattern
			got, err := NextIDAfter(cfg, test.others, 0)
			if err != nil || got != test.want {
				t.Fatalf("NextIDAfter = %q, %v, want %s", got, err, test.want)
			}
		})
	}
}

// A fresh series is written as wide as ledger.id asks, up to nine digits.
func TestNextIDAfterStartsAFreshSeriesNineDigitsWide(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{"tasks/phase-1.yaml": "[]\n"})
	pattern := `T-\d{9}`
	cfg.Ledger.ID = &pattern
	got, err := NextIDAfter(cfg, nil, 0)
	if err != nil || got != "T-000000001" {
		t.Fatalf("NextIDAfter = %q, %v, want T-000000001", got, err)
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
