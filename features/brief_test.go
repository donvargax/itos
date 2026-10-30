// The steps of itos brief (brief.feature, slice-204): a sentence of the
// output with the repository's folder name and top filled in, the top as
// git prints it; stdout, or one value of the JSON output, saved under a
// name and compared with a later run's; a JSON value's form (a prefix and
// hex digits, the full SHA of a commit by its header); and the entries of
// a list of the JSON output, each field checked against a value or against
// git's blob id of a file at HEAD.
package features

import (
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func initializeBriefSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^its output says "([^"]*)", the repository's folder name and top filled in$`, w.outputSaysWithTop)
	sc.Step(`^stdout is saved as "([^"]*)"$`, w.saveStdout)
	sc.Step(`^the JSON's "([^"]*)" is saved as "([^"]*)"$`, w.saveJSONValue)
	sc.Step(`^its JSON's "([^"]*)" matches the one saved as "([^"]*)"$`, w.jsonValueMatchesSaved)
	sc.Step(`^its JSON's "([^"]*)" differs from the one saved as "([^"]*)"$`, w.jsonValueDiffersFromSaved)
	sc.Step(`^its JSON's "([^"]*)" equals the stdout saved as "([^"]*)"$`, w.jsonValueIsSavedStdout)
	sc.Step(`^its JSON's "([^"]*)" starts with "([^"]*)" followed by (\d+) lowercase hex digits$`, w.jsonValueIsPrefixedHex)
	sc.Step(`^its JSON's "([^"]*)" names the commit "([^"]*)"$`, w.jsonValueNamesCommit)
	sc.Step(`^the "([^"]*)" of each entry of its JSON's "([^"]*)", in order, is "([^"]*)"$`, w.jsonListFieldsAre)
	sc.Step(`^its JSON's "([^"]*)" entry whose "([^"]*)" is "([^"]*)" has "([^"]*)" (true|false|"[^"]*")$`, w.jsonListEntryHas)
	sc.Step(`^its JSON's "([^"]*)" entry whose "([^"]*)" is "([^"]*)" has "([^"]*)" the git blob id of "([^"]*)" at HEAD$`, w.jsonListEntryHasBlob)
}

// The output says the text with <folder> and <top> filled in: the top as
// git prints it from the clone (its links resolved, forward slashes on
// Windows), never the test's own folder, and the folder its last name.
func (w *world) outputSaysWithTop(text string) error {
	top, err := w.gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	top = strings.TrimSpace(top)
	filled := strings.NewReplacer("<folder>", path.Base(top), "<top>", top).Replace(text)
	return w.outputSays(filled)
}

// saved is what the brief steps keep under a name: stdout, or one JSON
// value's text.
func (w *world) saved(name string) (string, error) {
	text, ok := w.briefSaved[name]
	if !ok {
		return "", fmt.Errorf("nothing was saved as %q", name)
	}
	return text, nil
}

func (w *world) save(name, text string) {
	if w.briefSaved == nil {
		w.briefSaved = map[string]string{}
	}
	w.briefSaved[name] = text
}

func (w *world) saveStdout(name string) error {
	w.save(name, w.stdout)
	return nil
}

// jsonAt is the value at the dotted path of standard output's one JSON
// object.
func (w *world) jsonAt(p string) (any, error) {
	var got any
	if err := json.Unmarshal([]byte(w.stdout), &got); err != nil {
		return nil, fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	for _, key := range strings.Split(p, ".") {
		object, ok := got.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("the JSON has no %s: %v is not an object\n%s", p, got, w.report())
		}
		if got, ok = object[key]; !ok {
			return nil, fmt.Errorf("the JSON has no %s\n%s", p, w.report())
		}
	}
	return got, nil
}

// jsonStringAt is the value at the dotted path, which must be a string.
func (w *world) jsonStringAt(p string) (string, error) {
	v, err := w.jsonAt(p)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("the JSON's %s is %s, not a string\n%s", p, jsonText(v), w.report())
	}
	return s, nil
}

func (w *world) saveJSONValue(p, name string) error {
	v, err := w.jsonAt(p)
	if err != nil {
		return err
	}
	w.save(name, jsonText(v))
	return nil
}

func (w *world) jsonValueMatchesSaved(p, name string) error {
	v, err := w.jsonAt(p)
	if err != nil {
		return err
	}
	want, err := w.saved(name)
	if err != nil {
		return err
	}
	if got := jsonText(v); got != want {
		return fmt.Errorf("the JSON's %s is %s, not the %s saved as %q\n%s", p, got, want, name, w.report())
	}
	return nil
}

func (w *world) jsonValueDiffersFromSaved(p, name string) error {
	v, err := w.jsonAt(p)
	if err != nil {
		return err
	}
	saved, err := w.saved(name)
	if err != nil {
		return err
	}
	if got := jsonText(v); got == saved {
		return fmt.Errorf("the JSON's %s is still %s, the one saved as %q\n%s", p, got, name, w.report())
	}
	return nil
}

// The JSON's value at the path is the stdout saved earlier, byte for byte.
func (w *world) jsonValueIsSavedStdout(p, name string) error {
	got, err := w.jsonStringAt(p)
	if err != nil {
		return err
	}
	want, err := w.saved(name)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("the JSON's %s is not the stdout saved as %q\n--- got\n%s--- saved\n%s", p, name, got, want)
	}
	return nil
}

func (w *world) jsonValueIsPrefixedHex(p, prefix string, digits int) error {
	got, err := w.jsonStringAt(p)
	if err != nil {
		return err
	}
	form := regexp.MustCompile(`^` + regexp.QuoteMeta(prefix) + `[0-9a-f]{` + strconv.Itoa(digits) + `}$`)
	if !form.MatchString(got) {
		return fmt.Errorf("the JSON's %s is %q, not %q and %d lowercase hex digits\n%s", p, got, prefix, digits, w.report())
	}
	return nil
}

// The JSON's value at the path is the full SHA of the one commit of the
// clone whose header is the one given.
func (w *world) jsonValueNamesCommit(p, header string) error {
	got, err := w.jsonStringAt(p)
	if err != nil {
		return err
	}
	listed, err := w.gitOutput("log", "--all", "--format=%H%x1f%s")
	if err != nil {
		return err
	}
	var shas []string
	for _, line := range strings.Split(strings.TrimSpace(listed), "\n") {
		sha, subject, _ := strings.Cut(line, "\x1f")
		if subject == header {
			shas = append(shas, sha)
		}
	}
	if len(shas) != 1 {
		return fmt.Errorf("%d commits have the header %q: %v", len(shas), header, shas)
	}
	if got != shas[0] {
		return fmt.Errorf("the JSON's %s is %q, not the commit %q, %s\n%s", p, got, header, shas[0], w.report())
	}
	return nil
}

// The field of each entry of the list under the key, in order, joined by
// ", ", is the list given.
func (w *world) jsonListFieldsAre(field, key, want string) error {
	entries, err := w.jsonList(key)
	if err != nil {
		return err
	}
	var got []string
	for _, entry := range entries {
		got = append(got, fmt.Sprint(entry[field]))
	}
	if joined := strings.Join(got, ", "); joined != want {
		return fmt.Errorf("the %q of the JSON's %q entries are %q, not %q\n%s", field, key, joined, want, w.report())
	}
	return nil
}

// jsonListEntry is the one entry of the list under the key whose field is
// the value; none or several is an error.
func (w *world) jsonListEntry(key, field, value string) (map[string]any, error) {
	entries, err := w.jsonList(key)
	if err != nil {
		return nil, err
	}
	var found []map[string]any
	for _, entry := range entries {
		if fmt.Sprint(entry[field]) == value {
			found = append(found, entry)
		}
	}
	if len(found) != 1 {
		return nil, fmt.Errorf("the JSON's %q has %d entries whose %q is %q, not one\n%s", key, len(found), field, value, w.report())
	}
	return found[0], nil
}

// The entry's other field is the JSON value given.
func (w *world) jsonListEntryHas(key, field, value, other, want string) error {
	entry, err := w.jsonListEntry(key, field, value)
	if err != nil {
		return err
	}
	var expected any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		return fmt.Errorf("the scenario's %s is not JSON: %v", want, err)
	}
	if got, has := entry[other]; !has || !reflect.DeepEqual(got, expected) {
		return fmt.Errorf("the JSON's %q entry whose %q is %q has %q %s, not %s\n%s", key, field, value, other, jsonText(got), want, w.report())
	}
	return nil
}

// The entry's other field is git's blob id of the file at HEAD.
func (w *world) jsonListEntryHasBlob(key, field, value, other, file string) error {
	entry, err := w.jsonListEntry(key, field, value)
	if err != nil {
		return err
	}
	blob, err := w.gitOutput("rev-parse", "HEAD:"+file)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(entry[other]); got != strings.TrimSpace(blob) {
		return fmt.Errorf("the JSON's %q entry whose %q is %q has %q %q, not the blob %s of %s at HEAD\n%s", key, field, value, other, got, strings.TrimSpace(blob), file, w.report())
	}
	return nil
}
