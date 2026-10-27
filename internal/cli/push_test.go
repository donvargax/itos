package cli

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// A scratch repository with a commit of its own past its upstream, a bare
// origin in its git folder, and a pre-push hook that makes a commit while
// the push runs (bug 21), as a commit made during the hook's unit tests;
// itos's own hooks are stand-ins (standInHooks).
func lateCommitRepo(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(name, "itos")
	}
	for _, name := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "itos@example.com")
	}
	// The real git for every git the scripts and the hooks start, as an itos
	// run hands it to what it starts (git.Export): an itos linked as git
	// would run git push here as itos push (T-104).
	t.Setenv(git.EnvGit, git.Bin(runtime.GOOS))
	// Made once for the package and copied: its commits and its origin are
	// twenty git starts.
	return scratchCopy(t, madeOnce(t, "late-commit", func(t *testing.T, dir string) {
		gitConfigRepoIn(t, dir, "version: 1\n")
		standInHooks(t)
		shIn(t, dir, noMaintenance+`git checkout -q -b main && echo start > README.md && git add README.md && git commit -q -m 'chore: start' &&
		git init -q --bare .git/origin.git && git -C .git/origin.git config maintenance.auto false && git remote add origin .git/origin.git && git push -q -u origin main &&
		echo mine > mine.md && git add mine.md && git commit -q -m 'docs: mine'`)
		hook := "#!/bin/sh\necho late > late.md && git add late.md && git commit -q -m 'docs: late' -- late.md\n"
		if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "pre-push"), []byte(hook), 0o755); err != nil {
			t.Fatal(err)
		}
	}))
}

// What git prints for the arguments in the current folder, trimmed.
func gitLine(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command(git.Bin(runtime.GOOS), args...).Output()
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

// A push that fetched its upstream but cannot reach the URL it pushes to
// (slice 90) exits 75, not git's 128, and says the remote cannot be reached.
func TestPushToARemoteOutOfReachExits75(t *testing.T) {
	lateCommitRepo(t)
	if out, err := exec.Command(git.Bin(runtime.GOOS), "config", "remote.origin.pushurl", "https://127.0.0.1:1/itos.git").CombinedOutput(); err != nil {
		t.Fatalf("%s: %s", err, out)
	}
	code, stdout, stderr := run("push", "--no-wait")
	if code != ExitTemporary {
		t.Fatalf("exit %d, want 75\n%s%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "origin cannot be reached") {
		t.Errorf("stderr does not say origin cannot be reached:\n%s", stderr)
	}
}

// A push the pre-push hook refuses exits 1, a check that said no, though
// the hook's own words are those of a remote out of reach.
func TestPushAHookRefusesExits1(t *testing.T) {
	dir := lateCommitRepo(t)
	hook := "#!/bin/sh\necho 'dial tcp 127.0.0.1:8080: connect: Connection refused' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "pre-push"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run("push", "--no-wait")
	if code != ExitPolicy {
		t.Fatalf("exit %d, want 1\n%s%s", code, stdout, stderr)
	}
}

// git push --no-verify, or git's abbreviation of it, is refused naming it
// and the hook it skips (slice 128); a core.hooksPath git -c sets is refused
// before anything is fetched.
func TestPushRefusesSkippingTheHooks(t *testing.T) {
	for _, arg := range []string{"--no-verify", "--no-veri"} {
		code, err := runPush([]string{"--no-wait", arg}, Out{Stdout: io.Discard, Stderr: io.Discard})
		if code != 0 || ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), arg+": it skips the pre-push hook") {
			t.Errorf("push %s = %d, %v", arg, code, err)
		}
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Chdir(t.TempDir())
	code, err := runPush(nil, Out{Stdout: io.Discard, Stderr: io.Discard})
	if code != 0 || ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), "core.hooksPath") {
		t.Errorf("push under git -c core.hooksPath = %d, %v", code, err)
	}
}

// pushAs sets the author and the committer of the scratch repository's
// commits.
func pushAs(t *testing.T) {
	t.Helper()
	for _, kv := range [][2]string{{"GIT_AUTHOR_NAME", "itos"}, {"GIT_COMMITTER_NAME", "itos"},
		{"GIT_AUTHOR_EMAIL", "itos@example.com"}, {"GIT_COMMITTER_EMAIL", "itos@example.com"}} {
		t.Setenv(kv[0], kv[1])
	}
}

// behindOrigin gives the scratch repository a bare origin in its git folder
// whose main is the branch up, which main tracks, and itos's own hooks as
// stand-ins: itos push then rebases main onto up.
func behindOrigin(t *testing.T) {
	t.Helper()
	standInHooks(t)
	t.Setenv(git.EnvGit, git.Bin(runtime.GOOS))
	gitLine(t, "init", "-q", "--bare", ".git/origin.git")
	gitLine(t, "remote", "add", "origin", ".git/origin.git")
	gitLine(t, "push", "-q", "origin", "up:main")
	gitLine(t, "fetch", "-q", "origin")
	gitLine(t, "branch", "-q", "--set-upstream-to=origin/main", "main")
}

// A rebase git stops with no conflict, at a commit that would overwrite an
// untracked file, is a rebase stopped all the same: the push says so, exit
// 1, and leaves the rebase to be continued or aborted.
func TestPushRebaseStoppedWithNoConflict(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	pushAs(t)
	gitLine(t, "checkout", "-q", "-b", "main")
	gitLine(t, "commit", "-q", "--allow-empty", "-m", "chore: start")
	gitLine(t, "checkout", "-q", "-b", "up")
	gitLine(t, "commit", "-q", "--allow-empty", "-m", "docs: theirs")
	gitLine(t, "checkout", "-q", "main")
	writeFile(t, "f.md", "mine\n")
	gitLine(t, "add", "f.md")
	gitLine(t, "commit", "-q", "-m", "docs: add f")
	gitLine(t, "rm", "-q", "f.md")
	gitLine(t, "commit", "-q", "-m", "docs: drop f")
	behindOrigin(t)
	writeFile(t, "f.md", "untracked\n")
	code, stdout, stderr := run("push", "--json")
	var got struct {
		Outcome string `json:"outcome"`
	}
	if code != ExitPolicy || json.Unmarshal([]byte(stdout), &got) != nil || got.Outcome != "rebase-stopped" {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if !(vcs.Git{}).Rebasing(t.Context()) || len((vcs.Git{}).Conflicted(t.Context())) > 0 {
		t.Errorf("rebasing %v, conflicted %v", (vcs.Git{}).Rebasing(t.Context()), (vcs.Git{}).Conflicted(t.Context()))
	}
}

// A rebase that stops on a conflict in the work registry says, beside git,
// what happened there: a take of the same item on both sides names the
// upstream's owner and how to give the item up (slice 66); an item added on
// each side names none as taken and says to keep both and continue, and
// nothing in the output advises git rebase --skip (bug 118).
func TestPushRebaseStoppedOnTheRegistry(t *testing.T) {
	item := func(id, owner, status string) string {
		return "  - { id: " + id + ", title: " + id + ", phase: 1, owner: " + owner + ", status: " + status + ", depends_on: [] }\n"
	}
	head := "phases: { 1: null }\nitems:\n"
	base := head + item("slice-9", "null", "todo")
	for _, c := range []struct {
		name, theirs, mine string
		says, not          []string
	}{
		{"a take on both sides", head + item("slice-9", "ana", "doing"), head + item("slice-9", "bo", "doing"),
			[]string{"slice-9 is taken: a commit on origin/main made ana its owner first.", "git rebase --skip"},
			[]string{"was changed on"}},
		{"an item added on each side", base + item("p1-from-ana", "null", "todo"), base + item("p1-from-bo", "null", "todo"),
			[]string{"added or\nchanged different items", "git rebase --continue"},
			[]string{"the same item", "rebase --skip", "is taken"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			gitConfigRepo(t, "version: 1\n")
			pushAs(t)
			registry := filepath.Join("tasks", "work-items.yaml")
			commit := func(text, subject string) {
				t.Helper()
				writeFiles(t, map[string]string{registry: text})
				gitLine(t, "add", registry)
				gitLine(t, "commit", "-q", "-m", subject)
			}
			gitLine(t, "checkout", "-q", "-b", "main")
			commit(base, "docs: a registry")
			gitLine(t, "checkout", "-q", "-b", "up")
			commit(c.theirs, "docs: theirs")
			gitLine(t, "checkout", "-q", "main")
			commit(c.mine, "docs: mine")
			behindOrigin(t)
			code, stdout, stderr := run("push")
			if code != ExitPolicy {
				t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
			}
			for _, text := range c.says {
				if !strings.Contains(stderr, text) {
					t.Errorf("no %q in:\n%s", text, stderr)
				}
			}
			for _, text := range c.not {
				if strings.Contains(stderr, text) {
					t.Errorf("%q in:\n%s", text, stderr)
				}
			}
		})
	}
}
