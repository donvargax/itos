package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The push slice's gh is the one on the PATH, run in the folder itos runs
// in, and its GitHub repository is the variable the config names for the
// watch's, else the remote's URL.
func TestPushPortsReachGhAndTheRepository(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	gitLine(t, "remote", "add", "origin", "git@github.com:o/r.git")
	var p pushPorts
	cfg, err := p.LoadConfig(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_REPOSITORY", "")
	if got := p.GitHubRepository(t.Context(), cfg, "origin"); got != "o/r" {
		t.Errorf("GitHubRepository = %q, want the remote's", got)
	}
	t.Setenv("GITHUB_REPOSITORY", "x/y")
	if got := p.GitHubRepository(t.Context(), cfg, "origin"); got != "x/y" {
		t.Errorf("GitHubRepository = %q, want the environment's", got)
	}
	if got := p.GitHubRepository(t.Context(), cfg, "nowhere"); got != "x/y" {
		t.Errorf("GitHubRepository of no remote = %q, want the environment's", got)
	}
	if runtime.GOOS == "windows" {
		t.Skip("the gh is a shell script")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\nprintf '%s' \"$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := (pushPorts{}).Gh(t.Context(), "pr", "list"); out != "pr list" || err != nil {
		t.Errorf("Gh = %q, %v", out, err)
	}
}
