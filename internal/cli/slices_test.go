package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/version"
)

// The version slice's ports read the binary's version and the config's
// requires, and judge one against the other.
func TestVersionPorts(t *testing.T) {
	config := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(config, []byte("version: 1\nrequires: \">=999.0.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ITOS_CONFIG", config)
	code, stdout, stderr := run("version", "--check")
	if code != ExitPolicy || stdout != "itos "+version.Version()+"\n" || !strings.Contains(stderr, "does not satisfy >=999.0.0") {
		t.Errorf("version --check: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, _ = run("--json", "version")
	if code != 0 || !strings.Contains(stdout, `"requires": ">=999.0.0"`) || !strings.Contains(stdout, `"satisfied": false`) {
		t.Errorf("--json version: exit %d, stdout %q", code, stdout)
	}
	t.Setenv("ITOS_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	if code, stdout, _ = run("--json", "version"); code != 0 || strings.Contains(stdout, "requires") {
		t.Errorf("--json version with no config: exit %d, stdout %q", code, stdout)
	}
	if code, _, _ = run("version", "--check"); code != ExitUsage {
		t.Errorf("version --check with no config: exit %d", code)
	}
}

// The guard slice reads stdin and asks config.Managed of the folder.
func TestGuardPorts(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "itos.yaml"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ITOS_CONFIG", "")
	if !(guardPorts{}).Managed(context.Background(), repo) || (guardPorts{}).Managed(context.Background(), t.TempDir()) {
		t.Error("Managed is not config.Managed")
	}
	input := filepath.Join(t.TempDir(), "input.json")
	cwd, err := json.Marshal(repo)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"tool_name":"Bash","tool_input":{"command":"git push"},"cwd":` + string(cwd) + `}`
	if err := os.WriteFile(input, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	stdin, err := os.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	saved := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() { os.Stdin = saved })
	code, stdout, stderr := run("guard", "claude-code")
	if code != 0 || !strings.Contains(stdout, `"permissionDecision":"deny"`) || stderr != "" {
		t.Errorf("guard claude-code: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// The followup slice keeps its threads in the clone's private store and
// reads the clock; a line it cannot read is a usage error, and a threads'
// file its domain refuses a data file itos refuses, both exit 2.
func TestFollowupPorts(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\n")
	if code, _, stderr := run("followup", "bogus"); code != ExitUsage || !strings.Contains(stderr, "followup has no subcommand bogus") {
		t.Errorf("followup bogus: exit %d, stderr %q", code, stderr)
	}
	before := time.Now().Truncate(time.Second)
	if code, _, stderr := run("followup", "add", "a", "--with", "ana", "--title", "T", "--note", "N"); code != 0 || stderr != "" {
		t.Fatalf("followup add: exit %d, stderr %q", code, stderr)
	}
	// Git names the folder by its long, resolved spelling; a Windows
	// runner's temp dir may be an 8.3 short one.
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, ".git", "itos", "follow-ups.yaml")
	text, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var stamp string
	for line := range strings.Lines(string(text)) {
		if at, ok := strings.CutPrefix(strings.TrimSpace(line), "- at: "); ok {
			stamp = strings.Trim(at, `"'`)
		}
	}
	if at, err := time.Parse(time.RFC3339, stamp); err != nil || at.Before(before) || at.After(time.Now()) {
		t.Errorf("the note is not dated now: %q, %v\n%s", stamp, err, text)
	}
	if err := os.WriteFile(file, []byte("threads: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run("followup"); code != ExitUsage || !strings.Contains(stderr, file) {
		t.Errorf("an invalid threads' file: exit %d, stderr %q", code, stderr)
	}
}
