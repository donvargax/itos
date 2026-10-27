package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

// fixtureTest is the one test of the module the -hook-env tests run it over: what it does is the
// FIXTURE_ACTION the test sets, which go test hands it through its environment.
const fixtureTest = `package a

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) error {
	return exec.Command(os.Getenv("FIXTURE_GIT"), args...).Run()
}

func appendTo(t *testing.T, path, text string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func TestFixture(t *testing.T) {
	dir := os.Getenv("GIT_DIR")
	switch os.Getenv("FIXTURE_ACTION") {
	case "clean":
		for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
			if os.Getenv(name) == "" {
				t.Errorf("%s is not set", name)
			}
		}
		if !strings.Contains(os.Getenv("GIT_CONFIG_PARAMETERS"), "core.hooksPath") {
			t.Errorf("GIT_CONFIG_PARAMETERS is %q", os.Getenv("GIT_CONFIG_PARAMETERS"))
		}
		// A commit through the environment is refused by a hook, so it changes nothing.
		if err := run(t, "commit", "-q", "--allow-empty", "-m", "leak"); err == nil {
			t.Error("a commit through the hook environment was not refused by a hook")
		}
	case "index":
		appendTo(t, os.Getenv("GIT_INDEX_FILE"), "x")
	case "config":
		appendTo(t, filepath.Join(dir, "config"), "[leak]\n\tkey = value\n")
	case "head":
		if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte("ref: refs/heads/leak\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	case "ref":
		// Through git the reference-transaction hook refuses it: a loose ref written by hand.
		sha, err := exec.Command(os.Getenv("FIXTURE_GIT"), "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "refs", "heads", "leak"), sha, 0o644); err != nil {
			t.Fatal(err)
		}
	case "fails":
		t.Fatal("this test fails")
	}
}
`

// fixture is a module with one package, example.com/fixture/a, holding fixtureTest, in a folder of
// its own with the action the test is to take and the real git.
func fixture(t *testing.T, action string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range map[string]string{
		"go.mod":                        "module example.com/fixture\n\ngo 1.21\n",
		"a/a.go":                        "package a\n",
		filepath.Join("a", "a_test.go"): fixtureTest,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("FIXTURE_ACTION", action)
	t.Setenv("FIXTURE_GIT", git.Bin(runtime.GOOS))
	return root
}

// scratchFolders are the folders -hook-env made in the temporary folder tmp.
func scratchFolders(t *testing.T, tmp string) []string {
	t.Helper()
	left, err := filepath.Glob(filepath.Join(tmp, "go-unit-tests-hook-env-*"))
	if err != nil {
		t.Fatal(err)
	}
	return left
}

// hookEnvIn is -hook-env over the fixture's packages, with a temporary folder of its own, which
// it must leave empty of what it made.
func hookEnvIn(t *testing.T, action string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	root := fixture(t, action)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	var out, errw bytes.Buffer
	code = run(append([]string{"-hook-env"}, args...), root, &out, &errw)
	if left := scratchFolders(t, tmp); len(left) > 0 {
		t.Errorf("-hook-env left %q behind", left)
	}
	return code, out.String(), errw.String()
}

func TestHookEnvPassesAPackageThatLeavesTheRepositoryAlone(t *testing.T) {
	code, stdout, stderr := hookEnvIn(t, "clean", "./...")
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "1 packages ran") {
		t.Errorf("stdout does not say what ran:\n%s", stdout)
	}
}

func TestHookEnvFailsWhatChangesTheScratchRepository(t *testing.T) {
	for _, c := range []struct{ action, changed string }{
		{"index", "index file"},
		{"config", "config file"},
		{"head", "HEAD"},
		{"ref", "refs"},
	} {
		t.Run(c.action, func(t *testing.T) {
			code, _, stderr := hookEnvIn(t, c.action, "./...")
			if code != 1 {
				t.Fatalf("exit %d, want 1\n%s", code, stderr)
			}
			if !strings.Contains(stderr, "the scratch repository's "+c.changed+" changed") {
				t.Errorf("stderr does not say the %s changed:\n%s", c.changed, stderr)
			}
			if strings.Contains(stderr, "a test failed") {
				t.Errorf("stderr blames a test, which passed:\n%s", stderr)
			}
			if !strings.Contains(stderr, "example.com/fixture/a") {
				t.Errorf("stderr does not name the packages that ran:\n%s", stderr)
			}
		})
	}
}

func TestHookEnvFailsAFailingTest(t *testing.T) {
	code, _, stderr := hookEnvIn(t, "fails", "./...")
	if code != 1 || !strings.Contains(stderr, "a test failed") || !strings.Contains(stderr, "example.com/fixture/a") {
		t.Errorf("exit %d, stderr:\n%s\nwant 1, a failed test and the package", code, stderr)
	}
	if strings.Contains(stderr, "changed") {
		t.Errorf("stderr says the scratch repository changed:\n%s", stderr)
	}
}

func TestHookEnvNeedsPackages(t *testing.T) {
	code, _, stderr := hookEnvIn(t, "clean")
	if code != 2 || !strings.Contains(stderr, "usage:") {
		t.Errorf("exit %d, stderr %q, want 2 and the usage", code, stderr)
	}
}

// The environment the run is itself handed by a hook is replaced, not added to.
func TestHookEnvReplacesTheEnvironmentItIsHanded(t *testing.T) {
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "elsewhere"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "index"))
	t.Setenv("GIT_CONFIG_PARAMETERS", shQuote("core.hooksPath")+"="+shQuote(t.TempDir()))
	if code, stdout, stderr := hookEnvIn(t, "clean", "./..."); code != 0 {
		t.Fatalf("exit %d, want 0\n%s\n%s", code, stdout, stderr)
	}
}

func TestHookEnvConfigParameter(t *testing.T) {
	for _, c := range []struct{ name, value, want string }{
		{"core.hooksPath", "/tmp/hooks", `'core.hooksPath'='/tmp/hooks'`},
		{"core.hooksPath", `C:\Users\x\hooks`, `'core.hooksPath'='C:/Users/x/hooks'`},
		{"core.hooksPath", "/tmp/it's", `'core.hooksPath'='/tmp/it'\''s'`},
		{"core.hooksPath", "/tmp/a!b", `'core.hooksPath'='/tmp/a'\!'b'`},
	} {
		if got := configParameter(c.name, c.value); got != c.want {
			t.Errorf("configParameter(%q, %q) = %s, want %s", c.name, c.value, got, c.want)
		}
	}
}
