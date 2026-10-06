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
	sc.Step(`^itos's "([^"]*)" hook is taken out of the git config$`, func(event string) error {
		return w.git("config", "--local", "--remove-section", "hook.itos-"+event)
	})
	sc.Step(`^the git config declares no "([^"]*)" hook$`, w.declaresNoHook)
	sc.Step(`^a claude on the PATH that records its arguments$`, func() error { return w.fakeClaude("", false) })
	sc.Step(`^a claude on the PATH that records its arguments, writing \.claude/settings\.local\.json as claude does$`,
		func() error { return w.fakeClaude("", true) })
	sc.Step(`^a claude on the PATH that lists the plugin "([^"]*)" as installed$`, func(id string) error { return w.fakeClaude(id, false) })
	sc.Step(`^no claude on the PATH$`, w.noClaude)

	sc.Step(`^the files init wrote are committed$`, w.initCommitted)
	sc.Step(`^the folder is a git repository$`, w.folderIsRepository)
	sc.Step(`^the config's commits\.since is HEAD's full SHA$`, w.sinceIsHead)
	sc.Step(`^the config has no commits\.since$`, func() error { return w.configLacks("commits", "since") })
	sc.Step(`^the config has no pin$`, func() error { return w.configLacks("pin") })
	sc.Step(`^`+noFileChanged+`$`, w.noFileChanged)
	sc.Step(`^the file "([^"]*)" does not exist$`, w.fileDoesNotExist)
	sc.Step(`^claude was given "([^"]*)"$`, func(args string) error { return w.claudeGiven(args, true) })
	sc.Step(`^claude was not given "([^"]*)"$`, func(args string) error { return w.claudeGiven(args, false) })

	sc.Step(`^the config's commits\.types gains "([^"]*)"$`, w.typesGain)
	sc.Step(`^the file "([^"]*)" says "([^"]*)" between the markers$`, func(path, text string) error {
		return w.saysBetweenMarkers(path, text, true)
	})
	sc.Step(`^the file "([^"]*)" does not say "([^"]*)" between the markers$`, func(path, text string) error {
		return w.saysBetweenMarkers(path, text, false)
	})
	sc.Step(`^the untracked file "([^"]*)" holding "([^"]*)"$`, w.untrackedFileHolding)
	sc.Step(`^the file "([^"]*)" names "([^"]*)"$`, w.fileNames)
}

// The file at the path, from the repository's top, holding the text as one
// line, never committed nor ignored: a file of the person's own that git
// shows untracked.
func (w *world) untrackedFileHolding(path, text string) error {
	return w.write(path, text+"\n")
}

// The file, from the repository's top, is there and names the text
// somewhere, a path say, whatever the text around it.
func (w *world) fileNames(path, name string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w\n%s", path, err, w.report())
	}
	if !strings.Contains(string(text), name) {
		return fmt.Errorf("%s does not name %q:\n%s", path, name, text)
	}
	return nil
}

// A repository with one commit, a README, and nothing of itos: no config in
// the root or in the git folder, no ledger, no hooks.
func (w *world) repositoryWithoutItos(message string) error {
	if err := w.undeclareHooks(w.dir); err != nil {
		return err
	}
	if err := w.write("README.md", "# Scratch\n"); err != nil {
		return err
	}
	return w.commit(message)
}

// What init wrote, committed as the adoption its ledger's T-1 is for, past
// the hooks it installed: the scenario is about what comes after.
// The adoption commit, made with no hook, and from then on the itos under
// test on the PATH as itos, so the hooks init installed, which call itos as
// its starter's hooks.bin says, run it and not whatever itos the caller has.
func (w *world) initCommitted() error {
	if err := w.commit("chore: adopt itos\n\nTask: T-1"); err != nil {
		return err
	}
	return w.itosOnPath()
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

// No entry of the repository's own git config is a hook for the event.
func (w *world) declaresNoHook(event string) error {
	out, _ := w.gitOutput("config", "--local", "--get-regexp", `^hook\..*\.event$`)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if _, value, _ := strings.Cut(line, " "); value == event {
			return fmt.Errorf("the git config declares a %s hook:\n%s\n%s", event, out, w.report())
		}
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

// The config itos finds gains a commit type at the end of commits.types,
// written back in place, uncommitted: the config changed after the last run.
func (w *world) typesGain(typ string) error {
	path := filepath.Join(w.dir, "itos.yaml")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(w.dir, stealthDir, "itos.yaml")
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("there is no config: %w\n%s", err, w.report())
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(text, &doc); err != nil || len(doc.Content) == 0 {
		return fmt.Errorf("the config is not YAML: %v\n%s", err, text)
	}
	types := mappingValue(mappingValue(doc.Content[0], "commits"), "types")
	if types == nil || types.Kind != yaml.SequenceNode {
		return fmt.Errorf("the config has no commits.types list:\n%s", text)
	}
	types.Content = append(types.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: typ})
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// The value of a key of a YAML mapping, nil when it is no mapping or has no
// such key.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// The markers itos writes its block of a project file between, each a line
// of its own.
const (
	beginMarker = "<!-- itos:begin -->"
	endMarker   = "<!-- itos:end -->"
)

// Whether the file's text between its markers, a line that is the begin
// marker and the first after it that is the end one, holds the text, as says
// it must or must not: a file without both markers has no block, which is no
// proof of what it would leave out, so it fails either way.
func (w *world) saysBetweenMarkers(path, text string, says bool) error {
	data, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("the file %s cannot be read: %w\n%s", path, err, w.report())
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	is := func(marker string) func(string) bool {
		return func(l string) bool { return strings.TrimSpace(l) == marker }
	}
	begin := slices.IndexFunc(lines, is(beginMarker))
	end := -1
	if begin >= 0 {
		if at := slices.IndexFunc(lines[begin+1:], is(endMarker)); at >= 0 {
			end = begin + 1 + at
		}
	}
	if end < 0 {
		return fmt.Errorf("the file %s has no %s line with an %s line after it:\n%s", path, beginMarker, endMarker, data)
	}
	block := strings.Join(lines[begin+1:end], "\n")
	switch found := strings.Contains(block, text); {
	case found && !says:
		return fmt.Errorf("the file %s says %q between its markers:\n%s", path, text, block)
	case !found && says:
		return fmt.Errorf("the file %s does not say %q between its markers:\n%s", path, text, block)
	}
	return nil
}
