package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/kind"
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
	if out, err := exec.Command(git.Bin(), "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	if err := os.WriteFile(filepath.Join(dir, "itos.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

// itos's two hooks declared in the scratch repository's git config, as hook
// install names them, each a command that runs nothing (: is sh's no-op), so
// a commit or a push the test makes is not refused for want of them (slice
// 91) and runs no check of theirs.
func standInHooks(t *testing.T) {
	t.Helper()
	for _, event := range hookEvents {
		for key, value := range map[string]string{"event": event, "command": ": itos hook " + event} {
			if out, err := exec.Command(git.Bin(), "config", "--local", "hook."+hookEntry(event)+"."+key, value).CombinedOutput(); err != nil {
				t.Fatalf("git config: %s", out)
			}
		}
	}
}

func localConfig(t *testing.T, key string) string {
	t.Helper()
	out, _ := exec.Command(git.Bin(), "config", "--local", "--get-all", key).Output()
	return string(out)
}

// Both hooks are declared once, the pre-push one whatever hooks.pre_push
// says, and a second run changes nothing; a commit and a push are ready
// only once they are.
func TestDeclareHooks(t *testing.T) {
	gitConfigRepo(t, "version: 1\nhooks: { bin: itos }\n")
	if err := hooksReady("commit-msg"); kind.Of(err) != kind.Missing ||
		!strings.Contains(err.Error(), "itos hook install") {
		t.Errorf("ready before hook install: %v", err)
	}
	code, stdout, stderr := run("hook", "install")
	if code != 0 || stdout != "wrote hook.itos-commit-msg\nwrote hook.itos-pre-push\n" {
		t.Fatalf("first run: exit %d\n%s%s", code, stdout, stderr)
	}
	if err := hooksReady("commit-msg", "pre-push"); err != nil {
		t.Errorf("not ready after hook install: %v", err)
	}
	code, stdout, _ = run("hook", "install")
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
	// The real git, past any itos linked as git (T-104): the fake would
	// otherwise exec a shim, which passes back to the fake as the first git
	// on the PATH that is not an itos, and the two run each other forever.
	real, err := git.Real()
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
	code, _, stderr := run("hook", "install")
	if code != ExitMissing || !strings.Contains(stderr, "git 2.30.0 runs no hook its config declares") ||
		!strings.Contains(stderr, "upgrade git to "+configHooksSince) {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if err := hooksReady("commit-msg"); kind.Of(err) != kind.Missing ||
		!strings.Contains(err.Error(), configHooksSince) {
		t.Errorf("ready with an old git: %v", err)
	}
	if got := localConfig(t, "hook.itos-commit-msg.command"); got != "" {
		t.Errorf("declared anyway: %q", got)
	}
}
