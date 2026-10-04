// The steps of itos work show (show.feature, slice 57): a feature file whose
// one live scenario is tagged for an item, committed; a commit naming
// scenarios in its Scenarios footer, or an item in its Item footer (slice
// 63); one text of the output before another; and an entry of a list in the
// JSON output.
package features

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func initializeShowSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the committed feature file "([^"]*)" with the live scenario "([^"]*)" tagged "([^"]*)"$`, w.committedTaggedScenario)
	sc.Step(`^the commit "([^"]*)" naming the scenarios "([^"]*)" on top of it$`, func(message, ids string) error {
		return w.commitOnTop(message + "\n\nScenarios: " + ids + "\n")
	})
	sc.Step(`^the commit "([^"]*)" naming the item "([^"]*)" in an Item footer on top of it$`, func(message, id string) error {
		return w.commitOnTop(message + "\n\nItem: " + id + "\n")
	})
	sc.Step(`^its output says "([^"]*)" before "([^"]*)"$`, w.outputSaysBefore)
	sc.Step(`^its JSON's "([^"]*)" has one entry whose "([^"]*)" is "([^"]*)"$`, w.jsonListHasOne)
}

// A feature file at the path, from the repository's top, of one live
// scenario whose tag line holds its ID and the tag, committed with a message
// that names neither; the config gains the kind tests.scenario, which reads
// features/.
func (w *world) committedTaggedScenario(path, id, tag string) error {
	w.config.scenarios = true
	if err := w.writeConfig(); err != nil {
		return err
	}
	text := fmt.Sprintf("Feature: Tagged\n\n  %s %s\n  Scenario: %s runs\n    When it runs\n", id, tag, path)
	if err := w.write(path, text); err != nil {
		return err
	}
	return w.commit("test: specify " + path)
}

// The output says the text, and says the later one after it.
func (w *world) outputSaysBefore(text, later string) error {
	output := w.output()
	at := strings.Index(output, text)
	if at < 0 {
		return fmt.Errorf("the output does not say %q\n%s", text, w.report())
	}
	if !strings.Contains(output[at+len(text):], later) {
		return fmt.Errorf("the output does not say %q after %q\n%s", later, text, w.report())
	}
	return nil
}

// Standard output is one JSON object whose list under the key has one
// entry, and one only, whose field is the value.
func (w *world) jsonListHasOne(key, field, want string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(w.stdout), &object); err != nil {
		return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	var entries []map[string]any
	if err := json.Unmarshal(object[key], &entries); err != nil {
		return fmt.Errorf("the JSON's %q is not a list of objects: %v\n%s", key, err, w.report())
	}
	found := 0
	for _, entry := range entries {
		if fmt.Sprint(entry[field]) == want {
			found++
		}
	}
	if found != 1 {
		return fmt.Errorf("the JSON's %q has %d entries whose %q is %q, not one\n%s", key, found, field, want, w.report())
	}
	return nil
}
