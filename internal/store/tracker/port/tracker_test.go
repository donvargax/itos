package port

import (
	"errors"
	"testing"
)

// A dirty document names itself.
func TestDirtySaysItsName(t *testing.T) {
	if got := (&Dirty{Name: "registry.yaml"}).Error(); got != "registry.yaml has changes no commit holds" {
		t.Errorf("Dirty says %q", got)
	}
}

// A refused step says its step and code, and why a document could not be
// put back, which it wraps.
func TestRefusedSaysItsStep(t *testing.T) {
	if got := (&Refused{Step: "the commit of a", Code: 1}).Error(); got != "the commit of a failed (exit 1)" {
		t.Errorf("Refused says %q", got)
	}
	restore := errors.New("a cannot be put back")
	r := &Refused{Step: "git add of a", Code: 128, Restore: restore}
	if got := r.Error(); got != "git add of a failed (exit 128): a cannot be put back" || !errors.Is(r, restore) {
		t.Errorf("Refused says %q", got)
	}
}
