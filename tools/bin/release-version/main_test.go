package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain drops the variables a git hook exports before any test runs, so
// release-version's own git, which inherits the environment, reads the
// scratch repository and not the one whose hook ran the tests.
func TestMain(m *testing.M) {
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_PREFIX"} {
		os.Unsetenv(name)
	}
	os.Exit(m.Run())
}

// repo is a scratch repository with a v6.2.0 release on its first commit,
// whose go.mod's module path ends in /v7, so a v7 version is never refused for
// its path, and whose pre-release marker, tools/bin/release-version/prerelease,
// says what mode gives (no file when "").
type repo struct {
	t   *testing.T
	dir string
}

func newRepo(t *testing.T, mode string) repo {
	t.Helper()
	return newRepoAt(t, mode, "example.com/m/v7")
}

// newRepoAt is newRepo with go.mod's module path at the release module.
func newRepoAt(t *testing.T, mode, module string) repo {
	t.Helper()
	r := repo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write("go.mod", "module "+module+"\n\ngo 1.25\n")
	if mode != "" {
		r.write(marker, mode)
	}
	r.git("add", ".")
	r.commit("chore: the first")
	r.git("tag", "v6.2.0")
	return r
}

func (r repo) git(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@localhost",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@localhost", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func (r repo) write(name, text string) {
	r.t.Helper()
	path := filepath.Join(r.dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r repo) commit(message string) {
	r.t.Helper()
	r.git("commit", "-q", "--allow-empty", "-m", message)
}

// move commits go.mod's module path moved to module, a commit that releases
// nothing by its type.
func (r repo) move(module string) {
	r.t.Helper()
	r.write("go.mod", "module "+module+"\n\ngo 1.25\n")
	r.git("add", "go.mod")
	r.commit("refactor: move the module path")
}

// mark commits the pre-release marker saying mode, or removes it when mode is
// "".
func (r repo) mark(mode string) {
	r.t.Helper()
	if mode == "" {
		r.git("rm", "-q", marker)
	} else {
		r.write(marker, mode)
		r.git("add", marker)
	}
	r.commit("ci: the release mode")
}

// cut runs release-version in the repository: its exit, its key=value lines
// and its stderr.
func (r repo) cut() (int, map[string]string, string) {
	r.t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(nil, r.dir, &stdout, &stderr)
	fields := map[string]string{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			fields[k] = v
		}
	}
	return code, fields, stderr.String()
}

func (r repo) wants(next, bump, prerelease string) {
	r.t.Helper()
	code, got, stderr := r.cut()
	if code != 0 || got["next"] != next || got["bump"] != bump || got["prerelease"] != prerelease || got["last"] != "v6.2.0" {
		r.t.Fatalf("exit %d, last=%s next=%s bump=%s prerelease=%s; want exit 0, last=v6.2.0 next=%s bump=%s prerelease=%s\n%s",
			code, got["last"], got["next"], got["bump"], got["prerelease"], next, bump, prerelease, stderr)
	}
}

const rcMode = "rc\n"

// With no marker, nothing changes: a breaking change cuts the major. itos.yaml
// is not read, so a release.prerelease key there, where T-118 read it, switches
// nothing on.
func TestPrereleaseOff(t *testing.T) {
	r := newRepo(t, "")
	r.commit("feat!: a key goes")
	r.wants("7.0.0", "major", "false")
	config := newRepo(t, "")
	config.write("itos.yaml", "version: 1\nrelease:\n  prerelease: rc\n")
	config.git("add", "itos.yaml")
	config.commit("feat!: a key goes")
	config.wants("7.0.0", "major", "false")
}

// The marker is read from HEAD, as go.mod is: written but not committed, it
// switches nothing on; committed with surrounding space, it does.
func TestPrereleaseMarkerAtHead(t *testing.T) {
	r := newRepo(t, "")
	r.commit("feat!: a key goes")
	r.write(marker, rcMode)
	r.wants("7.0.0", "major", "false")
	r.mark("\n  rc  \n\n")
	r.wants("7.0.0-rc.1", "major", "true")
}

// With the mode on, the commits that would cut 7.0.0 cut its candidates, one
// more each time a feat, a fix or a breaking change follows the last; with
// nothing releasable since the last candidate, nothing is cut; and removing
// the key cuts 7.0.0 from the commits since v6.2.0.
func TestPrereleaseCandidates(t *testing.T) {
	r := newRepo(t, rcMode)
	r.commit("feat!: a key goes")
	r.wants("7.0.0-rc.1", "major", "true")
	r.git("tag", "v7.0.0-rc.1")

	r.commit("docs: say so")
	r.commit("ci: run it")
	r.wants("", "none", "false")

	r.commit("fix: a bug")
	r.wants("7.0.0-rc.2", "major", "true")
	r.git("tag", "v7.0.0-rc.2")

	r.commit("feat: a thing")
	r.wants("7.0.0-rc.3", "major", "true")
	r.git("tag", "v7.0.0-rc.3")

	r.mark("")
	r.wants("7.0.0", "major", "false")
}

// The candidate's number counts up from the highest rc tag of that version
// HEAD reaches, by its number: another version's, a tag HEAD does not reach
// and a tag that is no candidate do not count.
func TestPrereleaseNumbering(t *testing.T) {
	r := newRepo(t, rcMode)
	r.commit("feat!: a key goes")
	r.git("tag", "v7.0.0-rc.2")
	r.git("tag", "v7.0.0-rc.10")
	r.git("tag", "v8.0.0-rc.40")
	r.git("tag", "v7.0.0-rc.x")
	r.git("tag", "v7.0.0-beta.50")
	r.git("checkout", "-q", "-b", "side")
	r.commit("fix: elsewhere")
	r.git("tag", "v7.0.0-rc.30")
	r.git("checkout", "-q", "main")
	r.commit("fix: a bug")
	r.wants("7.0.0-rc.11", "major", "true")
}

// A minor or a patch is cut as usual with the mode on.
func TestPrereleaseLeavesMinorAndPatch(t *testing.T) {
	r := newRepo(t, rcMode)
	r.write("go.mod", "module example.com/m/v6\n\ngo 1.25\n")
	r.git("add", "go.mod")
	r.commit("feat: a thing")
	r.wants("6.3.0", "minor", "false")
}

// The module path rule holds for a candidate: no v7 rc from a /v6 go.mod.
func TestPrereleaseModulePath(t *testing.T) {
	r := newRepo(t, rcMode)
	r.write("go.mod", "module example.com/m/v6\n\ngo 1.25\n")
	r.git("add", "go.mod")
	r.commit("feat!: a key goes")
	code, got, stderr := r.cut()
	if code != 1 || len(got) != 0 || !strings.Contains(stderr, "refusing v7.0.0-rc.1") || !strings.Contains(stderr, "example.com/m/v7") {
		t.Fatalf("exit %d, %v; want exit 1 refusing v7.0.0-rc.1 for example.com/m/v7, nothing on stdout\n%s", code, got, stderr)
	}
}

// A marker saying anything but rc, nothing included, stops it, naming the file.
func TestPrereleaseUnknownMode(t *testing.T) {
	for _, mode := range []string{"beta\n", "rc.1\n", "\n"} {
		r := newRepo(t, mode)
		r.commit("feat!: a key goes")
		code, got, stderr := r.cut()
		if code != 2 || len(got) != 0 || !strings.Contains(stderr, marker) {
			t.Fatalf("marker %q: exit %d, %v; want exit 2 naming %s, nothing on stdout\n%s", mode, code, got, marker, stderr)
		}
	}
}

// A module path moved ahead of the last release's major promises that major:
// after the move, a fix cuts 7.0.0's first candidate with the marker on and
// 7.0.0 itself without it, as a feat does, while a push with nothing
// releasable still cuts nothing.
func TestModulePathAhead(t *testing.T) {
	for _, c := range []struct {
		mode, change, next, prerelease string
	}{
		{rcMode, "fix: a bug", "7.0.0-rc.1", "true"},
		{"", "fix: a bug", "7.0.0", "false"},
		{rcMode, "feat: a thing", "7.0.0-rc.1", "true"},
		{"", "feat: a thing", "7.0.0", "false"},
	} {
		r := newRepoAt(t, c.mode, "example.com/m/v6")
		r.move("example.com/m/v7")
		r.wants("", "none", "false")
		r.commit("docs: say so")
		r.wants("", "none", "false")
		r.commit(c.change)
		r.wants(c.next, "major", c.prerelease)
		code, _, stderr := r.cut()
		if code != 0 || !strings.Contains(stderr, "example.com/m/v7, is ahead of the last release's major, 6") {
			t.Fatalf("%q: exit %d; want exit 0 saying the module path is ahead of major 6\n%s", c.change, code, stderr)
		}
	}
}

// With the mode on, the candidates of the major the path promises count up
// as a breaking change's do: a fix after rc.1 cuts rc.2, a docs commit after
// rc.2 nothing, and removing the marker cuts 7.0.0.
func TestModulePathAheadCandidates(t *testing.T) {
	r := newRepoAt(t, rcMode, "example.com/m/v6")
	r.move("example.com/m/v7")
	r.commit("feat: a thing")
	r.wants("7.0.0-rc.1", "major", "true")
	r.git("tag", "v7.0.0-rc.1")

	r.commit("fix: a bug")
	r.wants("7.0.0-rc.2", "major", "true")
	r.git("tag", "v7.0.0-rc.2")

	r.commit("docs: say so")
	r.wants("", "none", "false")

	r.mark("")
	r.wants("7.0.0", "major", "false")
}

// A path ahead by more than one major cuts the path's major, even over a
// breaking change's.
func TestModulePathAheadByTwo(t *testing.T) {
	r := newRepoAt(t, "", "example.com/m/v6")
	r.move("example.com/m/v8")
	r.commit("feat!: a key goes")
	r.wants("8.0.0", "major", "false")
}

// A path below the last release's major is still refused, a fix's patch as a
// feat's minor: no v6 version from a /v5 go.mod.
func TestModulePathBehind(t *testing.T) {
	r := newRepoAt(t, rcMode, "example.com/m/v5")
	r.commit("fix: a bug")
	code, got, stderr := r.cut()
	if code != 1 || len(got) != 0 || !strings.Contains(stderr, "refusing v6.2.1") || !strings.Contains(stderr, "example.com/m/v6") {
		t.Fatalf("exit %d, %v; want exit 1 refusing v6.2.1 for example.com/m/v6, nothing on stdout\n%s", code, got, stderr)
	}
}
