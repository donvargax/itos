//go:build unix

package providers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// gh runs in the folder itos runs in: a gh that answers with the folder it
// runs in gives it as the token, the login and, through the config's
// folder, the session's handle.
func TestGhRunsInTheFolderItosRunsIn(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\npwd -P\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if token, err := GhToken(dir); token != dir || err != nil {
		t.Errorf("GhToken = %q, %v; want %q", token, err, dir)
	}
	if login, err := GhLogin(dir); login != dir || err != nil {
		t.Errorf("GhLogin = %q, %v; want %q", login, err, dir)
	}
	cfg := &config.Loaded{VCS: &porttest.VCS{Root: dir}}
	cfg.Work.Identity.Provider = "github"
	if answer := IdentityProvider(cfg)(); answer.Handle != dir {
		t.Errorf("IdentityProvider = %+v; want the handle %q", answer, dir)
	}
}
