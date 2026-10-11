package cli

import (
	"slices"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
)

// The IDs itos task names as unknown are the words ledger.id matches, every
// word that is no flag with no ledger.id (slice 130), never a flag's value.
func TestUnknownIDs(t *testing.T) {
	tasks := []ledger.Task{{ID: "T-001"}}
	args := []string{"T-001", "OPS-8", "--pending", "T-099", "--skip", "T-9", "--group", "7", "--phase=3"}
	cfg := &config.Loaded{}
	cfg.Ledger.Group.Label = "phase"
	if got, want := unknownIDs(cfg, args, tasks), []string{"OPS-8", "T-099"}; !slices.Equal(got, want) {
		t.Errorf("with no ledger.id: got %q, want %q", got, want)
	}
	id := `T-\d+`
	cfg.Ledger.ID = &id
	if got, want := unknownIDs(cfg, args, tasks), []string{"T-099"}; !slices.Equal(got, want) {
		t.Errorf("with ledger.id %s: got %q, want %q", id, got, want)
	}
}
