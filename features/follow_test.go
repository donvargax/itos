// The steps of itos follow (follow.feature, slice 61): a command line that
// has to succeed before the run a scenario is about, in the repository or in
// its linked worktree, and a file of the repository that exists or says a
// text. Bug 16's: one command run many times at once, each run's exit
// checked (stealth.feature's work add uses them too), the threads file
// written directly, and a run under a time zone. Bug 17's: the line after or
// before a text in a file is blank. Slice 71's: a file has a line, alone or
// after another (init.feature's AGENTS.md block uses them too). Bug 35's: a
// command line run in the linked worktree, the run a scenario is about.
package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

func initializeFollowSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^itos has run the command line "([^"]*)"$`, func(line string) error {
		return w.hasRunLine(w.dir, line)
	})
	sc.Step(`^itos has run the command line "([^"]*)" in the linked worktree$`, func(line string) error {
		if w.linked == "" {
			return fmt.Errorf("the scenario adds no linked worktree")
		}
		return w.hasRunLine(w.linked, line)
	})
	sc.Step(`^itos runs the command line "([^"]*)" in the linked worktree$`, func(line string) error {
		if w.linked == "" {
			return fmt.Errorf("the scenario adds no linked worktree")
		}
		args, err := shellWords(line)
		if err != nil {
			return err
		}
		return w.itosIn(w.linked, args...)
	})
	sc.Step(`^the file "([^"]*)" exists$`, w.fileExists)
	sc.Step(`^the file "([^"]*)" says "([^"]*)"$`, w.fileSays)
	sc.Step(`^the file "([^"]*)" does not say "([^"]*)"$`, w.fileDoesNotSay)
	sc.Step(`^in the file "([^"]*)" the line (after|before) "([^"]*)" is blank$`, w.lineBesideIsBlank)
	sc.Step(`^the file "([^"]*)" has the line "([^"]*)"$`, func(path, line string) error {
		return w.fileHasLine(path, line, "")
	})
	sc.Step(`^the file "([^"]*)" has the line "([^"]*)" after the line "([^"]*)"$`, w.fileHasLine)

	sc.Step(`^itos runs "([^"]*)" with the notes "([^"]*)" to "([^"]*)" all at once$`, func(line, from, to string) error {
		notes, err := numbered(from, to)
		if err != nil {
			return err
		}
		w.atOnceWords = notes
		runs := make([][]string, len(notes))
		for i, note := range notes {
			runs[i] = append(strings.Fields(line), note)
		}
		return w.allAtOnce(runs)
	})
	sc.Step(`^every run exited 0$`, w.everyRunExited0)
	sc.Step(`^itos follow show ([^ ]+) lists every one of those notes$`, w.showListsEveryNote)
	sc.Step(`^the threads file holds a second YAML document after the thread "([^"]*)"$`, func(id string) error {
		return w.writeThreads(threadsText(id, "2026-10-04T09:30:15-07:00") + "---\n" + threadsText("flaky-bo", "2026-10-04T10:00:00-07:00"))
	})
	sc.Step(`^the threads file holds the thread "([^"]*)" with a note stamped "([^"]*)"$`, func(id, stamp string) error {
		return w.writeThreads(threadsText(id, stamp))
	})
	sc.Step(`^the threads file is as it was$`, w.threadsAsTheyWere)
	sc.Step(`^itos runs "([^"]*)" with TZ "([^"]*)"$`, func(args, zone string) error {
		w.vars = append(w.vars, "TZ="+zone)
		return w.itos(strings.Fields(args)...)
	})
}

// numberWords are what "one" to "twenty" counts through.
var numberWords = []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
	"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen", "twenty"}

// numbered is the words from one to the other, each a number word after the
// same prefix: "one" to "three", or "p1-one" to "p1-ten".
func numbered(from, to string) ([]string, error) {
	for i, start := range numberWords {
		prefix, ok := strings.CutSuffix(from, start)
		if !ok {
			continue
		}
		for j := i; j < len(numberWords); j++ {
			if prefix+numberWords[j] == to {
				var words []string
				for _, n := range numberWords[i : j+1] {
					words = append(words, prefix+n)
				}
				return words, nil
			}
		}
	}
	return nil, fmt.Errorf("%q to %q counts no number words up", from, to)
}

// One run of many at once: its arguments, exit and output.
type atOnceRun struct {
	args           []string
	exit           int
	stdout, stderr string
}

// Every command line run at the same moment in the repository, each a
// process of its own started together, so their loads and saves overlap;
// each run's exit and output kept for everyRunExited0.
func (w *world) allAtOnce(runs [][]string) error {
	w.markRun()
	w.atOnce = make([]atOnceRun, len(runs))
	start := make(chan struct{})
	failed := make([]error, len(runs))
	var done sync.WaitGroup
	for i, args := range runs {
		cmd := exec.Command(w.bin, args...)
		cmd.Dir = w.dir
		cmd.Env = w.env()
		var stdout, stderr strings.Builder
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		done.Add(1)
		go func() {
			defer done.Done()
			<-start
			err := cmd.Run()
			run := atOnceRun{args: args, stdout: stdout.String(), stderr: stderr.String()}
			var exit *exec.ExitError
			switch {
			case errors.As(err, &exit):
				run.exit = exit.ExitCode()
			case err != nil:
				failed[i] = fmt.Errorf("running itos %s: %w", strings.Join(args, " "), err)
			}
			w.atOnce[i] = run
		}()
	}
	close(start)
	done.Wait()
	return errors.Join(failed...)
}

// Every run of allAtOnce exited 0; the ones that did not, with what they
// said, otherwise.
func (w *world) everyRunExited0() error {
	if len(w.atOnce) == 0 {
		return fmt.Errorf("the scenario ran nothing at once")
	}
	var bad []string
	for _, r := range w.atOnce {
		if r.exit != 0 {
			bad = append(bad, fmt.Sprintf("itos %s exited %d\n--- stdout\n%s--- stderr\n%s", strings.Join(r.args, " "), r.exit, r.stdout, r.stderr))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%d of %d runs did not exit 0:\n%s", len(bad), len(w.atOnce), strings.Join(bad, "\n"))
	}
	return nil
}

// follow show of the thread prints a note line for every word allAtOnce
// added: a line whose text after its date and time is the word.
func (w *world) showListsEveryNote(id string) error {
	if err := w.itos("follow", "show", id); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("follow show %s exited %d\n%s", id, w.exit, w.report())
	}
	noted := map[string]bool{}
	for _, line := range strings.Split(w.stdout, "\n") {
		if fields := strings.Fields(line); len(fields) == 3 {
			noted[fields[2]] = true
		}
	}
	var missing []string
	for _, word := range w.atOnceWords {
		if !noted[word] {
			missing = append(missing, word)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%d of the %d notes are missing: %s\n%s", len(missing), len(w.atOnceWords), strings.Join(missing, ", "), w.report())
	}
	return nil
}

// The threads file, in itos's folder of the git folder.
func (w *world) threadsFile() string { return filepath.Join(w.dir, ".git", "itos", "follow-ups.yaml") }

// One thread, open, its one note stamped, as itos writes a threads file.
func threadsText(id, stamp string) string {
	return fmt.Sprintf("threads:\n  - id: %s\n    with: ana\n    title: The sync design\n    status: open\n    notes:\n      - at: %q\n        text: Started.\n", id, stamp)
}

// The threads file written directly with the text, kept for
// threadsAsTheyWere.
func (w *world) writeThreads(text string) error {
	if err := os.MkdirAll(filepath.Dir(w.threadsFile()), 0o700); err != nil {
		return err
	}
	w.threadsBefore = text
	return os.WriteFile(w.threadsFile(), []byte(text), 0o600)
}

// The threads file holds what the scenario wrote, byte for byte.
func (w *world) threadsAsTheyWere() error {
	data, err := os.ReadFile(w.threadsFile())
	if err != nil {
		return fmt.Errorf("the threads file cannot be read: %w\n%s", err, w.report())
	}
	if string(data) != w.threadsBefore {
		return fmt.Errorf("the threads file changed:\n%s\n%s", data, w.report())
	}
	return nil
}

// The command line, split as a shell splits it, run in the folder dir, which
// has to succeed: it sets up what the scenario is about.
func (w *world) hasRunLine(dir, line string) error {
	args, err := shellWords(line)
	if err != nil {
		return err
	}
	if err := w.itosIn(dir, args...); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("itos %s exited %d before the run the scenario is about\n%s", line, w.exit, w.report())
	}
	return nil
}

// The file, from the repository's top, is there.
func (w *world) fileExists(path string) error {
	if _, err := os.Stat(filepath.Join(w.dir, path)); err != nil {
		return fmt.Errorf("the file %s is not there: %w\n%s", path, err, w.report())
	}
	return nil
}

// The file, from the repository's top, holds the text.
func (w *world) fileSays(path, text string) error {
	data, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("the file %s cannot be read: %w\n%s", path, err, w.report())
	}
	if !strings.Contains(string(data), text) {
		return fmt.Errorf("the file %s does not say %q:\n%s", path, text, data)
	}
	return nil
}

// The file, from the repository's top, is there and does not hold the text:
// a file that is not there says nothing, which is no proof of what it would
// leave out, so it fails.
func (w *world) fileDoesNotSay(path, text string) error {
	data, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("the file %s cannot be read: %w\n%s", path, err, w.report())
	}
	if strings.Contains(string(data), text) {
		return fmt.Errorf("the file %s says %q:\n%s", path, text, data)
	}
	return nil
}

// The file, from the repository's top, has a line that is the text, its
// line ending and trailing spaces aside; with after given, such a line comes
// somewhere below the first line that is after, which the file must have.
// A line is matched whole, so a text inside a longer line does not count.
func (w *world) fileHasLine(path, line, after string) error {
	data, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("the file %s cannot be read: %w\n%s", path, err, w.report())
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	is := func(text string) func(string) bool {
		return func(l string) bool { return strings.TrimRight(l, " \t") == text }
	}
	from := 0
	if after != "" {
		at := slices.IndexFunc(lines, is(after))
		if at < 0 {
			return fmt.Errorf("the file %s has no line %q:\n%s", path, after, data)
		}
		from = at + 1
	}
	if !slices.ContainsFunc(lines[from:], is(line)) {
		if after != "" {
			return fmt.Errorf("the file %s has no line %q after the line %q:\n%s", path, line, after, data)
		}
		return fmt.Errorf("the file %s has no line %q:\n%s", path, line, data)
	}
	return nil
}

// In the file, from the repository's top, the line after (or before) the
// first line holding the text is blank: empty, or spaces alone. A text on
// no line, or on the first or last, fails.
func (w *world) lineBesideIsBlank(path, side, text string) error {
	data, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return fmt.Errorf("the file %s cannot be read: %w\n%s", path, err, w.report())
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	at := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, text) })
	if at < 0 {
		return fmt.Errorf("no line of %s says %q:\n%s", path, text, data)
	}
	beside := at + 1
	if side == "before" {
		beside = at - 1
	}
	if beside < 0 || beside >= len(lines) || strings.TrimSpace(lines[beside]) != "" {
		return fmt.Errorf("in %s the line %s %q is not blank:\n%s", path, side, text, data)
	}
	return nil
}
