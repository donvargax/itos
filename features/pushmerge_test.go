// The steps of push.pr.update merge (push-merge.feature), slice-213: a commit
// saved from the clone that the remote's branch has under the same SHA, the
// remote branch's head a merge, and a merge left in progress in the clone that
// the person resolves and commits.
package features

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cucumber/godog"
	"github.com/donvargax/itos/v8/internal/git"
)

func initializePushMergeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the remote's branch "([^"]*)" has the saved "([^"]*)" commit$`, w.remoteBranchHasSaved)
	sc.Step(`^the head of the remote's branch "([^"]*)" is the merge commit "([^"]*)"$`, w.remoteHeadIsMerge)
	sc.Step(`^the clone has a merge in progress$`, func() error {
		merging, err := w.cloneMerging()
		if err != nil || merging {
			return err
		}
		return fmt.Errorf("the clone has no merge in progress\n%s", w.report())
	})
	sc.Step(`^the clone resolves the conflict in "([^"]*)", keeping both sides, and commits the merge as "([^"]*)"$`, w.cloneResolvesMerge)
	// Bug 158: a file whose lines two sides change.
	sc.Step(`^the remote has gained the commit "([^"]*)" writing "([^"]*)" with the lines:$`, func(subject, path string, lines *godog.DocString) error {
		return w.remoteGainsWriting(path, subject, lines.Content+"\n")
	})
	sc.Step(`^the clone has the commit "([^"]*)" writing "([^"]*)" with the lines:$`, func(subject, path string, lines *godog.DocString) error {
		return w.cloneCommitsWriting(subject, path, lines.Content+"\n")
	})
}

// The commit saved from the clone's HEAD is an ancestor of the head of the
// remote's branch, the same SHA.
func (w *world) remoteBranchHasSaved(branch, name string) error {
	sha := w.workCommits[name]
	if sha == "" {
		return fmt.Errorf("no commit saved as %q", name)
	}
	if err := w.gitIn(w.remote(), "merge-base", "--is-ancestor", sha, "refs/heads/"+branch); err != nil {
		return fmt.Errorf("the remote's branch %q does not have the saved %q commit %s: %w\n%s", branch, name, sha, err, w.report())
	}
	return nil
}

// The head of the remote's branch has the subject and two parents.
func (w *world) remoteHeadIsMerge(branch, subject string) error {
	cmd := exec.Command(git.Bin(runtime.GOOS), "log", "-1", "--format=%P%n%s", "refs/heads/"+branch)
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git log refs/heads/%s in the remote: %w", branch, err)
	}
	parents, got, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if n := len(strings.Fields(parents)); got != subject || n != 2 {
		return fmt.Errorf("the head of the remote's branch %q is %q with %d parents, not the merge commit %q\n%s", branch, got, n, subject, w.report())
	}
	return nil
}

// Whether a merge is in progress in the clone: git keeps MERGE_HEAD in its
// git folder while one is.
func (w *world) cloneMerging() (bool, error) {
	out, err := w.gitOutput("rev-parse", "--git-path", "MERGE_HEAD")
	if err != nil {
		return false, err
	}
	file := strings.TrimSpace(out)
	if !filepath.IsAbs(file) {
		file = filepath.Join(w.dir, file)
	}
	_, err = os.Stat(file)
	return err == nil, nil
}

// The merge in progress resolved as the rebase's conflict is
// (cloneResolvesKeepingBoth's markers left out, the file staged), then
// committed past the hooks with the subject; nothing is touched when no merge
// is in progress.
func (w *world) cloneResolvesMerge(path, subject string) error {
	merging, err := w.cloneMerging()
	if err != nil {
		return err
	}
	if !merging {
		return fmt.Errorf("the clone has no merge in progress to resolve\n%s", w.report())
	}
	if err := w.keepBothSides(path); err != nil {
		return err
	}
	return w.git("commit", "-q", "--no-verify", "-m", subject)
}
