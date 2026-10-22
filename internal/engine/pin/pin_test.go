package pin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/release"
)

const sum = "1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a"

// releases serves each path's body, 404 for any other, and 500 for a body
// of "fail".
func releases(t *testing.T, files map[string]string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		switch {
		case !ok:
			http.NotFound(w, r)
		case body == "fail":
			http.Error(w, "down", http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv(release.Env, server.URL)
}

func TestReadConfig(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "itos.yaml")
	text := "version: 1\npin:\n  version: 9.1.0\n  checksums: " + sum + "\n"
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := ReadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Config{File: file, Text: text, Version: "9.1.0", Checksums: sum}); c != want {
		t.Errorf("ReadConfig = %+v, want %+v", c, want)
	}
	if err := os.WriteFile(file, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c, err := ReadConfig(file); err != nil || c.Version != "" || c.Checksums != "" {
		t.Errorf("a config pinning none: %+v, %v", c, err)
	}
}

// A folder in place of the config cannot be read on any OS; text that is no
// YAML is invalid. Both are config errors naming the file.
func TestReadConfigRefuses(t *testing.T) {
	dir := t.TempDir()
	var cerr *config.Error
	_, err := ReadConfig(dir)
	if !errors.As(err, &cerr) || cerr.File != dir || len(cerr.Problems) != 1 ||
		cerr.Problems[0].Rule != "config-unreadable" || cerr.Problems[0].Fix != "create "+dir+", or correct its YAML" ||
		!strings.HasPrefix(cerr.Problems[0].Message, "cannot be read: ") {
		t.Errorf("a folder: %#v", err)
	}
	file := filepath.Join(dir, "itos.yaml")
	if err := os.WriteFile(file, []byte("pin: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ReadConfig(file)
	if !errors.As(err, &cerr) || cerr.File != file || !strings.Contains(err.Error(), "cannot be read: ") {
		t.Errorf("no YAML: %#v", err)
	}
}

func TestReplaced(t *testing.T) {
	c := Config{Version: "9.1.0", Checksums: sum}
	other := strings.Repeat("2b", 32)
	cases := []struct {
		c    Config
		v    string
		sum  string
		want bool
	}{
		{c, "9.1.0", other, true},
		{c, "9.1.0", sum, false},
		{c, "9.2.0", other, false},
		{Config{Version: "9.1.0", Checksums: "none"}, "9.1.0", other, false},
	}
	for _, k := range cases {
		if got := k.c.Replaced(k.v, k.sum); got != k.want {
			t.Errorf("%+v.Replaced(%s, %s) = %v", k.c, k.v, k.sum, got)
		}
	}
}

func TestMoved(t *testing.T) {
	c := Config{File: "itos.yaml"}
	got, err := c.Moved("version: 1\npin:\n  version: 9.1.0 # kept\n  checksums: "+sum+"\n", "9.2.0", "ab")
	if err != nil {
		t.Fatal(err)
	}
	if want := "version: 1\npin:\n  version: 9.2.0 # kept\n  checksums: ab\n"; got != want {
		t.Errorf("Moved = %q, want %q", got, want)
	}
	_, err = c.Moved("pin: [9.1.0]\n", "9.2.0", "ab")
	if kind.Of(err) != kind.Usage || !strings.HasPrefix(err.Error(), "cannot edit the pin in itos.yaml (") ||
		!strings.HasSuffix(err.Error(), "): set pin.version to 9.2.0 and pin.checksums to ab") {
		t.Errorf("a list: %v", err)
	}
}

func TestPinnedNewest(t *testing.T) {
	sums := "aa  itos-9.2.0-linux-amd64.tar.gz\n"
	releases(t, map[string]string{"/latest/download/checksums.txt": sums})
	v, got, err := Pinned("")
	if err != nil || v != "9.2.0" || string(got) != sums {
		t.Errorf("Pinned() = %s, %q, %v", v, got, err)
	}
	releases(t, map[string]string{"/latest/download/checksums.txt": "aa  itos.schema.json\n"})
	_, _, err = Pinned("")
	if kind.Of(err) != kind.Missing || !strings.HasPrefix(err.Error(), "the newest itos's checksums.txt (") ||
		!strings.HasSuffix(err.Error(), "/latest/download/checksums.txt) names no archive of itos") {
		t.Errorf("no archive: %v", err)
	}
	releases(t, map[string]string{"/latest/download/checksums.txt": "fail"})
	_, _, err = Pinned("")
	if kind.Of(err) != kind.Temporary || !strings.HasPrefix(err.Error(), "cannot ask for the newest itos: ") {
		t.Errorf("a server failing: %v", err)
	}
}

func TestPinnedVersion(t *testing.T) {
	releases(t, map[string]string{
		"/download/v9.1.0/checksums.txt": "aa  itos-9.1.0-linux-amd64.tar.gz\n",
		"/download/v9.2.0/checksums.txt": "aa  itos.schema.json\n",
		"/download/v9.3.0/checksums.txt": "aa  itos-9.1.0-linux-amd64.tar.gz\n",
	})
	if v, sums, err := Pinned("9.1.0"); err != nil || v != "9.1.0" || len(sums) == 0 {
		t.Errorf("Pinned(9.1.0) = %s, %q, %v", v, sums, err)
	}
	if v, _, err := Pinned("9.2.0"); err != nil || v != "9.2.0" {
		t.Errorf("a checksums.txt naming no archive: %s, %v", v, err)
	}
	_, _, err := Pinned("9.3.0")
	if kind.Of(err) != kind.Missing || !strings.HasPrefix(err.Error(), "the checksums.txt of itos 9.3.0 (") ||
		!strings.HasSuffix(err.Error(), "/download/v9.3.0/checksums.txt) lists the archives of itos 9.1.0") {
		t.Errorf("another version's archives: %v", err)
	}
	_, _, err = Pinned("9.4.0")
	if kind.Of(err) != kind.Missing || !strings.HasPrefix(err.Error(), "cannot fetch itos 9.4.0: ") {
		t.Errorf("a release the server lacks: %v", err)
	}
}

// A new file gets the mode os.WriteFile gives 0o644 here, after the umask;
// a file there keeps the one it has, whatever the OS makes of a chmod.
func TestWriteKeepingMode(t *testing.T) {
	dir := t.TempDir()
	ref, file := filepath.Join(dir, "ref"), filepath.Join(dir, "file")
	if err := os.WriteFile(ref, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteKeepingMode(file, "one"); err != nil {
		t.Fatal(err)
	}
	if got, want := mode(t, file), mode(t, ref); got != want {
		t.Errorf("a new file: %v, want %v", got, want)
	}
	if err := os.Chmod(file, 0o600); err != nil {
		t.Fatal(err)
	}
	kept := mode(t, file)
	if err := WriteKeepingMode(file, "two"); err != nil {
		t.Fatal(err)
	}
	if got := mode(t, file); got != kept {
		t.Errorf("a file there: %v, want %v", got, kept)
	}
	if text, err := os.ReadFile(file); err != nil || string(text) != "two" {
		t.Errorf("text %q, %v", text, err)
	}
	if err := WriteKeepingMode(dir, "three"); err == nil {
		t.Error("wrote over a folder")
	}
}

func mode(t *testing.T, file string) os.FileMode {
	t.Helper()
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestReadInstallScript(t *testing.T) {
	t.Chdir(t.TempDir())
	if text, found, err := ReadInstallScript(); err != nil || found || text != "" {
		t.Errorf("none: %q, %v, %v", text, found, err)
	}
	if err := os.MkdirAll(InstallScript, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, found, err := ReadInstallScript(); found || err == nil ||
		!strings.HasPrefix(err.Error(), "cannot read tools/bin/install-itos: ") {
		t.Errorf("a folder: %v, %v", found, err)
	}
	if err := os.Remove(InstallScript); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(InstallScript, []byte("version=9.1.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if text, found, err := ReadInstallScript(); err != nil || !found || text != "version=9.1.0\n" {
		t.Errorf("a script: %q, %v, %v", text, found, err)
	}
}

func TestScriptVersion(t *testing.T) {
	if v, err := ScriptVersion("#!/bin/sh\nversion=9.1.0\nversion_note=\"version=$version\"\n"); err != nil || v != "9.1.0" {
		t.Errorf("ScriptVersion = %q, %v", v, err)
	}
	for script, want := range map[string]string{
		"version=latest\n":               "its version= line names no release's version: latest",
		"#!/bin/sh\n":                    "it has 0 version= lines, not one",
		"version=9.1.0\nversion=9.2.0\n": "it has 2 version= lines, not one",
	} {
		if v, err := ScriptVersion(script); kind.Of(err) != kind.Usage || err.Error() != want {
			t.Errorf("ScriptVersion(%q) = %q, %v; want %s", script, v, err, want)
		}
	}
}
