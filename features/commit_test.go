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

	sc.Step(`^the message file "([^"]*)" with the header "([^"]*)" and a body line of (\d+) characters$`,
		func(file, header string, n int) error {
			return w.writeMessageFile(file, header, wordsOf(n))
		})
	sc.Step(`^the message file "([^"]*)" with the header "([^"]*)" and a body list item of (\d+) characters$`,
		func(file, header string, n int) error {
			return w.writeMessageFile(file, header, "- "+wordsOf(n-2))
		})
	sc.Step(`^the message file "([^"]*)" with the header "([^"]*)" and the body lines "([^"]*)" and "([^"]*)"$`,
		func(file, header, a, b string) error {
			return w.writeMessageFile(file, header, a+"\n"+b)
		})
	sc.Step(`^the message file "([^"]*)" with the header "([^"]*)" and a body line whose wrap would start a line with "([^"]*)"$`,
		func(file, header, text string) error {
			return w.writeMessageFile(file, header, wordsOf(bodyLimit)+" "+text)
		})
	sc.Step(`^no line of HEAD's message is longer than (\d+) characters$`, w.headLinesFit)
	sc.Step(`^no line of HEAD's message starts with "([^"]*)"$`, w.headNoLineStartsWith)
	sc.Step(`^HEAD's message body has the same words as the file's, in order$`, w.headBodyHasFileWords)
	sc.Step(`^every line of HEAD's message body after the item's first starts with two spaces$`, w.itemLinesIndented)
	sc.Step(`^the message of HEAD has the line "([^"]*)"$`, w.headHasLine)
}

// Words of prose, n characters in all, the last cut short to fit: no word
// ends in a colon, so no line of them reads as a footer.
func wordsOf(n int) string {
	words := strings.Fields("the readme said how to build itos but not how to run its scenarios " +
		"so a reader who came for the tests found nothing and asked again")
	var b strings.Builder
	for i := 0; b.Len() < n; i++ {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(words[i%len(words)])
	}
	text := b.String()[:n]
	if strings.HasSuffix(text, " ") {
		text = text[:n-1] + "s"
	}
	return text
}

// bodyLimit is the built-in header lint's body-max-line-length. A body line
// of wordsOf(bodyLimit), a space and a text is one a greedy wrap at the limit
// breaks just before the text, so the text would start the second line.
const bodyLimit = 100

// A message file in the scratch repository, a header, a blank line and the
// body, which -F names relative to the folder itos runs in.
func (w *world) writeMessageFile(file, header, body string) error {
	w.messageFile = header + "\n\n" + body + "\n"
	return os.WriteFile(filepath.Join(w.dir, file), []byte(w.messageFile), 0o644)
}

// Each line of HEAD's message, footers included, is at most n characters.
func (w *world) headLinesFit(n int) error {
	message, err := w.newHeadMessage()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(message, "\n") {
		if len([]rune(line)) > n {
			return fmt.Errorf("a line of HEAD's message is %d characters long, over %d:\n%s", len([]rune(line)), n, message)
		}
	}
	return nil
}

// No line of HEAD's message, footers included, begins with the text.
func (w *world) headNoLineStartsWith(text string) error {
	message, err := w.newHeadMessage()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(message, "\n") {
		if strings.HasPrefix(line, text) {
			return fmt.Errorf("a line of HEAD's message starts with %q:\n%s", text, message)
		}
	}
	return nil
}

// HEAD's message body: the lines after its header, up to the trailers git
// reads at its end, which itos's footers are.
func (w *world) headBody() ([]string, string, error) {
	message, err := w.newHeadMessage()
	if err != nil {
		return nil, "", err
	}
	trailers, err := w.gitOutput("log", "-1", "--format=%(trailers:only)")
	if err != nil {
		return nil, "", err
	}
	lines := strings.Split(strings.TrimRight(message, "\n"), "\n")
	block := strings.Split(strings.TrimRight(trailers, "\n"), "\n")
	if strings.TrimSpace(trailers) != "" && len(block) < len(lines) &&
		slices.Equal(lines[len(lines)-len(block):], block) {
		lines = lines[:len(lines)-len(block)]
	}
	return lines[1:], message, nil
}

// HEAD's body holds the message file's body word for word: the wrapping
// broke lines, and lost, added or moved no word.
func (w *world) headBodyHasFileWords() error {
	body, message, err := w.headBody()
	if err != nil {
		return err
	}
	_, want, _ := strings.Cut(w.messageFile, "\n")
	if got := strings.Fields(strings.Join(body, "\n")); !slices.Equal(got, strings.Fields(want)) {
		return fmt.Errorf("HEAD's body does not have the file's words in order; the file's body:\n%s\nHEAD's message:\n%s",
			want, message)
	}
	return nil
}

// The body's list item goes on over more than one line, and each line after
// its first is indented by two spaces, under the item's text.
func (w *world) itemLinesIndented() error {
	body, message, err := w.headBody()
	if err != nil {
		return err
	}
	first := slices.IndexFunc(body, func(l string) bool { return strings.HasPrefix(l, "- ") })
	if first < 0 {
		return fmt.Errorf("HEAD's body has no list item:\n%s", message)
	}
	rest := body[first+1:]
	for len(rest) > 0 && rest[len(rest)-1] == "" {
		rest = rest[:len(rest)-1]
	}
	if len(rest) == 0 {
		return fmt.Errorf("HEAD's list item has no line after its first:\n%s", message)
	}
	for _, line := range rest {
		if !strings.HasPrefix(line, "  ") {
			return fmt.Errorf("a line after the list item's first does not start with two spaces, %q:\n%s", line, message)
		}
	}
	return nil
}

// HEAD's message has the line, whole.
func (w *world) headHasLine(line string) error {
	message, err := w.newHeadMessage()
	if err != nil {
		return err
	}
	if !slices.Contains(strings.Split(message, "\n"), line) {
		return fmt.Errorf("HEAD's message has no line %q:\n%s", line, message)
	}
	return nil
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
