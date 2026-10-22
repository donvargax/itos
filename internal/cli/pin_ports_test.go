package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/release"
)

// runPin reads its line through the slice and binds its ports to the config
// itos finds and the release server: a line it refuses is the usage error,
// and a version is fetched and pinned in that config.
func TestRunPinBindsItsPorts(t *testing.T) {
	scratchFolder(t)
	file := filepath.Join(t.TempDir(), "itos.yaml")
	writeFiles(t, map[string]string{file: "version: 1\npin: { version: 9.1.0 }\n"})
	t.Setenv("ITOS_CONFIG", file)
	sums := "aa  itos-9.2.0-linux-amd64.tar.gz\n"
	releaseServer(t, map[string]string{"/download/v9.2.0/checksums.txt": sums})

	var stdout, stderr strings.Builder
	o := Out{Stdout: &stdout, Stderr: &stderr}
	if _, err := runPin([]string{"newest"}, o); ExitCode(err) != ExitUsage {
		t.Errorf("newest: %v", err)
	}
	c, err := (pinPorts{}).Config(context.Background())
	if err != nil || c.File != file || c.Version != "9.1.0" {
		t.Errorf("the config: %+v, %v", c, err)
	}
	if v, got, err := (pinPorts{}).Release(context.Background(), "9.2.0"); err != nil || v != "9.2.0" || string(got) != sums {
		t.Errorf("the release: %s, %q, %v", v, got, err)
	}
	if code, err := runPin([]string{"9.2.0"}, o); code != 0 || err != nil {
		t.Fatalf("9.2.0: exit %d, %v, stderr %q", code, err, stderr.String())
	}
	text, _ := os.ReadFile(file)
	if !strings.Contains(string(text), "version: 9.2.0") || !strings.Contains(string(text), release.SHA256([]byte(sums))) {
		t.Errorf("the config is %q", text)
	}
	if !strings.HasPrefix(stdout.String(), "Pinned itos 9.2.0 in "+file+", from 9.1.0.\n") {
		t.Errorf("stdout %q", stdout.String())
	}
}
