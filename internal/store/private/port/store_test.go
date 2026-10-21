package port

import "testing"

// No repository says so.
func TestNoRepositorySaysSo(t *testing.T) {
	if got := (&NoRepository{}).Error(); got != "no git repository" {
		t.Errorf("NoRepository says %q", got)
	}
}
