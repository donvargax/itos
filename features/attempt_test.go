// The steps of itos attempt submit (attempt.feature, slice 207): a result
// file written with the values saved earlier filled in, a commit turning a
// scenario live, the remote branch's head saved under a name, one line of
// a JSON Lines file compared with a value, a saved value, null or a file's
// SHA-256, the count of a list of the JSON output, a read's blob at a saved
// commit, and the refs of the clone and of the remote saved and compared,
// one ref of each left out.
package features

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v8/internal/git"
)

func initializeAttemptSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the file "([^"]*)" holds the lines, the saved values filled in:$`, w.fileHoldsFilledLines)
	sc.Step(`^the commit "([^"]*)" turning the scenario "([^"]*)" live on top of it$`, w.commitTurningLive)
	sc.Step(`^the full SHA of the head of the remote's branch "([^"]*)" is saved as "([^"]*)"$`, w.saveRemoteBranchHead)
	// Each form of the line's value is its own pattern, none matching
	// another's text: a number, a quoted text, null, a saved value, a
	// file's SHA-256.
	sc.Step(`^line (\d+) of the file "([^"]*)" is a JSON object whose "([^"]*)" is (\d+)$`, func(n int, file, key, want string) error {
		return w.jsonLineValueIs(n, file, key, want)
	})
	sc.Step(`^line (\d+) of the file "([^"]*)" is a JSON object whose "([^"]*)" is ("[^"]*")$`, func(n int, file, key, want string) error {
		return w.jsonLineValueIs(n, file, key, want)
	})
	sc.Step(`^line (\d+) of the file "([^"]*)" is a JSON object whose "([^"]*)" is null$`, func(n int, file, key string) error {
		return w.jsonLineValueIs(n, file, key, "null")
	})
	sc.Step(`^line (\d+) of the file "([^"]*)" is a JSON object whose "([^"]*)" is the value saved as "([^"]*)"$`, w.jsonLineValueIsSaved)
	sc.Step(`^line (\d+) of the file "([^"]*)" is a JSON object whose "([^"]*)" is the SHA-256 of the file "([^"]*)"$`, w.jsonLineValueIsDigest)
	sc.Step(`^its JSON's "([^"]*)" has exactly (\d+) entr(?:y|ies)$`, w.jsonListHasEntries)
	sc.Step(`^its JSON's "([^"]*)" entry whose "([^"]*)" is "([^"]*)" has "([^"]*)" the git blob id of "([^"]*)" at the commit saved as "([^"]*)"$`,
		func(key, field, value, other, file, name string) error {
			rev, err := w.savedText(name)
			if err != nil {
				return err
			}
			return w.jsonListEntryHasBlobAt(key, field, value, other, file, rev)
		})
	sc.Step(`^the refs of the clone and of the remote are saved for comparison$`, w.saveRefs)
	sc.Step(`^the clone's refs match the saved ones, all but "([^"]*)"$`, func(but string) error {
		return w.refsMatch(false, but)
	})
	sc.Step(`^the remote's refs match the saved ones, all but "([^"]*)"$`, func(but string) error {
		return w.refsMatch(true, but)
	})
}

// attemptRefs are the refs of the clone and of the remote, each a ref's
// name to its object, as saved for a later comparison.
type attemptRefs struct {
	clone, remote map[string]string
}

// savedText is the value saved under the name as text: a JSON string,
// unquoted, any other JSON value as written.
func (w *world) savedText(name string) (string, error) {
	saved, err := w.saved(name)
	if err != nil {
		return "", err
	}
	var text string
	if err := json.Unmarshal([]byte(saved), &text); err == nil {
		return text, nil
	}
	return saved, nil
}

// placeholder is a saved value's name between angle brackets.
var placeholder = regexp.MustCompile(`<([^<>]+)>`)

// The file holds the lines, each <name> replaced by the value saved so.
func (w *world) fileHoldsFilledLines(path string, lines *godog.DocString) error {
	var missing error
	filled := placeholder.ReplaceAllStringFunc(lines.Content, func(m string) string {
		text, err := w.savedText(m[1 : len(m)-1])
		if err != nil && missing == nil {
			missing = err
		}
		return text
	})
	if missing != nil {
		return missing
	}
	return w.write(path, filled+"\n")
}

// The feature file holding the scenario has its tag line's @wip taken out,
// committed with the message.
func (w *world) commitTurningLive(message, id string) error {
	files, err := filepath.Glob(filepath.Join(w.dir, "features", "*.feature"))
	if err != nil {
		return err
	}
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		lines := strings.Split(string(text), "\n")
		for i, line := range lines {
			fields := strings.Fields(line)
			if len(fields) == 0 || fields[0] != id {
				continue
			}
			kept := []string{}
			for _, tag := range fields {
				if tag != "@wip" {
					kept = append(kept, tag)
				}
			}
			if len(kept) == len(fields) {
				return fmt.Errorf("the scenario %s is not @wip in %s", id, file)
			}
			lines[i] = "  " + strings.Join(kept, " ")
			if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
				return err
			}
			return w.commit(message)
		}
	}
	return fmt.Errorf("no feature file has the scenario %s", id)
}

// The remote's branch's head, saved by its full SHA under the name, as the
// last commit's is.
func (w *world) saveRemoteBranchHead(branch, name string) error {
	sha, err := w.remoteBranchHead(branch)
	if err != nil {
		return err
	}
	if w.workCommits == nil {
		w.workCommits = map[string]string{}
	}
	w.workCommits[name] = sha
	return nil
}

// jsonLine is the value under the key of the object on line n, from 1, of
// the file of the clone.
func (w *world) jsonLine(n int, file, key string) (any, error) {
	lines, err := w.recordedLines(file)
	if err != nil {
		return nil, err
	}
	if n < 1 || n > len(lines) {
		return nil, fmt.Errorf("%s has %d lines, no line %d: %q", file, len(lines), n, lines)
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(lines[n-1]), &object); err != nil {
		return nil, fmt.Errorf("line %d of %s is not a JSON object: %v\n%s", n, file, err, lines[n-1])
	}
	v, ok := object[key]
	if !ok {
		return nil, fmt.Errorf("line %d of %s has no %q: %s", n, file, key, lines[n-1])
	}
	return v, nil
}

// The value under the key on line n is the JSON value written.
func (w *world) jsonLineValueIs(n int, file, key, want string) error {
	got, err := w.jsonLine(n, file, key)
	if err != nil {
		return err
	}
	var expected any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		return fmt.Errorf("the scenario's %s is not JSON: %v", want, err)
	}
	if !reflect.DeepEqual(got, expected) {
		return fmt.Errorf("line %d of %s has %q %s, not %s", n, file, key, jsonText(got), want)
	}
	return nil
}

// The value under the key on line n is the text saved under the name.
func (w *world) jsonLineValueIsSaved(n int, file, key, name string) error {
	want, err := w.savedText(name)
	if err != nil {
		return err
	}
	return w.jsonLineValueIs(n, file, key, jsonText(want))
}

// The value under the key on line n is "sha256:" and the hex digest of the
// other file's bytes.
func (w *world) jsonLineValueIsDigest(n int, file, key, other string) error {
	data, err := os.ReadFile(filepath.Join(w.dir, other))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	return w.jsonLineValueIs(n, file, key, jsonText("sha256:"+hex.EncodeToString(sum[:])))
}

// The list under the key has n entries.
func (w *world) jsonListHasEntries(key string, n int) error {
	entries, err := w.jsonList(key)
	if err != nil {
		return err
	}
	if len(entries) != n {
		return fmt.Errorf("the JSON's %q has %d entries, not %d\n%s", key, len(entries), n, w.report())
	}
	return nil
}

// refsOf are the refs of the repository in dir, each name to its object.
func (w *world) refsOf(dir string) (map[string]string, error) {
	cmd := exec.Command(git.Bin(runtime.GOOS), "for-each-ref", "--format=%(refname) %(objectname)")
	cmd.Dir = dir
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref in %s: %w", dir, err)
	}
	refs := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name, object, ok := strings.Cut(line, " "); ok {
			refs[name] = object
		}
	}
	return refs, nil
}

func (w *world) saveRefs() error {
	clone, err := w.refsOf(w.dir)
	if err != nil {
		return err
	}
	remote, err := w.refsOf(w.remote())
	if err != nil {
		return err
	}
	w.attemptRefs = &attemptRefs{clone: clone, remote: remote}
	return nil
}

// The refs of the clone, or of the remote, are the saved ones, the ref
// named left out of both.
func (w *world) refsMatch(remote bool, but string) error {
	if w.attemptRefs == nil {
		return fmt.Errorf("no refs were saved")
	}
	which, dir, saved := "clone", w.dir, w.attemptRefs.clone
	if remote {
		which, dir, saved = "remote", w.remote(), w.attemptRefs.remote
	}
	now, err := w.refsOf(dir)
	if err != nil {
		return err
	}
	delete(now, but)
	was := map[string]string{}
	for name, object := range saved {
		if name != but {
			was[name] = object
		}
	}
	if reflect.DeepEqual(now, was) {
		return nil
	}
	return fmt.Errorf("the %s's refs changed:\n--- saved\n%s--- now\n%s", which, refsText(was), refsText(now))
}

func refsText(refs map[string]string) string {
	var lines []string
	for name, object := range refs {
		lines = append(lines, name+" "+object+"\n")
	}
	sort.Strings(lines)
	return strings.Join(lines, "")
}
