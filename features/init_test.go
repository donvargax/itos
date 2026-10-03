// The steps of itos init (init.feature): a repository with no itos at all, or
// a folder that is no repository yet, the config init writes read back as
// YAML wherever it is (itos.yaml in the root, or the stealth one in the git
// folder), and every file of the folder compared with what it was before the
// last run of itos. Claude Code is a fake claude first on the PATH, which
// records each run's arguments and answers claude plugin list --json; the
// real one is never on a scenario's PATH (callerPath).
package features

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

// The step that compares the folder with what it was before the last run:
// a scenario that has it records the folder before every run of itos.
const noFileChanged = "no file changed since the last run"

func initializeInitSteps(sc *godog.ScenarioContext, w *world) {
	sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
		w.snapshotRuns = slices.ContainsFunc(s.Steps, func(step *godog.Step) bool { return step.Text == noFileChanged })
		return ctx, nil
	})

	sc.Step(`^a repository that does not use itos, its one commit "([^"]*)"$`, w.repositoryWithoutItos)
	sc.Step(`^a folder that is not a git repository$`, w.folderWithoutGit)
	sc.Step(`^the feature file "([^"]*)" with the scenario "([^"]*)"$`, w.untrackedFeatureFile)
	sc.Step(`^the feature file "([^"]*)" with a scenario that has no tag$`, w.untaggedFeatureFile)
	sc.Step(`^the file "([^"]*)" is removed$`, w.removeFile)
	sc.Step(`^a claude on the PATH that records its arguments$`, func() error { return w.fakeClaude("", false) })
	sc.Step(`^a claude on the PATH that records its arguments, writing \.claude/settings\.local\.json as claude does$`,
		func() error { return w.fakeClaude("", true) })
	sc.Step(`^a claude on the PATH that lists the plugin "([^"]*)" as installed$`, func(id string) error { return w.fakeClaude(id, false) })
	sc.Step(`^no claude on the PATH$`, w.noClaude)

	sc.Step(`^the folder is a git repository$`, w.folderIsRepository)
	sc.Step(`^the config's commits\.since is HEAD's full SHA$`, w.sinceIsHead)
	sc.Step(`^the config has no commits\.since$`, func() error { return w.configLacks("commits", "since") })
	sc.Step(`^the config has no pin$`, func() error { return w.configLacks("pin") })
	sc.Step(`^`+noFileChanged+`$`, w.noFileChanged)
	sc.Step(`^the file "([^"]*)" does not exist$`, w.fileDoesNotExist)
	sc.Step(`^claude was given "([^"]*)"$`, func(args string) error { return w.claudeGiven(args, true) })
	sc.Step(`^claude was not given "([^"]*)"$`, func(args string) error { return w.claudeGiven(args, false) })
}

// A repository with one commit, a README, and nothing of itos: no config in
// the root or in the git folder, no ledger, no hooks.
func (w *world) repositoryWithoutItos(message string) error {
	if err := w.write("README.md", "# Scratch\n"); err != nil {
		return err
	}
	return w.commit(message)
}

// The scenario's folder with no git repository in it.
func (w *world) folderWithoutGit() error {
	return os.RemoveAll(filepath.Join(w.dir, ".git"))
}

// A feature file at the path, relative to the repository, with one live
// scenario, neither staged nor committed: a project's own, before itos.
func (w *world) untrackedFeatureFile(path, id string) error {
	return w.write(path, featureText(filepath.Base(path), id))
}

// A feature file at the path with one scenario and no tag at all, neither
// staged nor committed: a Cucumber project's own, which names no test.
func (w *world) untaggedFeatureFile(path string) error {
	return w.write(path, "Feature: "+filepath.Base(path)+"\n\n  Scenario: a page opens\n    When it opens\n")
}

func (w *world) removeFile(path string) error {
	return os.Remove(filepath.Join(w.dir, path))
}

// git finds a repository whose top is the scenario's folder.
func (w *world) folderIsRepository() error {
	top, err := w.gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("the folder is not a git repository: %w\n%s", err, w.report())
	}
	if !sameFolder(strings.TrimSpace(top), w.dir) {
		return fmt.Errorf("the repository's top is %s, not the folder %s", strings.TrimSpace(top), w.dir)
	}
	return nil
}

func sameFolder(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}

// The config itos finds in the folder, read as YAML: itos.yaml in the root,
// else the stealth one in the git folder.
func (w *world) foundConfig() (map[string]any, string, error) {
	path := filepath.Join(w.dir, "itos.yaml")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(w.dir, stealthDir, "itos.yaml")
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("there is no config: %w\n%s", err, w.report())
	}
	var config map[string]any
	if err := yaml.Unmarshal(text, &config); err != nil {
		return nil, "", fmt.Errorf("the config is not YAML: %v\n%s", err, text)
	}
	return config, string(text), nil
}

// The value at a key path of the config, and whether it has one.
func at(config map[string]any, path ...string) (any, bool) {
	var v any = config
	for _, key := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		if v, ok = m[key]; !ok {
			return nil, false
		}
	}
	return v, true
}

func (w *world) sinceIsHead() error {
	config, text, err := w.foundConfig()
	if err != nil {
		return err
	}
	head, err := w.head()
	if err != nil {
		return err
	}
	if since, _ := at(config, "commits", "since"); since != head {
		return fmt.Errorf("commits.since is %v, not HEAD, %s:\n%s", since, head, text)
	}
	return nil
}

func (w *world) configLacks(path ...string) error {
	config, text, err := w.foundConfig()
	if err != nil {
		return err
	}
	if v, ok := at(config, path...); ok {
		return fmt.Errorf("the config has %s: %v\n%s", strings.Join(path, "."), v, text)
	}
	return nil
}

// Every file under the folder, its git folder's included, as its mode and
// its text.
func (w *world) snapshot() (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(w.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(w.dir, path)
		files[rel] = info.Mode().String() + "\n" + string(text)
		return nil
	})
	return files, err
}

func (w *world) noFileChanged() error {
	if w.filesBefore == nil {
		return errors.New("the folder was not recorded before the last run")
	}
	now, err := w.snapshot()
	if err != nil {
		return err
	}
	var changed []string
	for path, before := range w.filesBefore {
		if after, ok := now[path]; !ok {
			changed = append(changed, path+" (removed)")
		} else if after != before {
			changed = append(changed, path)
		}
	}
	for path := range now {
		if _, ok := w.filesBefore[path]; !ok {
			changed = append(changed, path+" (added)")
		}
	}
	if len(changed) > 0 {
		slices.Sort(changed)
		return fmt.Errorf("files changed in the last run: %s\n%s", strings.Join(changed, ", "), w.report())
	}
	return nil
}

func (w *world) fileDoesNotExist(path string) error {
	if _, err := os.Stat(filepath.Join(w.dir, path)); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s exists\n%s", path, w.report())
	}
	return nil
}

// The file the fake claude records its runs in, one a line, each argument
// followed by a tab.
func (w *world) claudeRuns() string { return filepath.Join(w.support, "claude-runs") }

// A claude first on the PATH that records each run's arguments and exits 0.
// claude plugin list --json lists the plugin installed, at the user scope
// and enabled, or none when installed is "". With settings, claude plugin
// install --scope local writes .claude/settings.local.json in the folder it
// runs in, the repository's top, as Claude Code does.
func (w *world) fakeClaude(installed string, settings bool) error {
	bin, err := w.binOnPath()
	if err != nil {
		return err
	}
	list := "[]"
	if installed != "" {
		list = `[{"id":"` + installed + `","version":"2.3.0","scope":"user","enabled":true,"projectEnabled":false}]`
	}
	script := "#!/bin/sh\n" +
		"rec=" + quote(w.claudeRuns()) + "\n" +
		`for a in "$@"; do printf '%s\t' "$a"; done >> "$rec" && printf '\n' >> "$rec" || exit 99` + "\n" +
		`case "$1 $2" in` + "\n" +
		`"plugin list") printf '%s\n' ` + quote(list) + " ;;\n"
	if settings {
		script += `"plugin install") case " $* " in *" --scope local "*)` + "\n" +
			`  mkdir -p .claude && printf '{"enabledPlugins":{"itos@itos":true}}\n' > .claude/settings.local.json ;; esac ;;` + "\n"
	}
	script += "esac\nexit 0\n"
	return w.writeProgram(filepath.Join(bin, "claude"), script)
}

// No claude on the PATH: none of the scenario's, and the caller's never is
// (callerPath).
func (w *world) noClaude() error {
	err := os.Remove(programPath(filepath.Join(w.support, "bin", "claude")))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Whether a run of the fake claude began with these arguments, as given
// says it must or must not have.
func (w *world) claudeGiven(args string, given bool) error {
	text, err := os.ReadFile(w.claudeRuns())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	want := strings.Fields(args)
	var runs []string
	found := false
	for _, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		if line == "" {
			continue
		}
		got := strings.Split(strings.TrimSuffix(line, "\t"), "\t")
		runs = append(runs, strings.Join(got, " "))
		if len(got) >= len(want) && slices.Equal(got[:len(want)], want) {
			found = true
		}
	}
	if found != given {
		how := "was not"
		if found {
			how = "was"
		}
		return fmt.Errorf("claude %s given %q; its runs: %q\n%s", how, args, runs, w.report())
	}
	return nil
}
