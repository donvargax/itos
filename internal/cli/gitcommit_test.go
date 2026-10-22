package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Each -m is wrapped in its place, however it is written; -F's text goes to
// a file of its own, named where the file was; nothing changes without a
// limit or when git will refuse -m with -F.
func TestWrapBody(t *testing.T) {
	long := "chore: tidy\n\n" + strings.Repeat("word ", 30)
	wrapped := "chore: tidy\n\n" + strings.TrimSpace(strings.Repeat("word ", 20)) + "\n" + strings.TrimSpace(strings.Repeat("word ", 10))
	for _, c := range []struct{ args, want []string }{
		{[]string{"-m", long}, []string{"-m", wrapped}},
		{[]string{"--message=" + long, "-q"}, []string{"--message=" + wrapped, "-q"}},
		{[]string{"-am" + long}, []string{"-am" + wrapped}},
		{[]string{"--message", long, "--", "x"}, []string{"--message", wrapped, "--", "x"}},
	} {
		got, written, err := wrapBody(c.args, readGitArgs(c.args), 100, nil, "#")
		if err != nil || written != "" || !slices.Equal(got, c.want) {
			t.Errorf("wrapBody(%q) = %q, %q, %v", c.args, got, written, err)
		}
	}
	for _, args := range [][]string{{"-m", "x", "-F", "y"}, {"-m", long}} {
		limit := 100
		if len(args) == 2 {
			limit = 0
		}
		if got, _, _ := wrapBody(args, readGitArgs(args), limit, nil, "#"); !slices.Equal(got, args) {
			t.Errorf("wrapBody(%q, %d) = %q", args, limit, got)
		}
	}

	file := filepath.Join(t.TempDir(), "msg.txt")
	if err := os.WriteFile(file, []byte(long+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-F", file}
	got, written, err := wrapBody(args, readGitArgs(args), 100, nil, "#")
	if err != nil || written == "" || !slices.Equal(got, []string{"-F", written}) {
		t.Fatalf("wrapBody(%q) = %q, %q, %v", args, got, written, err)
	}
	defer os.Remove(written)
	if text, _ := os.ReadFile(written); string(text) != wrapped+"\n" {
		t.Errorf("the written message is %q", text)
	}
}

// A link flag takes the next argument as its value only when it has none of
// its own, written --task=<ids>, and the next is no option; one at the end
// with no value is a usage error.
func TestReadCommitFlags(t *testing.T) {
	f, err := readCommitFlags([]string{"--task=T-1", "notes.md", "--item", "a", "-m", "x"}, nil)
	if err != nil || !slices.Equal(f.tasks, []string{"T-1"}) || !slices.Equal(f.items, []string{"a"}) ||
		!slices.Equal(f.git, []string{"notes.md", "-m", "x"}) {
		t.Errorf("readCommitFlags = %+v, %v", f, err)
	}
	for _, args := range [][]string{{"--task"}, {"-m", "x", "--item"}, {"--scenarios", "-m", "x"}} {
		if _, err := readCommitFlags(args, nil); ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), "needs <") {
			t.Errorf("readCommitFlags(%q) = %v, not a usage error", args, err)
		}
	}
}

// The options itos commit refuses (slice 128) are read as git reads them: a
// cluster's letters (-am, -an), git's abbreviation of --no-verify, never a
// valued option's value (-m -n, -mn, --message --no-verify, -uno) nor what
// comes after "--"; the first refused is the one named.
func TestReadGitArgsRefuses(t *testing.T) {
	for args, want := range map[string]string{
		"--no-verify -m x":       "--no-verify",
		"--no-veri":              "--no-veri",
		"-n -m x":                "-n",
		"-a":                     "-a",
		"-am x":                  "-a",
		"-na":                    "-n",
		"-an":                    "-a",
		"-q --all":               "--all",
		"-m x -- --no-verify":    "",
		"-m -n":                  "",
		"-mn":                    "",
		"--message --no-verify":  "",
		"--message=-a":           "",
		"-uno":                   "",
		"-Sn":                    "",
		"--no-ver":               "",
		"--allow-empty --verify": "",
	} {
		if got := readGitArgs(strings.Fields(args)).refused; got != want {
			t.Errorf("readGitArgs(%q).refused = %q, want %q", args, got, want)
		}
	}
}

// A refused option is a usage error naming it and why; a core.hooksPath that
// git -c sets, handed on by the shim or by git, in any case, is refused too.
func TestRefuseCommitArgs(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	t.Chdir(t.TempDir())
	for _, name := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
	}
	for refused, says := range map[string]string{"-a": "git add <path>", "--all": "stages every change",
		"-n": "skips git's hooks", "--no-verify": "skips git's hooks"} {
		err := refuseCommitArgs(gitArgs{refused: refused})
		if ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), refused+":") || !strings.Contains(err.Error(), says) {
			t.Errorf("refusing %s: %v", refused, err)
		}
	}
	if err := refuseCommitArgs(gitArgs{}); err != nil {
		t.Errorf("nothing refused: %v", err)
	}
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_CONFIG_KEY_1", "Core.HooksPath")
	if err := refuseCommitArgs(gitArgs{}); ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), "core.hooksPath") {
		t.Errorf("git -c core.hooksPath through the shim: %v", err)
	}
	if code, err := gitCommit([]string{"-m", "chore: x"}, Out{Stdout: io.Discard, Stderr: io.Discard}); code != 0 ||
		ExitCode(err) != ExitUsage {
		t.Errorf("gitCommit under git -c core.hooksPath = %d, %v", code, err)
	}
	t.Setenv("GIT_CONFIG_KEY_1", "core.editor")
	if hooksPathSet() {
		t.Error("no core.hooksPath among the shim's keys")
	}
	for params, want := range map[string]bool{"'core.hooksPath'='/x'": true, "'core.hookspath=/x'": true,
		"'user.name'='core.hooksPath'": false, "'a'='x' 'CORE.HOOKSPATH'='y'": true,
		"'a'='x '\\''core.hooksPath='": false, "": false} {
		t.Setenv("GIT_CONFIG_PARAMETERS", params)
		if got := hooksPathSet(); got != want {
			t.Errorf("GIT_CONFIG_PARAMETERS %q: hooksPathSet = %v", params, got)
		}
	}
}

// Under --json, a commit made gives ok and the commit's SHA, git's own words
// going to stderr; one the hook refuses gives ok false and no commit.
func TestGitCommitJSONNamesTheCommit(t *testing.T) {
	rollbackRepo(t)
	writeFile(t, "a.md", "a\n")
	gitIn(t, "add", "--", "a.md")
	var stdout, stderr strings.Builder
	code, err := gitCommit([]string{"-m", "docs: add a"}, Out{Stdout: &stdout, Stderr: &stderr, JSON: true})
	var got struct {
		OK     bool    `json:"ok"`
		Commit *string `json:"commit"`
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout.String()), &got) != nil {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
	if head := strings.TrimSpace(gitIn(t, "rev-parse", "HEAD")); !got.OK || got.Commit == nil || *got.Commit != head {
		t.Errorf("stdout %s, HEAD %s", stdout.String(), head)
	}
	refusingHook(t)
	writeFile(t, "a.md", "b\n")
	gitIn(t, "add", "--", "a.md")
	stdout.Reset()
	code, err = gitCommit([]string{"-m", "docs: change a"}, Out{Stdout: &stdout, Stderr: &stderr, JSON: true})
	got.Commit = nil
	if code != 1 || err != nil || json.Unmarshal([]byte(stdout.String()), &got) != nil || got.OK || got.Commit != nil {
		t.Errorf("a refused commit: exit %d, %v\n%s", code, err, stdout.String())
	}
}
