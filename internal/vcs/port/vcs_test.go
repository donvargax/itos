package port

import "testing"

// A change that does not apply says the paths it conflicts in.
func TestConflictsSayTheirPaths(t *testing.T) {
	if got, want := (&Conflicts{Paths: []string{"a.txt", "dir/b.txt"}}).Error(), "the change conflicts in a.txt, dir/b.txt"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
