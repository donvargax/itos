// The steps of itos push (push.feature): a remote, a bare repository made
// from the scratch one, a clone of it where itos runs, commits the remote
// gains from another clone, and what the remote's branch holds afterwards.
package features

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// The line an uncommitted change adds to a file of the clone.
const uncommittedLine = "An uncommitted change.\n"

func initializePushSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a clone of it, where itos runs$`, w.cloneOfRemote)
	sc.Step(`^the clone's git config says "([^"]*)" is "([^"]*)"$`, func(key, value string) error {
		return w.git("config", "--local", key, value)
	})
	sc.Step(`^the remote has gained the commit "([^"]*)" touching "([^"]*)"$`, w.remoteGainsCommit)
	sc.Step(`^the clone has the commit "([^"]*)" touching "([^"]*)"$`, w.cloneCommits)
	sc.Step(`^the clone's "([^"]*)" has an uncommitted change$`, w.uncommittedChange)
	sc.Step(`^the clone's "([^"]*)" still has its uncommitted change$`, w.stillUncommitted)
	sc.Step(`^the remote's branch ends with "([^"]*)" then "([^"]*)", with no merge commit$`, w.remoteEndsWith)
	sc.Step(`^the remote's branch does not have "([^"]*)"$`, w.remoteLacks)
}

// The remote: a bare repository in the support folder, as a host keeps one.
func (w *world) remote() string { return filepath.Join(w.support, "origin.git") }

// The scratch repository's history in a bare remote, and a clone of that,
// its main tracking the remote's, with the files the scratch repository has
// not committed laid into it. itos and every later step run in the clone.
func (w *world) cloneOfRemote() error {
	if err := w.git("clone", "-q", "--bare", w.dir, w.remote()); err != nil {
		return err
	}
	clone, err := os.MkdirTemp("", "itos-features-clone-")
	if err != nil {
		return err
	}
	if err := w.gitIn(w.support, "clone", "-q", w.remote(), clone); err != nil {
		os.RemoveAll(clone)
		return err
	}
	if err := w.layUncommitted(clone); err != nil {
		os.RemoveAll(clone)
		return err
	}
	w.origin, w.dir = w.dir, clone
	return nil
}

// A commit made in another clone of the remote and pushed to its main, the
// file written as the commit's own line.
func (w *world) remoteGainsCommit(subject, path string) error {
	other := filepath.Join(w.support, "other")
	if _, err := os.Stat(other); err != nil {
		if err := w.gitIn(w.support, "clone", "-q", w.remote(), other); err != nil {
			return err
		}
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
	return w.gitIn(other, "push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main")
}

// A commit in the clone of that one file, written as the commit's own line.
func (w *world) cloneCommits(subject, path string) error {
	if err := writeLine(w.dir, path, subject); err != nil {
		return err
	}
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	return w.git("commit", "-q", "--no-verify", "-m", subject, "--", path)
}

// The file at path in the folder dir, holding one line naming the commit, so
// two commits touching the same file conflict.
func writeLine(dir, path, subject string) error {
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte("A line for "+subject+".\n"), 0o644)
}

func (w *world) uncommittedChange(path string) error {
	full := filepath.Join(w.dir, path)
	text, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	return os.WriteFile(full, append(text, uncommittedLine...), 0o644)
}

// The file still ends with the uncommitted line, and git still sees it
// changed against HEAD: nothing stashed it, nothing committed it.
func (w *world) stillUncommitted(path string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return err
	}
	if !strings.HasSuffix(string(text), uncommittedLine) {
		return fmt.Errorf("%s lost its uncommitted change:\n%s", path, text)
	}
	if err := w.git("diff", "--quiet", "HEAD", "--", path); err == nil {
		return fmt.Errorf("git sees no change to %s against HEAD", path)
	}
	return nil
}

// The subjects of the remote's main, newest first, and its merge commits.
func (w *world) remoteHistory() (subjects, merges []string, err error) {
	read := func(args ...string) ([]string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = w.remote()
		cmd.Env = w.env()
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git %s in the remote: %w", strings.Join(args, " "), err)
		}
		if text := strings.TrimSpace(string(out)); text != "" {
			return strings.Split(text, "\n"), nil
		}
		return nil, nil
	}
	if subjects, err = read("log", "--format=%s", "main"); err != nil {
		return nil, nil, err
	}
	merges, err = read("rev-list", "--merges", "main")
	return subjects, merges, err
}

func (w *world) remoteEndsWith(older, newer string) error {
	subjects, merges, err := w.remoteHistory()
	if err != nil {
		return err
	}
	if len(merges) > 0 {
		return fmt.Errorf("the remote's main has merge commits %v:\n%s", merges, strings.Join(subjects, "\n"))
	}
	if len(subjects) < 2 || subjects[0] != newer || subjects[1] != older {
		return fmt.Errorf("the remote's main does not end with %q then %q:\n%s", older, newer, strings.Join(subjects, "\n"))
	}
	return nil
}

func (w *world) remoteLacks(subject string) error {
	subjects, _, err := w.remoteHistory()
	if err != nil {
		return err
	}
	if slices.Contains(subjects, subject) {
		return fmt.Errorf("the remote's main has %q:\n%s", subject, strings.Join(subjects, "\n"))
	}
	return nil
}
