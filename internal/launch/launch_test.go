package launch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v2/internal/release"
	"github.com/donvargax/itos/v2/internal/version"
)

const sums = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestListed(t *testing.T) {
	text := []byte("AA  itos-1.0.0-linux-amd64.tar.gz\nbb *itos-1.0.0-windows-amd64.zip\ncc  itos.schema.json\n")
	cases := []struct {
		asset, want string
		ok          bool
	}{
		{"itos-1.0.0-linux-amd64.tar.gz", "aa", true},
		{"itos-1.0.0-windows-amd64.zip", "bb", true},
		{"itos-1.0.0-darwin-arm64.tar.gz", "", false},
	}
	for _, c := range cases {
		got, ok := listed(text, c.asset)
		if got != c.want || ok != c.ok {
			t.Errorf("listed(%q) = %q, %t; want %q, %t", c.asset, got, ok, c.want, c.ok)
		}
	}
}

// offline keeps a test off the network and off any real cache: an empty
// cache of its own, nothing to ask, and no CI or silencing set. It gives the
// cache.
func offline(t *testing.T) string {
	t.Helper()
	cache := t.TempDir()
	t.Setenv(EnvCache, cache)
	t.Setenv(release.Env, "http://127.0.0.1:1/no-release-server")
	t.Setenv(EnvNoUpdate, "1")
	t.Setenv(EnvNoUpdateNotice, "")
	t.Setenv("CI", "")
	return cache
}

// writeConfig writes an itos.yaml in a scratch folder and makes it the
// working directory for the test.
func writeConfig(t *testing.T, text string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "itos.yaml"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChoose(t *testing.T) {
	own := version.Version()
	pinned := "version: 1\npin: { version: \"9.1.0\", checksums: \"" + sums + "\" }\n"
	cases := []struct {
		name, config, env string
		want              target
		launch            bool
	}{
		{"a pin of another version runs it, held to its checksums", pinned, "", target{"9.1.0", sums}, true},
		{"no pin runs this binary", "version: 1\n", "", target{}, false},
		{"a pin of this binary's version runs it", "version: 1\npin: { version: \"" + own + "\", checksums: \"" + sums + "\" }\n", "", target{}, false},
		{"a pin the launcher cannot use runs this binary", "version: 1\npin: { version: v9 }\n", "", target{}, false},
		{"a config that is not YAML runs this binary", "pin: [\n", "", target{}, false},
		{"ITOS_VERSION overrides the pin, trusting checksums.txt", pinned, "9.2.0", target{"9.2.0", ""}, true},
		{"ITOS_VERSION naming the pin keeps its checksums", pinned, "9.1.0", target{"9.1.0", sums}, true},
		{"ITOS_VERSION naming this binary runs it", pinned, own, target{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			offline(t)
			writeConfig(t, c.config)
			t.Setenv(EnvVersion, c.env)
			got, launch := choose(nil, io.Discard)
			if got != c.want || launch != c.launch {
				t.Errorf("choose = %v, %t; want %v, %t", got, launch, c.want, c.launch)
			}
		})
	}
}

// Handed names the version choose would run by name, never the newest
// release, and nothing where choose runs this binary.
func TestHanded(t *testing.T) {
	own := version.Version()
	pinned := "version: 1\npin: { version: \"9.1.0\", checksums: \"" + sums + "\" }\n"
	cases := []struct {
		name, config, env, want string
		byEnv                   bool
	}{
		{"a pin of another version", pinned, "", "9.1.0", false},
		{"ITOS_VERSION over the pin", pinned, "1.9.0", "1.9.0", true},
		{"ITOS_VERSION that is no version", pinned, "latest", "", false},
		{"a pin of this binary's version", "version: 1\npin: { version: \"" + own + "\", checksums: \"" + sums + "\" }\n", "", "", false},
		{"a pin the launcher cannot use", "version: 1\npin: { version: v1 }\n", "", "", false},
		{"no pin", "version: 1\n", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			offline(t)
			writeConfig(t, c.config)
			t.Setenv(EnvVersion, c.env)
			if got, byEnv := Handed(nil); got != c.want || byEnv != c.byEnv {
				t.Errorf("Handed = %q, %t; want %q, %t", got, byEnv, c.want, c.byEnv)
			}
		})
	}
}

// Only hook pre-tool-use, and only for a version older than the guard, is
// answered by the launcher.
func TestUnguarded(t *testing.T) {
	hook := []string{"hook", "pre-tool-use"}
	cases := []struct {
		name string
		args []string
		v    string
		want bool
	}{
		{"the hook for a version before the guard", hook, "2.0.0", true},
		{"the hook after global flags", []string{"--root", "x", "hook", "pre-tool-use"}, "2.2.9", true},
		{"the hook for the guard's first version", hook, "2.3.0", false},
		{"the hook for a later version", hook, "9.1.0", false},
		{"the hook for no version", hook, "latest", false},
		{"another hook", []string{"hook", "commit-msg", "msg"}, "2.0.0", false},
		{"another command", []string{"task", "T-001"}, "2.0.0", false},
		{"the hook's help", []string{"help", "hook", "pre-tool-use"}, "2.0.0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := unguarded(c.args, target{version: c.v}); got != c.want {
				t.Errorf("unguarded(%q, %q) = %t; want %t", c.args, c.v, got, c.want)
			}
		})
	}
}

func TestChooseReadsTheConfigWhereItosDoes(t *testing.T) {
	offline(t)
	writeConfig(t, "version: 1\n")
	if err := os.MkdirAll("sub", 0o755); err != nil {
		t.Fatal(err)
	}
	pinned := "version: 1\npin: { version: \"9.1.0\", checksums: \"" + sums + "\" }\n"
	if err := os.WriteFile(filepath.Join("sub", "other.yaml"), []byte(pinned), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVersion, "")
	for _, args := range [][]string{
		{"--root", "sub", "--config", "other.yaml", "version"},
		{"version", "--config", "sub/other.yaml"},
	} {
		if got, launch := choose(args, io.Discard); !launch || got.version != "9.1.0" {
			t.Errorf("choose(%q) = %v, %t; want 9.1.0", args, got, launch)
		}
	}
}

// The stealth config, in the git folder of a repository with no itos.yaml,
// is read for its pin, and one with no pin is as no config at all: the
// newest release; a project's config with no pin is not.
func TestReadConfigFindsTheStealthConfig(t *testing.T) {
	offline(t)
	t.Setenv(EnvVersion, "")
	t.Setenv("ITOS_CONFIG", "")
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	t.Chdir(dir)
	stealth := filepath.Join(".git", "itos", "itos.yaml")
	if err := os.MkdirAll(filepath.Dir(stealth), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(file, text string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(stealth, "version: 1\npin: { version: \"9.1.0\", checksums: \""+sums+"\" }\n")
	if got, launch := choose(nil, io.Discard); !launch || got.version != "9.1.0" {
		t.Errorf("a pinned stealth config: choose = %v, %t; want 9.1.0", got, launch)
	}
	if c := readConfig([]string{"--root", dir, "version"}); c.state != pinned {
		t.Errorf("a pinned stealth config under --root: %+v", c)
	}
	write(stealth, "version: 1\n")
	if c := readConfig(nil); c.state != absent {
		t.Errorf("a stealth config with no pin: %+v, want absent", c)
	}
	write("itos.yaml", "version: 1\n")
	if c := readConfig(nil); c.state != unpinned || c.file != "itos.yaml" {
		t.Errorf("an itos.yaml in the root with no pin: %+v, want unpinned", c)
	}
}

func tarGz(t *testing.T, name, text string) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	w := tar.NewWriter(gz)
	if err := w.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(text)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestExtract(t *testing.T) {
	got, err := extract(tarGz(t, binaryName(), "binary"), "a.tar.gz")
	if err != nil || string(got) != "binary" {
		t.Errorf("extract = %q, %v; want the binary", got, err)
	}
	if _, err := extract(tarGz(t, "other", "x"), "a.tar.gz"); err == nil {
		t.Error("extract found a binary in an archive without one")
	}
}

func TestStoreAndCached(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "9.1.0")
	text := []byte("checksums\n")
	pinned := target{"9.1.0", release.SHA256(text)}
	if cached(dir, pinned) {
		t.Fatal("an empty cache holds the release")
	}
	if err := store(cache, dir, []byte("binary"), text); err != nil {
		t.Fatal(err)
	}
	if !cached(dir, pinned) || !cached(dir, target{version: "9.1.0"}) {
		t.Error("the stored release is not cached")
	}
	if cached(dir, target{"9.1.0", sums}) {
		t.Error("a release checked against another checksums.txt counts as cached")
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 1 {
		t.Errorf("the cache holds %v, not the release alone", entries)
	}
}

func TestBinaryCommand(t *testing.T) {
	for args, want := range map[string]bool{
		"pin":                    true,
		"pin 9.1.0":              true,
		"--json pin":             true,
		"init":                   true,
		"init --stealth":         true,
		"git-shim install":       true,
		"git-shim uninstall":     true,
		"git-shim run commit":    false,
		"version":                false,
		"work --as someone":      false,
		"--config pin.yaml work": false,
	} {
		if got := binaryCommand(strings.Fields(args)); got != want {
			t.Errorf("binaryCommand(%s) = %v, want %v", args, got, want)
		}
	}
}
