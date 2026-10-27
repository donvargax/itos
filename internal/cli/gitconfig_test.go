package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/kind"
)

// scratchEnv points git away from the repository a hook runs the tests in and
// from the user's git config.
func scratchEnv(t *testing.T) {
	t.Helper()
	disableScratchMaintenance(t)
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("ITOS_CONFIG", "")
}

// A scratch repository with a config, as the current folder, away from the
// repository a hook runs the tests in and from the user's git config. The
// repository git init makes is made once for the package and copied.
func gitConfigRepo(t *testing.T, config string) string {
	t.Helper()
	return gitConfigRepoIn(t, t.TempDir(), config)
}

// gitConfigRepoIn is gitConfigRepo in the folder dir.
func gitConfigRepoIn(t *testing.T, dir, config string) string {
	t.Helper()
	scratchEnv(t)
	fresh := madeOnce(t, "git-init", func(t *testing.T, fresh string) {
		if out, err := exec.Command(git.Bin(runtime.GOOS), "init", "-q", fresh).CombinedOutput(); err != nil {
			t.Fatalf("git init: %s", out)
		}
	})
	copyTree(t, fresh, dir)
	if err := os.WriteFile(filepath.Join(dir, "itos.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

// scratchCopy is a copy of the folder template, made once by an earlier
// madeOnce, as the current folder under gitConfigRepo's environment.
func scratchCopy(t *testing.T, template string) string {
	t.Helper()
	scratchEnv(t)
	dir := t.TempDir()
	copyTree(t, template, dir)
	t.Chdir(dir)
	return dir
}

// itos's two hooks declared in the scratch repository's git config, as hook
// install names them, each a command that runs nothing (: is sh's no-op), so
// a commit or a push the test makes is not refused for want of them (slice
// 91) and runs no check of theirs.
func standInHooks(t *testing.T) {
	t.Helper()
	file := filepath.Join(".git", "config")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]map[string]string{}
	for _, event := range hookEvents {
		settings[fmt.Sprintf("[hook %q]", hookEntry(event))] = map[string]string{
			"event": event, "command": ": itos hook " + event,
		}
	}
	// These fresh repositories have Git's canonical sections and one-line
	// values, not arbitrary config syntax. Keep every unrelated line intact.
	var text strings.Builder
	seen := map[string]bool{}
	section := ""
	for _, line := range strings.SplitAfter(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = trimmed
		}
		if key, _, assigned := strings.Cut(trimmed, "="); assigned {
			key = strings.ToLower(strings.TrimSpace(key))
			if value, known := settings[section][key]; known {
				line = fmt.Sprintf("\t%s = %s\n", key, value)
				seen[section+key] = true
			}
		}
		text.WriteString(line)
	}
	for _, event := range hookEvents {
		section := fmt.Sprintf("[hook %q]", hookEntry(event))
		if seen[section+"event"] && seen[section+"command"] {
			continue
		}
		fmt.Fprintf(&text, "\n%s\n", section)
		for _, key := range []string{"event", "command"} {
			if !seen[section+key] {
				fmt.Fprintf(&text, "\t%s = %s\n", key, settings[section][key])
			}
		}
	}
	if err := os.WriteFile(file, []byte(text.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func localConfig(t *testing.T, key string) string {
	t.Helper()
	out, _ := exec.Command(git.Bin(runtime.GOOS), "config", "--local", "--get-all", key).Output()
	return string(out)
}

func TestStandInHooksFixture(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "replace"}[existing], func(t *testing.T) {
			gitConfigRepo(t, "version: 1\n")
			file := filepath.Join(".git", "config")
			original, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			settings := "\n# Keep the repository's own settings.\n[fixture]\n\tkept = existing\n" +
				"[hook \"itos-commit-msg\"]\n\tnote = kept\n"
			if existing {
				settings += "[hook \"itos-commit-msg\"]\n\tevent = old\n\tcommand = old\n" +
					"[hook \"itos-pre-push\"]\n\tevent = old\n\tcommand = old\n"
			}
			if err := os.WriteFile(file, append(original, []byte(settings)...), 0o644); err != nil {
				t.Fatal(err)
			}
			trace := filepath.Join(t.TempDir(), "git-trace.json")
			t.Setenv("GIT_TRACE2_EVENT", trace)
			starts := func() int {
				t.Helper()
				text, err := os.ReadFile(trace)
				if err != nil {
					t.Fatal(err)
				}
				decoder := json.NewDecoder(strings.NewReader(string(text)))
				count := 0
				for {
					var record struct{ Event string }
					if err := decoder.Decode(&record); err == io.EOF {
						return count
					} else if err != nil {
						t.Fatal(err)
					}
					if record.Event == "start" {
						count++
					}
				}
			}
			gitIn(t, "config", "--local", "--get", "fixture.kept")
			if got := starts(); got != 1 {
				t.Fatalf("real Git trace recorded %d process starts, want 1", got)
			}
			if err := os.WriteFile(trace, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			standInHooks(t)
			standInHooks(t)
			if got := starts(); got != 0 {
				t.Fatalf("standInHooks started %d Git processes, want 0", got)
			}
			t.Setenv("GIT_TRACE2_EVENT", "")
			for key, want := range map[string]string{
				"hook.itos-commit-msg.event":   "commit-msg\n",
				"hook.itos-commit-msg.command": ": itos hook commit-msg\n",
				"hook.itos-pre-push.event":     "pre-push\n",
				"hook.itos-pre-push.command":   ": itos hook pre-push\n",
				"hook.itos-commit-msg.note":    "kept\n",
				"fixture.kept":                 "existing\n",
			} {
				if got := gitIn(t, "config", "--local", "--get-all", key); got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
			text, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(text), string(original)) || !strings.Contains(string(text), "# Keep the repository's own settings.\n") {
				t.Errorf("existing config was not preserved:\n%s", text)
			}
		})
	}
}

// Both hooks are declared once, the pre-push one whatever hooks.pre_push
// says, and a second run changes nothing; a commit and a push are ready
// only once they are.
func TestDeclareHooks(t *testing.T) {
	gitConfigRepo(t, "version: 1\nhooks: { bin: itos }\n")
	if err := hooksReady("", "commit-msg"); kind.Of(err) != kind.Missing ||
		!strings.HasPrefix(err.Error(), "itos's commit-msg hook is not declared") ||
		!strings.Contains(err.Error(), "itos hook install") {
		t.Errorf("ready before hook install: %v", err)
	}
	if err := hooksReady("", "commit-msg", "pre-push"); kind.Of(err) != kind.Missing ||
		!strings.HasPrefix(err.Error(), "itos's commit-msg and pre-push hooks are not declared") {
		t.Errorf("both ready before hook install: %v", err)
	}
	code, stdout, stderr := run("hook", "install")
	if code != 0 || stdout != "wrote hook.itos-commit-msg\nwrote hook.itos-pre-push\n" {
		t.Fatalf("first run: exit %d\n%s%s", code, stdout, stderr)
	}
	if err := hooksReady("", "commit-msg", "pre-push"); err != nil {
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
	real, err := git.Real(runtime.GOOS)
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
	if err := hooksReady("", "commit-msg"); kind.Of(err) != kind.Missing ||
		!strings.Contains(err.Error(), "git 2.30.0 runs no hook its config declares, and no hook file in .git/hooks") {
		t.Errorf("ready with an old git: %v", err)
	}
	kept := filepath.Join(".git", "hooks", "pre-push")
	if err := os.WriteFile(kept, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run("hook", "install")
	if code != 0 || !strings.Contains(stdout, "written .git/hooks/commit-msg") ||
		!strings.Contains(stdout, "replaced .git/hooks/pre-push") {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	if err := hooksReady("", hookEvents...); err != nil {
		t.Errorf("ready with itos's hook files: %v", err)
	}
	if text, err := os.ReadFile(kept + ".before-itos"); err != nil || string(text) != "#!/bin/sh\nexit 0\n" {
		t.Errorf("the kept hook: %q, %v", text, err)
	}
	if info, err := os.Stat(kept); err != nil || runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Errorf("itos's pre-push hook is not executable: %v", err)
	}
	if got := localConfig(t, "hook.itos-commit-msg.command"); got != "" {
		t.Errorf("declared anyway: %q", got)
	}
}

// A hook file that cannot be written, its folder under a file or the file
// a folder, is hookFiles' error.
func TestHookFilesWriteFails(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (hookFiles{}).WriteHookFile(t.Context(), filepath.Join(file, "commit-msg"), "x"); err == nil {
		t.Error("a folder under a file: no error")
	}
	if err := (hookFiles{}).WriteHookFile(t.Context(), dir, "x"); err == nil {
		t.Error("a file that is a folder: no error")
	}
}
