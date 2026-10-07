// The steps of itos draft (draft.feature, slice 96): a file of the
// repository changed and left uncommitted, the header of the commit before
// the last, and a merge in progress that leaves no change to commit; and
// (slice 99) an edit draft's copy of a file, read, changed and named.
package features

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func initializeDraftSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the file "([^"]*)" is changed to hold "([^"]*)"$`, func(path, text string) error {
		return w.write(path, text+"\n")
	})
	sc.Step(`^the commit before the last has the header "([^"]*)"$`, w.headerBeforeLastIs)
	sc.Step(`^a merge is in progress, with no change to commit$`, w.mergeInProgress)
	sc.Step(`^the draft "([^"]*)"'s copy of "([^"]*)" says "([^"]*)"$`, w.draftCopySays)
	sc.Step(`^the draft "([^"]*)"'s copy of "([^"]*)" is changed to hold "([^"]*)"$`, w.draftCopyChanged)
	sc.Step(`^its output names the draft "([^"]*)"'s copy of "([^"]*)"$`, w.outputNamesDraftCopy)
}

// Where itos draft edit keeps the draft's copy of the path, as its help
// says: edits/<id>/<path> in the drafts folder of itos's folder in the git
// common dir, as git names it for the scratch repository.
func (w *world) draftCopy(id, path string) (string, error) {
	common, err := w.gitOutput("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.Join(strings.TrimSpace(common), "itos", "drafts", "edits", id, filepath.FromSlash(path)), nil
}

// The draft's copy of the path holds the text.
func (w *world) draftCopySays(id, path, text string) error {
	copied, err := w.draftCopy(id, path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(copied)
	if err != nil {
		return fmt.Errorf("the draft %s's copy of %s cannot be read: %w\n%s", id, path, err, w.report())
	}
	if !strings.Contains(string(data), text) {
		return fmt.Errorf("the draft %s's copy of %s does not say %q:\n%s", id, path, text, data)
	}
	return nil
}

// The draft's copy of the path, written over with the text as one line, as
// a person editing it there would leave it.
func (w *world) draftCopyChanged(id, path, text string) error {
	copied, err := w.draftCopy(id, path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(copied); err != nil {
		return fmt.Errorf("the draft %s has no copy of %s to change: %w\n%s", id, path, err, w.report())
	}
	return os.WriteFile(copied, []byte(text+"\n"), 0o600)
}

// The output has a line that is the draft's copy of the path, whole, so a
// person or an agent can open it.
func (w *world) outputNamesDraftCopy(id, path string) error {
	copied, err := w.draftCopy(id, path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(w.stdout, "\n") {
		if strings.TrimSpace(line) == copied {
			return nil
		}
	}
	return fmt.Errorf("no line of the output is the draft %s's copy of %s, %s\n%s", id, path, copied, w.report())
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
