package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A core.hooksPath that git -c sets, handed on by the shim or by git, in
// any case, is refused before anything else, a usage error.
func TestGitCommitRefusesHooksPath(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	t.Chdir(t.TempDir())
	for _, name := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
	}
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_CONFIG_KEY_1", "Core.HooksPath")
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

// With no itos config, the command reaches git, whose exit code is itos's
// (bug 120): an unknown option is git's to refuse, 129.
func TestGitCommitWithoutAConfig(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	t.Chdir(t.TempDir())
	gitIn(t, "init", "-q")
	var stderr strings.Builder
	code, err := gitCommit([]string{"--zz-bogus"}, Out{Stdout: io.Discard, Stderr: &stderr})
	if code != 129 || err != nil || !strings.Contains(stderr.String(), "unknown option") {
		t.Errorf("exit %d, %v\n%s", code, err, stderr.String())
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

// The ports bind the stealth check, the config's two loads and the working
// tree: check-message reads a file from it, and a commit under the stealth
// config writes its links as the new commit's note, setting
// notes.rewriteRef, while a project's config that does not load is the
// error, before git runs.
func TestCommitPortsBindTheRepository(t *testing.T) {
	_, gitDir := notesRepo(t)
	writeFile(t, filepath.Join(gitDir, "itos", "itos.yaml"), "version: 1\ncommits:\n  types: [docs]\n"+
		"  footers:\n    Task: { source: ledger }\n")
	if err := os.Mkdir("sub", 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join("sub", "msg.txt"), "docs: add b\n")
	var stdout, stderr strings.Builder
	code, err := runCommit([]string{"check-message", "msg.txt"}, Out{Stdout: &stdout, Stderr: &stderr, Origin: "sub"})
	if code != 0 || err != nil {
		t.Errorf("check-message: exit %d, %v\n%s", code, err, stderr.String())
	}
	writeFile(t, "b.md", "b\n")
	gitIn(t, "add", "--", "b.md")
	standInHooks(t)
	for name, value := range map[string]string{"GIT_AUTHOR_NAME": "A", "GIT_AUTHOR_EMAIL": "a@example.com",
		"GIT_COMMITTER_NAME": "A", "GIT_COMMITTER_EMAIL": "a@example.com"} {
		t.Setenv(name, value)
	}
	code, err = runCommit([]string{"--task", "T-1", "-m", "docs: add b"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err != nil {
		t.Fatalf("a stealth commit: exit %d, %v\n%s", code, err, stderr.String())
	}
	if note := gitIn(t, "notes", "--ref", "refs/notes/itos", "show", "HEAD"); note != "Task: T-1\n" {
		t.Errorf("the note is %q", note)
	}
	if ref := gitIn(t, "config", "--get", "notes.rewriteRef"); ref != "refs/notes/itos\n" {
		t.Errorf("notes.rewriteRef is %q", ref)
	}
	if err := os.Remove(filepath.Join(gitDir, "itos", "itos.yaml")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "itos.yaml", "version: [\n")
	if _, err := runCommit([]string{"-m", "docs: x"}, Out{Stdout: &stdout, Stderr: &stderr}); err == nil {
		t.Error("a config that does not load: no error")
	}
}
