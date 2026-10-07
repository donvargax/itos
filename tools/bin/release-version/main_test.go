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
// whose go.mod's module path ends in /v7 and whose itos.yaml says what
// config gives (none when ""), so a v7 version is never refused for its path.
type repo struct {
	t   *testing.T
	dir string
}

func newRepo(t *testing.T, config string) repo {
	t.Helper()
	r := repo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write("go.mod", "module example.com/m/v7\n\ngo 1.25\n")
	if config != "" {
		r.write("itos.yaml", config)
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
	if err := os.WriteFile(filepath.Join(r.dir, name), []byte(text), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r repo) commit(message string) {
	r.t.Helper()
	r.git("commit", "-q", "--allow-empty", "-m", message)
}

// configure commits itos.yaml as text, or removes it when text is "".
func (r repo) configure(text string) {
	r.t.Helper()
	if text == "" {
		r.git("rm", "-q", "itos.yaml")
	} else {
		r.write("itos.yaml", text)
		r.git("add", "itos.yaml")
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

const rcMode = "version: 1\nrelease:\n  prerelease: rc\n"

// With the mode off, nothing changes: a breaking change cuts the major.
func TestPrereleaseOff(t *testing.T) {
	r := newRepo(t, "version: 1\n")
	r.commit("feat!: a key goes")
	r.wants("7.0.0", "major", "false")
	bare := newRepo(t, "")
	bare.commit("feat!: a key goes")
	bare.wants("7.0.0", "major", "false")
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

	r.configure("version: 1\n")
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

// A mode other than rc stops it, naming the key.
func TestPrereleaseUnknownMode(t *testing.T) {
	r := newRepo(t, "version: 1\nrelease:\n  prerelease: beta\n")
	r.commit("feat!: a key goes")
	code, got, stderr := r.cut()
	if code != 2 || len(got) != 0 || !strings.Contains(stderr, "release.prerelease") {
		t.Fatalf("exit %d, %v; want exit 2 naming release.prerelease, nothing on stdout\n%s", code, got, stderr)
	}
}
