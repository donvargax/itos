package cli

import (
	"path/filepath"
	"runtime"
	"testing"
)

// A relative hooks folder is under root and named with forward slashes; an
// absolute one is the folder itself, on Windows a drive letter's (bug 35).
func TestHookFile(t *testing.T) {
	shown, full := hookFile(".", ".git/hooks", "commit-msg")
	if shown != ".git/hooks/commit-msg" || full != filepath.Join(".", ".git", "hooks", "commit-msg") {
		t.Errorf("relative: %q, %q", shown, full)
	}
	abs := "/srv/shared/hooks"
	if runtime.GOOS == "windows" {
		abs = "C:/shared/hooks"
	}
	shown, full = hookFile(".", abs, "pre-push")
	want := filepath.Join(abs, "pre-push")
	if shown != want || full != want {
		t.Errorf("absolute: %q, %q, want %q", shown, full, want)
	}
}
