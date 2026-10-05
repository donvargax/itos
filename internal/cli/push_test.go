package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A scratch repository with a commit of its own past its upstream, a bare
// origin in its git folder, and a pre-push hook that makes a commit while
// the push runs (bug 21), as a commit made during the hook's unit tests.
func lateCommitRepo(t *testing.T) string {
	t.Helper()
	dir := gitConfigRepo(t, "version: 1\n")
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(name, "itos")
	}
	for _, name := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "itos@example.com")
	}
	sh := func(script string) {
		t.Helper()
		cmd := exec.Command("sh", "-c", script)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %s", script, out)
		}
	}
	sh(`git checkout -q -b main && echo start > README.md && git add README.md && git commit -q -m 'chore: start' &&
		git init -q --bare .git/origin.git && git remote add origin .git/origin.git && git push -q -u origin main &&
		echo mine > mine.md && git add mine.md && git commit -q -m 'docs: mine'`)
	hook := "#!/bin/sh\necho late > late.md && git add late.md && git commit -q -m 'docs: late' -- late.md\n"
	if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "pre-push"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// What git prints for the arguments in the current folder, trimmed.
func gitLine(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		t.Fatalf("git %s: %s", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

// The commit pushed is HEAD as the rebase left it, resolved once: the one
// the hook made moves HEAD while git pushes, and is neither pushed nor
// named.
func TestPushNamesTheCommitPushed(t *testing.T) {
	lateCommitRepo(t)
	code, stdout, stderr := run("push", "--no-wait")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if got := gitLine(t, "log", "-1", "--format=%s", "HEAD"); got != "docs: late" {
		t.Fatalf("the hook made no commit: HEAD is %q", got)
	}
	pushed := gitLine(t, "--git-dir", ".git/origin.git", "rev-parse", "main")
	if subject := gitLine(t, "log", "-1", "--format=%s", pushed); subject != "docs: mine" {
		t.Errorf("origin's main is at %q, not docs: mine", subject)
	}
	short := gitLine(t, "rev-parse", "--short", pushed)
	if want := "Pushed " + short + " to origin/main.\n"; stdout != want {
		t.Errorf("stdout %q, want %q", stdout, want)
	}
}

// Under --json, the commit key is the one pushed, past the wait, which
// reads the config and has no provider to wait with.
func TestPushJSONCommitIsTheOnePushed(t *testing.T) {
	lateCommitRepo(t)
	code, stdout, stderr := run("push", "--json")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	var got struct {
		Outcome string `json:"outcome"`
		Commit  string `json:"commit"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("%s: %s", err, stdout)
	}
	pushed := gitLine(t, "--git-dir", ".git/origin.git", "rev-parse", "main")
	if got.Outcome != "pushed" || got.Commit != pushed {
		t.Errorf("outcome %q commit %q, want pushed %s", got.Outcome, got.Commit, pushed)
	}
}
