//go:build linux || darwin

package adapter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// fsizeChildEnv selects the child role of TestFsizeChild: the folder the
// store is in.
const fsizeChildEnv = "ITOS_TEST_FSIZE_CHILD"

// A write the disk takes only part of is the disk's error, and the file it
// was to replace is left as it was: a file size limit (RLIMIT_FSIZE) stops
// the write past its first bytes, as a full disk would, with the same
// error on every system that has the limit. Go ignores the SIGXFSZ the
// kernel sends with it. The limit is a process's, so the limited write runs
// in a child of this test binary (TestFsizeChild): lowered here, it would
// fail every other write of the process, the testing package's own log and
// the coverage profile among them.
func TestStoreKeepsTheFileAWriteFailsToReplace(t *testing.T) {
	dir := clone(t, true)
	store := Store{VCS: vcs.Git{Dir: dir}}
	if err := store.Write(t.Context(), "threads.yaml", []byte("old\n")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFsizeChild$")
	child.Env = append(slices.DeleteFunc(os.Environ(), func(entry string) bool {
		return strings.HasPrefix(entry, "GOCOVERDIR=") || strings.HasPrefix(entry, fsizeChildEnv+"=")
	}), fsizeChildEnv+"="+dir)
	out, err := child.CombinedOutput()
	if err != nil {
		t.Errorf("the child did not see its write cut short: %v\n%s", err, out)
	}
	path, _ := store.Path(t.Context(), "threads.yaml")
	if text, err := os.ReadFile(path); err != nil || string(text) != "old\n" {
		t.Errorf("the file reads %q, %v", text, err)
	}
}

// TestFsizeChild is the child of TestStoreKeepsTheFileAWriteFailsToReplace,
// and does nothing without the folder in fsizeChildEnv. It lowers its own
// file size limit and writes: it exits 0 when the write failed with the
// limit's error, else 1 or 2 with a line saying what happened.
func TestFsizeChild(t *testing.T) {
	dir := os.Getenv(fsizeChildEnv)
	if dir == "" {
		return
	}
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		fmt.Println("the file size limit cannot be read:", err)
		os.Exit(2)
	}
	limit.Cur = 4
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		fmt.Println("the file size limit cannot be lowered:", err)
		os.Exit(2)
	}
	err := Store{VCS: vcs.Git{Dir: dir}}.Write(context.Background(), "threads.yaml", []byte("longer than the limit\n"))
	if !errors.Is(err, syscall.EFBIG) {
		fmt.Printf("the write gave %v, not %v\n", err, syscall.EFBIG)
		os.Exit(1)
	}
	os.Exit(0)
}
