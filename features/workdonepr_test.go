// The steps of work done under push.mode pr (work-done-pr.feature, slice 176):
// where an item's commit is, as a pull request leaves it. A branch of the
// remote holds the item's commit, which the clone's main lacks; the remote's
// main has gained it, which the clone lacks; the clone's main fast-forwards to
// it, so that only this clone's main has it; the clone switches to the branch.
// The item's commit is proof_test.go's: the clone's, linked to the item by an
// Item: footer, the config's registry footer turned on for it. Slice 181's
// steps (work-done-pr-trust.feature) add how GitHub merges: it rebase-merges a
// branch onto the remote's main, each commit made anew by GitHub, and a head
// that only the pull request's ref has once its branch is gone.
package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v8/internal/git"
)

func initializeWorkDonePRSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the remote's branch "([^"]*)" has the item "([^"]*)"'s commit touching "([^"]*)", which the clone's main lacks$`, w.remoteBranchHasItemCommit)
	sc.Step(`^the remote's main has gained the item "([^"]*)"'s commit touching "([^"]*)", which the clone lacks$`, w.remoteGainsItemCommit)
	sc.Step(`^the clone fast-forwards its main to the branch "([^"]*)"$`, w.cloneFastForwards)
	sc.Step(`^GitHub rebase-merges the remote's branch "([^"]*)" onto the remote's main$`, func(branch string) error {
		return w.githubRebaseMerges(branch, false)
	})
	sc.Step(`^GitHub rebase-merges the remote's branch "([^"]*)" onto the remote's main and deletes the branch$`, func(branch string) error {
		return w.githubRebaseMerges(branch, true)
	})
	sc.Step(`^the remote's branch "([^"]*)" and its ref "([^"]*)" have the item "([^"]*)"'s commit touching "([^"]*)", made in a clone of their own$`, w.remoteBranchAndRefHaveItemCommit)
	sc.Step(`^the clone commits its changes and pushes them to the remote's main$`, w.cloneCommitsChangesAndPushes)
	sc.Step(`^the clone switches to its branch "([^"]*)"$`, func(branch string) error {
		return w.git("checkout", "-q", branch)
	})
}

// The clone's main moved to the remote's branch by a fast-forward, the full
// SHAs of main before and after kept for the steps that plan over what it
// brought (citrust_test.go).
func (w *world) cloneFastForwards(branch string) error {
	before, err := w.head()
	if err != nil {
		return err
	}
	if err := w.git("merge", "-q", "--ff-only", "origin/"+branch); err != nil {
		return err
	}
	after, err := w.head()
	if err != nil {
		return err
	}
	w.ffBefore, w.ffAfter = before, after
	return nil
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
	return w.remoteBranchHasItemCommitNaming(branch, id, "", path)
}

// remoteBranchHasItemCommit's commit with a Task: footer beside its Item:
// one when task is not "".
func (w *world) remoteBranchHasItemCommitNaming(branch, id, task, path string) error {
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
	footers := "Item: " + id + "\n"
	if task != "" {
		footers += "Task: " + task + "\n"
	}
	for _, args := range [][]string{
		{"add", "--", path},
		{"commit", "-q", "--no-verify", "-m", subject + "\n\n" + footers},
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

// GitHub merges the branch by rebase: in a clone of its own, as pr
// update-branch rebases, it replays the branch's commits that the remote's
// main lacks onto main, each made anew with GitHub as its committer, so every
// SHA is new even when main gained nothing, and pushes them to main past the
// hooks, leaving the branch where it was, or, when deleting is set, deleting
// it as GitHub deletes a merged branch. A commit that kept its SHA fails the
// step.
func (w *world) githubRebaseMerges(branch string, deleting bool) error {
	dir := w.ghRebaseDir()
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := w.gitIn(w.support, "clone", "-q", "--branch", branch, w.remote(), dir); err != nil {
		return err
	}
	theirs, err := w.commitsIn(dir, "origin/main..HEAD")
	if err != nil {
		return err
	}
	if len(theirs) == 0 {
		return fmt.Errorf("the remote's branch %q has no commit its main lacks to merge", branch)
	}
	rebase := exec.Command(git.Bin(runtime.GOOS), "rebase", "-q", "--force-rebase", "origin/main")
	rebase.Dir = dir
	rebase.Env = append(w.env(), "GIT_COMMITTER_NAME=GitHub", "GIT_COMMITTER_EMAIL=noreply@github.com")
	if out, err := rebase.CombinedOutput(); err != nil {
		return fmt.Errorf("git rebase --force-rebase origin/main: %v\n%s", err, out)
	}
	ours, err := w.commitsIn(dir, "origin/main..HEAD")
	if err != nil {
		return err
	}
	if len(ours) != len(theirs) || slices.ContainsFunc(ours, func(sha string) bool { return slices.Contains(theirs, sha) }) {
		return fmt.Errorf("GitHub's rebase kept a commit's SHA: the branch's %v, the merge's %v", theirs, ours)
	}
	if err := w.gitIn(dir, "push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main"); err != nil {
		return err
	}
	if deleting {
		return w.gitIn(w.remote(), "branch", "-q", "-D", branch)
	}
	return nil
}

// The full SHAs of the commits the range names in the repository in dir.
func (w *world) commitsIn(dir, rng string) ([]string, error) {
	cmd := exec.Command(git.Bin(runtime.GOOS), "rev-list", rng)
	cmd.Dir = dir
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-list %s: %w", rng, err)
	}
	return strings.Fields(string(out)), nil
}

// The remote has the branch, and the ref, at one commit above its main, the
// item's commit touching the file, made in a clone of the remote's main that no
// other step uses: the clone itos runs in never has the commit's object, until
// something fetches it, as with a head GitHub made by update-branch.
func (w *world) remoteBranchAndRefHaveItemCommit(branch, ref, id, path string) error {
	if err := w.itemFooterOn(); err != nil {
		return err
	}
	dir := filepath.Join(w.support, "head")
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := w.gitIn(w.support, "clone", "-q", w.remote(), dir); err != nil {
		return err
	}
	subject := "chore: build " + id
	if err := writeLine(dir, path, subject); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"add", "--", path},
		{"commit", "-q", "--no-verify", "-m", subject + "\n\nItem: " + id + "\n"},
		{"push", "-q", "--no-verify", "origin", "HEAD:refs/heads/" + branch, "HEAD:" + ref},
	} {
		if err := w.gitIn(dir, args...); err != nil {
			return err
		}
	}
	return nil
}

// Every change of the clone's working tree, committed past the hooks naming
// the ledger's first task, as the config's commits do, and pushed to the
// remote's main.
func (w *world) cloneCommitsChangesAndPushes() error {
	if len(w.ledger) == 0 {
		return errors.New("the ledger has no task for the commit to name")
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-q", "--no-verify", "-m", "chore: commit the clone's changes\n\nTask: " + w.ledger[0].id + "\n"},
		{"push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main"},
	} {
		if err := w.git(args...); err != nil {
			return err
		}
	}
	return nil
}
