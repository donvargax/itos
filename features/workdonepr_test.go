// The steps of work done under push.mode pr (work-done-pr.feature, slice 176):
// where an item's commit is, as a pull request leaves it. A branch of the
// remote holds the item's commit, which the clone's main lacks; the remote's
// main has gained it, which the clone lacks; the clone's main fast-forwards to
// it, so that only this clone's main has it; the clone switches to the branch.
// The item's commit is proof_test.go's: the clone's, linked to the item by an
// Item: footer, the config's registry footer turned on for it.
package features

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func initializeWorkDonePRSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the remote's branch "([^"]*)" has the item "([^"]*)"'s commit touching "([^"]*)", which the clone's main lacks$`, w.remoteBranchHasItemCommit)
	sc.Step(`^the remote's main has gained the item "([^"]*)"'s commit touching "([^"]*)", which the clone lacks$`, w.remoteGainsItemCommit)
	sc.Step(`^the clone fast-forwards its main to the branch "([^"]*)"$`, func(branch string) error {
		return w.git("merge", "-q", "--ff-only", "origin/"+branch)
	})
	sc.Step(`^the clone switches to its branch "([^"]*)"$`, func(branch string) error {
		return w.git("checkout", "-q", branch)
	})
}

// itemFooterOn turns the config's registry footer on, committed and pushed,
// so that an item's commit links to it.
func (w *world) itemFooterOn() error {
	if w.config.itemFooter {
		return nil
	}
	w.config.itemFooter = true
	return w.pushConfig("chore: link commits to items")
}

// The remote has the branch, one commit above the clone's main, which is the
// item's commit touching the file; the clone is left on its main, without the
// branch, which its remote-tracking ref still names.
func (w *world) remoteBranchHasItemCommit(branch, id, path string) error {
	if err := w.itemFooterOn(); err != nil {
		return err
	}
	current, err := w.gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	current = strings.TrimSpace(current)
	if err := w.git("checkout", "-q", "-b", branch); err != nil {
		return err
	}
	subject := "chore: build " + id
	if err := writeLine(w.dir, path, subject); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"add", "--", path},
		{"commit", "-q", "--no-verify", "-m", subject + "\n\nItem: " + id + "\n"},
		{"push", "-q", "--no-verify", "origin", branch},
		{"checkout", "-q", current},
		{"branch", "-q", "-D", branch},
	} {
		if err := w.git(args...); err != nil {
			return err
		}
	}
	return nil
}

// The remote's main gains the item's commit touching the file, made in
// another clone, which the scenario's clone has not fetched.
func (w *world) remoteGainsItemCommit(id, path string) error {
	if err := w.itemFooterOn(); err != nil {
		return err
	}
	other := filepath.Join(w.support, "other")
	if _, err := os.Stat(other); err != nil {
		if err := w.gitIn(w.support, "clone", "-q", w.remote(), other); err != nil {
			return err
		}
	}
	subject := "chore: build " + id
	if err := writeLine(other, path, subject); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"add", "--", path},
		{"commit", "-q", "--no-verify", "-m", subject + "\n\nItem: " + id + "\n"},
		{"push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main"},
	} {
		if err := w.gitIn(other, args...); err != nil {
			return err
		}
	}
	return nil
}
