// The steps of itos init --policy (init.feature, slice 108): a value of the
// config init wrote compared with YAML, the fresh ledger and registry it
// wrote read back at the paths the policy chose, a source file compared with
// what its commit holds, and the folder that must stay no repository.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

func initializePolicyInitSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the output's JSON field "([^"]*)" is "([^"]*)"$`, w.jsonFieldIs)
	sc.Step(`^the config value "([^"]*)" equals the YAML:$`, func(path string, want *godog.DocString) error {
		return w.configValueIs(path, want.Content)
	})
	sc.Step(`^the config has no value at "([^"]*)"$`, func(path string) error {
		return w.configLacks(strings.Split(path, ".")...)
	})
	sc.Step(`^the ledger file "([^"]*)" contains only the adoption task "([^"]*)"$`, func(path, id string) error {
		return w.ledgerFileHolds(path, id)
	})
	sc.Step(`^the ledger file "([^"]*)" contains no tasks$`, func(path string) error { return w.ledgerFileHolds(path, "") })
	sc.Step(`^the registry file "([^"]*)" has no items and its group "([^"]*)" under "([^"]*)" is unowned$`,
		w.registryFileFresh)
	sc.Step(`^the file "([^"]*)" has the same contents as its committed version$`, w.fileAsCommitted)
	sc.Step(`^the folder is still not a git repository$`, w.folderIsNoRepository)
}

// The top-level field of the JSON object the last run printed is the text.
func (w *world) jsonFieldIs(field, want string) error {
	var object map[string]any
	if err := json.Unmarshal([]byte(w.stdout), &object); err != nil {
		return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	if got, ok := object[field]; !ok || fmt.Sprint(got) != want {
		return fmt.Errorf("the JSON's %q is %v, not %q\n%s", field, got, want, w.report())
	}
	return nil
}

// The config's value at a dotted key path equals the YAML, compared as
// values, not as text, so its quoting and flow style do not matter.
func (w *world) configValueIs(path, want string) error {
	config, text, err := w.foundConfig()
	if err != nil {
		return err
	}
	got, ok := at(config, strings.Split(path, ".")...)
	if !ok {
		return fmt.Errorf("the config has no %s:\n%s", path, text)
	}
	var expected any
	if err := yaml.Unmarshal([]byte(want), &expected); err != nil {
		return fmt.Errorf("the scenario's YAML does not parse: %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		return fmt.Errorf("the config's %s is %#v, not %#v:\n%s", path, got, expected, text)
	}
	return nil
}

// The ledger file, from the repository's top, is a YAML list of tasks: the
// one task with the ID, or none when the ID is "".
func (w *world) ledgerFileHolds(path, id string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("the ledger file %s cannot be read: %w\n%s", path, err, w.report())
	}
	var tasks []map[string]any
	if err := yaml.Unmarshal(text, &tasks); err != nil {
		return fmt.Errorf("the ledger file %s is not a list of tasks: %v\n%s", path, err, text)
	}
	switch {
	case id == "" && len(tasks) != 0:
		return fmt.Errorf("the ledger file %s holds %d tasks, not none:\n%s", path, len(tasks), text)
	case id != "" && (len(tasks) != 1 || fmt.Sprint(tasks[0]["id"]) != id):
		return fmt.Errorf("the ledger file %s does not hold only the task %s:\n%s", path, id, text)
	}
	return nil
}

// The registry file, from the repository's top, has no item, and under the
// key the group 1 (or another) is there with no owner.
func (w *world) registryFileFresh(path, group, key string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("the registry file %s cannot be read: %w\n%s", path, err, w.report())
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(text, &doc); err != nil || len(doc.Content) == 0 {
		return fmt.Errorf("the registry file %s is not YAML: %v\n%s", path, err, text)
	}
	root := doc.Content[0]
	if items := mappingValue(root, "items"); items != nil && (items.Kind != yaml.SequenceNode || len(items.Content) > 0) {
		return fmt.Errorf("the registry file %s has items:\n%s", path, text)
	}
	groups := mappingValue(root, key)
	if groups == nil || groups.Kind != yaml.MappingNode {
		return fmt.Errorf("the registry file %s has no %s mapping:\n%s", path, key, text)
	}
	owner := mappingValue(groups, group)
	if owner == nil || owner.Tag != "!!null" {
		return fmt.Errorf("the registry file %s has no unowned group %s under %s:\n%s", path, group, key, text)
	}
	return nil
}

// The file, from the repository's top, holds what HEAD's commit holds at
// that path.
func (w *world) fileAsCommitted(path string) error {
	committed, err := w.gitOutput("show", "HEAD:"+path)
	if err != nil {
		return fmt.Errorf("%s is not committed: %w", path, err)
	}
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w\n%s", path, err, w.report())
	}
	if string(text) != committed {
		return fmt.Errorf("%s differs from its committed version:\n%s\n--- committed\n%s", path, text, committed)
	}
	return nil
}

// The scenario's folder still has no git folder: nothing ran git init.
func (w *world) folderIsNoRepository() error {
	if _, err := os.Stat(filepath.Join(w.dir, ".git")); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the folder has a .git: %v\n%s", err, w.report())
	}
	return nil
}
