// The steps of itos next (next.feature, slice 203): the revision the JSON
// output names, its shape, and one run's revision kept under a name for a
// later run's to be compared with.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/cucumber/godog"
)

func initializeNextSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^its JSON's revision is "sha256:" and 64 lowercase hex digits$`, w.revisionIsDigest)
	sc.Step(`^its JSON's revision is saved as "([^"]*)"$`, w.saveRevision)
	sc.Step(`^its JSON's revision is the one saved as "([^"]*)"$`, func(name string) error {
		return w.revisionCompares(name, true)
	})
	sc.Step(`^its JSON's revision differs from the one saved as "([^"]*)"$`, func(name string) error {
		return w.revisionCompares(name, false)
	})
	// slice 209 (next-forge.feature): the head's checks the recording gh's
	// api graphql answers (nodes/07-checks.sh), and how many calls it had.
	sc.Step(`^the recording gh answers that the pull request's head's checks are "([^"]*)"$`, func(state string) error {
		if _, err := os.Stat(programPath(filepath.Join(w.support, "bin", "gh"))); err != nil {
			return errors.New("no recording gh: put one on the PATH first")
		}
		return os.WriteFile(filepath.Join(w.support, "gh-checks"), []byte(state+"\n"), 0o644)
	})
	sc.Step(`^the recording gh was called as "([^"]*)" (\d+) times$`, w.ghCalledTimes)
	// slice-216: auto-merge on for the branch's pull request, the mark the
	// recording gh's pr merge --auto leaves, which its pr view and its
	// graphql answer (nodes/08-auto-merge.sh) read.
	sc.Step(`^the recording gh's pull request for the branch "([^"]*)" has auto-merge on$`, func(branch string) error {
		if _, err := os.Stat(programPath(filepath.Join(w.support, "bin", "gh"))); err != nil {
			return errors.New("no recording gh: put one on the PATH first")
		}
		return os.WriteFile(filepath.Join(w.support, "gh-auto-merge-"+branch), nil, 0o644)
	})
}

// revision is the last run's JSON "revision", a text.
func (w *world) revision() (string, error) {
	var object struct {
		Revision *string `json:"revision"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &object); err != nil {
		return "", fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	if object.Revision == nil {
		return "", fmt.Errorf("the JSON has no revision, or one that is no text\n%s", w.report())
	}
	return *object.Revision, nil
}

// digest is a revision's shape: sha256: and 64 lowercase hex digits.
var digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func (w *world) revisionIsDigest() error {
	revision, err := w.revision()
	if err != nil {
		return err
	}
	if !digest.MatchString(revision) {
		return fmt.Errorf("the revision %q is not sha256: and 64 lowercase hex digits\n%s", revision, w.report())
	}
	return nil
}

// The last run's revision, kept under the name.
func (w *world) saveRevision(name string) error {
	revision, err := w.revision()
	if err != nil {
		return err
	}
	if w.revisions == nil {
		w.revisions = map[string]string{}
	}
	w.revisions[name] = revision
	return nil
}

// The last run's revision is the one saved under the name (same), or
// another.
func (w *world) revisionCompares(name string, same bool) error {
	revision, err := w.revision()
	if err != nil {
		return err
	}
	saved, ok := w.revisions[name]
	if !ok {
		return fmt.Errorf("no revision was saved as %q", name)
	}
	if same && revision != saved {
		return fmt.Errorf("the revision %q is not the one saved as %q, %q\n%s", revision, name, saved, w.report())
	}
	if !same && revision == saved {
		return fmt.Errorf("the revision %q is still the one saved as %q\n%s", revision, name, w.report())
	}
	return nil
}
