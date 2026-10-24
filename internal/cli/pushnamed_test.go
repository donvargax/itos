package cli

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

// A scratch repository on main, pushed to a bare origin in its git folder,
// with the annotated tag v1 on its HEAD and the branch b one commit past it;
// itos's own hooks are stand-ins. sh runs a script in it.
func namedPushRepo(t *testing.T) (dir string, sh func(string)) {
	t.Helper()
	dir = gitConfigRepo(t, "version: 1\n")
	standInHooks(t)
	for _, kv := range [][2]string{{"GIT_AUTHOR_NAME", "itos"}, {"GIT_COMMITTER_NAME", "itos"},
		{"GIT_AUTHOR_EMAIL", "itos@example.com"}, {"GIT_COMMITTER_EMAIL", "itos@example.com"}} {
		t.Setenv(kv[0], kv[1])
	}
	t.Setenv(git.EnvGit, git.Bin(runtime.GOOS))
	sh = func(script string) {
		t.Helper()
		cmd := exec.Command("sh", "-c", script)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %s", script, out)
		}
	}
	sh(`git checkout -q -b main && echo start > README.md && git add README.md && git commit -q -m 'chore: start' &&
		git init -q --bare .git/origin.git && git remote add origin .git/origin.git && git push -q -u origin main &&
		git tag -a -m v1 v1 && git checkout -q -b b && echo b > b.md && git add b.md && git commit -q -m 'docs: b' &&
		git checkout -q main`)
	return dir, sh
}

// What the ref names in the bare origin, "" for nothing.
func originRef(t *testing.T, ref string) string {
	t.Helper()
	out, _ := exec.Command(git.Bin(runtime.GOOS), "--git-dir", ".git/origin.git", "rev-parse", "--verify", "--quiet", ref).Output()
	return strings.TrimSpace(string(out))
}

// The tag and the branch are pushed to origin's refs of their names, and
// nothing else: HEAD's branch stays where it was, and so does the tag the
// branch's commit is not.
func TestPushNamedPushesWhatItNames(t *testing.T) {
	_, sh := namedPushRepo(t)
	sh(`echo more > more.md && git add more.md && git commit -q -m 'docs: more' && git tag -a -m v0 v0 HEAD^`)
	main := originRef(t, "refs/heads/main")
	code, stdout, stderr := run("push", "--tag", "v1", "--ref", "b", "--no-wait")
	if code != 0 || stdout != "Pushed tag v1, branch b to origin.\n" {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if originRef(t, "refs/tags/v1") != gitLine(t, "rev-parse", "refs/tags/v1") ||
		originRef(t, "refs/heads/b") != gitLine(t, "rev-parse", "refs/heads/b") {
		t.Errorf("origin does not have v1 and b as the clone does")
	}
	if originRef(t, "refs/heads/main") != main || originRef(t, "refs/tags/v0") != "" {
		t.Errorf("the push sent more than it named")
	}
	// A tag origin has as this clone does is left out, and with nothing
	// left there is nothing to push.
	code, stdout, stderr = run("push", "--tag", "v1")
	if code != 0 || stdout != "Nothing to push: origin has every tag named.\n" {
		t.Errorf("push of a tag origin has: exit %d\n%s%s", code, stdout, stderr)
	}
	code, stdout, _ = run("-q", "push", "--tag", "v0")
	if code != 0 || stdout != "" || originRef(t, "refs/tags/v0") == "" {
		t.Errorf("push -q --tag v0: exit %d, stdout %q", code, stdout)
	}
}

// Under --json the remote, the tags and the branches named are given, and
// no branch of HEAD's.
func TestPushNamedJSON(t *testing.T) {
	namedPushRepo(t)
	code, stdout, stderr := run("push", "--json", "--tag", "v1")
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	want := map[string]any{"schema": 1.0, "ok": true, "outcome": "pushed", "remote": "origin", "tags": []any{"v1"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("--json = %v, want %v", got, want)
	}
	code, stdout, _ = run("push", "--json", "--ref", "b")
	got = nil
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 ||
		!reflect.DeepEqual(got["refs"], []any{"b"}) || got["tags"] != nil {
		t.Errorf("--json --ref b: exit %d, %s", code, stdout)
	}
}

// A tag or a branch this clone does not have, and a tag origin has as
// another object, are refused before anything is pushed: none of the tags
// named lands.
func TestPushNamedRefuses(t *testing.T) {
	_, sh := namedPushRepo(t)
	code, _, stderr := run("push", "--tag", "v1", "--tag", "v9")
	if code != ExitPolicy || stderr != "itos push: this clone has no tags/v9; nothing was pushed\n" {
		t.Errorf("--tag v9: exit %d, %s", code, stderr)
	}
	code, _, stderr = run("push", "--ref", "nope")
	if code != ExitPolicy || !strings.Contains(stderr, "no heads/nope") {
		t.Errorf("--ref nope: exit %d, %s", code, stderr)
	}
	sh(`git push -q origin HEAD:refs/tags/v2 && git tag -a -m v2 v2 b`)
	theirs, ours := originRef(t, "refs/tags/v2"), gitLine(t, "rev-parse", "refs/tags/v2")
	code, stdout, stderr := run("push", "--tag", "v1", "--tag", "v2")
	if code != ExitPolicy || stdout != "" || !strings.Contains(stderr, "itos push: origin already has the tag v2, at "+
		theirs[:7]+", not this clone's "+ours[:7]+"; a tag is never moved, so nothing was pushed.\n") {
		t.Errorf("--tag v2: exit %d\n%s%s", code, stdout, stderr)
	}
	if originRef(t, "refs/tags/v1") != "" {
		t.Errorf("v1 landed beside a refused v2")
	}
}

// A push the pre-push hook refuses exits 1, and one that cannot reach the
// remote 75, whether reading its tags or pushing a branch.
func TestPushNamedFailures(t *testing.T) {
	dir, sh := namedPushRepo(t)
	hook := filepath.Join(dir, ".git", "hooks", "pre-push")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho no >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run("push", "--ref", "b")
	if code != ExitPolicy || !strings.Contains(stderr, "itos push: the push of branch b to origin failed") {
		t.Errorf("a hook's refusal: exit %d\n%s", code, stderr)
	}
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	sh(`git remote set-url origin https://127.0.0.1:1/itos.git`)
	for _, flag := range []string{"--tag", "--ref"} {
		name := map[string]string{"--tag": "v1", "--ref": "b"}[flag]
		code, _, stderr := run("push", flag, name)
		if code != ExitTemporary || !strings.Contains(stderr, "origin cannot be reached") {
			t.Errorf("push %s %s out of reach: exit %d\n%s", flag, name, code, stderr)
		}
	}
	// Reading the remote's tags is what failed, before any push.
	code, stdout, stderr := run("push", "--json", "--tag", "v1")
	if code != ExitTemporary || !strings.Contains(stdout, `"outcome": "fetch-failed"`) {
		t.Errorf("push --json --tag v1 out of reach: exit %d\n%s%s", code, stdout, stderr)
	}
}

// A plain push leaves the tags behind, whatever push.followTags says.
func TestPushFollowsNoTags(t *testing.T) {
	_, sh := namedPushRepo(t)
	sh(`git config push.followTags true && echo more > more.md && git add more.md && git commit -q -m 'docs: more' &&
		git tag -a -m v2 v2`)
	if code, stdout, stderr := run("push", "--no-wait"); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if originRef(t, "refs/tags/v2") != "" || originRef(t, "refs/tags/v1") != "" {
		t.Errorf("a plain push sent a tag")
	}
}

// A git that cannot start for the push exits 3, saying so: here the folder
// it would run in is gone.
func TestPushNamedGitThatCannotStart(t *testing.T) {
	namedPushRepo(t)
	var stderr strings.Builder
	code, err := runPush([]string{"--ref", "b"}, Out{Stdout: io.Discard, Stderr: &stderr, Origin: "no-such-folder"})
	if err != nil || code != ExitMissing || !strings.HasPrefix(stderr.String(), "itos: cannot run git: ") {
		t.Errorf("push --ref b with no git to run = %d, %v\n%s", code, err, stderr.String())
	}
}

// The routine's own edges: a branch with no commit is nothing to push; with
// no upstream and no origin there is nowhere to push, exit 3; with no
// upstream but an origin with no branch, HEAD goes to origin's branch of its
// name; and once it is there, and origin's HEAD names it, there is nothing to
// push.
func TestPushRoutineEdges(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	standInHooks(t)
	for _, kv := range [][2]string{{"GIT_AUTHOR_NAME", "itos"}, {"GIT_COMMITTER_NAME", "itos"},
		{"GIT_AUTHOR_EMAIL", "itos@example.com"}, {"GIT_COMMITTER_EMAIL", "itos@example.com"}} {
		t.Setenv(kv[0], kv[1])
	}
	t.Setenv(git.EnvGit, git.Bin(runtime.GOOS))
	sh := func(script string) {
		t.Helper()
		if out, err := exec.Command("sh", "-c", script).CombinedOutput(); err != nil {
			t.Fatalf("%s: %s", script, out)
		}
	}
	sh(`git checkout -q -b main`)
	if code, stdout, stderr := run("push"); code != 0 || stdout != "Nothing to push: main has no commit yet.\n" {
		t.Errorf("no commit: exit %d\n%s%s", code, stdout, stderr)
	}
	sh(`echo start > README.md && git add README.md && git commit -q -m 'chore: start'`)
	if code, _, stderr := run("push"); code != ExitMissing ||
		stderr != "itos push: main has no upstream and there is no remote origin to push to\n" {
		t.Errorf("no remote: exit %d\n%s", code, stderr)
	}
	sh(`git init -q --bare .git/origin.git && git -C .git/origin.git symbolic-ref HEAD refs/heads/main && git remote add origin .git/origin.git`)
	if code, stdout, stderr := run("push", "--no-wait"); code != 0 || !strings.HasPrefix(stdout, "Pushed ") ||
		originRef(t, "refs/heads/main") != gitLine(t, "rev-parse", "HEAD") {
		t.Errorf("no upstream, an origin: exit %d\n%s%s", code, stdout, stderr)
	}
	if code, stdout, stderr := run("push"); code != 0 || stdout != "Nothing to push: main is at origin/main.\nmain has no upstream, so itos push used origin/main; itos push --ref main pushes it to its own name.\n" {
		t.Errorf("at origin: exit %d\n%s%s", code, stdout, stderr)
	}
}

// An option push does not take is a usage error, exit 2, as is a forcing
// one, before anything else runs.
func TestPushRefusesOtherOptions(t *testing.T) {
	for _, arg := range []string{"--follow-tags", "-uf"} {
		code, err := runPush([]string{arg}, Out{Stdout: io.Discard, Stderr: io.Discard})
		if code != 0 || ExitCode(err) != ExitUsage {
			t.Errorf("push %s = %d, %v", arg, code, err)
		}
	}
}
