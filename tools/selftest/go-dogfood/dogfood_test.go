// Package dogfood holds the self-test of tools/bin/itos (T-060's check, kept as a step in T-125,
// a Go test since T-143), which the hooks, CI's steps, the ledger's checks and the nightly call
// (itos.yaml's hooks.bin): it runs the Go binary and needs no Node. Run in a scratch worktree of
// the current tree (uncommitted edits included, scratch.Repo), with a PATH that holds go, git and
// sh but no node:
//
//   - version, config check and task list run on this repository, the first call building the
//     binary into .tools/bin/itos, stamped with the version tools/bin/dev-version gives the
//     checkout from git describe (T-069);
//   - a call with nothing changed builds nothing;
//   - a Go source added, a Go source removed and a new commit (a new version) each rebuild it,
//     and the next call runs the new build, a new commit's stamped with its version;
//   - a Go source that does not compile fails the call with exit 3 and one line naming the
//     build, never by running the old binary.
//
// Not on windows, where tools/bin/itos, a shell script, is not what CI's platform job runs and
// the PATH without node is built from symbolic links.
package dogfood

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

// hidden is what the PATH built here leaves out.
var hidden = []string{"node", "nodejs"}

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	if runtime.GOOS == "windows" {
		t.Skip("tools/bin/itos is a shell script, and the PATH without node is made of symbolic links")
	}
	repo := scratch.NewRepo(t, "go-dogfood-selftest")
	dir := repo.Dir
	links := t.TempDir()

	// The PATH less Node: each folder that holds node is replaced by a folder of links to
	// everything else in it, as the corpus runner's hide does.
	var path []string
	for i, folder := range filepath.SplitList(os.Getenv("PATH")) {
		if folder == "" {
			continue
		}
		holds := false
		for _, name := range hidden {
			if _, err := os.Stat(filepath.Join(folder, name)); err == nil {
				holds = true
			}
		}
		if !holds {
			path = append(path, folder)
			continue
		}
		copied := filepath.Join(links, fmt.Sprintf("path-%d", i))
		if err := os.Mkdir(copied, 0o755); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(folder)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !slices.Contains(hidden, e.Name()) {
				os.Symlink(filepath.Join(folder, e.Name()), filepath.Join(copied, e.Name()))
			}
		}
		path = append(path, copied)
	}
	noNode := strings.Join(path, string(os.PathListSeparator))

	repo.Open()
	runEnv := func() []string { return scratch.With(repo.Env, "PATH="+noNode) }
	found := func(command string) bool {
		return scratch.Run(dir, runEnv(), "", "/bin/sh", "-c", "command -v "+command).Code == 0
	}
	itos := func(args ...string) scratch.Result {
		return scratch.Run(dir, runEnv(), "", filepath.Join(dir, "tools", "bin", "itos"), args...)
	}
	binary := filepath.Join(dir, ".tools", "bin", "itos")
	built := func() time.Time {
		info, err := os.Stat(binary)
		if err != nil {
			return time.Time{}
		}
		return info.ModTime()
	}
	// The binary as if built a minute ago, so the next edit is newer whatever the file system's
	// clock resolution.
	age := func() {
		then := time.Now().Add(-time.Minute)
		os.Chtimes(binary, then, then)
	}
	// What the scratch checkout's builds must say (tools/bin/dev-version).
	version := func() string {
		return strings.TrimSpace(scratch.Run(dir, runEnv(), "", filepath.Join(dir, "tools", "bin", "dev-version"), dir).Stdout)
	}

	t.Run("the PATH built without node holds go, git and sh but no node", func(t *testing.T) {
		if found("node") {
			t.Errorf("the PATH built without node still finds it: %s", noNode)
		}
		for _, tool := range []string{"go", "git", "sh"} {
			if !found(tool) {
				t.Errorf("the PATH built without node lost %s", tool)
			}
		}
		if !built().IsZero() {
			t.Error("the scratch copy already holds .tools/bin/itos")
		}
	})

	// The commands, on this repository, with no Node.
	t.Run("version runs with no node, building the binary", func(t *testing.T) {
		r := itos("version")
		if r.Code != 0 || r.Stdout != "itos "+version()+"\n" {
			t.Errorf("tools/bin/itos version with no node: exit %d, %q\n%s", r.Code, r.Stdout, r.Stderr)
		}
		if built().IsZero() {
			t.Error("the first call left no binary at .tools/bin/itos")
		}
	})
	t.Run("config check runs with no node", func(t *testing.T) {
		if r := itos("config", "check"); r.Code != 0 {
			t.Errorf("tools/bin/itos config check with no node: exit %d\n%s", r.Code, r.Stderr)
		}
	})
	t.Run("task list runs with no node", func(t *testing.T) {
		if r := itos("task", "list"); r.Code != 0 || !strings.Contains(r.Stdout, "T-060") {
			t.Errorf("tools/bin/itos task list with no node: exit %d\n%s", r.Code, r.Output())
		}
	})

	t.Run("a call with nothing changed builds nothing", func(t *testing.T) {
		before := built()
		if r := itos("version"); r.Code != 0 || !built().Equal(before) {
			t.Errorf("a call with nothing changed rebuilt the binary: exit %d\n%s", r.Code, r.Stderr)
		}
	})

	// A Go source added: rebuilt, and the new build is what runs.
	const marker = "go-dogfood: the new build"
	source := filepath.Join(dir, "cmd", "itos", "dogfood.go")
	t.Run("a Go source added rebuilds the binary", func(t *testing.T) {
		age()
		before := built()
		text := "package main\n\nimport \"os\"\n\nfunc init() { os.Stderr.WriteString(\"" + marker + "\\n\") }\n"
		if err := os.WriteFile(source, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		r := itos("version")
		if r.Code != 0 || built().Equal(before) || !strings.Contains(r.Stderr, marker) {
			t.Errorf("a Go source added did not rebuild the binary: exit %d\n%s", r.Code, r.Stderr)
		}
	})

	t.Run("a Go source removed rebuilds the binary", func(t *testing.T) {
		age()
		before := built()
		os.Remove(source)
		r := itos("version")
		if r.Code != 0 || built().Equal(before) || strings.Contains(r.Stderr, marker) {
			t.Errorf("a Go source removed did not rebuild the binary: exit %d\n%s", r.Code, r.Stderr)
		}
	})

	// A new commit, so a new version: restamped with it, though no source is newer than the
	// binary.
	t.Run("a new commit restamps the binary", func(t *testing.T) {
		was := version()
		repo.Commit("go-dogfood: a new commit")
		before := built()
		r := itos("version")
		now := version()
		if now == was || r.Code != 0 || r.Stdout != "itos "+now+"\n" || built().Equal(before) {
			t.Errorf("a new commit was not stamped: %s then %s, exit %d, %q\n%s", was, now, r.Code, r.Stdout, r.Stderr)
		}
	})

	// A Go source that does not compile: exit 3 and one line naming the build, the old binary
	// not run.
	broken := filepath.Join(dir, "cmd", "itos", "broken.go")
	t.Run("a Go source that does not compile exits 3, naming the build", func(t *testing.T) {
		if err := os.WriteFile(broken, []byte("package main\n\nfunc broken( {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		r := itos("version")
		first, _, _ := strings.Cut(r.Stderr, "\n")
		if r.Code != 3 || r.Stdout != "" || !strings.HasPrefix(first, "itos: the Go binary does not build (go build ./cmd/itos): ") {
			t.Errorf("a Go source that does not compile: exit %d, stdout %q\n%s", r.Code, r.Stdout, r.Stderr)
		}
	})
	t.Run("the build fixed runs again", func(t *testing.T) {
		os.Remove(broken)
		if r := itos("version"); r.Code != 0 || r.Stdout != "itos "+version()+"\n" {
			t.Errorf("the build fixed did not run again: exit %d\n%s", r.Code, r.Stderr)
		}
	})
}
