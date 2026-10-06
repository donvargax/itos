// The steps of the built-in moves rule on a kind whose adapter is a command
// (moves-command.feature, slice 82): the kind's adapter, a script in the
// support folder that prints the protocol's object a file of the repository
// holds at the tree it is asked for, supporting at or not; and that file,
// tests.json, committed or staged with the tests a scenario lists.
package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// The kind's command adapter: the file of the repository it lists the tests
// of, and its supports_at.
type commandAdapter struct {
	file       string
	supportsAt bool
}

// One test of tests.json: its ID as the scenario writes it (its tag), its
// title ("" for none) and whether it is live.
type listedTest struct {
	id, title string
	live      bool
}

func initializeMovesCommandSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the kind's adapter is a command listing the tests of "([^"]*)", supporting at$`, w.commandAdapterListing)
	sc.Step(`^the kind's adapter does not support at$`, w.adapterDoesNotSupportAt)
	sc.Step(`^the committed tests\.json lists the live test "([^"]*)" titled "([^"]*)"$`, func(id, title string) error {
		if err := w.writeTestsJSON(listedTest{id, title, true}); err != nil {
			return err
		}
		return w.commit("feat: add the test " + id)
	})
	sc.Step(`^tests\.json is staged listing the test "([^"]*)" titled "([^"]*)" as wip$`, func(id, title string) error {
		return w.stageTestsJSON(listedTest{id, title, false})
	})
	sc.Step(`^tests\.json is staged listing the live test "([^"]*)" titled "([^"]*)"$`, func(id, title string) error {
		return w.stageTestsJSON(listedTest{id, title, true})
	})
	sc.Step(`^tests\.json is staged listing the live test "([^"]*)" titled "([^"]*)" and the wip test "([^"]*)"$`,
		func(id, title, wip string) error {
			return w.stageTestsJSON(listedTest{id, title, true}, listedTest{wip, "", false})
		})
}

// The adapter: `<command> list --at <tree>` prints the file as the working
// tree, the index or the commit holds it, which is the protocol's object
// itself, or an empty list where the tree has no such file. The config's
// kind tests.scenario gains it, with supports_at: true.
func (w *world) commandAdapterListing(file string) error {
	script := fmt.Sprintf(`# The protocol's list of the tests %[1]s holds at the tree.
[ "$1" = list ] && [ "$2" = --at ] || { echo "usage: list --at <tree>" >&2; exit 2; }
case "$3" in
worktree) cat %[1]s 2>/dev/null ;;
index) git show :%[1]s 2>/dev/null ;;
*) git show "$3":%[1]s 2>/dev/null ;;
esac || printf '%%s\n' '{"protocol":1,"tests":[],"files":[]}'
`, quote(file))
	if err := os.WriteFile(filepath.Join(w.support, "adapter.sh"), []byte(script), 0o644); err != nil {
		return err
	}
	w.config.adapter = &commandAdapter{file: file, supportsAt: true}
	return w.writeConfig()
}

func (w *world) adapterDoesNotSupportAt() error {
	if w.config.adapter == nil {
		return fmt.Errorf("the kind's adapter is not a command")
	}
	w.config.adapter.supportsAt = false
	return w.writeConfig()
}

// The adapter's command, as the config writes it.
func (w *world) adapterCommand() string {
	return "sh " + quote(filepath.ToSlash(filepath.Join(w.support, "adapter.sh")))
}

// tests.json at the repository's top: the protocol's object listing the
// tests, each in tests.json, its ID without the tag's @ (the protocol's ID
// has no tag prefix) and its title only when it has one.
func (w *world) writeTestsJSON(tests ...listedTest) error {
	listed := []map[string]any{}
	for _, t := range tests {
		entry := map[string]any{"id": strings.TrimPrefix(t.id, "@"), "file": "tests.json", "live": t.live}
		if t.title != "" {
			entry["title"] = t.title
		}
		listed = append(listed, entry)
	}
	text, err := json.Marshal(map[string]any{"protocol": 1, "tests": listed, "files": []string{"tests.json"}})
	if err != nil {
		return err
	}
	return w.write("tests.json", string(text)+"\n")
}

func (w *world) stageTestsJSON(tests ...listedTest) error {
	if err := w.writeTestsJSON(tests...); err != nil {
		return err
	}
	return w.git("add", "tests.json")
}
