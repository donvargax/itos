package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// budget runs doc-budget in a scratch folder holding files, each of the given
// size, and a caps file with the given text, returning its exit status and
// what it printed.
func budget(t *testing.T, files map[string]int, capsText string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	for name, size := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, bytes.Repeat([]byte("x"), size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if capsText != "" {
		if err := os.WriteFile(filepath.Join(dir, "caps.json"), []byte(capsText), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-caps", "caps.json"}, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

const twoCaps = `{"caps": [
	{"file": "AGENTS.md", "bytes": 100},
	{"file": "docs/ARCHITECTURE.md", "bytes": 200}
]}`

func TestWithinCapsPasses(t *testing.T) {
	code, stdout, stderr := budget(t, map[string]int{"AGENTS.md": 100, "docs/ARCHITECTURE.md": 150}, twoCaps)
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr:\n%s", code, stderr)
	}
	for _, want := range []string{"AGENTS.md is 100 bytes, within its cap of 100", "docs/ARCHITECTURE.md is 150 bytes, within its cap of 200"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestOverCapFailsNamingEachFile(t *testing.T) {
	caps := `{"caps": [
		{"file": "AGENTS.md", "bytes": 100},
		{"file": "docs/ARCHITECTURE.md", "bytes": 200},
		{"file": "docs/ORCHESTRATING.md", "bytes": 50}
	]}`
	code, _, stderr := budget(t, map[string]int{"AGENTS.md": 101, "docs/ARCHITECTURE.md": 200, "docs/ORCHESTRATING.md": 80}, caps)
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr:\n%s", code, stderr)
	}
	for _, want := range []string{
		"AGENTS.md is 101 bytes, over its cap of 100 by 1",
		"docs/ORCHESTRATING.md is 80 bytes, over its cap of 50 by 30",
		"Take text out first",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "ARCHITECTURE") {
		t.Errorf("stderr names a file within its cap:\n%s", stderr)
	}
}

func TestGoneFileFails(t *testing.T) {
	code, _, stderr := budget(t, map[string]int{"AGENTS.md": 10}, twoCaps)
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr:\n%s", code, stderr)
	}
	if want := "docs/ARCHITECTURE.md is gone"; !strings.Contains(stderr, want) {
		t.Errorf("stderr lacks %q:\n%s", want, stderr)
	}
}

func TestUnreadableCapsExits2(t *testing.T) {
	files := map[string]int{"AGENTS.md": 10}
	for name, text := range map[string]string{
		"missing":       "",
		"not JSON":      `{"caps": [`,
		"unknown field": `{"caps": [{"file": "AGENTS.md", "bytes": 100, "lines": 3}]}`,
		"no caps":       `{"caps": []}`,
		"zero cap":      `{"caps": [{"file": "AGENTS.md", "bytes": 0}]}`,
		"twice":         `{"caps": [{"file": "AGENTS.md", "bytes": 100}, {"file": "AGENTS.md", "bytes": 200}]}`,
		"outside":       `{"caps": [{"file": "../AGENTS.md", "bytes": 100}]}`,
		"unclean":       `{"caps": [{"file": "./AGENTS.md", "bytes": 100}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := budget(t, files, text)
			if code != 2 {
				t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr)
			}
			if !strings.Contains(stderr, "cannot read the caps in caps.json") {
				t.Errorf("stderr does not say the caps file could not be read:\n%s", stderr)
			}
		})
	}
}

// TestIndexJudgesTheStagedFiles stages the caps and the files in a scratch
// repository, then changes the working tree: -index judges what is staged.
func TestIndexJudgesTheStagedFiles(t *testing.T) {
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	code, _, stderr := budget(t, map[string]int{"AGENTS.md": 100, "docs/ARCHITECTURE.md": 150}, twoCaps)
	if code != 0 {
		t.Fatalf("exit %d on the working tree, want 0; stderr:\n%s", code, stderr)
	}
	gitRun := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	gitRun("init", "-q")
	gitRun("add", "caps.json", "AGENTS.md")
	// Within its cap staged, over it in the working tree; over its cap and never staged.
	if err := os.WriteFile("AGENTS.md", bytes.Repeat([]byte("x"), 500), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("caps.json", []byte(`{"caps": [{"file": "AGENTS.md", "bytes": 1}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, errs bytes.Buffer
	code = run([]string{"-caps", "caps.json", "-index"}, &stdout, &errs)
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr:\n%s", code, errs.String())
	}
	if want := "doc-budget: AGENTS.md is 100 bytes, within its cap of 100"; !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
	}
	if want := "docs/ARCHITECTURE.md is gone"; !strings.Contains(errs.String(), want) {
		t.Errorf("stderr lacks %q:\n%s", want, errs.String())
	}
}
