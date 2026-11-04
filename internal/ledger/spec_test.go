package ledger

import (
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// The kind key wins when it is text, whatever the id says; else the id gives
// the kind, a task's only where the config has a ledger.id.
func TestKindOf(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{})
	for _, c := range []struct {
		name string
		item *value.Map
		want string
	}{
		{"a slice by its id", value.NewMap("id", "slice-9"), "slice"},
		{"a bug by its id", value.NewMap("id", "bug-9"), "bug"},
		{"a task by ledger.id", value.NewMap("id", "T-9"), "task"},
		{"an id of no kind", value.NewMap("id", "p1-idea"), ""},
		{"no id", value.NewMap("title", "No id"), ""},
		{"the key over a task's id", value.NewMap("id", "T-9", "kind", "slice"), "slice"},
		{"the key over an id of no kind", value.NewMap("id", "chore-1", "kind", "task"), "task"},
		{"a key that is not text", value.NewMap("id", "bug-9", "kind", 1.0), "bug"},
	} {
		if got := KindOf(c.item, cfg); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	cfg.Ledger.ID = nil
	if got := KindOf(value.NewMap("id", "T-9"), cfg); got != "" {
		t.Errorf("no ledger.id: %q", got)
	}
}

// A task's checks are its first entry's; an entry with no check still is an
// entry, and an id that only starts like another's is none.
func TestChecksOf(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{"tasks/phase-1.yaml": `- { id: T-1, done_when: [{ run: "go vet" }, { run: "go test" }] }
- { id: T-1, done_when: [] }
- { id: T-2, done_when: [] }
- { id: T-3 }
- { id: T-4, done_when: "go vet" }
- { id: T-50, done_when: [{ run: "go vet" }] }
`})
	for _, c := range []struct {
		id     string
		checks int
		entry  bool
	}{
		{"T-1", 2, true},
		{"T-2", 0, true},
		{"T-3", 0, true},
		{"T-4", 0, true},
		{"T-5", 0, false},
	} {
		checks, entry, err := ChecksOf(cfg, c.id)
		if err != nil || len(checks) != c.checks || entry != c.entry {
			t.Errorf("%s: %d checks, entry %v, %v; want %d, %v", c.id, len(checks), entry, err, c.checks, c.entry)
		}
	}
}

// A ledger that cannot be read is the error, not a task with no checks.
func TestChecksOfUnreadable(t *testing.T) {
	cfg := scratchLedger(t, map[string]string{"tasks/phase-1.yaml": "{ id: T-1, done_when: [\n"})
	if checks, entry, err := ChecksOf(cfg, "T-1"); err == nil || entry || checks != nil {
		t.Errorf("not YAML: %v, %v, %v", checks, entry, err)
	}
}
