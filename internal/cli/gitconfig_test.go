package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v3/internal/git"
)

// A scratch repository with a config, as the current folder, away from the
// repository a hook runs the tests in and from the user's git config.
func gitConfigRepo(t *testing.T, config string) string {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("ITOS_CONFIG", "")
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	if err := os.WriteFile(filepath.Join(dir, "itos.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

func localConfig(t *testing.T, key string) string {
	t.Helper()
	out, _ := exec.Command("git", "config", "--local", "--get-all", key).Output()
	return string(out)
}

// The hooks are declared once, the pre-push one only with hooks.pre_push,
// and a second run changes nothing.
func TestDeclareHooks(t *testing.T) {
	gitConfigRepo(t, "version: 1\nhooks: { bin: itos, pre_push: { per_base: \"true\", whole: \"true\" } }\n")
	code, stdout, stderr := run("hooks", "install", "--manager", "git-config")
	if code != 0 || stdout != "Using the git config (--manager git-config)\nwrote hook.itos-commit-msg\nwrote hook.itos-pre-push\n" {
		t.Fatalf("first run: exit %d\n%s%s", code, stdout, stderr)
	}
	code, stdout, _ = run("hooks", "install", "--manager", "git-config")
	if code != 0 || !strings.Contains(stdout, "unchanged hook.itos-commit-msg\nunchanged hook.itos-pre-push\n") {
		t.Errorf("second run: exit %d\n%s", code, stdout)
	}
	if got := localConfig(t, "hook.itos-commit-msg.command"); got != "itos hook commit-msg\n" {
		t.Errorf("commit-msg command: %q", got)
	}
	if got := localConfig(t, "hook.itos-pre-push.event"); got != "pre-push\n" {
		t.Errorf("pre-push event: %q", got)
	}
}

// A git that runs no hook its config declares is a missing environment.
func TestDeclareHooksOldGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake git is a shell script, which windows does not run as git")
	}
	gitConfigRepo(t, "version: 1\n")
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	fake := "#!/bin/sh\nfor a in \"$@\"; do [ \"$a\" = hook ] && { echo \"git: 'hook' is not a git command\" >&2; exit 1; }; done\n" +
		"[ \"$1\" = --version ] && { echo 'git version 2.30.0'; exit 0; }\nexec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	// A run under itos (a hook's) names the real git, which would win.
	t.Setenv(git.EnvGit, "")
	code, _, stderr := run("hooks", "install", "--manager", "git-config")
	if code != ExitMissing || !strings.Contains(stderr, "needs a git that runs the hooks its config declares") ||
		!strings.Contains(stderr, "which 2.30.0 does not") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if got := localConfig(t, "hook.itos-commit-msg.command"); got != "" {
		t.Errorf("declared anyway: %q", got)
	}
}
