// The steps. itos is a black box here: each scenario builds a scratch git
// repository in a temporary folder, runs the binary ITOS_BIN names
// (tools/bin/itos by default, relative to the repository's root) in it, and
// reads its exit code and output. Nothing here imports or reads itos's code,
// so the same steps judge any implementation of it.
package features

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// One scenario's state.
type world struct {
	root    string // the itos checkout: where go.mod is
	bin     string // the itos binary under test
	dir     string // the scratch repository
	support string // files the scenario needs outside the repository
	config  scratchConfig
	commits []string // the scratch repository's commits, oldest first

	exit           int
	stdout, stderr string
}

// What a scenario sets in the scratch repository's itos.yaml.
type scratchConfig struct {
	headerLint        bool     // the header lint delegated to commitlint's conventional config
	headerLintCommand string   // the header lint delegated to this command
	since             string   // commits.since
	rangeCheck        bool     // a range check that records where its range starts
	recordingShell    bool     // shell is the recording shell
	ciSteps           []string // ci.steps
	stopAtFirst       *bool    // ci.stop_at_first_failure
	registry          string   // work.registry
	taskChecks        *bool    // hooks.commit_msg.task_checks
	checkTimeout      int      // hooks.commit_msg.check_timeout, when above 0
}

func initializeScenario(sc *godog.ScenarioContext) {
	w := &world{}
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		return ctx, w.setUp()
	})
	sc.After(func(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
		os.RemoveAll(w.dir)
		os.RemoveAll(w.support)
		return ctx, err
	})

	sc.Step(`^a repository made from a template, its first commit "([^"]*)"$`, w.templateRepository)
	sc.Step(`^the commit "([^"]*)" on top of it$`, w.commitOnTop)
	sc.Step(`^a repository whose ledger has the task "([^"]*)"$`, w.repositoryWithTask)
	sc.Step(`^a change to "([^"]*)" is staged$`, w.stageChange)
	sc.Step(`^the header lint is commitlint's conventional config$`, w.conventionalHeaderLint)
	sc.Step(`^commits\.since names the first commit$`, w.sinceFirstCommit)
	sc.Step(`^commits\.since is "([^"]*)"$`, w.sinceIs)
	sc.Step(`^a range check that records where its range starts$`, w.recordingRangeCheck)
	sc.Step(`^the config's shell is the recording shell$`, w.recordingShell)
	sc.Step(`^the task "([^"]*)" has the check "([^"]*)"$`, w.taskHasCheck)
	sc.Step(`^the CI steps are "([^"]*)"$`, func(step string) error { return w.ciStepsAre(step) })
	sc.Step(`^the CI steps are "([^"]*)" then a step that records it ran$`, func(step string) error {
		return w.ciStepsAre(step, "printf 'ran\\n' > "+recordingStepFile)
	})
	sc.Step(`^the header lint is the command "([^"]*)"$`, w.headerLintIs)
	sc.Step(`^ci\.stop_at_first_failure is (true|false)$`, w.stopAtFirstFailureIs)
	sc.Step(`^work\.registry is "([^"]*)"$`, w.registryIs)
	sc.Step(`^the work registry at "([^"]*)" has the item "([^"]*)" with the status "([^"]*)"$`, w.registryAt)
	sc.Step(`^the work registry at "([^"]*)" with the item "([^"]*)" with the status "([^"]*)" is staged$`, w.stagedRegistryAt)
	sc.Step(`^the working tree's "([^"]*)" sets the item "([^"]*)" to the status "([^"]*)"$`, w.workingRegistry)
	sc.Step(`^the commit of a registry at "([^"]*)" with the item "([^"]*)" with the status "([^"]*)"$`, w.committedRegistryAt)
	sc.Step(`^an itos\.yaml with the unknown key "([^"]*)" is staged$`, w.stagedConfigKey)
	sc.Step(`^a ledger whose task "([^"]*)" has the unknown key "([^"]*)" is staged$`, w.stagedLedgerKey)
	sc.Step(`^the task "([^"]*)" has the static check "([^"]*)"$`, func(task, check string) error {
		return w.stagedChecks(task, staticCheck(check))
	})
	sc.Step(`^the task "([^"]*)" has a static check that records it ran$`, func(task string) error {
		return w.stagedChecks(task, staticCheck(recordingCheck))
	})
	sc.Step(`^the task "([^"]*)" has the late check "([^"]*)" then a static check that records it ran$`, func(task, check string) error {
		return w.stagedChecks(task, fmt.Sprintf("{ run: %q, cost: late }", check), staticCheck(recordingCheck))
	})
	sc.Step(`^hooks\.commit_msg\.task_checks is (true|false)$`, w.taskChecksAre)
	sc.Step(`^hooks\.commit_msg\.check_timeout is (\d+)$`, w.checkTimeoutIs)

	sc.Step(`^itos verifies every commit up to HEAD$`, func() error { return w.itos("verify", "", "HEAD") })
	sc.Step(`^itos checks the config$`, func() error { return w.itos("config", "check") })
	sc.Step(`^itos checks the work registry$`, func() error { return w.itos("work", "check") })
	sc.Step(`^itos runs the task "([^"]*)"$`, func(task string) error { return w.itos("task", task) })
	sc.Step(`^itos runs CI over every commit up to HEAD$`, func() error { return w.itos("ci", "run", "", "HEAD") })
	sc.Step(`^the commit-msg hook checks the message "([^"]*)"$`, w.commitMsgHook)
	sc.Step(`^the commit-msg hook checks the message:$`, func(message *godog.DocString) error {
		return w.commitMsgHook(message.Content + "\n")
	})

	sc.Step(`^itos exits with code (\d+)$`, w.exitsWith)
	sc.Step(`^its output says "([^"]*)"$`, w.outputSays)
	sc.Step(`^its output names the rule "([^"]*)"$`, w.outputNamesRule)
	sc.Step(`^the range check started at the first commit$`, w.rangeCheckStartedAtFirst)
	sc.Step(`^the recording shell ran "([^"]*)"$`, w.recordingShellRan)
	sc.Step(`^the recording shell ran the range check$`, w.recordingShellRanRangeCheck)
	sc.Step(`^the recording step ran$`, func() error { return w.recordingStepRan(true) })
	sc.Step(`^the recording step did not run$`, func() error { return w.recordingStepRan(false) })
	sc.Step(`^the recording check did not run$`, func() error { return w.recordingCheckRan(false) })
}

func (w *world) setUp() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	w.root = root
	w.bin = os.Getenv("ITOS_BIN")
	if w.bin == "" {
		w.bin = "tools/bin/itos"
	}
	if !filepath.IsAbs(w.bin) {
		w.bin = filepath.Join(root, w.bin)
	}
	if w.dir, err = os.MkdirTemp("", "itos-features-"); err != nil {
		return err
	}
	if w.support, err = os.MkdirTemp("", "itos-features-support-"); err != nil {
		return err
	}
	return w.git("init", "-q", "-b", "main")
}

// The folder holding go.mod, above the working directory go test gives.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working directory")
		}
		dir = parent
	}
}

// The environment every command runs in: the caller's, less what would make
// git or itos read anything but the scratch repository (a hook's GIT_DIR, CI's
// settings), with no global or system git config and a fixed identity.
func (w *world) env() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "ITOS_") ||
			strings.HasPrefix(name, "GITHUB_") || name == "CI" {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=itos features",
		"GIT_AUTHOR_EMAIL=features@localhost",
		"GIT_COMMITTER_NAME=itos features",
		"GIT_COMMITTER_EMAIL=features@localhost",
	)
}

func (w *world) git(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = w.dir
	cmd.Env = w.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

func (w *world) head() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = w.dir
	cmd.Env = w.env()
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func (w *world) write(path, text string) error {
	full := filepath.Join(w.dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(text), 0o644)
}

// Every file staged, then one commit with the message.
func (w *world) commit(message string) error {
	if err := w.git("add", "-A"); err != nil {
		return err
	}
	if err := w.git("commit", "-q", "--no-verify", "-m", message); err != nil {
		return err
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	w.commits = append(w.commits, sha)
	return nil
}

// A word for sh: the text in single quotes.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// The scratch repository's itos.yaml: a ledger of tasks/phase-<n>.yaml, the
// Conventional Commit types, a Task: footer the non-feature types need,
// docs commits held to prose, and a people list, plus what the scenario set.
func (w *world) writeConfig() error {
	var b strings.Builder
	b.WriteString("version: 1\n")
	if w.config.recordingShell {
		fmt.Fprintf(&b, "shell: [%q]\n", w.recordingShellPath())
	}
	b.WriteString(`ledger: { files: "tasks/phase-{group}.yaml", id: "T-\\d+" }
commits:
  types: [feat, fix, refactor, perf, test, build, ci, chore, docs, style, revert]
  footers:
    Task:
      source: ledger
      required_for: [refactor, perf, test, build, ci, chore, revert]
      validate_for: all
      read_at: commit
  scopes:
    docs: { only: ["**/*.md", "docs/**", "tasks/**"] }
`)
	if w.config.since != "" {
		fmt.Fprintf(&b, "  since: %q\n", w.config.since)
	}
	if w.config.headerLint {
		// commitlint from the itos checkout, which installs it, with only
		// @commitlint/config-conventional: no footer rules of its own.
		config := filepath.Join(w.support, "commitlint.config.mjs")
		text := `export default { extends: ["@commitlint/config-conventional"] };` + "\n"
		if err := os.WriteFile(config, []byte(text), 0o644); err != nil {
			return err
		}
		lint := fmt.Sprintf("%s --cwd %s --config %s",
			quote(filepath.Join(w.root, "node_modules", ".bin", "commitlint")), quote(w.root), quote(config))
		fmt.Fprintf(&b, "  header_lint:\n    use: command\n    hook: %q\n    stdin: %q\n", lint+" --edit {file}", lint)
	} else if w.config.headerLintCommand != "" {
		lint := w.config.headerLintCommand
		fmt.Fprintf(&b, "  header_lint:\n    use: command\n    hook: %q\n    stdin: %q\n", lint, lint)
	}
	if w.config.rangeCheck {
		record := "printf '%s\\n' {from} > " + quote(filepath.Join(w.support, "range-from"))
		fmt.Fprintf(&b, `tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    range_checks:
      - name: record
        range: %q
`, record)
	}
	if len(w.config.ciSteps) > 0 || w.config.stopAtFirst != nil {
		b.WriteString("ci:\n  steps:")
		if len(w.config.ciSteps) == 0 {
			b.WriteString(" []")
		}
		b.WriteString("\n")
		for _, step := range w.config.ciSteps {
			fmt.Fprintf(&b, "    - %q\n", step)
		}
		if w.config.stopAtFirst != nil {
			fmt.Fprintf(&b, "  stop_at_first_failure: %t\n", *w.config.stopAtFirst)
		}
	}
	b.WriteString("work: { ")
	if w.config.registry != "" {
		fmt.Fprintf(&b, "registry: %q, ", w.config.registry)
	}
	b.WriteString("people: { source: yaml, file: people.yaml } }\n")
	if w.config.taskChecks != nil || w.config.checkTimeout > 0 {
		b.WriteString("hooks:\n  commit_msg:\n")
		if w.config.taskChecks != nil {
			fmt.Fprintf(&b, "    task_checks: %t\n", *w.config.taskChecks)
		}
		if w.config.checkTimeout > 0 {
			fmt.Fprintf(&b, "    check_timeout: %d\n", w.config.checkTimeout)
		}
	}
	return w.write("itos.yaml", b.String())
}

// Where a scratch repository's work registry starts: itos's default.
const startingRegistry = "tasks/work-items.yaml"

// The files every scratch repository starts with: its config, a ledger with
// one task, an empty work registry, its people and a README.
func (w *world) startingFiles(task string) error {
	files := map[string]string{
		"tasks/phase-1.yaml": fmt.Sprintf("- { id: %s, type: chore, title: Tidy }\n", task),
		startingRegistry:     "phases: {}\nitems: []\n",
		"people.yaml":        "- someone\n",
		"README.md":          "# Scratch\n",
	}
	for path, text := range files {
		if err := w.write(path, text); err != nil {
			return err
		}
	}
	return w.writeConfig()
}

// Given steps.

func (w *world) templateRepository(message string) error {
	w.config.headerLint = true
	if err := w.startingFiles("T-001"); err != nil {
		return err
	}
	return w.commit(message)
}

func (w *world) commitOnTop(message string) error {
	if err := w.write("README.md", "# Scratch\n\nA line for "+message+".\n"); err != nil {
		return err
	}
	return w.commit(message)
}

func (w *world) repositoryWithTask(task string) error {
	if err := w.startingFiles(task); err != nil {
		return err
	}
	return w.commit("chore: start\n\nTask: " + task + "\n")
}

func (w *world) stageChange(path string) error {
	full := filepath.Join(w.dir, path)
	text, err := os.ReadFile(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := w.write(path, string(text)+"A staged change.\n"); err != nil {
		return err
	}
	return w.git("add", "--", path)
}

func (w *world) conventionalHeaderLint() error {
	w.config.headerLint = true
	return w.writeConfig()
}

func (w *world) sinceFirstCommit() error {
	if len(w.commits) == 0 {
		return errors.New("the repository has no commit yet")
	}
	return w.sinceIs(w.commits[0])
}

func (w *world) sinceIs(value string) error {
	w.config.since = value
	return w.writeConfig()
}

func (w *world) recordingRangeCheck() error {
	w.config.rangeCheck = true
	return w.writeConfig()
}

// The recording shell: a script in the support folder that appends each
// command it is given to shell-log beside it, then runs it with sh -c.
func (w *world) recordingShellPath() string { return filepath.Join(w.support, "recording-shell") }

func (w *world) recordingShell() error {
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$1\" >> %s\nexec sh -c \"$1\"\n",
		quote(filepath.Join(w.support, "shell-log")))
	if err := os.WriteFile(w.recordingShellPath(), []byte(script), 0o755); err != nil {
		return err
	}
	w.config.recordingShell = true
	return w.writeConfig()
}

// The ledger's one task, with one check.
func (w *world) taskHasCheck(task, check string) error {
	return w.write("tasks/phase-1.yaml",
		fmt.Sprintf("- { id: %s, type: chore, title: Tidy, done_when: [{ run: %q }] }\n", task, check))
}

// The file, in the scratch repository, that the recording step writes.
const recordingStepFile = "step-ran"

// The file, in the scratch repository, that the recording check writes, and
// the check.
const recordingCheckFile = "check-ran"

var recordingCheck = "printf 'ran\\n' > " + recordingCheckFile

// A check of the ledger's that says cost: static, which its command alone
// would not make it, as the scratch config has no static patterns.
func staticCheck(command string) string { return fmt.Sprintf("{ run: %q, cost: static }", command) }

// The ledger's one task with these checks, staged: the commit-msg hook reads
// the ledger as the commit will hold it.
func (w *world) stagedChecks(task string, checks ...string) error {
	if err := w.write("tasks/phase-1.yaml", fmt.Sprintf(
		"- { id: %s, type: chore, title: Tidy, done_when: [%s] }\n", task, strings.Join(checks, ", "))); err != nil {
		return err
	}
	return w.git("add", "--", "tasks/phase-1.yaml")
}

func (w *world) taskChecksAre(value string) error {
	on := value == "true"
	w.config.taskChecks = &on
	return w.writeConfig()
}

func (w *world) checkTimeoutIs(seconds int) error {
	w.config.checkTimeout = seconds
	return w.writeConfig()
}

func (w *world) ciStepsAre(steps ...string) error {
	w.config.ciSteps = steps
	return w.writeConfig()
}

func (w *world) headerLintIs(command string) error {
	w.config.headerLintCommand = command
	return w.writeConfig()
}

func (w *world) stopAtFirstFailureIs(value string) error {
	stop := value == "true"
	w.config.stopAtFirst = &stop
	return w.writeConfig()
}

func (w *world) registryIs(path string) error {
	w.config.registry = path
	return w.writeConfig()
}

// The repository's one work registry is at path, with one unowned item of
// phase 1: the registry it started with is removed when it is elsewhere, so
// nothing is left where itos would otherwise look. The working tree and the
// index both hold it, so what reads the staged tree reads it too.
func (w *world) registryAt(path, item, status string) error {
	if path != startingRegistry {
		if err := os.Remove(filepath.Join(w.dir, startingRegistry)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := w.workingRegistry(path, item, status); err != nil {
		return err
	}
	return w.git("add", "-A", "--", path, startingRegistry)
}

// The registry at path holds one unowned item of phase 1, in the working tree
// alone: nothing is staged.
func (w *world) workingRegistry(path, item, status string) error {
	return w.write(path, fmt.Sprintf(
		"phases: { 1: null }\nitems:\n  - { id: %s, title: %s, phase: 1, owner: null, status: %s, depends_on: [] }\n",
		item, item, status))
}

// The same registry, staged, with the starting one's removal when it moved.
func (w *world) stagedRegistryAt(path, item, status string) error {
	return w.registryAt(path, item, status)
}

// The same registry, committed past the hooks.
func (w *world) committedRegistryAt(path, item, status string) error {
	if err := w.registryAt(path, item, status); err != nil {
		return err
	}
	return w.commit("docs: a registry")
}

// The scratch repository's itos.yaml with one more top-level key, staged.
func (w *world) stagedConfigKey(key string) error {
	if err := w.writeConfig(); err != nil {
		return err
	}
	text, err := os.ReadFile(filepath.Join(w.dir, "itos.yaml"))
	if err != nil {
		return err
	}
	if err := w.write("itos.yaml", string(text)+key+": true\n"); err != nil {
		return err
	}
	return w.git("add", "--", "itos.yaml")
}

// The ledger's one task with one more key, staged.
func (w *world) stagedLedgerKey(task, key string) error {
	if err := w.write("tasks/phase-1.yaml",
		fmt.Sprintf("- { id: %s, type: chore, title: Tidy, %s: Tidy }\n", task, key)); err != nil {
		return err
	}
	return w.git("add", "--", "tasks/phase-1.yaml")
}

// When steps.

// itos with these arguments, in the scratch repository.
func (w *world) itos(args ...string) error {
	cmd := exec.Command(w.bin, args...)
	cmd.Dir = w.dir
	cmd.Env = w.env()
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	w.stdout, w.stderr = stdout.String(), stderr.String()
	var exit *exec.ExitError
	switch {
	case err == nil:
		w.exit = 0
	case errors.As(err, &exit):
		w.exit = exit.ExitCode()
	default:
		return fmt.Errorf("running %s: %w", w.bin, err)
	}
	return nil
}

func (w *world) commitMsgHook(message string) error {
	file := filepath.Join(w.support, "COMMIT_EDITMSG")
	if err := os.WriteFile(file, []byte(message), 0o644); err != nil {
		return err
	}
	return w.itos("hook", "commit-msg", file)
}

// Then steps.

func (w *world) output() string { return w.stdout + w.stderr }

func (w *world) report() string {
	return fmt.Sprintf("exit %d\n--- stdout\n%s--- stderr\n%s", w.exit, w.stdout, w.stderr)
}

func (w *world) exitsWith(code int) error {
	if w.exit != code {
		return fmt.Errorf("itos exited %d, not %d\n%s", w.exit, code, w.report())
	}
	return nil
}

func (w *world) outputSays(text string) error {
	if !strings.Contains(w.output(), text) {
		return fmt.Errorf("the output does not say %q\n%s", text, w.report())
	}
	return nil
}

func (w *world) outputNamesRule(rule string) error {
	if !strings.Contains(w.output(), "["+rule+"]") {
		return fmt.Errorf("the output does not name the rule [%s]\n%s", rule, w.report())
	}
	return nil
}

func (w *world) rangeCheckStartedAtFirst() error {
	text, err := os.ReadFile(filepath.Join(w.support, "range-from"))
	if err != nil {
		return fmt.Errorf("the range check did not run: %w\n%s", err, w.report())
	}
	if got := strings.TrimSpace(string(text)); got != w.commits[0] {
		return fmt.Errorf("the range check started at %q, not the first commit %s\n%s", got, w.commits[0], w.report())
	}
	return nil
}

// The commands the recording shell was given, one a line.
func (w *world) shellLog() ([]string, error) {
	text, err := os.ReadFile(filepath.Join(w.support, "shell-log"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(string(text), "\n"), "\n"), err
}

func (w *world) recordingShellRan(command string) error {
	log, err := w.shellLog()
	if err != nil {
		return err
	}
	for _, line := range log {
		if line == command {
			return nil
		}
	}
	return fmt.Errorf("the recording shell did not run %q; it ran %q\n%s", command, log, w.report())
}

// The range check is the only command that names the file it records to.
func (w *world) recordingShellRanRangeCheck() error {
	log, err := w.shellLog()
	if err != nil {
		return err
	}
	file := quote(filepath.Join(w.support, "range-from"))
	for _, line := range log {
		if strings.Contains(line, file) {
			return nil
		}
	}
	return fmt.Errorf("the recording shell did not run the range check; it ran %q\n%s", log, w.report())
}

func (w *world) recordingCheckRan(want bool) error {
	_, err := os.Stat(filepath.Join(w.dir, recordingCheckFile))
	ran := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if ran != want {
		return fmt.Errorf("the recording check ran: %t, not %t\n%s", ran, want, w.report())
	}
	return nil
}

func (w *world) recordingStepRan(want bool) error {
	_, err := os.Stat(filepath.Join(w.dir, recordingStepFile))
	ran := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if ran != want {
		return fmt.Errorf("the recording step ran: %t, not %t\n%s", ran, want, w.report())
	}
	return nil
}
