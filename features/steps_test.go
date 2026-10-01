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
	headerLint bool   // the header lint delegated to commitlint's conventional config
	since      string // commits.since
	rangeCheck bool   // a range check that records where its range starts
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

	sc.Step(`^itos verifies every commit up to HEAD$`, func() error { return w.itos("verify", "", "HEAD") })
	sc.Step(`^itos checks the config$`, func() error { return w.itos("config", "check") })
	sc.Step(`^the commit-msg hook checks the message "([^"]*)"$`, w.commitMsgHook)
	sc.Step(`^the commit-msg hook checks the message:$`, func(message *godog.DocString) error {
		return w.commitMsgHook(message.Content + "\n")
	})

	sc.Step(`^itos exits with code (\d+)$`, w.exitsWith)
	sc.Step(`^its output says "([^"]*)"$`, w.outputSays)
	sc.Step(`^its output names the rule "([^"]*)"$`, w.outputNamesRule)
	sc.Step(`^the range check started at the first commit$`, w.rangeCheckStartedAtFirst)
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
	b.WriteString(`version: 1
ledger: { files: "tasks/phase-{group}.yaml", id: "T-\\d+" }
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
	b.WriteString("work: { people: { source: yaml, file: people.yaml } }\n")
	return w.write("itos.yaml", b.String())
}

// The files every scratch repository starts with: its config, a ledger with
// one task, an empty work registry, its people and a README.
func (w *world) startingFiles(task string) error {
	files := map[string]string{
		"tasks/phase-1.yaml":   fmt.Sprintf("- { id: %s, type: chore, title: Tidy }\n", task),
		"docs/work-items.yaml": "phases: {}\nitems: []\n",
		"people.yaml":          "- someone\n",
		"README.md":            "# Scratch\n",
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
