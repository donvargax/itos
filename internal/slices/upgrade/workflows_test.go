package upgrade

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func workflow(text string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(text)} }

// uses lists, as the report tells them.
func told(uses []actionUse) []string {
	var lines []string
	for _, u := range uses {
		lines = append(lines, u.line())
	}
	return lines
}

func wantLines(t *testing.T, what string, got []actionUse, want ...string) {
	t.Helper()
	if g := strings.Join(told(got), "\n"); g != strings.Join(want, "\n") {
		t.Errorf("%s:\n%s\nwant:\n%s", what, g, strings.Join(want, "\n"))
	}
}

// Only a step's uses of donvargax/itos@<ref> is named, by the line of its
// uses: key, its ref and with.version as written, an alias read as what it
// stands for, sorted by file then line.
func TestScanWorkflowsNamesSteps(t *testing.T) {
	uses, problems := scanWorkflows(fstest.MapFS{
		".github/workflows/b.yaml": workflow("jobs:\n  one:\n    steps:\n      - uses: \"donvargax/itos@v9.2.0\"\n"),
		".github/workflows/a.yml": workflow(`name: a
jobs:
  one:
    steps:
      - uses: donvargax/itos@2222222222222222222222222222222222222222 # v9.1.0
        with:
          version: ${{ vars.ITOS_VERSION }}
      - name: numeric
        with: { version: 9.1 }
        uses: donvargax/itos@main
      - uses: donvargax/itos@v1
        with:
          version: ""
      - uses: donvargax/itos@v2
        with:
          version: [9.1.0]
      - &shared
        uses: donvargax/itos@v3
  two:
    steps:
      - *shared
`),
		".github/workflows/c.yml": workflow(`jobs:
  reuse:
    uses: donvargax/itos/.github/workflows/release.yml@v9.1.0
  steps-not-a-list:
    steps: donvargax/itos@v9.1.0
  steps-of-scalars:
    steps: [donvargax/itos@v9.1.0]
  not-a-job: donvargax/itos@v9.1.0
  runs:
    steps:
      - uses: donvargax/itos@
      - uses: donvargax/itos
      - uses: donvargax/itos-cc@v9.1.0
      - uses: Donvargax/itos@v9.1.0
      - uses: someone/wrapper@v1
        with:
          uses: donvargax/itos@v9.1.0
      - uses: [donvargax/itos@v9.1.0]
      - run: |
          echo "uses: donvargax/itos@v9.1.0"
`),
		".github/workflows/empty.yml":      workflow(""),
		".github/workflows/list.yml":       workflow("- uses: donvargax/itos@v9.1.0\n"),
		".github/workflows/sub/d.yml":      workflow("jobs:\n  one:\n    steps:\n      - uses: donvargax/itos@v9.1.0\n"),
		".github/workflows/README.md":      workflow("- uses: donvargax/itos@v9.1.0"),
		".github/workflows/e.yml.disabled": workflow("jobs:\n  one:\n    steps:\n      - uses: donvargax/itos@v9.1.0\n"),
		".github/actions/f/action.yml":     workflow("runs:\n  steps:\n    - uses: donvargax/itos@v9.1.0\n"),
	})
	if len(problems) != 0 {
		t.Errorf("problems: %v", problems)
	}
	wantLines(t, "uses", uses,
		".github/workflows/a.yml:5: donvargax/itos@2222222222222222222222222222222222222222 (version: ${{ vars.ITOS_VERSION }})",
		".github/workflows/a.yml:10: donvargax/itos@main (version: 9.1)",
		".github/workflows/a.yml:11: donvargax/itos@v1 (version: \"\")",
		".github/workflows/a.yml:14: donvargax/itos@v2",
		".github/workflows/a.yml:18: donvargax/itos@v3",
		".github/workflows/a.yml:18: donvargax/itos@v3",
		".github/workflows/b.yaml:4: donvargax/itos@v9.2.0")
}

// A workflow written with CRLF line breaks is numbered by its lines, and a
// byte order mark does not hide its first line.
func TestScanWorkflowsLineBreaks(t *testing.T) {
	const steps = "jobs:\r\n  one:\r\n    steps:\r\n      - uses: actions/checkout@v4\r\n      - uses: donvargax/itos@v9.1.0\r\n"
	uses, problems := scanWorkflows(fstest.MapFS{
		".github/workflows/crlf.yml": workflow(steps),
		".github/workflows/lf.yml":   workflow(strings.ReplaceAll(steps, "\r\n", "\n")),
		".github/workflows/bom.yml":  workflow("\ufeff" + steps),
	})
	if len(problems) != 0 {
		t.Errorf("problems: %v", problems)
	}
	wantLines(t, "uses", uses,
		".github/workflows/bom.yml:5: donvargax/itos@v9.1.0",
		".github/workflows/crlf.yml:5: donvargax/itos@v9.1.0",
		".github/workflows/lf.yml:5: donvargax/itos@v9.1.0")
}

// A file that is not YAML is a problem saying why, and the other files are
// still scanned.
func TestScanWorkflowsNotYAML(t *testing.T) {
	uses, problems := scanWorkflows(fstest.MapFS{
		".github/workflows/a.yml":   workflow("jobs:\n  one:\n    steps:\n      - uses: donvargax/itos@v9.1.0\n"),
		".github/workflows/bad.yml": workflow("name: bad\njobs:\n  check:\n    steps: [\n"),
	})
	wantLines(t, "uses", uses, ".github/workflows/a.yml:4: donvargax/itos@v9.1.0")
	if len(problems) != 1 || problems[0].File != ".github/workflows/bad.yml" {
		t.Fatalf("problems: %v", problems)
	}
	if m := problems[0].Message; !strings.HasPrefix(m, "cannot be read as YAML: ") || !strings.Contains(m, "line 4") {
		t.Errorf("a message that says why: %q", m)
	}
}

// failingFS cannot read the files it lists, or list a folder at all.
type failingFS struct {
	fs.FS
	unlistable bool
}

func (f failingFS) Open(name string) (fs.File, error) {
	switch {
	case f.unlistable && name == workflowsDir:
		return nil, &fs.PathError{Op: "open", Path: name, Err: errors.New("denied")}
	case strings.HasSuffix(name, "locked.yml"):
		return nil, &fs.PathError{Op: "open", Path: name, Err: errors.New("locked away")}
	}
	return f.FS.Open(name)
}

// A file it cannot read is a problem saying why and not where: the file is
// named by its own field.
func TestScanWorkflowsUnreadable(t *testing.T) {
	files := fstest.MapFS{
		".github/workflows/a.yml":      workflow("jobs:\n  one:\n    steps:\n      - uses: donvargax/itos@v9.1.0\n"),
		".github/workflows/locked.yml": workflow("jobs: {}\n"),
	}
	uses, problems := scanWorkflows(failingFS{FS: files})
	wantLines(t, "uses", uses, ".github/workflows/a.yml:4: donvargax/itos@v9.1.0")
	if len(problems) != 1 || problems[0] != (scanProblem{".github/workflows/locked.yml", "cannot be read: locked away"}) {
		t.Errorf("problems: %v", problems)
	}

	uses, problems = scanWorkflows(failingFS{FS: files, unlistable: true})
	if len(uses) != 0 || len(problems) != 1 || problems[0] != (scanProblem{workflowsDir, "cannot be listed: denied"}) {
		t.Errorf("a folder it cannot list: %v, %v", uses, problems)
	}

	if got := why(errors.New("plain")); got != "plain" {
		t.Errorf("why = %q", got)
	}
}

// No folder is no workflow, and no problem.
func TestScanWorkflowsNone(t *testing.T) {
	for name, fsys := range map[string]fs.FS{
		"no .github":    fstest.MapFS{"itos.yaml": workflow("version: 1\n")},
		"no workflows":  fstest.MapFS{".github/CODEOWNERS": workflow("* @me\n")},
		"a real folder": os.DirFS(t.TempDir()),
	} {
		uses, problems := scanWorkflows(fsys)
		if uses == nil || problems == nil || len(uses)+len(problems) != 0 {
			t.Errorf("%s: %v, %v", name, uses, problems)
		}
	}
}

// writeWorkflow writes a workflow of the project the test runs in, with
// the line breaks given.
func writeWorkflow(t *testing.T, name, text string) {
	t.Helper()
	path := filepath.Join(workflowsDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

const launches = "jobs:\n  one:\n    steps:\n      - uses: actions/checkout@v4\n      - uses: donvargax/itos@v9.1.0\n"

// The report names the lines before By hand, then says what chooses the
// itos that runs: the pin's, or with no pin the release each line installs.
func TestRunNamesWorkflowLines(t *testing.T) {
	p := project(t, pinOf("9.1.0"), nil)
	writeWorkflow(t, "ci.yml", strings.ReplaceAll(launches, "\n", "\r\n"))
	writeWorkflow(t, "bad.yaml", "jobs: [\n")
	code, printed, err := upgradeRun(p, false)
	if code != 0 || err != nil {
		t.Fatalf("exit %d, %v, %s", code, err, printed)
	}
	section := "\nWorkflow steps that launch the itos action, none of which upgrade moves:\n" +
		"  .github/workflows/ci.yml:5: donvargax/itos@v9.1.0\n" +
		"  .github/workflows/bad.yaml cannot be read as YAML: "
	i, j := strings.Index(printed, section), strings.Index(printed, "\nBy hand: nothing.")
	if i < 0 || j < i || !strings.Contains(printed[i:j], ", so its steps are not named\n\nThe action installs a launcher, "+
		"the release its ref or its version input names, and the repository's pin chooses the itos that runs: "+
		"itos 9.3.0, whatever these lines say.\n") {
		t.Errorf("printed:\n%s", printed)
	}
	if text, _ := os.ReadFile(filepath.Join(workflowsDir, "ci.yml")); string(text) != strings.ReplaceAll(launches, "\n", "\r\n") {
		t.Errorf("the workflow was edited: %q", text)
	}

	p = project(t, "version: 1\n", nil)
	writeScript(t, script("9.1.0", oldSum, oldSum))
	writeWorkflow(t, "ci.yml", launches)
	_, printed, _ = upgradeRun(p, false)
	if !strings.Contains(printed, "  .github/workflows/ci.yml:5: donvargax/itos@v9.1.0\n\nThe config pins no itos, so each of "+
		"these lines runs the release it installs; itos pin pins one.\n\nBy hand") ||
		strings.Contains(printed, "chooses the itos that runs") {
		t.Errorf("no pin:\n%s", printed)
	}
}

// A project already on the version has its lines named too; one whose
// workflows name none, or fail to read none, prints nothing of them.
func TestRunAlreadyNamesWorkflowLines(t *testing.T) {
	p := project(t, pinOf("9.3.0"), nil)
	writeWorkflow(t, "ci.yml", launches)
	_, printed, _ := upgradeRun(p, false)
	if want := "This project is already on itos 9.3.0: nothing to upgrade.\n\n" +
		"Workflow steps that launch the itos action, none of which upgrade moves:\n" +
		"  .github/workflows/ci.yml:5: donvargax/itos@v9.1.0\n\n" +
		"The action installs a launcher, the release its ref or its version input names, and the repository's pin " +
		"chooses the itos that runs: itos 9.3.0, whatever these lines say.\n"; printed != want {
		t.Errorf("already:\n%s\nwant:\n%s", printed, want)
	}

	p = project(t, pinOf("9.3.0"), nil)
	writeWorkflow(t, "ci.yml", "jobs:\n  one:\n    steps:\n      - run: echo\n")
	if _, printed, _ := upgradeRun(p, false); printed != "This project is already on itos 9.3.0: nothing to upgrade.\n" {
		t.Errorf("no lines:\n%s", printed)
	}

	// Only a workflow that cannot be read: the problem, and no sentence.
	p = project(t, pinOf("9.3.0"), nil)
	writeWorkflow(t, "bad.yml", "jobs: [\n")
	_, printed, _ = upgradeRun(p, false)
	if !strings.Contains(printed, "  .github/workflows/bad.yml cannot be read as YAML: ") ||
		strings.Contains(printed, "The action installs") || strings.Contains(printed, "pins no itos") {
		t.Errorf("a problem alone:\n%s", printed)
	}
}

// --json carries the lines and the problems; a stealth config and a refusal
// name none, and no workflow is read for them.
func TestRunJSONWorkflowLines(t *testing.T) {
	p := project(t, pinOf("9.1.0"), nil)
	writeWorkflow(t, "ci.yml", launches+"        with:\n          version: 9.1.0\n")
	writeWorkflow(t, "bad.yml", "jobs: [\n")
	_, printed, _ := upgradeRun(p, true)
	got := compact(t, printed)
	if !strings.Contains(got, `"action_uses":[{"file":".github/workflows/ci.yml","line":5,"ref":"v9.1.0","version_input":"9.1.0"}],`+
		`"action_scan_problems":[{"file":".github/workflows/bad.yml","message":"cannot be read as YAML: yaml: `) {
		t.Errorf("--json:\n%s", got)
	}

	p = project(t, pinOf("9.1.0"), nil)
	writeWorkflow(t, "ci.yml", launches)
	p.stealth = true
	_, printed, _ = upgradeRun(p, true)
	if got := compact(t, printed); !strings.HasSuffix(got, `"action_uses":[],"action_scan_problems":[]}`) {
		t.Errorf("stealth:\n%s", got)
	}
	p.stealth = true
	p.file = filepath.Join(filepath.Dir(p.file), "itos.yaml")
	p.latest = "9.1.0"
	_, printed, _ = upgradeRun(p, true)
	if got := compact(t, printed); !strings.HasSuffix(got, `"action_uses":[],"action_scan_problems":[]}`) {
		t.Errorf("stealth, already:\n%s", got)
	}

	p = project(t, "version: 1\n", nil)
	writeWorkflow(t, "ci.yml", launches)
	code, printed, _ := upgradeRun(p, true)
	if got := compact(t, printed); code != policy || !strings.HasSuffix(got, `"action_uses":[],"action_scan_problems":[]}`) {
		t.Errorf("a refusal: %d %s", code, got)
	}
}
