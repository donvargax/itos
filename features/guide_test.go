// The steps of itos go and itos guide (guide.feature, slice 64): a file
// committed with the text it holds, as a repository keeps its own
// orchestrating notes or its decision records (config.feature, slice 74), and the config key that names another such file; and
// this clone's own notes (slice 68), kept uncommitted in the git common dir.
package features

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func initializeGuideSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the committed file "([^"]*)" holding "([^"]*)"$`, w.committedFileHolding)
	sc.Step(`^the committed file "([^"]*)" holding the lines:$`, func(path string, lines *godog.DocString) error {
		return w.committedFileHolding(path, lines.Content)
	})
	sc.Step(`^guide\.orchestrating is "([^"]*)"$`, func(path string) error {
		return w.configSets("guide.orchestrating", path)
	})
	sc.Step(`^this clone's own notes holding "([^"]*)"$`, w.localNotesHolding)
}

// The file at the path, from the repository's top, holding the text as one
// line, or a doc string's lines (a decision record, slice 74), committed with everything else the scenario has written so far.
func (w *world) committedFileHolding(path, text string) error {
	if err := w.write(path, text+"\n"); err != nil {
		return err
	}
	return w.commit("docs: add " + path)
}

// This clone's own notes, itos/notes.md in the git common dir as git names it
// for the scratch repository, holding the text as one line, never committed.
func (w *world) localNotesHolding(text string) error {
	common, err := w.gitOutput("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	file := filepath.Join(strings.TrimSpace(common), "itos", "notes.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(text+"\n"), 0o644)
}
