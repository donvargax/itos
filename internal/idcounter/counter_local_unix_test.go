//go:build unix

package idcounter

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A counter that was read but cannot be written back reserves nothing. The
// counter file is a named pipe, so the test knows when Mint is reading it and
// can put a directory in its place before the read ends; the replacement of
// the file then fails.
func TestMintLocalReturnsNoNumberWhenTheCounterCannotBeWritten(t *testing.T) {
	common := t.TempDir()
	dir := filepath.Join(common, "itos")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, localFile)
	if err := syscall.Mkfifo(file, 0o600); err != nil {
		t.Fatal(err)
	}
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := Mint(Store{CommonDir: common, LocalOnly: true}, "slice", 0)
		done <- result{n, err}
	}()
	// Opening the pipe to write waits until Mint opens it to read.
	w, err := os.OpenFile(file, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("counters:\n  slice: 4\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if r := <-done; r.err == nil || r.n != 0 {
		t.Fatalf("Mint = (%d, %v), want (0, error)", r.n, r.err)
	}
}
