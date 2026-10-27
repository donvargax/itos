// The steps of status's fetch (status.feature, slice 196): a remote whose
// main names a commit it cannot send, a clone with no remote, on a branch
// the remote lacks or on a detached HEAD, a ref of the clone compared with
// the remote's main, and a line of the JSON's "unread".
package features

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v8/internal/git"
)

func initializeStatusFetchSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the remote's main has gained a commit whose objects the remote cannot send$`, w.remoteMainUnsendable)
	sc.Step(`^the clone has no remote$`, func() error { return w.git("remote", "remove", "origin") })
	sc.Step(`^the clone's HEAD is detached at its commit$`, func() error { return w.git("checkout", "-q", "--detach") })
	sc.Step(`^the clone is on the new branch "([^"]*)", which the remote does not have$`, func(branch string) error {
		return w.git("checkout", "-q", "-b", branch)
	})
	sc.Step(`^the clone's ref "([^"]*)" names the remote's main$`, w.cloneRefNamesRemoteMain)
	sc.Step(`^its JSON's "([^"]*)" has a line saying "([^"]*)"$`, w.jsonListHasLine)
}

// The remote's main moves to a commit over a tree naming a blob the remote
// lacks (git mktree --missing, then commit-tree): ls-remote lists the
// commit, but upload-pack cannot pack it, so a fetch of it fails.
func (w *world) remoteMainUnsendable() error {
	const absent = "0123456789abcdef0123456789abcdef01234567"
	made := func(stdin string, args ...string) (string, error) {
		cmd := exec.Command(git.Bin(runtime.GOOS), args...)
		cmd.Dir, cmd.Env = w.remote(), w.env()
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out)), nil
	}
	tree, err := made("100644 blob "+absent+"\tlost.txt\n", "mktree", "--missing")
	if err != nil {
		return err
	}
	commit, err := made("", "commit-tree", "-p", "refs/heads/main", "-m", "feat: a commit the remote cannot send", tree)
	if err != nil {
		return err
	}
	_, err = made("", "update-ref", "refs/heads/main", commit)
	return err
}

// The clone's ref names the commit the remote's main names.
func (w *world) cloneRefNamesRemoteMain(ref string) error {
	theirs, ours := w.refIn(w.remote(), "refs/heads/main"), w.refIn(w.dir, ref)
	switch {
	case ours == "":
		return fmt.Errorf("the clone has no ref %s\n%s", ref, w.report())
	case ours != theirs:
		return fmt.Errorf("the clone's %s is %s, not the remote's main %s\n%s", ref, ours, theirs, w.report())
	}
	return nil
}

// A line of the list the JSON's field holds says the text.
func (w *world) jsonListHasLine(field, text string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(w.stdout), &object); err != nil {
		return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	var lines []string
	if err := json.Unmarshal(object[field], &lines); err != nil {
		return fmt.Errorf("the JSON's %q is not a list of lines: %v\n%s", field, err, w.report())
	}
	for _, line := range lines {
		if strings.Contains(line, text) {
			return nil
		}
	}
	return fmt.Errorf("no line of the JSON's %q says %q: %q\n%s", field, text, lines, w.report())
}
