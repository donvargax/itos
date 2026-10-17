package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/release"
)

// writeDirect runs initWrite itself, past the check init makes for a config
// already there, its offers declined.
func writeDirect(t *testing.T, stealth bool) error {
	t.Helper()
	declined := rulesFlag{given: true}
	_, err := initWrite(stealth, false, pluginOffer{flag: pluginFlag{given: true}}, shimOffer{flag: shimFlag{given: true}},
		rulesOffer{flag: declined}, io.Discard, Out{Stdout: io.Discard, Stderr: io.Discard})
	return err
}

// A file init cannot write, or a folder it cannot make, stops it with the
// error, each of its starter files in turn.
func TestInitWriteStopsAtAFileItCannotWrite(t *testing.T) {
	for _, c := range []struct {
		name   string
		files  map[string]string
		locked string
		want   string
	}{
		{"the config", nil, ".", "itos.yaml"},
		{"the ledger's folder", map[string]string{"tasks": "mine\n"}, "", "tasks"},
		{"the registry", map[string]string{"tasks/phase-1.yaml": "mine\n"}, "tasks", "tasks/work-items.yaml"},
		{"the smoke set", map[string]string{"features/a.feature": "Feature: A\n"}, "features", "features/smoke.yaml"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := initScratch(t, "commit", c.files)
			if c.locked != "" {
				readOnly(t, filepath.Join(dir, c.locked))
			}
			_, _, _, err := initRun(t, false)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("%v", err)
			}
		})
	}
}

// The scenarios are listed before anything is written: a config there that
// does not load, or whose kind cannot list them, stops init with nothing
// written; and under a stealth config, a git that cannot name its folder.
func TestInitWriteStopsBeforeWriting(t *testing.T) {
	for name, config := range map[string]string{
		"a config that does not load": "version: [\n",
		"a list that fails":           "version: 1\ntests: { scenario: { adapter: { command: sh fails.sh } } }\n",
	} {
		t.Run(name, func(t *testing.T) {
			initScratch(t, "commit", map[string]string{"itos.yaml": config, "fails.sh": "exit 3\n",
				"features/a.feature": "Feature: A\n"})
			if err := writeDirect(t, false); err == nil {
				t.Error("no error")
			}
			if _, err := os.Stat(starterLedger); err == nil {
				t.Errorf("%s was written", starterLedger)
			}
		})
	}
	initScratch(t, "commit", nil)
	useFailingGit(t)
	if err := writeDirect(t, true); err == nil || !strings.Contains(err.Error(), "rev-parse --git-common-dir") {
		t.Errorf("stealth, git failing: %v", err)
	}
}

// Under --json init prints one object: its config, the files, HEAD as
// since, and the pin, its version and its checksums' digest, or with no
// release server none and why.
func TestInitWriteUnderJSON(t *testing.T) {
	type report struct {
		Config     string
		Since      string
		Pin        *struct{ Version, Checksums string }
		PinProblem string `json:"pin_problem"`
	}
	sums := "aa  itos-9.2.0-linux-amd64.tar.gz\n"
	for _, served := range []bool{true, false} {
		initScratch(t, "commit", nil)
		if served {
			releaseServer(t, map[string]string{"/latest/download/checksums.txt": sums})
		}
		code, stdout, _, err := initRun(t, true)
		var got report
		if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Config != "itos.yaml" ||
			len(got.Since) != 40 || (got.Pin != nil) != served || (got.PinProblem == "") != served ||
			(served && (got.Pin.Version != "9.2.0" || got.Pin.Checksums != release.SHA256([]byte(sums)))) {
			t.Errorf("served %v: exit %d, %v:\n%s", served, code, err, stdout)
		}
	}
}

// Hooks that cannot be declared stop init once its files are written; an
// entry under itos's name that does not call itos is its exit code, 1.
func TestInitWriteWhereTheHooksFail(t *testing.T) {
	t.Run("git config failing", func(t *testing.T) {
		initScratch(t, "commit", nil)
		useWrappedGit(t, "-C . config --local --replace-all", "", "")
		if _, _, _, err := initRun(t, false); err == nil || !strings.Contains(err.Error(), "injected git failure") {
			t.Errorf("%v", err)
		}
	})
	t.Run("a hook of another's", func(t *testing.T) {
		initScratch(t, "commit", nil)
		gitIn(t, "config", "--local", "hook.itos-commit-msg.command", "echo mine")
		gitIn(t, "config", "--local", "hook.itos-commit-msg.event", "commit-msg")
		code, stdout, stderr, err := initRun(t, false)
		if code != ExitPolicy || err != nil || !strings.Contains(stderr, "does not call itos") ||
			strings.Contains(stdout, "Commit what init wrote") {
			t.Errorf("exit %d, %v:\n%s%s", code, err, stdout, stderr)
		}
	})
}

// The plugin installed for the project is declared in its settings, which
// init names among what to commit; installed at another scope, it leaves
// the project's settings out, even a changed one that is there.
func TestInitWriteNamesTheProjectsPluginSettings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake claude is a shell script")
	}
	for _, c := range []struct {
		scope string
		files map[string]string
		want  string
	}{
		{"project", nil, "git add itos.yaml tasks/phase-1.yaml tasks/work-items.yaml .claude/settings.json, "},
		{"user", map[string]string{".claude/settings.json": "{\"mine\": true}\n"},
			"git add itos.yaml tasks/phase-1.yaml tasks/work-items.yaml, "},
	} {
		t.Run(c.scope, func(t *testing.T) {
			dir := initScratch(t, "commit", c.files)
			bin := filepath.Join(dir, ".git", "bin")
			putFile(t, filepath.Join(bin, "claude"), "#!/bin/sh\ncase \"$1 $2 $5\" in\n\"plugin list \"*) echo '[]' ;;\n"+
				"\"plugin install project\") mkdir -p .claude && echo '{}' > .claude/settings.json ;;\nesac\n")
			if err := os.Chmod(filepath.Join(bin, "claude"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			var stdout strings.Builder
			code, err := initCommand([]string{"--plugin", c.scope, "--no-agent-rules", "--no-git-shim"},
				Out{Stdout: &stdout, Stderr: &stdout})
			want := c.want + "then itos commit --task T-1 -m 'chore: adopt itos'\n"
			if code != 0 || err != nil || !strings.HasSuffix(stdout.String(), want) {
				t.Errorf("exit %d, %v:\n%s", code, err, stdout.String())
			}
		})
	}
}
