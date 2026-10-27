package launch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/version"
)

const sums = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// here is the platform the tests run on, as cmd/itos detects it.
var here = Platform{runtime.GOOS, runtime.GOARCH}

// standIn stands in for internal/cli's command line, which cmd/itos hands
// the launcher: Parse reads --config and --root with their values, and the
// switches the tests write, wherever they stand, the rest the command, and
// names no extension; UpgradeTarget hands nothing on; ExitCode gives 75 to an
// error that may pass when run again, 3 to a missing environment and 70 to
// any other.
var standIn = CLI{
	Parse: func(args []string) Line {
		var l Line
		for i := 0; i < len(args); i++ {
			switch a := args[i]; a {
			case "--config", "--root":
				i++
				if a == "--config" {
					l.Config = args[i]
				} else {
					l.Root = args[i]
				}
			case "-h", "--help":
				l.Help = true
			case "--json":
			default:
				l.Rest = append(l.Rest, a)
			}
		}
		return l
	},
	UpgradeTarget: func([]string) (Handoff, bool, error) { return Handoff{}, false, nil },
	ExitCode: func(err error) int {
		switch kind.Of(err) {
		case kind.Temporary:
			return 75
		case kind.Missing:
			return 3
		}
		return 70
	},
}

// A release's binary and archive are named by the OS and the architecture:
// itos in a tar.gz, itos.exe in a zip on windows.
func TestPlatformNames(t *testing.T) {
	for _, c := range []struct {
		p               Platform
		binary, archive string
	}{
		{Platform{"linux", "amd64"}, "itos", "itos-9.1.0-linux-amd64.tar.gz"},
		{Platform{"darwin", "arm64"}, "itos", "itos-9.1.0-darwin-arm64.tar.gz"},
		{Platform{"windows", "amd64"}, "itos.exe", "itos-9.1.0-windows-amd64.zip"},
	} {
		if got := c.p.binary(); got != c.binary {
			t.Errorf("%v's binary is %s", c.p, got)
		}
		if got := c.p.archive("9.1.0"); got != c.archive {
			t.Errorf("%v's archive is %s", c.p, got)
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
			got, launch := choose(standIn, nil, io.Discard, here)
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
			if got, byEnv := Handed(standIn, nil); got != c.want || byEnv != c.byEnv {
				t.Errorf("Handed = %q, %t; want %q, %t", got, byEnv, c.want, c.byEnv)
			}
		})
	}
}

// Only guard claude-code, and only for a version older than it (v6.0.0, which
// renamed hook pre-tool-use), is answered by the launcher.
func TestUnguarded(t *testing.T) {
	guard := []string{"guard", "claude-code"}
	cases := []struct {
		name string
		args []string
		v    string
		want bool
	}{
		{"the guard for a version before it", guard, "2.0.0", true},
		{"the guard for a version with hook pre-tool-use", guard, "5.4.0", true},
		{"the guard after global flags", []string{"--root", "x", "guard", "claude-code"}, "5.9.9", true},
		{"the guard for its first version", guard, "6.0.0", false},
		{"the guard for a later version", guard, "9.1.0", false},
		{"the guard for no version", guard, "latest", false},
		{"the old name, which the version runs", []string{"hook", "pre-tool-use"}, "5.4.0", false},
		{"a hook", []string{"hook", "commit-msg", "msg"}, "2.0.0", false},
		{"another command", []string{"task", "T-001"}, "2.0.0", false},
		{"the guard's help", []string{"help", "guard", "claude-code"}, "2.0.0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := unguarded(standIn, c.args, target{version: c.v}); got != c.want {
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
		if got, launch := choose(standIn, args, io.Discard, here); !launch || got.version != "9.1.0" {
			t.Errorf("choose(standIn, %q) = %v, %t; want 9.1.0", args, got, launch)
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
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	dir := t.TempDir()
	if out, err := exec.Command(git.Bin(runtime.GOOS), "init", "-q", dir).CombinedOutput(); err != nil {
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
	if got, launch := choose(standIn, nil, io.Discard, here); !launch || got.version != "9.1.0" {
		t.Errorf("a pinned stealth config: choose = %v, %t; want 9.1.0", got, launch)
	}
	if c := readConfig(standIn, []string{"--root", dir, "version"}); c.state != pinned {
		t.Errorf("a pinned stealth config under --root: %+v", c)
	}
	write(stealth, "version: 1\n")
	if c := readConfig(standIn, nil); c.state != absent {
		t.Errorf("a stealth config with no pin: %+v, want absent", c)
	}
	write("itos.yaml", "version: 1\n")
	if c := readConfig(standIn, nil); c.state != unpinned || c.file != "itos.yaml" {
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
	got, err := extract(tarGz(t, "itos", "binary"), "a.tar.gz", "itos")
	if err != nil || string(got) != "binary" {
		t.Errorf("extract = %q, %v; want the binary", got, err)
	}
	if _, err := extract(tarGz(t, "other", "x"), "a.tar.gz", "itos"); err == nil {
		t.Error("extract found a binary in an archive without one")
	}
}

func TestStoreAndCached(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "9.1.0")
	text := []byte("checksums\n")
	pinned := target{"9.1.0", release.SHA256(text)}
	if cached(dir, pinned, here) {
		t.Fatal("an empty cache holds the release")
	}
	if err := store(cache, dir, []byte("binary"), text, here); err != nil {
		t.Fatal(err)
	}
	if !cached(dir, pinned, here) || !cached(dir, target{version: "9.1.0"}, here) {
		t.Error("the stored release is not cached")
	}
	if cached(dir, target{"9.1.0", sums}, here) {
		t.Error("a release checked against another checksums.txt counts as cached")
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 1 {
		t.Errorf("the cache holds %v, not the release alone", entries)
	}
}

// A release another run cached first is kept, the very folder (bug 44); one
// checked against another checksums.txt is replaced, nothing left beside it.
func TestStoreKeepsWhatAnotherRunCached(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "9.1.0")
	text := []byte("checksums\n")
	if err := store(cache, dir, []byte("first"), text, here); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store(cache, dir, []byte("second"), text, here); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(dir)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("the release cached first was replaced (%v)", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, here.binary())); string(got) != "first" {
		t.Errorf("the cached binary is %q, not the one cached first", got)
	}

	other := []byte("other checksums\n")
	if err := store(cache, dir, []byte("third"), other, here); err != nil {
		t.Fatal(err)
	}
	if !cached(dir, target{"9.1.0", release.SHA256(other)}, here) {
		t.Error("the release checked against another checksums.txt was not replaced")
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 1 {
		t.Errorf("the cache holds %v, not the release alone", entries)
	}
}

func TestBinaryCommand(t *testing.T) {
	offline(t)
	writeConfig(t, "version: 1\n")
	for args, want := range map[string]bool{
		"pin":                    true,
		"pin 9.1.0":              true,
		"--json pin":             true,
		"upgrade":                true,
		"init":                   true,
		"init --stealth":         true,
		"git-shim install":       true,
		"git-shim uninstall":     true,
		"git-shim run commit":    false,
		"git-shim":               false,
		"hook install":           false,
		"":                       false,
		"version":                false,
		"work --as someone":      false,
		"--config pin.yaml work": false,
	} {
		if got := binaryCommand(standIn, strings.Fields(args)); got != want {
			t.Errorf("binaryCommand(%s) = %v, want %v", args, got, want)
		}
	}
}

// init is the binary's own only where the config pins nothing: a pinned
// version runs it, as it runs every other command (bug 104), while pin and
// upgrade stay the binary's whatever the pin says.
func TestBinaryCommandLeavesInitToThePin(t *testing.T) {
	offline(t)
	pinned := "version: 1\npin: { version: \"9.1.0\", checksums: \"" + sums + "\" }\n"
	cases := []struct {
		name, config, args string
		want               bool
	}{
		{"init under a pin", pinned, "init --agent-rules", false},
		{"init --stealth under a pin", pinned, "init --stealth", false},
		{"pin under a pin", pinned, "pin 9.2.0", true},
		{"upgrade under a pin", pinned, "upgrade", true},
		{"init with no config", "", "init", true},
		{"init under a pin the launcher cannot use", "version: 1\npin: { version: v9 }\n", "init", true},
		{"init with --config naming a pinned config", "version: 1\n", "--config other.yaml init", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			if c.config != "" {
				if err := os.WriteFile(filepath.Join(dir, "itos.yaml"), []byte(c.config), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "other.yaml"), []byte(pinned), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := binaryCommand(standIn, strings.Fields(c.args)); got != c.want {
				t.Errorf("binaryCommand(%s) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

// Only a first argument --version is the command version.
func TestArgs(t *testing.T) {
	for args, want := range map[string]string{
		"--version":         "version",
		"--version --json":  "version --json",
		"--version --check": "version --check",
		"--json --version":  "--json --version",
		"-v":                "-v",
		"-V":                "-V",
		"version":           "version",
		"pin --version":     "pin --version",
		"work --as someone": "work --as someone",
		"":                  "",
	} {
		if got := strings.Join(Args(strings.Fields(args)), " "); got != want {
			t.Errorf("Args(%s) = %s, want %s", args, got, want)
		}
	}
}

// A pinned project's itos version names the pin and the itos called on
// stderr; other commands and an unpinned project say nothing of it.
func TestPinLine(t *testing.T) {
	own := version.Version()
	pinned := "version: 1\npin: { version: \"9.1.0\", checksums: \"" + sums + "\" }\n"
	line := "this repository pins 9.1.0; the itos that was called is " + own + "\n"
	cases := []struct{ name, config, args, want string }{
		{"version under a pin", pinned, "version", line},
		{"version --json under a pin", pinned, "--json version", line},
		{"another command under a pin", pinned, "work --as someone", ""},
		{"version --help under a pin", pinned, "version --help", ""},
		{"version with no pin", "version: 1\n", "version", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			offline(t)
			writeConfig(t, c.config)
			t.Setenv(EnvVersion, "")
			var stderr bytes.Buffer
			choose(standIn, strings.Fields(c.args), &stderr, here)
			if stderr.String() != c.want {
				t.Errorf("stderr %q, want %q", stderr.String(), c.want)
			}
		})
	}
}
