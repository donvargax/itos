// The steps of itos draft (draft.feature, slice 96): a file of the
// repository changed and left uncommitted, the header of the commit before
// the last, and a merge in progress that leaves no change to commit.
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
	sc.Step(`^a merge is in progress, with no change to commit$`, w.mergeInProgress)
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

// A merge of a branch whose one commit changes nothing, stopped before its
// commit: MERGE_HEAD is there and no tracked file has a change, so only the
// merge itself tells the checkout is busy.
func (w *world) mergeInProgress() error {
	branch, err := w.gitOutput("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return err
	}
	for _, args := range [][]string{
		{"checkout", "-q", "-b", "side"},
		{"commit", "-q", "--no-verify", "--allow-empty", "-m", "chore: nothing"},
		{"checkout", "-q", strings.TrimSpace(branch)},
		{"merge", "-q", "--no-ff", "--no-commit", "side"},
	} {
		if err := w.git(args...); err != nil {
			return err
		}
	}
	return nil
}
