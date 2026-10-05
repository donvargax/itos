package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A fixture of the last release's corpus: a help case, two usage errors (one
// with files_after, which leaves with it), and the refusals that are not
// usage errors, which stay judged word for word: config check's FAIL line, an
// "itos: … is missing" about a file the config names, and a usage-shaped
// refusal that pins its output with stderr_has.
const oldCorpus = `cases:
  - name: itos --help
    argv: [--help]
    exit: 0
    stdout: "usage: itos\n"
  - name: init refuses --agent-rules under --stealth
    argv: [init, --stealth, --agent-rules]
    exit: 2
    stderr: "itos: init --stealth --agent-rules is not written yet (itos --help)\n"
  - name: push takes no arguments
    argv: [push, now]
    exit: 2
    stdout: ""
    stderr: "itos: push takes no arguments (itos --help)\n"
    files_after:
      itos.yaml: null
  - name: a config that is not version 1
    argv: [config, check]
    exit: 2
    stderr: "FAIL itos.yaml: version 2 is not 1\n"
  - name: a smoke file the config names is missing
    argv: [smoke]
    exit: 2
    stderr: "itos: features/smoke.yaml is missing\n"
  - name: a refusal pinned by stderr_has
    argv: [wave]
    exit: 2
    stderr: "itos: unknown command: wave (itos --help)\n"
    stderr_has: unknown command
  - name: itos version
    argv: [version]
    exit: 0
    stdout: "itos {{version}}\n"
`

// readyOldCorpus leaves a usage error's case out of the run, as it leaves out
// a help case, and keeps every other refusal, judged word for word (T-095).
func TestReadyOldCorpusLeavesUsageErrorsOut(t *testing.T) {
	top, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not on the PATH, and oldCorpusScript runs in it: skipping")
	}
	modules := filepath.Join(top, "node_modules")
	if _, err := os.Stat(filepath.Join(modules, "yaml")); err != nil {
		t.Skipf("%s has no yaml package, which oldCorpusScript reads the fixtures with (vp install): skipping", modules)
	}

	tree := t.TempDir()
	dir := filepath.Join(tree, "tools/itos/conformance")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cli.yaml"), []byte(oldCorpus), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := prepare(tree, top); err != nil {
		t.Fatal(err)
	}

	help, usage, cases, err := readyOldCorpus(tree)
	if err != nil {
		t.Fatal(err)
	}
	if help != "1 help case (cli.yaml 1)" {
		t.Errorf("help cases: got %q", help)
	}
	if usage != "2 usage errors (cli.yaml 2)" {
		t.Errorf("usage errors: got %q", usage)
	}
	if got := len(cases["cli.yaml"]); got != 7 {
		t.Errorf("the case names, as written, help cases and usage errors included: got %d, want 7: %q", got, cases["cli.yaml"])
	}

	written, err := os.ReadFile(filepath.Join(dir, "cli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	var wrong []string
	for _, gone := range []string{
		"name: itos --help",
		"init refuses --agent-rules under --stealth",
		"push takes no arguments",
		"files_after",
	} {
		if strings.Contains(text, gone) {
			wrong = append(wrong, fmt.Sprintf("it should leave out %q", gone))
		}
	}
	for _, kept := range []string{
		"name: a config that is not version 1",
		`stderr: "FAIL itos.yaml: version 2 is not 1\n"`,
		"name: a smoke file the config names is missing",
		`stderr: "itos: features/smoke.yaml is missing\n"`,
		"name: a refusal pinned by stderr_has",
		`stderr: "itos: unknown command: wave (itos --help)\n"`,
		"name: itos version",
	} {
		if !strings.Contains(text, kept) {
			wrong = append(wrong, fmt.Sprintf("it should keep %q, judged word for word", kept))
		}
	}
	if len(wrong) > 0 {
		t.Errorf("the run's fixture:\n  %s\nit reads:\n%s", strings.Join(wrong, "\n  "), text)
	}
}
