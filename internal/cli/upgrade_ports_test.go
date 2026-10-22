package cli

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/release"
)

// runUpgrade reads its line through the slice and binds its ports to the
// config itos finds, the release server, the files on disk and the real
// git: a line it refuses is the usage error, and an upgrade moves the pin,
// the install script and a stale block of the rules, and removes the key a
// release's schema removes.
func TestRunUpgradeBindsItsPorts(t *testing.T) {
	pinned := func(v string) string {
		return "version: 1\npin: { version: " + v + ", checksums: " + release.SHA256([]byte(sumsOf(v))) + " }\n"
	}
	script := func(v, sum string) string {
		return "#!/bin/sh\nversion=" + v + "\nLinux-x86_64) platform=linux-amd64 sum=" + sum + " ;;\n"
	}
	file := upgradeIn(t, "9.3.0", pinned("9.1.0")+"ci:\n  watch:\n    timeout: 600\n", map[string]string{
		"/download/v9.3.0/itos.schema.json": `{"x-itos-removed": ["ci.watch.timeout"]}`,
	})
	writeFiles(t, map[string]string{pin.InstallScript: script("9.1.0", strings.Repeat("1", 64)), agentsFile: "Mine.\n\n" + olderBlock})

	var stdout, stderr strings.Builder
	o := Out{Stdout: &stdout, Stderr: &stderr}
	if _, err := runUpgrade([]string{"--force"}, o); ExitCode(err) != ExitUsage {
		t.Errorf("--force: %v", err)
	}
	ctx := context.Background()
	if c, err := (upgradePorts{}).Config(ctx); err != nil || c.File != file || c.Version != "9.1.0" {
		t.Errorf("the config: %+v, %v", c, err)
	}
	if _, err := (upgradePorts{}).Fetch(ctx, release.URL("9.3.0", "nothing.json")); !release.NotFound(err) {
		t.Errorf("an asset the release lacks: %v", err)
	}
	if (upgradePorts{}).Stealth(ctx, file) {
		t.Errorf("%s is stealth", file)
	}
	if code, err := runUpgrade(nil, o); code != 0 || err != nil {
		t.Fatalf("upgrade: exit %d, %v, stderr %q", code, err, stderr.String())
	}
	if text, _ := os.ReadFile(file); !strings.HasPrefix(string(text), pinned("9.3.0")) || strings.Contains(string(text), "timeout") {
		t.Errorf("the config is %q", text)
	}
	if text, _ := os.ReadFile(pin.InstallScript); string(text) != script("9.3.0", linuxSum) {
		t.Errorf("the install script is %q", text)
	}
	if text, _ := os.ReadFile(agentsFile); string(text) != "Mine.\n\n"+rulesBlock {
		t.Errorf("AGENTS.md is %q", text)
	}
	if want := "Upgraded itos from 9.1.0 to 9.3.0: edited " + file + ", " + pin.InstallScript + " and " + agentsFile + ", and committed nothing."; !strings.HasPrefix(stdout.String(), want) {
		t.Errorf("stdout %q, want it to start %q", stdout.String(), want)
	}
}
