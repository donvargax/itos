// The steps of the commands that write the work registry (work.feature,
// slices 52 and 53): a registry of items with owners, dependencies and
// ideas, committed, so a command's own commit holds only its change; a
// scenario tagged for an item; and what the registry reads after, and what
// the last commit holds.
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
	sc.Step(`^a feature file with the scenario "([^"]*)" tagged "([^"]*)"$`, w.taggedScenario)

	sc.Step(`^the registry's item "([^"]*)" has the status "([^"]*)" and the owner "([^"]*)"$`, w.registryItemIs)
	sc.Step(`^the work registry beside the config gives the item "([^"]*)" the status "([^"]*)"$`, func(id, status string) error {
		return w.registryItemIs(id, status, "")
	})
	sc.Step(`^the registry has no item "([^"]*)"$`, w.registryLacks)
	sc.Step(`^the registry's item "([^"]*)" is a (slice|task) whose why starts with "([^"]*)"$`, w.registryItemKind)
	sc.Step(`^the registry's item "([^"]*)" depends on "([^"]*)"$`, w.registryItemDependsOn)
	sc.Step(`^the last commit's header is "([^"]*)"$`, w.lastHeaderIs)
	sc.Step(`^the last commit touches only "([^"]*)"$`, w.lastTouchesOnly)
	sc.Step(`^"([^"]*)" is still staged$`, w.stillStaged)
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
	if err := w.write(w.data(startingRegistry), "phases: { 1: null }\nitems:\n"+strings.Join(w.registryLines, "")); err != nil {
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

// The item has the status and, when one is given, the owner.
func (w *world) registryItemIs(id, status, owner string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	if item["status"] != status {
		return fmt.Errorf("%s's status is %v, not %s\n%s", id, item["status"], status, w.report())
	}
	if owner != "" && item["owner"] != owner {
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

func (w *world) lastTouchesOnly(path string) error {
	out, err := w.gitOutput("diff-tree", "--no-commit-id", "--name-only", "-r", "--root", "HEAD")
	if err != nil {
		return err
	}
	if got := strings.Fields(out); !slices.Equal(got, []string{path}) {
		return fmt.Errorf("the last commit touches %v, not only %s\n%s", got, path, w.report())
	}
	return nil
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
