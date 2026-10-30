// The steps of itos next (next.feature, slice 203): the revision the JSON
// output names, its shape, and one run's revision kept under a name for a
// later run's to be compared with.
package features

import (
	"encoding/json"
	"fmt"
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
