// Package hookenv holds the test that the git environment a hook is handed stops at the unit
// tests the hooks run (p1-hook-env-unset-tracked, T-142).
//
// git hands a pre-commit hook the index it commits from in GIT_INDEX_FILE (an index.lock under
// git commit -a) and, in a linked worktree such as an agent's under .claude/worktrees/, GIT_DIR.
// tools/bin/pre-commit and itos's pre-push hook run tools/bin/go-unit-tests, whose go test hands
// its environment to every test; a test that makes a scratch repository without clearing them
// (internal/source's did, on 2026-10-11) then commits into, and leaves an index.lock in, the
// repository whose hook ran it. go-unit-tests reads the change from the hook's index first, then
// drops them. The tests here run it with a fake go that records the environment it is given.
package hookenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

// The variables a hook's environment may carry that point git at a repository or its index.
var pointers = []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY"}

// program is tools/bin/go-unit-tests built, by its absolute path.
func program(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "go-unit-tests")
	build := exec.Command("go", "build", "-o", path, "github.com/donvargax/itos/v8/tools/bin/go-unit-tests")
	build.Env = clean()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build go-unit-tests: %v\n%s", err, out)
	}
	return path
}

// fakeBin is a folder for the front of the PATH holding a go that answers go list with one
// package, example.com/m/internal/x in the repository's internal/x, and on go test writes its
// environment to record; and the real git as git, never an itos linked as git.
func fakeBin(t *testing.T, record string) string {
	t.Helper()
	bin := t.TempDir()
	goScript := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"list) echo \"example.com/m/internal/x|$(git rev-parse --show-toplevel)/internal/x|\" ;;\n" +
		"test) env > '" + record + "' ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(goScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(git.Bin(runtime.GOOS), filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	return bin
}

// gitIn runs the real git in dir with env laid over the test's environment less the pointers.
func gitIn(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command(git.Bin(runtime.GOOS), args...)
	cmd.Dir = dir
	cmd.Env = append(clean(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// clean is the test's environment less the pointers, as the test may itself run under a hook.
func clean() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		keep := true
		for _, p := range pointers {
			keep = keep && name != p
		}
		if keep {
			env = append(env, kv)
		}
	}
	return env
}

// A hook in a linked worktree, committing with -a: the change is read from the index git
// commits from, and go test runs with neither that index nor the git folder in its environment.
func TestGoUnitTestsDropTheHooksGitEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake go and git are shell scripts")
	}
	dir := t.TempDir()
	gitIn(t, dir, nil, "init", "-q")
	src := filepath.Join(dir, "internal", "x", "x.go")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The commit's index, apart from the repository's own, which stays empty: a change read
	// from the default index finds nothing to test.
	index := filepath.Join(dir, ".git", "index.lock")
	hook := []string{"GIT_INDEX_FILE=" + index, "GIT_DIR=" + filepath.Join(dir, ".git")}
	gitIn(t, dir, hook, "add", "internal/x/x.go")

	record := filepath.Join(t.TempDir(), "env")
	cmd := exec.Command(program(t), "--cached")
	cmd.Dir = dir
	cmd.Env = append(clean(), append(hook, "PATH="+fakeBin(t, record)+string(os.PathListSeparator)+os.Getenv("PATH"))...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go-unit-tests --cached: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "$ go test example.com/m/internal/x") {
		t.Fatalf("go-unit-tests did not read the change from the hook's index (GIT_INDEX_FILE):\n%s", out)
	}
	recorded, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("go test never ran: %v\n%s", err, out)
	}
	for _, line := range strings.Split(string(recorded), "\n") {
		name, _, _ := strings.Cut(line, "=")
		for _, p := range pointers {
			if name == p {
				t.Errorf("go test was handed the hook's %s", line)
			}
		}
	}
}
