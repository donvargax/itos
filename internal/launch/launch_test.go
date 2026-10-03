package launch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

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
			writeConfig(t, c.config)
			t.Setenv(EnvVersion, c.env)
			got, launch := choose(nil)
			if got != c.want || launch != c.launch {
				t.Errorf("choose = %v, %t; want %v, %t", got, launch, c.want, c.launch)
			}
		})
	}
}

func TestChooseReadsTheConfigWhereItosDoes(t *testing.T) {
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
		if got, launch := choose(args); !launch || got.version != "9.1.0" {
			t.Errorf("choose(%q) = %v, %t; want 9.1.0", args, got, launch)
		}
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
	pinned := target{"9.1.0", sha256Hex(text)}
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
