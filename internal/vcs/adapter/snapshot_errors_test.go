package adapter

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
)

const snapshotGitFault = "ITOS_VCS_TEST_SNAPSHOT_GIT_FAULT"
const snapshotRealGit = "ITOS_VCS_TEST_SNAPSHOT_REAL_GIT"

// runSnapshotGit is the test binary acting as git. It forwards to real git,
// failing only the requested setup command, or making alternates unwritable by
// putting a directory at its file path after a successful init.
func runSnapshotGit(fault string) int {
	args := os.Args[1:]
	command := args
	// The snapshot's own commands run in its folder (-C dir); the commands
	// that read the source run in the folder itos runs in.
	inSnapshot := len(command) > 2 && command[0] == "-C"
	if inSnapshot {
		command = command[2:]
	}
	fail := strings.Join(command, " ") == fault
	switch fault {
	case "config --local":
		fail = inSnapshot && len(command) > 2 && command[0] == "config" && command[1] == "--local" && command[2] != "--add"
	case "config --local --add":
		fail = inSnapshot && len(command) > 2 && command[0] == "config" && command[1] == "--local" && command[2] == "--add"
	case "config --null":
		fail = len(command) > 1 && command[0] == "config" && slices.Contains(command, "--null")
	case "symbolic-ref HEAD":
		fail = inSnapshot && len(command) > 1 && command[0] == "symbolic-ref" && command[1] == "HEAD"
	case "symbolic-ref ref":
		fail = inSnapshot && len(command) > 1 && command[0] == "symbolic-ref" && command[1] != "HEAD"
	case "init", "read-tree", "checkout-index", "for-each-ref", "update-ref":
		fail = len(command) > 0 && command[0] == fault
	}
	if fail {
		fmt.Fprintln(os.Stderr, "snapshot setup command refused")
		return 93
	}
	cmd := exec.Command(os.Getenv(snapshotRealGit), args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintln(os.Stderr, err)
		return 94
	}
	if len(command) > 0 && command[0] == "init" {
		dir := command[len(command)-1]
		blocked := map[string]string{
			"alternates-directory":  filepath.Join(dir, ".git", "objects", "info", "alternates"),
			"packed-refs-directory": filepath.Join(dir, ".git", "packed-refs"),
		}[fault]
		if blocked != "" {
			if err := os.MkdirAll(blocked, 0o755); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 95
			}
		}
	}
	return 0
}

func snapshotRepo(t *testing.T) (Git, string, string) {
	t.Helper()
	dir := alone(t)
	gitIn(t, dir, "", "init", "-q")
	write(t, dir, "input", "staged")
	gitIn(t, dir, "", "add", "input")
	g := Git{Dir: dir}
	tree, err := g.IndexTree(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return g, tree, filepath.Join(dir, ".git", "itos", "snapshots")
}

func TestTaskSnapshotCommandFailures(t *testing.T) {
	for _, fault := range []string{
		"rev-parse --path-format=absolute --git-common-dir",
		"ls-tree -r -z", // the tree argument is appended below
		"rev-parse --local-env-vars",
		"rev-parse --path-format=absolute --git-path objects",
		"rev-parse --show-object-format",
		"init", "config --local", "alternates-directory", "read-tree", "checkout-index",
	} {
		t.Run(fault, func(t *testing.T) {
			g, tree, base := snapshotRepo(t)
			// Forward to the executable setup actually used, including ITOS_GIT.
			// Real() searches PATH afresh and ignores that inherited executable.
			real := g.git().Command(t.Context(), "--version").Path
			useFakeGit(t, "unused")
			if fault == "ls-tree -r -z" {
				fault += " " + tree
			}
			t.Setenv(snapshotGitFault, fault)
			t.Setenv(snapshotRealGit, real)
			if fault == "checkout-index" {
				t.Setenv("PATH", t.TempDir())
			}
			snapshot, err := g.TaskSnapshot(t.Context(), tree)
			if err == nil || kind.Of(err) != kind.Missing {
				t.Fatalf("setup = %+v, %v", snapshot, err)
			}
			if snapshot.Dir != "" || snapshot.Remove != nil {
				t.Errorf("failed setup returned a usable checkout: %+v", snapshot)
			}
			if fault != "rev-parse --path-format=absolute --git-common-dir" && !strings.Contains(err.Error(), "cannot create task-check snapshot") {
				t.Errorf("missing setup context: %v", err)
			}
			entries, readErr := os.ReadDir(base)
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			if len(entries) != 0 {
				t.Errorf("partial resources remain: %v", entries)
			}
			text, readErr := os.ReadFile(filepath.Join(g.Dir, "input"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(text) != "staged\n" {
				t.Errorf("source changed: %q", text)
			}
		})
	}
}

func TestTaskSnapshotTemporaryDirectoryFailure(t *testing.T) {
	g, tree, base := snapshotRepo(t)
	refused := errors.New("temporary checkout creation refused")
	_, err := g.taskSnapshotThrough(t.Context(), tree, func(dir, pattern string) (string, error) {
		if dir != base || pattern != "run-" {
			t.Errorf("temporary checkout at %q / %q", dir, pattern)
		}
		return "", refused
	})
	if !errors.Is(err, refused) || kind.Of(err) != kind.Missing {
		t.Fatalf("creation error = %v", err)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("partial checkout remains: %v", entries)
	}
}

type failingIndexFile struct {
	*os.File
	writeErr, closeErr error
}

func (f failingIndexFile) Write(data []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.File.Write(data)
}
func (f failingIndexFile) Close() error { return errors.Join(f.File.Close(), f.closeErr) }

func TestIndexTreeCopyFailures(t *testing.T) {
	for _, fault := range []string{"create", "write", "close"} {
		t.Run(fault, func(t *testing.T) {
			dir := alone(t)
			gitIn(t, dir, "", "init", "-q")
			write(t, dir, "input", "staged")
			gitIn(t, dir, "", "add", "input")
			refused := errors.New("index copy " + fault + " refused")
			var name string
			copies := t.TempDir()
			create := func() (indexFile, error) {
				if fault == "create" {
					return nil, refused
				}
				file, err := os.CreateTemp(copies, "index-*")
				if err != nil {
					return nil, err
				}
				name = file.Name()
				f := failingIndexFile{File: file}
				if fault == "write" {
					f.writeErr = refused
				}
				if fault == "close" {
					f.closeErr = refused
				}
				return f, nil
			}
			tree, err := (Git{Dir: dir}).indexTreeThrough(t.Context(), create)
			if tree != "" || !errors.Is(err, refused) {
				t.Fatalf("capture = %q, %v", tree, err)
			}
			if kind.Of(err) != kind.Missing {
				t.Errorf("index setup failure kind = %v, want missing environment", kind.Of(err))
			}
			if name != "" {
				if _, err := os.Stat(name); !os.IsNotExist(err) {
					t.Errorf("index copy remains: %v", err)
				}
			}
		})
	}
}

func TestIndexTreeMissingAndUnreadableIndex(t *testing.T) {
	for _, state := range []string{"outside", "missing", "directory", "corrupt", "mktree-fails"} {
		t.Run(state, func(t *testing.T) {
			dir := alone(t)
			g := Git{Dir: dir}
			if state != "outside" {
				gitIn(t, dir, "", "init", "-q")
			}
			if state == "directory" {
				t.Setenv("GIT_INDEX_FILE", t.TempDir())
			}
			if state == "corrupt" {
				write(t, filepath.Join(dir, ".git"), "index", "not an index")
			}
			if state == "mktree-fails" {
				real := g.git().Command(t.Context(), "--version").Path
				useFakeGit(t, "unused")
				t.Setenv(snapshotGitFault, "mktree")
				t.Setenv(snapshotRealGit, real)
			}
			tree, err := g.IndexTree(t.Context())
			if state == "missing" {
				if err != nil || tree != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
					t.Fatalf("empty index = %q, %v", tree, err)
				}
			} else if err == nil || kind.Of(err) != kind.Missing || tree != "" {
				t.Fatalf("unreadable index = %q, %v", tree, err)
			}
		})
	}
}

func TestIndexTreeMissingNeedsNoTemporaryFile(t *testing.T) {
	dir := alone(t)
	gitIn(t, dir, "", "init", "-q")
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, blocked)
	}
	tree, err := (Git{Dir: dir}).IndexTree(t.Context())
	if err != nil || tree != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
		t.Fatalf("missing index = %q, %v", tree, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "index")); !os.IsNotExist(err) {
		t.Errorf("capture created a source index: %v", err)
	}
	data, err := os.ReadFile(blocked)
	if err != nil || string(data) != "untouched" {
		t.Errorf("temporary location changed: %q, %v", data, err)
	}
}
