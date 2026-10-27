package tests

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/git"
)

// A Gherkin kind's test file is a .feature file under its root, or anywhere
// when it names none; a kind the config lacks has none.
func TestStagesTestFileGherkin(t *testing.T) {
	cfg := load(t, kinds)
	for _, c := range []struct {
		paths []string
		want  bool
	}{
		{nil, false},
		{[]string{"README.md", "features/smoke.yaml"}, false},
		{[]string{"features.feature", "other/a.feature", "featuresx/a.feature"}, false},
		{[]string{"README.md", "features/a.feature"}, true},
		{[]string{"features/deep/b.feature"}, true},
		{[]string{"./features/c.feature"}, true},
	} {
		if got := StagesTestFile(cfg, "scenario", c.paths, "HEAD", nil); got != c.want {
			t.Errorf("%q: %v, want %v", c.paths, got, c.want)
		}
	}
	if StagesTestFile(cfg, "missing", []string{"features/a.feature"}, "HEAD", nil) {
		t.Error("a kind the config lacks has a test file")
	}
	if !featureAmong(config.Kind{}, []string{"anywhere/a.feature"}) {
		t.Error("a kind with no root reads no .feature file")
	}
	dot := "."
	if !featureAmong(config.Kind{Root: &dot}, []string{"a.feature"}) {
		t.Error("a kind rooted at the top reads no .feature file")
	}
}

// A command kind's test file is one its adapter lists at the index, joined
// to its root, or, for a path the working tree no longer has, one it listed
// at the commit before; a listing that fails names none.
func TestStagesTestFileCommand(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Chdir(dir)
	run := func(args ...string) {
		t.Helper()
		out, err := exec.Command(git.Bin(runtime.GOOS), append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), out)
		}
	}
	// The adapter lists a.test.ts at the index, and gone.test.ts too at any
	// commit.
	adapter := `if [ "$3" = index ]; then f='"a.test.ts"'; else f='"a.test.ts","gone.test.ts"'; fi
printf '{"protocol":1,"tests":[],"files":[%s]}\n' "$f"
`
	// This one lists only the index, and fails at a commit.
	indexOnly := "[ \"$3\" = index ] && exec sh adapter.sh \"$@\"\nexit 1\n"
	for path, text := range map[string]string{"adapter.sh": adapter, "index-only.sh": indexOnly, "src/a.test.ts": "a\n", "src/gone.test.ts": "gone\n", "src/util.ts": "u\n"} {
		if err := os.MkdirAll(dir+"/src", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/"+path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	command := func(c string) *config.Loaded {
		return load(t, "version: 1\ntests:\n  unit: { adapter: { command: \""+c+"\", supports_at: true }, root: src }\n")
	}
	cfg := command("sh adapter.sh")
	run("init", "-q", "-b", "main")
	// Before the first commit HEAD names no commit, so it lists nothing.
	if err := os.Rename("src/gone.test.ts", "gone.keep"); err != nil {
		t.Fatal(err)
	}
	if StagesTestFile(cfg, "unit", []string{"src/gone.test.ts"}, "HEAD", nil) {
		t.Error("an unborn HEAD listed a deleted file")
	}
	if err := os.Rename("gone.keep", "src/gone.test.ts"); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "test: start")
	run("rm", "-q", "src/gone.test.ts")
	for _, c := range []struct {
		paths []string
		want  bool
	}{
		{[]string{"src/util.ts"}, false},
		{[]string{"src/util.ts", "src/a.test.ts"}, true},
		{[]string{"src/gone.test.ts"}, true},
		{[]string{"src/never.ts"}, false},
		{[]string{"a.test.ts"}, false},
	} {
		if got := StagesTestFile(cfg, "unit", c.paths, "HEAD", nil); got != c.want {
			t.Errorf("%q: %v, want %v", c.paths, got, c.want)
		}
	}
	if StagesTestFile(command("false"), "unit", []string{"src/a.test.ts"}, "HEAD", nil) {
		t.Error("an adapter that fails listed a test file")
	}
	if StagesTestFile(command("sh index-only.sh"), "unit", []string{"src/gone.test.ts"}, "HEAD", nil) {
		t.Error("an adapter that fails at the commit before listed a deleted file")
	}
	if !listedAmong(config.Kind{}, List{Files: []string{"src/a.test.ts"}}, []string{"src/a.test.ts"}) {
		t.Error("a kind with no root reads its listing's files from the top")
	}
}
