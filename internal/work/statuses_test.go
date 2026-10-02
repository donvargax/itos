package work

import (
	"os"
	"path/filepath"
	"testing"
)

func statusesOf(t *testing.T, text string) Statuses {
	t.Helper()
	file := filepath.Join(t.TempDir(), "work-items.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return ItemStatuses(file)
}

// The first item of an id wins; an item without a status, or with null, has
// none; a registry that cannot be read, or holds a null item, gives none.
func TestItemStatuses(t *testing.T) {
	s := statusesOf(t, "items:\n  - { id: T-1, status: doing }\n  - { id: T-1, status: done }\n  - { id: T-2 }\n  - { id: T-3, status: null }\n")
	if got, ok := s.Of("T-1"); !ok || got != "doing" {
		t.Errorf("T-1: got %v, %v", got, ok)
	}
	for _, id := range []string{"T-2", "T-3", "T-4"} {
		if got, ok := s.Of(id); ok {
			t.Errorf("%s: got %v", id, got)
		}
	}
	for _, text := range []string{"items:\n  - { id: T-1, status: doing }\n  - null\n", "items: {}\n", "items: [\n"} {
		if got, ok := statusesOf(t, text).Of("T-1"); ok {
			t.Errorf("%q: got %v", text, got)
		}
	}
	if _, ok := ItemStatuses(filepath.Join(t.TempDir(), "none.yaml")).Of("T-1"); ok {
		t.Error("a missing registry has a status")
	}
}
