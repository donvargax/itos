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
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// One scenario's state.
type world struct {
	root          string // the itos checkout: where go.mod is
	bin           string // the itos binary under test
	dir           string // the scratch repository
	support       string // files the scenario needs outside the repository
	config        scratchConfig
	commits       []string          // the scratch repository's commits, oldest first
	scenarioFiles map[string]string // each scenario ID written, to its feature file
	ledger        []ledgerTask      // the tasks of tasks/phase-1.yaml, in order

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
	ciTests           string   // a kind of named tests, with run and recognize templates, run by the last of ci.steps
	stopAtFirst       *bool    // ci.stop_at_first_failure
	costStatic        []string // ci.cost.static
	registry          string   // work.registry
	taskChecks        *bool    // hooks.commit_msg.task_checks
	checkTimeout      int      // hooks.commit_msg.check_timeout, when above 0
	statuses          []string // work.statuses
	groupsKey         string   // work.groups_key
	smoke             bool     // tests.scenario has a smoke set, features/smoke.yaml
	smokeEveryFile    *bool    // tests.scenario.smoke.every_file
	hooksManager      string   // hooks.manager
	hooksBin          string   // hooks.bin
	settings          []setting
}

// One task of the scratch ledger: its ID and its checks, each written as YAML.
type ledgerTask struct {
	id     string
	checks []string
}

// One key the scenario sets by its dotted path, to a string. A key under
// ledger or commits goes into that section; any other starts a section of its
// own, so it may not be one the config already writes.
type setting struct{ key, value string }

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
	sc.Step(`^a repository whose ledger has the task "([^"]*)"$`, func(task string) error {
		return w.repositoryWithTask(task)
	})
	sc.Step(`^a repository whose ledger has the tasks "([^"]*)" and "([^"]*)"$`, func(a, b string) error {
		return w.repositoryWithTask(a, b)
	})
	sc.Step(`^a change to "([^"]*)" is staged$`, w.stageChange)
	sc.Step(`^the ledger folder is missing$`, w.ledgerFolderMissing)
	sc.Step(`^the header lint is commitlint's conventional config$`, w.conventionalHeaderLint)
	sc.Step(`^commits\.since names the first commit$`, w.sinceFirstCommit)
	sc.Step(`^commits\.since is "([^"]*)"$`, w.sinceIs)
	sc.Step(`^a range check that records where its range starts$`, w.recordingRangeCheck)
	sc.Step(`^the config's shell is the recording shell$`, w.recordingShell)
	sc.Step(`^the task "([^"]*)" has the check "([^"]*)"$`, w.taskHasCheck)
	sc.Step(`^the CI steps are "([^"]*)"$`, func(step string) error { return w.ciStepsAre(step) })
	sc.Step(`^the CI steps run the named tests of the kind "([^"]*)"$`, w.ciStepsRunTests)
	sc.Step(`^the smoke set is empty$`, w.emptySmokeSet)
	sc.Step(`^the task "([^"]*)" has a check that runs the scenario "([^"]*)"$`, func(task, id string) error {
		return w.taskHasCheck(task, "run-scenarios "+id)
	})
	sc.Step(`^the commit "([^"]*)" naming the task "([^"]*)" on top of it$`, func(message, task string) error {
		return w.commitOnTop(message + "\n\nTask: " + task + "\n")
	})
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
	sc.Step(`^the tasks "([^"]*)" and "([^"]*)" each have the counting check$`, func(a, b string) error {
		if err := w.stagedChecks(a, countingCheck("run")); err != nil {
			return err
		}
		return w.stagedChecks(b, countingCheck("run"))
	})
	sc.Step(`^the task "([^"]*)" has the counting check as (run|fails)$`, func(task, mode string) error {
		return w.stagedChecks(task, countingCheck(mode))
	})
	sc.Step(`^hooks\.commit_msg\.task_checks is (true|false)$`, w.taskChecksAre)
	sc.Step(`^hooks\.commit_msg\.check_timeout is (\d+)$`, w.checkTimeoutIs)
	sc.Step(`^work\.statuses is "([^"]*)"$`, w.statusesAre)
	sc.Step(`^the work registry has the item "([^"]*)" with the status "([^"]*)"$`, func(item, status string) error {
		return w.workingRegistry(startingRegistry, item, status)
	})
	sc.Step(`^work\.groups_key is "([^"]*)"$`, w.groupsKeyIs)
	sc.Step(`^the work registry gives the group "([^"]*)" to the owner "([^"]*)" under "([^"]*)"$`, w.registryGroupOwner)
	sc.Step(`^a feature file "([^"]*)" with the live scenario "([^"]*)"$`, w.featureFile)
	sc.Step(`^the smoke set lists only "([^"]*)"$`, w.smokeSetLists)
	sc.Step(`^smoke\.every_file is (true|false)$`, w.smokeEveryFileIs)
	sc.Step(`^a "([^"]*)" folder$`, w.folder)
	sc.Step(`^hooks\.manager is "([^"]*)"$`, w.hooksManagerIs)
	sc.Step(`^hooks\.bin is "([^"]*)"$`, w.hooksBinIs)
	sc.Step(`^ci\.cost\.static is "([^"]*)"$`, w.costStaticIs)
	sc.Step(`^"([^"]*)" is a script that records it ran$`, w.recordingScript)
	sc.Step(`^the config sets "([^"]*)" to "([^"]*)"$`, w.configSets)

	sc.Step(`^itos verifies every commit up to HEAD$`, func() error { return w.itos("verify", "", "HEAD") })
	sc.Step(`^itos checks the config$`, func() error { return w.itos("config", "check") })
	sc.Step(`^itos checks the work registry$`, func() error { return w.itos("work", "check") })
	sc.Step(`^itos checks the smoke set$`, func() error { return w.itos("tests", "smoke", "check", "scenario") })
	sc.Step(`^itos installs the hooks$`, func() error { return w.itos("hooks", "install") })
	sc.Step(`^itos runs the task "([^"]*)"$`, func(task string) error { return w.itos("task", task) })
	sc.Step(`^itos runs the tasks "([^"]*)" and "([^"]*)"$`, func(a, b string) error { return w.itos("task", a, b) })
	sc.Step(`^itos runs the pending tasks$`, func() error { return w.itos("task", "--pending") })
	sc.Step(`^itos runs the tasks of the group "([^"]*)"$`, func(group string) error {
		return w.itos("task", "--group", group)
	})
	sc.Step(`^itos lists the tasks$`, func() error { return w.itos("task", "list") })
	sc.Step(`^itos runs CI over every commit up to HEAD$`, func() error { return w.itos("ci", "run", "", "HEAD") })
	sc.Step(`^itos runs CI over the commits after the first$`, func() error {
		if len(w.commits) == 0 {
			return errors.New("the repository has no commit yet")
		}
		return w.itos("ci", "run", w.commits[0], "HEAD")
	})
	sc.Step(`^the commit-msg hook checks the message "([^"]*)"$`, w.commitMsgHook)
	sc.Step(`^the commit-msg hook checks the message:$`, func(message *godog.DocString) error {
		return w.commitMsgHook(message.Content + "\n")
	})

	sc.Step(`^itos exits with code (\d+)$`, w.exitsWith)
	sc.Step(`^its output says "([^"]*)"$`, w.outputSays)
	sc.Step(`^its output does not say "([^"]*)"$`, w.outputDoesNotSay)
	sc.Step(`^its output names the rule "([^"]*)"$`, w.outputNamesRule)
	sc.Step(`^its output lists "([^"]*)" as "([^"]*)"$`, w.outputLists)
	sc.Step(`^the counting check ran once$`, func() error { return w.countingCheckRan(1) })
	sc.Step(`^the range check started at the first commit$`, w.rangeCheckStartedAtFirst)
	sc.Step(`^the recording shell ran "([^"]*)"$`, w.recordingShellRan)
	sc.Step(`^the recording shell ran the range check$`, w.recordingShellRanRangeCheck)
	sc.Step(`^the recording step ran$`, func() error { return w.recordingStepRan(true) })
	sc.Step(`^the recording step did not run$`, func() error { return w.recordingStepRan(false) })
	sc.Step(`^the recording check ran$`, func() error { return w.recordingCheckRan(true) })
	sc.Step(`^the recording check did not run$`, func() error { return w.recordingCheckRan(false) })
	sc.Step(`^the file "([^"]*)" calls itos$`, w.fileCallsItos)
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
	b.WriteString(`ledger:
  files: "tasks/phase-{group}.yaml"
  id: "T-\\d+"
`)
	b.WriteString(w.settingsUnder("ledger"))
	b.WriteString(`commits:
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
		fmt.Fprintf(&b, "  header_lint:\n    hook: %q\n    stdin: %q\n", lint+" --edit {file}", lint)
	} else if w.config.headerLintCommand != "" {
		lint := w.config.headerLintCommand
		fmt.Fprintf(&b, "  header_lint:\n    hook: %q\n    stdin: %q\n", lint, lint)
	}
	b.WriteString(w.settingsUnder("commits"))
	if w.config.rangeCheck || w.config.smoke || w.config.ciTests != "" {
		kind := "scenario"
		if w.config.ciTests != "" {
			kind = w.config.ciTests
		}
		fmt.Fprintf(&b, `tests:
  %s:
    root: features
    id: "ID-[A-Z]+-\\d+"
    tag_prefix: "@"
`, kind)
	}
	if w.config.ciTests != "" {
		// A runner that only says what it would run, as the conformance case's
		// does, and a check written as "run-scenarios <pattern>" read back as a
		// selection of the kind.
		b.WriteString(`    run:
      whole: "echo run every scenario"
      select: "printf 'run %s\\n' {pattern}"
      ids_pattern: "@(?:{ids})\\b"
    recognize:
      - { command: "run-scenarios {pattern}", as: pattern }
`)
	}
	if w.config.smoke || w.config.ciTests != "" {
		b.WriteString("    smoke:\n      file: features/smoke.yaml\n")
		if w.config.smokeEveryFile != nil {
			fmt.Fprintf(&b, "      every_file: %t\n", *w.config.smokeEveryFile)
		}
	}
	if w.config.rangeCheck {
		record := "printf '%s\\n' {from} > " + quote(filepath.Join(w.support, "range-from"))
		fmt.Fprintf(&b, "    range_checks:\n      - name: record\n        range: %q\n", record)
	}
	if len(w.config.ciSteps) > 0 || w.config.ciTests != "" || w.config.stopAtFirst != nil || len(w.config.costStatic) > 0 {
		b.WriteString("ci:\n  steps:")
		if len(w.config.ciSteps) == 0 && w.config.ciTests == "" {
			b.WriteString(" []")
		}
		b.WriteString("\n")
		for _, step := range w.config.ciSteps {
			fmt.Fprintf(&b, "    - %q\n", step)
		}
		if w.config.ciTests != "" {
			fmt.Fprintf(&b, "    - { tests: %s }\n", w.config.ciTests)
		}
		if w.config.stopAtFirst != nil {
			fmt.Fprintf(&b, "  stop_at_first_failure: %t\n", *w.config.stopAtFirst)
		}
		if len(w.config.costStatic) > 0 {
			b.WriteString("  cost:\n    static:\n")
			for _, pattern := range w.config.costStatic {
				fmt.Fprintf(&b, "      - %q\n", pattern)
			}
		}
	}
	b.WriteString("work: { ")
	if w.config.registry != "" {
		fmt.Fprintf(&b, "registry: %q, ", w.config.registry)
	}
	if w.config.groupsKey != "" {
		fmt.Fprintf(&b, "groups_key: %q, ", w.config.groupsKey)
	}
	if w.config.statuses != nil {
		fmt.Fprintf(&b, "statuses: [%s], ", strings.Join(w.config.statuses, ", "))
	}
	b.WriteString("people: { source: yaml, file: people.yaml } }\n")
	if w.config.hooksManager != "" || w.config.hooksBin != "" || w.config.taskChecks != nil || w.config.checkTimeout > 0 {
		b.WriteString("hooks:\n")
	}
	if w.config.hooksManager != "" {
		fmt.Fprintf(&b, "  manager: %q\n", w.config.hooksManager)
	}
	if w.config.hooksBin != "" {
		fmt.Fprintf(&b, "  bin: %q\n", w.config.hooksBin)
	}
	if w.config.taskChecks != nil || w.config.checkTimeout > 0 {
		b.WriteString("  commit_msg:\n")
		if w.config.taskChecks != nil {
			fmt.Fprintf(&b, "    task_checks: %t\n", *w.config.taskChecks)
		}
		if w.config.checkTimeout > 0 {
			fmt.Fprintf(&b, "    check_timeout: %d\n", w.config.checkTimeout)
		}
	}
	for _, s := range w.config.settings {
		if path := strings.Split(s.key, "."); path[0] != "ledger" && path[0] != "commits" {
			b.WriteString(nested(path, s.value, 0))
		}
	}
	return w.write("itos.yaml", b.String())
}

// The scenario's settings under a section the config writes, as lines below it.
func (w *world) settingsUnder(section string) string {
	var b strings.Builder
	for _, s := range w.config.settings {
		if path := strings.Split(s.key, "."); path[0] == section && len(path) > 1 {
			b.WriteString(nested(path[1:], s.value, 1))
		}
	}
	return b.String()
}

// The YAML lines that set the key path to value, the first key at the given
// depth.
func nested(path []string, value string, depth int) string {
	var b strings.Builder
	for i, key := range path {
		b.WriteString(strings.Repeat("  ", depth+i) + key + ":")
		if i < len(path)-1 {
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, " %q\n", value)
	return b.String()
}

// Where a scratch repository's work registry starts: itos's default.
const startingRegistry = "tasks/work-items.yaml"

// The files every scratch repository starts with: its config, a ledger with
// the tasks, none with a check, an empty work registry, its people and a
// README.
func (w *world) startingFiles(tasks ...string) error {
	w.ledger = nil
	for _, id := range tasks {
		w.ledger = append(w.ledger, ledgerTask{id: id})
	}
	files := map[string]string{
		"tasks/phase-1.yaml": w.ledgerText(),
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

// Each task's title, by its place in the ledger, so that no title is a word a
// status line could be read as.
var taskTitles = []string{"Tidy", "Sweep", "Dust"}

// The scratch ledger, tasks/phase-1.yaml: one line a task, with its checks.
func (w *world) ledgerText() string {
	var b strings.Builder
	for i, task := range w.ledger {
		fmt.Fprintf(&b, "- { id: %s, type: chore, title: %s", task.id, taskTitles[i%len(taskTitles)])
		if len(task.checks) > 0 {
			fmt.Fprintf(&b, ", done_when: [%s]", strings.Join(task.checks, ", "))
		}
		b.WriteString(" }\n")
	}
	return b.String()
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

func (w *world) repositoryWithTask(tasks ...string) error {
	if err := w.startingFiles(tasks...); err != nil {
		return err
	}
	return w.commit("chore: start\n\nTask: " + strings.Join(tasks, " ") + "\n")
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

// The ledger's folder, tasks/, gone from the working tree, the index and HEAD:
// a repository whose config has a ledger footer and no ledger. Its removal is
// committed alone, whatever else is staged, so that the commit-msg hook checks
// a commit that stages none of itos's data, and reads no ledger in the index
// or, falling back, in the working tree.
func (w *world) ledgerFolderMissing() error {
	if err := os.RemoveAll(filepath.Join(w.dir, "tasks")); err != nil {
		return err
	}
	if err := w.git("rm", "-r", "-q", "--cached", "--", "tasks"); err != nil {
		return err
	}
	if err := w.git("commit", "-q", "--no-verify", "-m", "chore: drop the ledger", "--", "tasks"); err != nil {
		return err
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	w.commits = append(w.commits, sha)
	return nil
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

// The ledger's task, with one check and no cost: of its own, staged: the
// commit-msg hook reads the ledger as the commit will hold it, and itos task
// reads the working tree, which holds the same.
func (w *world) taskHasCheck(task, check string) error {
	return w.stagedChecks(task, fmt.Sprintf("{ run: %q }", check))
}

// A script at the path in the scratch repository that writes the recording
// check's file when it runs, whatever its arguments: a check that calls it
// ran exactly when the file is there.
func (w *world) recordingScript(path string) error {
	script := "#!/bin/sh\nprintf 'ran\\n' > " + quote(filepath.Join(w.dir, recordingCheckFile)) + "\n"
	if err := w.write(path, script); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(w.dir, path), 0o755)
}

// The file, in the scratch repository, that the recording step writes.
const recordingStepFile = "step-ran"

// The file, in the scratch repository, that the recording check writes, and
// the check.
const recordingCheckFile = "check-ran"

var recordingCheck = "printf 'ran\\n' > " + recordingCheckFile

// The file, in the scratch repository, that the counting check appends a line
// to each time it runs.
const countingCheckFile = "check-runs"

// The counting check, in the mode given (run or fails): it appends a line to
// its file, then exits 3, so it fails as a run: and passes as a fails:.
func countingCheck(mode string) string {
	return fmt.Sprintf("{ %s: %q }", mode, "printf 'ran\\n' >> "+countingCheckFile+"; exit 3")
}

// A check of the ledger's that says cost: static, which its command alone
// would not make it, as the scratch config has no static patterns.
func staticCheck(command string) string { return fmt.Sprintf("{ run: %q, cost: static }", command) }

// The ledger's task with these checks, its other tasks as they were, staged:
// the commit-msg hook reads the ledger as the commit will hold it. A task the
// ledger does not have is added after the others.
func (w *world) stagedChecks(task string, checks ...string) error {
	i := slices.IndexFunc(w.ledger, func(t ledgerTask) bool { return t.id == task })
	if i < 0 {
		w.ledger = append(w.ledger, ledgerTask{id: task})
		i = len(w.ledger) - 1
	}
	w.ledger[i].checks = checks
	if err := w.write("tasks/phase-1.yaml", w.ledgerText()); err != nil {
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

func (w *world) statusesAre(list string) error {
	w.config.statuses = strings.Split(list, ", ")
	return w.writeConfig()
}

func (w *world) groupsKeyIs(key string) error {
	w.config.groupsKey = key
	return w.writeConfig()
}

// The registry at its default path gives the group to the owner under key,
// and holds one unowned item of that group: a registry read for its owners
// anywhere but key finds the group unlisted. The owner is made one of the
// people, so that owning the group is no problem of its own.
func (w *world) registryGroupOwner(group, owner, key string) error {
	if err := w.write("people.yaml", "- someone\n- "+owner+"\n"); err != nil {
		return err
	}
	return w.write(startingRegistry, fmt.Sprintf(
		"%s: { %s: %s }\nitems:\n  - { id: W-1, title: One, phase: %s, owner: null, status: todo, depends_on: [] }\n",
		key, group, owner, group))
}

// A feature file under features/ with one live scenario, which the config's
// scenario kind (with a smoke set) reads.
func (w *world) featureFile(file, id string) error {
	if w.scenarioFiles == nil {
		w.scenarioFiles = map[string]string{}
	}
	w.scenarioFiles[id] = file
	text := fmt.Sprintf("Feature: %s\n\n  %s\n  Scenario: %s runs\n    When it runs\n", file, id, id)
	if err := w.write(filepath.Join("features", file), text); err != nil {
		return err
	}
	w.config.smoke = true
	return w.writeConfig()
}

// The smoke set holds the one scenario, under the feature file it is in.
func (w *world) smokeSetLists(id string) error {
	file, ok := w.scenarioFiles[id]
	if !ok {
		return fmt.Errorf("no feature file has the scenario %s", id)
	}
	return w.write("features/smoke.yaml", fmt.Sprintf(
		"- file: %s\n  scenarios: [{ id: %q, why: the one listed }]\n", file, id))
}

func (w *world) smokeEveryFileIs(value string) error {
	every := value == "true"
	w.config.smokeEveryFile = &every
	return w.writeConfig()
}

func (w *world) folder(path string) error {
	return os.MkdirAll(filepath.Join(w.dir, path), 0o755)
}

func (w *world) hooksManagerIs(manager string) error {
	w.config.hooksManager = manager
	return w.writeConfig()
}

func (w *world) hooksBinIs(bin string) error {
	w.config.hooksBin = bin
	return w.writeConfig()
}

func (w *world) costStaticIs(pattern string) error {
	w.config.costStatic = []string{pattern}
	return w.writeConfig()
}

func (w *world) configSets(key, value string) error {
	w.config.settings = append(w.config.settings, setting{key, value})
	return w.writeConfig()
}

// CI's one step runs the kind's named tests. The kind has a feature file with
// two live scenarios, @ID-A-01 and @ID-A-02, and a smoke set of the second, so
// a push's run selects the smoke set and what a check adds to it.
func (w *world) ciStepsRunTests(kind string) error {
	feature := "Feature: A\n\n  @ID-A-01\n  Scenario: One\n    When one runs\n\n" +
		"  @ID-A-02\n  Scenario: Two\n    When two runs\n"
	if err := w.write("features/a.feature", feature); err != nil {
		return err
	}
	if err := w.write("features/smoke.yaml",
		"- file: a.feature\n  scenarios: [{ id: \"@ID-A-02\", why: the one listed }]\n"); err != nil {
		return err
	}
	w.config.ciTests = kind
	return w.writeConfig()
}

// The smoke set lists no file, so a push selects only what its commits name.
func (w *world) emptySmokeSet() error {
	return w.write("features/smoke.yaml", "[]\n")
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

func (w *world) outputDoesNotSay(text string) error {
	if strings.Contains(w.output(), text) {
		return fmt.Errorf("the output says %q\n%s", text, w.report())
	}
	return nil
}

func (w *world) outputNamesRule(rule string) error {
	if !strings.Contains(w.output(), "["+rule+"]") {
		return fmt.Errorf("the output does not name the rule [%s]\n%s", rule, w.report())
	}
	return nil
}

// A line of the output names the task, as a word of its own, and gives it the
// status, as whole words: the status table's line for the task.
func (w *world) outputLists(task, status string) error {
	word := regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(status) + `(\s|$)`)
	for _, line := range strings.Split(w.output(), "\n") {
		if slices.Contains(strings.Fields(line), task) && word.MatchString(line) {
			return nil
		}
	}
	return fmt.Errorf("the output does not list %s as %q\n%s", task, status, w.report())
}

// The counting check's file has one line a run.
func (w *world) countingCheckRan(times int) error {
	text, err := os.ReadFile(filepath.Join(w.dir, countingCheckFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if runs := strings.Count(string(text), "\n"); runs != times {
		return fmt.Errorf("the counting check ran %d times, not %d\n%s", runs, times, w.report())
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

// The file is a hook that hands its work to itos's hook command.
func (w *world) fileCallsItos(path string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w\n%s", path, err, w.report())
	}
	if !strings.Contains(string(text), "itos hook ") {
		return fmt.Errorf("%s does not call itos:\n%s\n%s", path, text, w.report())
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
