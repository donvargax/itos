// The steps of prepush-refs.feature (slice 182): hooks.pre_push's commands
// record which of them ran and with which base, another branch the remote
// gains from the clone it gains commits from, a merge of a remote branch in
// the clone, an origin with no branch the clone tracks, and a tag the remote
// holds, read as the commit it names.
package features

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

func initializePrePushRefSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^hooks\.pre_push's commands record which of them ran, and with which base$`, w.recordingPrePushBases)
	sc.Step(`^hooks\.pre_push's per_base alone ran, once, with the base "([^"]*)"$`, func(subject string) error {
		return w.prePushRan("per_base", subject)
	})
	sc.Step(`^hooks\.pre_push's per_base alone ran, twice, with the bases "([^"]*)" and "([^"]*)"$`, func(a, b string) error {
		return w.prePushRan("per_base", a, b)
	})
	sc.Step(`^hooks\.pre_push's whole alone ran, once$`, func() error { return w.prePushRan("whole") })
	sc.Step(`^the remote has gained the branch "([^"]*)" with the commit "([^"]*)" touching "([^"]*)"$`, w.remoteGainsBranch)
	sc.Step(`^the clone has merged the remote's branch "([^"]*)"$`, func(branch string) error {
		return w.git("merge", "-q", "--no-ff", "--no-edit", "--no-verify", "origin/"+branch)
	})
	sc.Step(`^the clone's remote origin is a new repository with no branches, of which the clone tracks none$`, w.originWithNoBranches)
	sc.Step(`^the remote's tag "([^"]*)" names its main's head$`, w.remoteTagNamesMain)
}

// hooks.pre_push's per_base appends "per_base <the base>" and whole appends
// "whole" to the file "none of hooks.pre_push's commands ran" reads, the
// config committed and pushed past the hooks as recordingPrePush's is.
func (w *world) recordingPrePushBases() error {
	w.config.prePushBases = true
	return w.pushPrePushConfig("chore: record the pre-push commands")
}

// The lines the commands recorded are exactly the command's name, for per_base
// followed by the full SHA of the commit with each subject in the clone, in
// either order, and no other.
func (w *world) prePushRan(command string, subjects ...string) error {
	text, err := os.ReadFile(w.prePushRecord())
	if err != nil {
		return fmt.Errorf("none of hooks.pre_push's commands ran: %w\n%s", err, w.report())
	}
	var got, want []string
	got = strings.Split(strings.TrimSpace(strings.ReplaceAll(string(text), "\r", "")), "\n")
	if len(subjects) == 0 {
		want = []string{command}
	}
	for _, subject := range subjects {
		sha, err := w.cloneCommitBySubject(subject)
		if err != nil {
			return err
		}
		want = append(want, command+" "+sha)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return fmt.Errorf("hooks.pre_push's commands recorded:\n%s\nwant:\n%s\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"), w.report())
	}
	return nil
}

// The full SHA of the commit with the subject, among the clone's refs.
func (w *world) cloneCommitBySubject(subject string) (string, error) {
	out, err := w.gitOutput("log", "--all", "--format=%H %s")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n") {
		if sha, s, _ := strings.Cut(line, " "); s == subject {
			return sha, nil
		}
	}
	return "", fmt.Errorf("the clone has no commit %q:\n%s", subject, out)
}

// A branch made in the other clone of the remote at its main, with one commit
// as remoteGainsCommit makes it, pushed, the clone left on its main.
func (w *world) remoteGainsBranch(branch, subject, path string) error {
	other := filepath.Join(w.support, "other")
	if _, err := os.Stat(other); err != nil {
		if err := w.gitIn(w.support, "clone", "-q", w.remote(), other); err != nil {
			return err
		}
	}
	if err := w.gitIn(other, "checkout", "-q", "-b", branch, "main"); err != nil {
		return err
	}
	if err := writeLine(other, path, subject); err != nil {
		return err
	}
	if err := w.gitIn(other, "add", "--", path); err != nil {
		return err
	}
	if err := w.gitIn(other, "commit", "-q", "--no-verify", "-m", subject); err != nil {
		return err
	}
	if err := w.gitIn(other, "push", "-q", "--no-verify", "origin", "HEAD:refs/heads/"+branch); err != nil {
		return err
	}
	return w.gitIn(other, "checkout", "-q", "main")
}

// origin points at a new empty bare repository, and every ref the clone
// holds under refs/remotes/origin/ is gone, as a new project's clone has none.
func (w *world) originWithNoBranches() error {
	empty := filepath.Join(w.support, "empty.git")
	if err := w.gitIn(w.support, "init", "-q", "--bare", empty); err != nil {
		return err
	}
	if err := w.git("remote", "set-url", "origin", empty); err != nil {
		return err
	}
	out, err := w.gitOutput("for-each-ref", "--format=%(refname)", "refs/remotes/origin/")
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(out) {
		if err := w.git("update-ref", "-d", ref); err != nil {
			return err
		}
	}
	if left, _ := w.gitOutput("for-each-ref", "refs/remotes/"); strings.TrimSpace(left) != "" {
		return errors.New("the clone still tracks:\n" + left)
	}
	return nil
}

// The commit the remote's tag names, peeled, is the remote's main.
func (w *world) remoteTagNamesMain(tag string) error {
	named := w.refIn(w.remote(), "refs/tags/"+tag+"^{commit}")
	main := w.refIn(w.remote(), "refs/heads/main")
	if named == "" || named != main {
		return fmt.Errorf("the remote's tag %q names %q, not its main's head %s\n%s", tag, named, main, w.report())
	}
	return nil
}
