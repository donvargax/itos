package features

import (
	"github.com/donvargax/itos/v8/internal/git"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gitForTest runs the real git in dir, with no config of the machine's.
func gitForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(git.Bin(runtime.GOOS), args...)
	cmd.Dir = dir
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=a", "GIT_AUTHOR_EMAIL=a@localhost", "GIT_COMMITTER_NAME=a", "GIT_COMMITTER_EMAIL=a@localhost")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// What appendGitConfig writes is what git config --local writes, byte for byte, and git reads
// it back as the values given.
func TestAppendGitConfigWritesWhatGitConfigWrites(t *testing.T) {
	values := []string{
		"/tmp/fixture-hooks/itos hook commit-msg",
		"'/tmp/with space/itos' hook pre-push",
		"/tmp/it's/itos hook commit-msg",
		` leading`, `trailing `, "semi;colon", "hash#mark", `back\slash`, `a "quoted" word`, "tab\there", "line\nbreak",
	}
	byGit, byUs := t.TempDir(), t.TempDir()
	for _, dir := range []string{byGit, byUs} {
		gitForTest(t, dir, "init", "-q", "-b", "main")
	}
	var entries [][2]string
	for i, value := range values {
		key := "k" + string(rune('a'+i))
		gitForTest(t, byGit, "config", "--local", "hook.itos-x."+key, value)
		entries = append(entries, [2]string{key, value})
	}
	if done, err := appendGitConfig(byUs, configSection{`hook "itos-x"`, entries}); !done || err != nil {
		t.Fatalf("appendGitConfig: %v, %v", done, err)
	}
	want, err := os.ReadFile(filepath.Join(byGit, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(byUs, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("the config text differs\n--- git config\n%s--- appendGitConfig\n%s", want, got)
	}
	for i, value := range values {
		if read := gitForTest(t, byUs, "config", "--local", "--get", "hook.itos-x.k"+string(rune('a'+i))); read != strings.TrimSpace(value) {
			t.Errorf("git reads %q back as %q", value, read)
		}
	}
}

func TestAppendGitConfigLeavesAGitFolderThatIsAFileToGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if done, err := appendGitConfig(dir, configSection{"a", [][2]string{{"b", "c"}}}); done || err != nil {
		t.Fatalf("appendGitConfig on a gitfile: %v, %v", done, err)
	}
}

// headSHA is what git rev-parse HEAD says, for a loose ref, a packed one and a detached HEAD, and
// says nothing for what is not plain to read.
func TestHeadSHAIsRevParseHEAD(t *testing.T) {
	dir := t.TempDir()
	gitForTest(t, dir, "init", "-q", "-b", "main")
	if sha, ok := headSHA(dir); ok {
		t.Fatalf("an unborn branch has HEAD %q", sha)
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitForTest(t, dir, "add", "-A")
	gitForTest(t, dir, "commit", "-q", "-m", "one")
	for _, step := range []struct {
		name string
		do   func()
	}{
		{"a loose ref", func() {}},
		{"a packed ref", func() { gitForTest(t, dir, "pack-refs", "--all", "--prune") }},
		{"a detached HEAD", func() { gitForTest(t, dir, "checkout", "-q", "--detach") }},
		{"a second commit", func() { gitForTest(t, dir, "commit", "-q", "--allow-empty", "-m", "two") }},
	} {
		step.do()
		want := gitForTest(t, dir, "rev-parse", "HEAD")
		if got, ok := headSHA(dir); !ok || got != want {
			t.Errorf("%s: headSHA = %q, %v; rev-parse HEAD = %q", step.name, got, ok, want)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/nowhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if sha, ok := headSHA(dir); ok {
		t.Errorf("a HEAD naming no ref has %q", sha)
	}
}

// A copy of the template's git folder is a repository git reads as a fresh one.
func TestInitRepositoryMakesWhatGitInitMakes(t *testing.T) {
	t.Cleanup(removeCallerPath)
	t.Cleanup(removeRepoTemplate)
	w := &world{}
	dir := t.TempDir()
	if err := w.initRepository(dir); err != nil {
		t.Fatal(err)
	}
	if got := gitForTest(t, dir, "symbolic-ref", "HEAD"); got != "refs/heads/main" {
		t.Errorf("HEAD names %q", got)
	}
	if got := gitForTest(t, dir, "rev-parse", "--is-inside-work-tree", "--git-dir"); got != "true\n.git" {
		t.Errorf("git reads the copy as %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitForTest(t, dir, "add", "-A")
	gitForTest(t, dir, "commit", "-q", "-m", "one")
	if sha, ok := headSHA(dir); !ok || sha != gitForTest(t, dir, "rev-parse", "HEAD") {
		t.Errorf("headSHA after a commit in the copy: %q, %v", sha, ok)
	}
}
