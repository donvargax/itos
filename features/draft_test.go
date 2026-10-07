// The steps of itos draft (draft.feature, slice 96): a file of the
// repository changed and left uncommitted, and the header of the commit
// before the last.
package features

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func initializeDraftSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the file "([^"]*)" is changed to hold "([^"]*)"$`, func(path, text string) error {
		return w.write(path, text+"\n")
	})
	sc.Step(`^the commit before the last has the header "([^"]*)"$`, w.headerBeforeLastIs)
}

// The header of HEAD's first parent.
func (w *world) headerBeforeLastIs(header string) error {
	out, err := w.gitOutput("log", "-1", "--format=%s", "HEAD~1")
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(out); got != header {
		return fmt.Errorf("the commit before the last has the header %q, not %q\n%s", got, header, w.report())
	}
	return nil
}
