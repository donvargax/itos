// The steps of itos follow (follow.feature, slice 61): a command line that
// has to succeed before the run a scenario is about, in the repository or in
// its linked worktree, and a file of the repository that exists or says a
// text.
package features

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func initializeFollowSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^itos has run the command line "([^"]*)"$`, func(line string) error {
		return w.hasRunLine(w.dir, line)
	})
	sc.Step(`^itos has run the command line "([^"]*)" in the linked worktree$`, func(line string) error {
		if w.linked == "" {
			return fmt.Errorf("the scenario adds no linked worktree")
		}
		return w.hasRunLine(w.linked, line)
	})
	sc.Step(`^the file "([^"]*)" exists$`, w.fileExists)
	sc.Step(`^the file "([^"]*)" says "([^"]*)"$`, w.fileSays)
}

// The command line, split as a shell splits it, run in the folder dir, which
// has to succeed: it sets up what the scenario is about.
func (w *world) hasRunLine(dir, line string) error {
	args, err := shellWords(line)
	if err != nil {
		return err
	}
	if err := w.itosIn(dir, args...); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("itos %s exited %d before the run the scenario is about\n%s", line, w.exit, w.report())
	}
	return nil
}

// The file, from the repository's top, is there.
func (w *world) fileExists(path string) error {
	if _, err := os.Stat(filepath.Join(w.dir, path)); err != nil {
		return fmt.Errorf("the file %s is not there: %w\n%s", path, err, w.report())
	}
	return nil
}

// The file, from the repository's top, holds the text.
func (w *world) fileSays(path, text string) error {
	data, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("the file %s cannot be read: %w\n%s", path, err, w.report())
	}
	if !strings.Contains(string(data), text) {
		return fmt.Errorf("the file %s does not say %q:\n%s", path, text, data)
	}
	return nil
}
