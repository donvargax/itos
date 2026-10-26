// The steps of CI trusting a merged pull request's green head (ci-trust.feature,
// slice 188). The world is work-done-pr-trust.feature's: the remote, the
// clone, GitHub's rebase merge and the fake GitHub's run of a branch's head.
// The fake GitHub also lists the pull request that merged the remote's main
// (range_test.go), and itos plans or runs CI over the commits the clone's last
// fast-forward brought.
package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v8/internal/git"
)

func initializeCITrustSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the remote's branch "([^"]*)" has the item "([^"]*)"'s commit naming the task "([^"]*)" and touching "([^"]*)", which the clone's main lacks$`,
		func(branch, id, task, file string) error {
			return w.remoteBranchHasItemCommitNaming(branch, id, task, file)
		})
	sc.Step(`^the remote has gained the commit "([^"]*)" deleting "([^"]*)"$`, w.remoteGainsDeletion)
	sc.Step(`^the fake GitHub lists the pull request "([^"]*)" as rebase-merged into the remote's main from the head of the remote's (branch|ref) "([^"]*)"$`,
		w.fakeListsMergedPull)
	sc.Step(`^itos plans CI over the commits the fast-forward brought(, with --json)?$`, func(withJSON string) error {
		args, err := w.broughtRange("plan")
		if err != nil {
			return err
		}
		if withJSON != "" {
			args = append(args, "--json")
		}
		return w.itos(args...)
	})
	sc.Step(`^itos runs CI over the commits the fast-forward brought$`, func() error {
		args, err := w.broughtRange("run")
		if err != nil {
			return err
		}
		return w.itos(args...)
	})
}

// ci plan or ci run over the commits the clone's last fast-forward brought:
// from its main before to its main after, by their full SHAs.
func (w *world) broughtRange(sub string) ([]string, error) {
	if w.ffBefore == "" || w.ffAfter == "" {
		return nil, errors.New("the clone has not fast-forwarded its main: say so first")
	}
	return []string{"ci", sub, w.ffBefore, w.ffAfter}, nil
}

// A commit made in another clone of the remote that deletes the file, pushed
// to the remote's main.
func (w *world) remoteGainsDeletion(subject, file string) error {
	other := filepath.Join(w.support, "other")
	if _, err := os.Stat(other); err != nil {
		if err := w.gitIn(w.support, "clone", "-q", w.remote(), other); err != nil {
			return err
		}
	} else {
		for _, args := range [][]string{{"fetch", "-q", "origin"}, {"reset", "-q", "--hard", "origin/main"}} {
			if err := w.gitIn(other, args...); err != nil {
				return err
			}
		}
	}
	for _, args := range [][]string{
		{"rm", "-q", "--", file},
		{"commit", "-q", "--no-verify", "-m", subject},
		{"push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main"},
	} {
		if err := w.gitIn(other, args...); err != nil {
			return err
		}
	}
	return nil
}

// The fake GitHub lists, for the remote's main as it is now, the one pull
// request that merged it: numbered by the address's last segment, closed,
// merged, with the head of the remote's branch or ref as it is now. Any
// other commit's pulls are an empty list.
func (w *world) fakeListsMergedPull(address, kind, name string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	number, err := strconv.Atoi(path.Base(address))
	if err != nil {
		return fmt.Errorf("the pull request's address %q does not end in its number", address)
	}
	main, err := w.remoteHead()
	if err != nil {
		return err
	}
	rev := "refs/heads/" + name
	if kind == "ref" {
		rev = name
	}
	cmd := exec.Command(git.Bin(runtime.GOOS), "rev-parse", "--verify", rev)
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git rev-parse %s in the remote: %w", rev, err)
	}
	pull := fakePull{Number: number, HTMLURL: address, State: "closed", MergedAt: "2026-10-26T10:00:00Z", MergeCommitSHA: main}
	pull.Head.SHA, pull.Head.Ref = strings.TrimSpace(string(out)), name
	pull.Base.Ref = "main"
	g := w.github
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pulls == nil {
		g.pulls = map[string][]fakePull{}
	}
	g.pulls[main] = append(g.pulls[main], pull)
	return nil
}
