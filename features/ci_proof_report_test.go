// The steps of the code proof's scheduling report (ci-proof-report.feature,
// slice 155), over the fixture and helpers of the proof summary's
// (ci_proof_summary_test.go): lines of ci plan's and ci run's streams read
// whole and by their place, a tag's full SHA written into an expected line,
// and the two edits of the scratch config and one of the repository's
// objects the scenarios need.
package features

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// A tag's full SHA in an expected line: {summary-base-sha}.
var tagSHAPlaceholder = regexp.MustCompile(`\{(.+?)-sha\}`)

func initializeProofReportSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^(stdout|stderr) contains the complete line "([^"]*)" exactly once, the tags' full SHAs filled in$`, func(stream, line string) error {
		line, err := w.fillTagSHAs(line)
		if err != nil {
			return err
		}
		return w.lineOnce(stream, line)
	})
	sc.Step(`^in (stdout|stderr) the line "([^"]*)" comes right before the line "([^"]*)", the tags' full SHAs filled in$`, func(stream, first, second string) error {
		first, err := w.fillTagSHAs(first)
		if err != nil {
			return err
		}
		second, err = w.fillTagSHAs(second)
		if err != nil {
			return err
		}
		lines := w.streamLines(stream)
		for i := 0; i+1 < len(lines); i++ {
			if lines[i] == first && lines[i+1] == second {
				return nil
			}
		}
		return fmt.Errorf("%s has no line %q right before the line %q\n%s", stream, first, second, w.report())
	})
	sc.Step(`^in (stdout|stderr) the line "([^"]*)" comes before the line "([^"]*)"$`, func(stream, first, second string) error {
		lines := w.streamLines(stream)
		at := slices.Index(lines, first)
		later := slices.Index(lines, second)
		if at < 0 || later <= at {
			return fmt.Errorf("in %s the line %q (%d) does not come before the line %q (%d)\n%s", stream, first, at, second, later, w.report())
		}
		return nil
	})
	sc.Step(`^the last line of stdout is "([^"]*)"$`, func(line string) error {
		lines := logLines(strings.TrimSuffix(w.stdout, "\n"))
		if last := lines[len(lines)-1]; last != line {
			return fmt.Errorf("the last line of stdout is %q, not %q\n%s", last, line, w.report())
		}
		return nil
	})

	sc.Step(`^the tree of the commit tagged "([^"]*)" is missing from the repository's objects$`, func(tag string) error {
		out, err := w.gitOutput("rev-parse", tag+"^{tree}")
		if err != nil {
			return err
		}
		tree := strings.TrimSpace(out)
		object := filepath.Join(w.dir, ".git", "objects", tree[:2], tree[2:])
		if err := os.Remove(object); err != nil {
			return fmt.Errorf("the tree %s of %s is not a loose object, so it cannot be deleted: %w", tree, tag, err)
		}
		return nil
	})
	sc.Step(`^the scratch itos\.yaml's commits\.since is the full SHA of the tag "([^"]*)"$`, func(tag string) error {
		sha, err := w.tagSHA(tag)
		if err != nil {
			return err
		}
		return w.editConfig(func(text string) (string, error) {
			if strings.Count(text, "\ncommits:\n") != 1 {
				return "", errors.New("the scratch itos.yaml does not have exactly one commits section")
			}
			return strings.Replace(text, "\ncommits:\n", "\ncommits:\n  since: "+sha+"\n", 1), nil
		})
	})
	sc.Step(`^the scratch itos\.yaml's proof section is removed$`, func() error {
		return w.editConfig(func(text string) (string, error) {
			lines := strings.Split(text, "\n")
			start := slices.Index(lines, "proof:")
			if start < 0 {
				return "", errors.New("the scratch itos.yaml has no proof section")
			}
			end := start + 1
			for end < len(lines) && (lines[end] == "" || lines[end][0] == ' ') {
				end++
			}
			return strings.Join(slices.Delete(lines, start, end), "\n"), nil
		})
	})
}

// The scratch root's itos.yaml, edited.
func (w *world) editConfig(edit func(string) (string, error)) error {
	text, err := w.recordedText("itos.yaml")
	if err != nil {
		return err
	}
	if text, err = edit(text); err != nil {
		return err
	}
	return w.write("itos.yaml", text)
}

// The text with each {<tag>-sha} the full SHA of that tag's commit.
func (w *world) fillTagSHAs(text string) (string, error) {
	var failed error
	filled := tagSHAPlaceholder.ReplaceAllStringFunc(text, func(placeholder string) string {
		sha, err := w.tagSHA(tagSHAPlaceholder.FindStringSubmatch(placeholder)[1])
		failed = errors.Join(failed, err)
		return sha
	})
	return filled, failed
}
