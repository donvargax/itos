// The steps of the commands that write the work registry (work.feature,
// slices 52 to 54) and the ledger (task add, task.feature, slice 55): a
// registry of items with owners, dependencies and ideas, committed, so a
// command's own commit holds only its change; a scenario tagged for an
// item; a ledger file deleted and a commit-msg hook that refuses, for a
// commit that cannot be made (bug 13); and what the registry reads after
// (an item's status, owner, kind, title, dependencies and why), a ledger
// file's task, what the last commit holds, and what git status reports; an
// idea whose refs are a flow list over several lines or a block list, and
// the refs an item has after (bug 14).
package features

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

func initializeWorkSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the work registry has the item "([^"]*)" owned by nobody with the status "([^"]*)"$`, func(id, status string) error {
		return w.registryItem(id, "null", status, "", "")
	})
	sc.Step(`^the work registry has the item "([^"]*)" owned by "([^"]*)" with the status "([^"]*)"$`, func(id, owner, status string) error {
		return w.registryItem(id, owner, status, "", "")
	})
	sc.Step(`^the work registry has the item "([^"]*)" owned by nobody with the status "([^"]*)", depending on "([^"]*)"$`, func(id, status, dep string) error {
		return w.registryItem(id, "null", status, dep, "")
	})
	sc.Step(`^the work registry has the idea "([^"]*)" owned by nobody$`, func(id string) error {
		return w.registryItem(id, "null", "todo", "", "idea")
	})
	sc.Step(`^the work registry has the idea "([^"]*)" owned by nobody, its refs a flow list over several lines$`, func(id string) error {
		return w.registryIdeaWithRefs(id, "\n      [\n        features/x.feature,\n        docs/y.md,\n      ]\n")
	})
	sc.Step(`^the work registry has the idea "([^"]*)" owned by nobody, its refs a block list of "([^"]*)" and "([^"]*)"$`, func(id, a, b string) error {
		return w.registryIdeaWithRefs(id, fmt.Sprintf("\n      - %s\n      - %s\n", a, b))
	})
	sc.Step(`^a feature file with the scenario "([^"]*)" tagged "([^"]*)"$`, w.taggedScenario)

	sc.Step(`^the registry's item "([^"]*)" has the status "([^"]*)" and the owner "([^"]*)"$`, w.registryItemIs)
	sc.Step(`^the work registry beside the config gives the item "([^"]*)" the status "([^"]*)"$`, func(id, status string) error {
		return w.registryItemIs(id, status, "")
	})
	sc.Step(`^the registry has no item "([^"]*)"$`, w.registryLacks)
	sc.Step(`^the registry's item "([^"]*)" is a (slice|task) whose why starts with "([^"]*)"$`, w.registryItemKind)
	sc.Step(`^the registry's item "([^"]*)" depends on "([^"]*)"$`, w.registryItemDependsOn)
	sc.Step(`^the registry's item "([^"]*)" is an? (idea|slice|task) titled "([^"]*)" with the status "([^"]*)"$`, w.registryItemMade)
	sc.Step(`^the registry's item "([^"]*)" is titled "([^"]*)"$`, func(id, title string) error {
		return w.registryItemField(id, "title", title)
	})
	sc.Step(`^the registry's item "([^"]*)" has a why ending with "([^"]*)"$`, w.registryWhyEnds)
	sc.Step(`^the registry's item "([^"]*)" has the refs "([^"]*)"$`, w.registryItemRefs)
	sc.Step(`^the registry's item "([^"]*)" has no refs$`, func(id string) error { return w.registryItemRefs(id, "") })
	sc.Step(`^the last commit's header is "([^"]*)"$`, w.lastHeaderIs)
	sc.Step(`^the last commit's header is not "([^"]*)"$`, w.lastHeaderIsNot)
	sc.Step(`^the last commit touches only "([^"]*)"$`, func(path string) error { return w.lastTouchesOnly(path) })
	sc.Step(`^the last commit touches only "([^"]*)" and "([^"]*)"$`, func(a, b string) error { return w.lastTouchesOnly(a, b) })
	sc.Step(`^the ledger file "([^"]*)" has the task "([^"]*)" with the check "([^"]*)"$`, w.ledgerFileHasTask)
	sc.Step(`^"([^"]*)" is still staged$`, w.stillStaged)

	sc.Step(`^the ledger file "([^"]*)" is deleted and the deletion not committed$`, w.ledgerFileDeleted)
	sc.Step(`^a commit-msg hook that refuses every commit$`, w.refusingCommitMsgHook)
	sc.Step(`^the ledger has no task "([^"]*)"$`, w.ledgerLacks)
	sc.Step(`^git reports no change to the working tree or the index$`, w.gitStatusClean)
}

// One more item of phase 1, a group nobody owns, in the registry where the
// scenario keeps itos's data: its owner (null for nobody), status, one
// dependency when given and a kind when given. An owner is added to the
// people, so the registry stays sound, and in a project the registry and the
// people are committed past the hooks, so a command that commits the
// registry commits only its own change.
func (w *world) registryItem(id, owner, status, dep, kind string) error {
	deps := "[]"
	if dep != "" {
		deps = "[" + dep + "]"
	}
	line := fmt.Sprintf("  - { id: %s, title: %s, phase: 1, owner: %s, status: %s", id, id, owner, status)
	if kind != "" {
		line += ", kind: " + kind
	}
	w.registryLines = append(w.registryLines, line+", depends_on: "+deps+" }\n")
	if err := w.writeRegistryLines(); err != nil {
		return err
	}
	if owner != "null" {
		people := filepath.Join(w.dir, w.data("people.yaml"))
		text, err := os.ReadFile(people)
		if err != nil {
			return err
		}
		if !slices.Contains(strings.Split(string(text), "\n"), "- "+owner) {
			if err := os.WriteFile(people, append(text, []byte("- "+owner+"\n")...), 0o644); err != nil {
				return err
			}
		}
	}
	if w.dataDir != "" {
		return nil
	}
	return w.commit("docs: a registry")
}

// The registry's lines written where the scenario keeps itos's data, its
// one phase before them.
func (w *world) writeRegistryLines() error {
	return w.write(w.data(startingRegistry), "phases: { 1: null }\nitems:\n"+strings.Join(w.registryLines, ""))
}

// One more idea of phase 1, todo and owned by nobody, written as a block
// mapping whose last key is refs, its value the text after the colon (bug
// 14: a flow list the formatter wrapped over several lines, or a block
// list), committed as registryItem commits.
func (w *world) registryIdeaWithRefs(id, refs string) error {
	w.registryLines = append(w.registryLines, fmt.Sprintf(
		"  - id: %s\n    title: %s\n    phase: 1\n    owner: null\n    status: todo\n    kind: idea\n    depends_on: []\n    refs:%s",
		id, id, refs))
	if err := w.writeRegistryLines(); err != nil {
		return err
	}
	if w.dataDir != "" {
		return nil
	}
	return w.commit("docs: a registry")
}

// The item's refs are the comma-separated ones, in order; none ("") is no
// refs key, null or an empty list.
func (w *world) registryItemRefs(id, refs string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	got := []string{}
	if list, ok := item["refs"].([]any); ok {
		for _, r := range list {
			got = append(got, fmt.Sprint(r))
		}
	} else if item["refs"] != nil {
		return fmt.Errorf("%s's refs are %v, not a list\n%s", id, item["refs"], w.report())
	}
	want := []string{}
	if refs != "" {
		want = strings.Split(refs, ",")
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("%s's refs are %q, not %q\n%s", id, got, want, w.report())
	}
	return nil
}

// The registry's items, read from where the scenario keeps itos's data.
func (w *world) registryItems() ([]map[string]any, error) {
	text, err := os.ReadFile(filepath.Join(w.dir, w.data(startingRegistry)))
	if err != nil {
		return nil, err
	}
	var registry struct {
		Items []map[string]any `yaml:"items"`
	}
	if err := yaml.Unmarshal(text, &registry); err != nil {
		return nil, fmt.Errorf("the registry does not read: %w\n%s", err, text)
	}
	return registry.Items, nil
}

// The registry's item with the id, an error naming the registry when it has
// none.
func (w *world) registryItemOf(id string) (map[string]any, error) {
	items, err := w.registryItems()
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if fmt.Sprint(item["id"]) == id {
			return item, nil
		}
	}
	return nil, fmt.Errorf("the registry has no item %q\n%s", id, w.report())
}

// The item has the status and, when one is given, the owner (nobody for
// none).
func (w *world) registryItemIs(id, status, owner string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	if item["status"] != status {
		return fmt.Errorf("%s's status is %v, not %s\n%s", id, item["status"], status, w.report())
	}
	// nobody is an owner of null, as the Given steps write it.
	if owner == "nobody" && item["owner"] != nil || owner != "" && owner != "nobody" && item["owner"] != owner {
		return fmt.Errorf("%s's owner is %v, not %s\n%s", id, item["owner"], owner, w.report())
	}
	return nil
}

func (w *world) registryLacks(id string) error {
	items, err := w.registryItems()
	if err != nil {
		return err
	}
	for _, item := range items {
		if fmt.Sprint(item["id"]) == id {
			return fmt.Errorf("the registry still has the item %q\n%s", id, w.report())
		}
	}
	return nil
}

func (w *world) registryItemKind(id, kind, start string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	if item["kind"] != kind {
		return fmt.Errorf("%s's kind is %v, not %s\n%s", id, item["kind"], kind, w.report())
	}
	why, _ := item["why"].(string)
	if !strings.HasPrefix(why, start) {
		return fmt.Errorf("%s's why is %q, which does not start with %q\n%s", id, why, start, w.report())
	}
	return nil
}

func (w *world) registryItemDependsOn(id, dep string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	deps, _ := item["depends_on"].([]any)
	for _, d := range deps {
		if fmt.Sprint(d) == dep {
			return nil
		}
	}
	return fmt.Errorf("%s depends on %v, not %s\n%s", id, deps, dep, w.report())
}

func (w *world) lastHeaderIs(header string) error {
	out, err := w.gitOutput("log", "-1", "--format=%s")
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(out); got != header {
		return fmt.Errorf("the last commit's header is %q, not %q\n%s", got, header, w.report())
	}
	return nil
}

func (w *world) lastHeaderIsNot(header string) error {
	out, err := w.gitOutput("log", "-1", "--format=%s")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == header {
		return fmt.Errorf("the last commit's header is %q\n%s", header, w.report())
	}
	return nil
}

// The last commit touches the paths and nothing else, in any order.
func (w *world) lastTouchesOnly(paths ...string) error {
	out, err := w.gitOutput("diff-tree", "--no-commit-id", "--name-only", "-r", "--root", "HEAD")
	if err != nil {
		return err
	}
	got, want := strings.Fields(out), slices.Clone(paths)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return fmt.Errorf("the last commit touches %v, not only %v\n%s", got, paths, w.report())
	}
	return nil
}

// The ledger file, read where the scenario keeps itos's data, has the task,
// one of whose checks runs the command (run:).
func (w *world) ledgerFileHasTask(file, id, command string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, w.data(file)))
	if err != nil {
		return err
	}
	var tasks []struct {
		ID       string `yaml:"id"`
		DoneWhen []struct {
			Run string `yaml:"run"`
		} `yaml:"done_when"`
	}
	if err := yaml.Unmarshal(text, &tasks); err != nil {
		return fmt.Errorf("%s does not read: %w\n%s", file, err, text)
	}
	for _, t := range tasks {
		if t.ID != id {
			continue
		}
		for _, c := range t.DoneWhen {
			if c.Run == command {
				return nil
			}
		}
		return fmt.Errorf("%s's task %s has no check that runs %q\n%s\n%s", file, id, command, text, w.report())
	}
	return fmt.Errorf("%s has no task %s\n%s\n%s", file, id, text, w.report())
}

func (w *world) stillStaged(path string) error {
	out, err := w.gitOutput("diff", "--cached", "--name-only")
	if err != nil {
		return err
	}
	if !slices.Contains(strings.Fields(out), path) {
		return fmt.Errorf("%s is not staged; the staged files are %q\n%s", path, out, w.report())
	}
	return nil
}

// A feature file of one scenario, its tag line the scenario's ID and the
// tags, committed, so a command reading the scenarios at HEAD finds it; the
// config gains the kind tests.scenario, which reads features/.
func (w *world) taggedScenario(id, tags string) error {
	w.config.scenarios = true
	if err := w.writeConfig(); err != nil {
		return err
	}
	text := fmt.Sprintf("Feature: Tagged\n\n  %s %s\n  Scenario: %s runs\n    When it runs\n", id, tags, id)
	if err := w.write(filepath.Join("features", "tagged.feature"), text); err != nil {
		return err
	}
	return w.commit("test: a tagged scenario")
}

// The item is of the kind, titled so, with the status.
func (w *world) registryItemMade(id, kind, title, status string) error {
	if err := w.registryItemField(id, "kind", kind); err != nil {
		return err
	}
	if err := w.registryItemField(id, "title", title); err != nil {
		return err
	}
	return w.registryItemField(id, "status", status)
}

// The item's key reads as the text.
func (w *world) registryItemField(id, key, want string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(item[key]); got != want {
		return fmt.Errorf("%s's %s is %q, not %q\n%s", id, key, got, want, w.report())
	}
	return nil
}

// The item's why ends with the text, its last line break aside.
func (w *world) registryWhyEnds(id, end string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	why, _ := item["why"].(string)
	if !strings.HasSuffix(strings.TrimRight(why, "\n"), end) {
		return fmt.Errorf("%s's why is %q, which does not end with %q\n%s", id, why, end, w.report())
	}
	return nil
}

// The ledger file, where the scenario keeps itos's data, removed from the
// working tree: git sees it deleted, the deletion neither staged nor
// committed.
func (w *world) ledgerFileDeleted(file string) error {
	return os.Remove(filepath.Join(w.dir, w.data(file)))
}

// A commit-msg hook in git's own hooks folder that says it refuses and exits
// 1, so every commit that runs the hooks fails.
func (w *world) refusingCommitMsgHook() error {
	out, err := w.gitOutput("rev-parse", "--git-path", "hooks")
	if err != nil {
		return err
	}
	dir := strings.TrimSpace(out)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(w.dir, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "commit-msg"), []byte("#!/bin/sh\necho 'the hook refuses every commit' >&2\nexit 1\n"), 0o755)
}

// No file of the ledger, every one ledger.files names in its folder, has the
// task.
func (w *world) ledgerLacks(id string) error {
	files, err := filepath.Glob(filepath.Join(w.dir, w.data(strings.ReplaceAll(w.ledgerFilesPattern(), "{group}", "*"))))
	if err != nil {
		return err
	}
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var tasks []map[string]any
		if err := yaml.Unmarshal(text, &tasks); err != nil {
			return fmt.Errorf("%s does not read: %w\n%s", file, err, text)
		}
		for _, t := range tasks {
			if fmt.Sprint(t["id"]) == id {
				return fmt.Errorf("%s still has the task %s\n%s\n%s", file, id, text, w.report())
			}
		}
	}
	return nil
}

// git status lists nothing: no file changed, staged or untracked.
func (w *world) gitStatusClean() error {
	out, err := w.gitOutput("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("git status reports:\n%s\n%s", out, w.report())
	}
	return nil
}
