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

	sc.Step(`^the message of HEAD has the footer "([^"]*)"$`, w.headHasFooter)
	sc.Step(`^the message of HEAD says "([^"]*)"$`, w.headMessageSays)
	sc.Step(`^no commit was made$`, w.noCommitMade)
}

// The commit-msg hook in the repository's hooks folder (git's own, no hook
// manager): a shim that runs the itos under test, as the one hooks install
// writes runs hooks.bin.
func (w *world) commitMsgHookInstalled() error {
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
	shim := "#!/bin/sh\nexec " + quote(w.bin) + " hook commit-msg \"$1\"\n"
	return os.WriteFile(filepath.Join(dir, "commit-msg"), []byte(shim), 0o755)
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
