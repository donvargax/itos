// The steps of tests list's fingerprints (tests-list.feature, slice 166): feature files whose
// scenarios run steps with a table under a Background and feature tags, the edits that change
// what a scenario runs or leave it, and the listing's JSON read back for each test's fingerprint.
package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

func initializeTestsListSteps(sc *godog.ScenarioContext, w *world) {
	// The fingerprints the last "itos has listed" read, by ID; a test listed without one is
	// held as "".
	var before map[string]string

	sc.Step(`^the scenario kind has live IDs "([^"]*)" and "([^"]*)" in two feature files$`, func(a, b string) error {
		return w.liveScenarioFiles(a, b)
	})
	sc.Step(`^the scenario kind has the live outline "([^"]*)" with two Examples rows$`, w.liveOutline)
	sc.Step(`^a kind whose command adapter lists the live test "([^"]*)" in "([^"]*)"$`, w.commandKindListing)
	sc.Step(`^itos has listed the scenario kind's fingerprints$`, func() error {
		listed, err := w.listedFingerprints()
		before = listed
		return err
	})

	edits := map[string]func(id string) error{
		`a comment is added above the scenario "([^"]*)"`: w.editingScenario(func(text, id string) string {
			return strings.Replace(text, "  @"+id+"\n", "  # Why it is here.\n  @"+id+"\n", 1)
		}),
		`the description of the feature holding "([^"]*)" is reworded`: w.editingScenario(func(text, _ string) string {
			return strings.Replace(text, "  What it covers.\n", "  What it covers, put another way.\n", 1)
		}),
		`a @wip scenario is added to the feature file holding "([^"]*)"`: w.editingScenario(func(text, _ string) string {
			return text + "\n  @ID-WIP-01 @wip\n  Scenario: Not ready yet\n    When it is written\n"
		}),
		`the scenario "([^"]*)" is renamed`: w.editingScenario(func(text, id string) string {
			return strings.Replace(text, "Scenario: "+id+" runs\n", "Scenario: "+id+" runs, renamed\n", 1)
		}),
		`a step of the scenario "([^"]*)" is reworded`: w.editingScenario(func(text, _ string) string {
			return strings.Replace(text, "    Then it is done\n", "    Then it is done again\n", 1)
		}),
		`a table row of a step of the scenario "([^"]*)" is changed`: w.editingScenario(func(text, _ string) string {
			return strings.Replace(text, "      | one | 1     |\n", "      | one | 2     |\n", 1)
		}),
		`the Background of the feature holding "([^"]*)" gains a step`: w.editingScenario(func(text, _ string) string {
			return strings.Replace(text, "    Given a repository\n", "    Given a repository\n    And it is set up\n", 1)
		}),
		`the scenario "([^"]*)" gains the tag "@slow"`: w.editingScenario(func(text, id string) string {
			return strings.Replace(text, "  @"+id+"\n", "  @"+id+" @slow\n", 1)
		}),
		`the feature holding "([^"]*)" gains the tag "@slow"`: w.editingScenario(func(text, _ string) string {
			return strings.Replace(text, "@phase-1\nFeature:", "@phase-1 @slow\nFeature:", 1)
		}),
		`a row of the Examples of "([^"]*)" is changed`: w.editingScenario(func(text, _ string) string {
			return strings.Replace(text, "      | twice |\n", "      | again |\n", 1)
		}),
	}
	for pattern, edit := range edits {
		sc.Step(`^`+pattern+`$`, edit)
	}
	sc.Step(`^the scenario "([^"]*)" moves unchanged to the feature file of "([^"]*)"$`, w.moveScenarioTo)

	sc.Step(`^each listed test has a fingerprint of 64 lowercase hex digits$`, func() error {
		listed, err := w.listedTests()
		if err != nil {
			return err
		}
		if len(listed) == 0 {
			return fmt.Errorf("the listing has no test\n%s", w.report())
		}
		for _, t := range listed {
			if !fingerprintShape.MatchString(t.fingerprint) {
				return fmt.Errorf("%s has the fingerprint %q, not 64 lowercase hex digits\n%s", t.id, t.fingerprint, w.report())
			}
		}
		return nil
	})
	sc.Step(`^"([^"]*)" and "([^"]*)" have different fingerprints$`, func(a, b string) error {
		listed, err := w.listedTests()
		if err != nil {
			return err
		}
		byID := fingerprintsByID(listed)
		if byID[a] == byID[b] {
			return fmt.Errorf("%s and %s have the same fingerprint %q\n%s", a, b, byID[a], w.report())
		}
		return nil
	})
	sc.Step(`^the fingerprint of "([^"]*)" (is unchanged|has changed)$`, func(id, verdict string) error {
		was := before[id]
		after, err := w.listedFingerprints()
		if err != nil {
			return err
		}
		now := after[id]
		for _, f := range []string{was, now} {
			if !fingerprintShape.MatchString(f) {
				return fmt.Errorf("%s is listed with the fingerprint %q before and %q after, not 64 lowercase hex digits each\n%s",
					id, was, now, w.report())
			}
		}
		if (was == now) != (verdict == "is unchanged") {
			return fmt.Errorf("the fingerprint of %s was %s and is %s\n%s", id, was, now, w.report())
		}
		return nil
	})
	sc.Step(`^the listed test "([^"]*)" has no fingerprint$`, func(id string) error {
		var report struct {
			Tests []map[string]any `json:"tests"`
		}
		if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
			return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
		}
		for _, t := range report.Tests {
			if t["id"] != id {
				continue
			}
			if f, has := t["fingerprint"]; has {
				return fmt.Errorf("%s has the fingerprint %v\n%s", id, f, w.report())
			}
			return nil
		}
		return fmt.Errorf("the listing has no test %s\n%s", id, w.report())
	})
}

// fingerprintShape is a fingerprint as tests list gives it: a SHA-256 in lowercase hex.
var fingerprintShape = regexp.MustCompile(`^[0-9a-f]{64}$`)

// The text of a feature file whose one live scenario runs a step with a table and another after
// it, under a Background and a feature tag, with a description and a name of its own that the
// fingerprint leaves out.
func fingerprintedFeature(file, id string) string {
	return fmt.Sprintf(`@phase-1
Feature: %s
  What it covers.

  Background:
    Given a repository

  @%s
  Scenario: %s runs
    When it runs with
      | key | value |
      | one | 1     |
    Then it is done
`, file, id, id)
}

// Each scenario live in a feature file of its own under features/, in the working tree, which
// the config's scenario kind reads.
func (w *world) liveScenarioFiles(ids ...string) error {
	if w.scenarioFiles == nil {
		w.scenarioFiles = map[string]string{}
	}
	for _, id := range ids {
		file := strings.ToLower(id) + ".feature"
		w.scenarioFiles[id] = file
		if err := w.write(filepath.Join("features", file), fingerprintedFeature(file, id)); err != nil {
			return err
		}
	}
	w.config.scenarios = true
	return w.writeConfig()
}

// A feature file under features/ whose one live scenario is an outline with two Examples rows.
func (w *world) liveOutline(id string) error {
	if w.scenarioFiles == nil {
		w.scenarioFiles = map[string]string{}
	}
	file := strings.ToLower(id) + ".feature"
	w.scenarioFiles[id] = file
	text := fmt.Sprintf(`Feature: %s

  @%s
  Scenario Outline: %s runs <what>
    When it runs <what>

    Examples:
      | what  |
      | once  |
      | twice |
`, file, id, id)
	if err := w.write(filepath.Join("features", file), text); err != nil {
		return err
	}
	w.config.scenarios = true
	return w.writeConfig()
}

// A kind, fixture, beside the scenario kind, whose command adapter lists the one live test in
// the file, the scenario kind holding a live scenario of its own (for its text listing).
func (w *world) commandKindListing(id, file string) error {
	listing, err := json.Marshal(map[string]any{
		"protocol": 1, "files": []string{file},
		"tests": []map[string]any{{"id": id, "file": file, "live": true}},
	})
	if err != nil {
		return err
	}
	script := filepath.Join(w.support, "fixture-adapter.sh")
	if err := os.WriteFile(script, []byte("printf '%s\\n' "+quote(string(listing))+"\n"), 0o644); err != nil {
		return err
	}
	w.config.commandKind = &commandKind{name: "fixture", wipTag: "@wip", command: "sh " + quote(filepath.ToSlash(script))}
	return w.liveScenarioFiles("ID-A-01")
}

// An edit of the working tree's feature file that holds the scenario, as a step's function of
// the scenario's ID.
func (w *world) editingScenario(edit scenarioEdit) func(id string) error {
	return func(id string) error {
		_, err := w.editScenario(id, edit)
		return err
	}
}

// The scenario's block, from its tag line to its file's end, taken out of its file, which is
// deleted, and written unchanged at the end of the other scenario's file.
func (w *world) moveScenarioTo(id, other string) error {
	from, ok := w.scenarioFiles[id]
	to, found := w.scenarioFiles[other]
	if !ok || !found {
		return fmt.Errorf("no feature file has the scenario %s or %s", id, other)
	}
	text, err := os.ReadFile(filepath.Join(w.dir, "features", from))
	if err != nil {
		return err
	}
	at := strings.Index(string(text), "  @"+id+"\n")
	if at < 0 {
		return fmt.Errorf("%s has no tag line of %s", from, id)
	}
	target, err := os.ReadFile(filepath.Join(w.dir, "features", to))
	if err != nil {
		return err
	}
	if err := w.write(filepath.Join("features", to), string(target)+"\n"+string(text)[at:]); err != nil {
		return err
	}
	w.scenarioFiles[id] = to
	return os.Remove(filepath.Join(w.dir, "features", from))
}

// One test of the listing's JSON: its ID and its fingerprint, "" when it has none.
type fingerprinted struct{ id, fingerprint string }

// The tests of the last run's JSON listing, an error when it exited otherwise than 0 or printed
// no listing.
func (w *world) listedTests() ([]fingerprinted, error) {
	if w.exit != 0 {
		return nil, fmt.Errorf("itos exited %d\n%s", w.exit, w.report())
	}
	var report struct {
		Tests []struct {
			ID          string `json:"id"`
			Fingerprint string `json:"fingerprint"`
		} `json:"tests"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
		return nil, fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	listed := []fingerprinted{}
	for _, t := range report.Tests {
		listed = append(listed, fingerprinted{t.ID, t.Fingerprint})
	}
	return listed, nil
}

// The scenario kind's fingerprints as tests list --json gives them now, by ID.
func (w *world) listedFingerprints() (map[string]string, error) {
	if err := w.itos("tests", "list", "scenario", "--json"); err != nil {
		return nil, err
	}
	listed, err := w.listedTests()
	return fingerprintsByID(listed), err
}

// Each listed test's fingerprint by its ID.
func fingerprintsByID(listed []fingerprinted) map[string]string {
	byID := map[string]string{}
	for _, t := range listed {
		byID[t.id] = t.fingerprint
	}
	return byID
}
