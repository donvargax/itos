//go:build linux || darwin

package adapter

import (
	"os"
	"syscall"
	"testing"

	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// A write the disk takes only part of is the disk's error, and the file it
// was to replace is left as it was: a file size limit (RLIMIT_FSIZE) stops
// the write past its first bytes, as a full disk would, with the same
// error on every system that has the limit. Go ignores the SIGXFSZ the
// kernel sends with it.
func TestStoreKeepsTheFileAWriteFailsToReplace(t *testing.T) {
	clone(t, true)
	store := Store{VCS: vcs.Git{}}
	if err := store.Write(t.Context(), "threads.yaml", []byte("old\n")); err != nil {
		t.Fatal(err)
	}
	var was syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &was); err != nil {
		t.Fatal(err)
	}
	limited := was
	limited.Cur = 4
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limited); err != nil {
		t.Skipf("the file size limit cannot be lowered: %v", err)
	}
	err := store.Write(t.Context(), "threads.yaml", []byte("longer than the limit\n"))
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &was); err != nil {
		t.Fatal(err)
	}
	if err == nil {
		t.Error("a write cut short gave no error")
	}
	path, _ := store.Path(t.Context(), "threads.yaml")
	if text, err := os.ReadFile(path); err != nil || string(text) != "old\n" {
		t.Errorf("the file reads %q, %v", text, err)
	}
}
