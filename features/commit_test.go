// The steps of itos commit (commit-command.feature): the commit-msg hook a
// commit made through itos runs, the arguments itos is given as a shell
// would split them, and what the commit made, or that none was.
package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

func initializeCommitSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the commit-msg hook is installed$`, w.commitMsgHookInstalled)

	sc.Step(`^itos commits with the arguments "([^"]*)"$`, func(line string) error {
		args, err := shellWords(line)
		if err != nil {
			return err
		}
		return w.itos(append([]string{"commit"}, args...)...)
	})

	sc.Step(`^itos has committed with the arguments "([^"]*)"$`, w.itosHasCommitted)
	sc.Step(`^git commits with the message "([^"]*)"$`, func(message string) error {
		return w.run(w.dir, "git", "commit", "-m", message)
	})
	sc.Step(`^git amends HEAD with the message "([^"]*)"$`, w.amendHead)

	sc.Step(`^the message of HEAD has the footer "([^"]*)"$`, w.headHasFooter)
	sc.Step(`^the message of HEAD has the footer "([^"]*)" once$`, w.headHasFooterOnce)
	sc.Step(`^the message of HEAD says "([^"]*)"$`, w.headMessageSays)
	sc.Step(`^the message of HEAD does not say "([^"]*)"$`, w.headMessageDoesNotSay)
	sc.Step(`^the itos note on HEAD says "([^"]*)"$`, w.headNoteSays)
	sc.Step(`^no commit was made$`, w.noCommitMade)
	sc.Step(`^the commit is refused$`, w.commitRefused)
}

// A commit made through itos, which has to succeed: the scenario's commit
// from then on.
func (w *world) itosHasCommitted(line string) error {
	args, err := shellWords(line)
	if err != nil {
		return err
	}
	if err := w.itos(append([]string{"commit"}, args...)...); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("itos commit failed\n%s", w.report())
	}
	if _, err := w.newHeadMessage(); err != nil {
		return err
	}
	head, err := w.head()
	if err != nil {
		return err
	}
	w.commits = append(w.commits, head)
	return nil
}

// git commit --amend, with the hooks, which has to make a new HEAD: the
// scenario's last commit from then on.
func (w *world) amendHead(message string) error {
	if len(w.commits) == 0 {
		return errors.New("the repository has no commit of the scenario's to amend")
	}
	if err := w.run(w.dir, "git", "commit", "--amend", "-m", message); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("git commit --amend failed\n%s", w.report())
	}
	head, err := w.head()
	if err != nil {
		return err
	}
	if head == w.commits[len(w.commits)-1] {
		return fmt.Errorf("git commit --amend left HEAD at %s\n%s", head, w.report())
	}
	w.commits[len(w.commits)-1] = head
	return nil
}

// The commit-msg hook in the repository's hooks folder (git's own, no hook
// manager): a shim that runs the itos under test, as the one hooks install
// writes runs hooks.bin.
func (w *world) commitMsgHookInstalled() error { return w.hookInstalled("commit-msg", `"$1"`) }

// The hook in the repository's hooks folder, a shim running the itos under
// test's `hook <name>` with the arguments git gives it.
func (w *world) hookInstalled(name, args string) error {
	out, err := w.gitOutput("rev-parse", "--git-path", "hooks")
	if err != nil {
		return err
	}
	dir := strings.TrimSpace(out)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(w.dir, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	shim := "#!/bin/sh\nexec " + quote(w.bin) + " hook " + name + " " + args + "\n"
	return os.WriteFile(filepath.Join(dir, name), []byte(shim), 0o755)
}

// A line split into words as sh splits it, for the quoting the scenarios
// use: whitespace between words, and single quotes around a word's spaces.
func shellWords(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord, quoted := false, false
	for _, r := range line {
		switch {
		case r == '\'':
			quoted, inWord = !quoted, true
		case !quoted && (r == ' ' || r == '\t'):
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if quoted {
		return nil, fmt.Errorf("an unclosed quote in %q", line)
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}

// A git command's stdout in the scratch repository.
func (w *world) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = w.dir
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// The message of HEAD, made after the scenario's own commits.
func (w *world) newHeadMessage() (string, error) {
	if err := w.noCommitMade(); err == nil {
		return "", fmt.Errorf("no commit was made\n%s", w.report())
	}
	return w.gitOutput("log", "-1", "--format=%B")
}

// HEAD's trailers, as git reads them, hold the footer, a line of its own.
func (w *world) headHasFooter(footer string) error {
	if _, err := w.newHeadMessage(); err != nil {
		return err
	}
	out, err := w.gitOutput("log", "-1", "--format=%(trailers:only,unfold)")
	if err != nil {
		return err
	}
	if !slices.Contains(strings.Split(out, "\n"), footer) {
		message, _ := w.gitOutput("log", "-1", "--format=%B")
		return fmt.Errorf("HEAD has no footer %q; its message:\n%s", footer, message)
	}
	return nil
}

// HEAD's trailers, as git reads them, hold the footer on exactly one line.
// HEAD is read as it is: an amend that writes nothing new, made in the same
// second as the commit it amends, is that very commit again, its SHA too, so
// the scenario's exit code, not a new HEAD, says the amend ran.
func (w *world) headHasFooterOnce(footer string) error {
	out, err := w.gitOutput("log", "-1", "--format=%(trailers:only,unfold)")
	if err != nil {
		return err
	}
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if line == footer {
			n++
		}
	}
	if n != 1 {
		message, _ := w.gitOutput("log", "-1", "--format=%B")
		return fmt.Errorf("HEAD has the footer %q %d times, not once; its message:\n%s", footer, n, message)
	}
	return nil
}

func (w *world) headMessageSays(text string) error {
	message, err := w.newHeadMessage()
	if err != nil {
		return err
	}
	if !strings.Contains(message, text) {
		return fmt.Errorf("the message of HEAD does not say %q:\n%s", text, message)
	}
	return nil
}

func (w *world) headMessageDoesNotSay(text string) error {
	message, err := w.newHeadMessage()
	if err != nil {
		return err
	}
	if strings.Contains(message, text) {
		return fmt.Errorf("the message of HEAD says %q:\n%s", text, message)
	}
	return nil
}

// HEAD's note in refs/notes/itos, where the stealth mode keeps the footers,
// has the line.
func (w *world) headNoteSays(line string) error {
	note, err := w.gitOutput("notes", "--ref=refs/notes/itos", "show", "HEAD")
	if err != nil {
		return fmt.Errorf("HEAD has no itos note: %w\n%s", err, w.report())
	}
	if !slices.Contains(strings.Split(note, "\n"), line) {
		return fmt.Errorf("the itos note on HEAD does not say %q:\n%s", line, note)
	}
	return nil
}

// git failed, and HEAD is still the scenario's last commit.
func (w *world) commitRefused() error {
	if w.exit == 0 {
		return fmt.Errorf("the commit went through\n%s", w.report())
	}
	return w.noCommitMade()
}

// HEAD is still the scenario's last commit.
func (w *world) noCommitMade() error {
	if len(w.commits) == 0 {
		return errors.New("the repository has no commit of the scenario's")
	}
	head, err := w.head()
	if err != nil {
		return err
	}
	if last := w.commits[len(w.commits)-1]; head != last {
		return fmt.Errorf("HEAD is %s, a commit made after the scenario's last, %s", head, last)
	}
	return nil
}
