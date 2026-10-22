package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/release"
)

const (
	linuxSum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	winSum   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// releaseServer serves each path's body, 404 for any other, and 500 for
// a body of "fail".
func releaseServer(t *testing.T, files map[string]string) {
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

// sumsOf is a checksums.txt of the version, for linux-amd64 and
// windows-amd64.
func sumsOf(v string) string {
	return linuxSum + "  itos-" + v + "-linux-amd64.tar.gz\n" + winSum + "  itos-" + v + "-windows-amd64.zip\n"
}

// upgradeIn is a project folder, the folder the test runs in, its config
// the text, and a release server whose newest is latest, offering 9.1.0,
// 9.2.0 and 9.3.0 and each one's upgrading.json, the files given laid over
// them.
func upgradeIn(t *testing.T, latest, config string, files map[string]string) string {
	t.Helper()
	served := map[string]string{"/latest/download/checksums.txt": sumsOf(latest)}
	for i, v := range []string{"9.1.0", "9.2.0", "9.3.0"} {
		served["/download/v"+v+"/checksums.txt"] = sumsOf(v)
		previous := ""
		if i > 0 {
			previous = []string{"9.1.0", "9.2.0"}[i-1]
		}
		served["/download/v"+v+"/upgrading.json"] = `{"schema": 1, "version": "` + v + `", "previous": "` + previous + `"}`
	}
	for path, body := range files {
		served[path] = body
	}
	releaseServer(t, served)
	dir := t.TempDir()
	t.Chdir(dir)
	file := filepath.Join(dir, "itos.yaml")
	t.Setenv("ITOS_CONFIG", file)
	if config != "" {
		if err := os.WriteFile(file, []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return file
}

// UpgradeTarget reads an upgrade the launcher can hand on, and leaves
// anything else to the binary called (slice 126).
func TestUpgradeTarget(t *testing.T) {
	upgradeIn(t, "9.3.0", "", nil)
	for _, line := range []string{"", "status", "upgrade --help", "help upgrade", "upgrade 9.1.0 9.2.0", "upgrade --force",
		"upgrade -- 9.2.0", "upgrade nine"} {
		if h, ok, err := UpgradeTarget(strings.Fields(line)); ok || err != nil || h.Version != "" {
			t.Errorf("%q: %v, %t, %v", line, h, ok, err)
		}
	}
	h, ok, err := UpgradeTarget([]string{"--json", "upgrade"})
	if !ok || err != nil || h.Version != "9.3.0" || string(h.Checksums) != sumsOf("9.3.0") ||
		!slices.Equal(h.Args, []string{"--json", "upgrade", "9.3.0"}) {
		t.Errorf("the newest: %v, %t, %v", h, ok, err)
	}
	h, ok, err = UpgradeTarget([]string{"upgrade", "v9.2.0"})
	if !ok || err != nil || h.Version != "9.2.0" || !slices.Equal(h.Args, []string{"upgrade", "9.2.0"}) {
		t.Errorf("a version: %v, %t, %v", h, ok, err)
	}
	h, ok, err = UpgradeTarget([]string{"upgrade", "9.9.0"})
	if !ok || kind.Of(err) != kind.Missing || h.Args != nil {
		t.Errorf("a version the server does not have: %v, %t, %v", h, ok, err)
	}
}
