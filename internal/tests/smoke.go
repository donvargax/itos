package tests

import (
	"errors"
	"fmt"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/source"
	"github.com/donvargax/itos/v5/internal/value"
)

// SmokeScenario is one test of a smoke set, with why it is there.
type SmokeScenario struct {
	ID  string
	Why string
}

// SmokeFile is one entry of a kind's smoke set (tests.<kind>.smoke.file), a
// YAML list of { file, more, scenarios: [{ id, why }] }: a file relative to
// the kind's root, why it has more than one (required when it does), and its
// scenarios, each with why it is there.
type SmokeFile struct {
	File      string
	More      *string
	Scenarios []SmokeScenario
}

// SmokeFileOf is the kind's smoke file, as the config names it.
func SmokeFileOf(cfg *config.Loaded, name string) (string, error) {
	k, err := KindOf(cfg, name)
	if err != nil {
		return "", err
	}
	if k.Smoke.File == nil || *k.Smoke.File == "" {
		return "", errors.New("tests." + name + ".smoke.file is missing")
	}
	return *k.Smoke.File, nil
}

// scenarioProblem is what is wrong with one scenario of an entry, if
// anything. A missing why is the smoke rule's to report, so it reads as empty.
func scenarioProblem(s any, at string) (SmokeScenario, string) {
	m, ok := s.(*value.Map)
	id, isString := value.Prop(s, "id").(string)
	if !ok || !isString {
		return SmokeScenario{}, at + " has a scenario without an id"
	}
	for _, k := range m.Keys() {
		if k != "id" && k != "why" {
			return SmokeScenario{}, fmt.Sprintf("%s: %s has an unknown key %s", at, id, k)
		}
	}
	why := m.At("why")
	if why == value.Undefined {
		why = ""
	}
	text, ok := why.(string)
	if !ok {
		return SmokeScenario{}, fmt.Sprintf("%s: %s's why is not text", at, id)
	}
	return SmokeScenario{ID: id, Why: text}, ""
}

// entryProblem is what is wrong with one entry of the list, if anything.
func entryProblem(e any, n int) (SmokeFile, string) {
	m, ok := e.(*value.Map)
	if !ok {
		return SmokeFile{}, fmt.Sprintf("entry %d is not a map", n)
	}
	file, ok := m.At("file").(string)
	if !ok {
		return SmokeFile{}, fmt.Sprintf("entry %d has no file", n)
	}
	at := fmt.Sprintf("entry %d (%s)", n, file)
	for _, k := range m.Keys() {
		if k != "file" && k != "more" && k != "scenarios" {
			return SmokeFile{}, at + " has an unknown key " + k
		}
	}
	entry := SmokeFile{File: file}
	if more := m.At("more"); more != value.Undefined {
		text, ok := more.(string)
		if !ok {
			return SmokeFile{}, at + ": more is not text"
		}
		entry.More = &text
	}
	scenarios, ok := m.At("scenarios").([]any)
	if !ok {
		return SmokeFile{}, at + " has no list of scenarios"
	}
	entry.Scenarios = []SmokeScenario{}
	for _, s := range scenarios {
		scenario, problem := scenarioProblem(s, at)
		if problem != "" {
			return SmokeFile{}, problem
		}
		entry.Scenarios = append(entry.Scenarios, scenario)
	}
	return entry, ""
}

// parseSmoke is a smoke list's text held to its shape, or an error naming the
// file and the entry.
func parseSmoke(text, where string) ([]SmokeFile, error) {
	parsed, err := value.Parse(text)
	if err != nil {
		return nil, err
	}
	if parsed == nil {
		parsed = []any{}
	}
	list, ok := parsed.([]any)
	if !ok {
		return nil, errors.New(where + ": is not a list of files")
	}
	smoke := []SmokeFile{}
	for i, e := range list {
		entry, problem := entryProblem(e, i+1)
		if problem != "" {
			return nil, errors.New(where + ": " + problem)
		}
		smoke = append(smoke, entry)
	}
	return smoke, nil
}

// LoadSmoke is the kind's smoke set in the working tree.
func LoadSmoke(cfg *config.Loaded, name string) ([]SmokeFile, error) {
	path, err := SmokeFileOf(cfg, name)
	if err != nil {
		return nil, err
	}
	if !source.Has(path) {
		return nil, fmt.Errorf("%s is missing (tests.%s.smoke.file)", path, name)
	}
	text, err := source.Read(path)
	if err != nil {
		return nil, err
	}
	return parseSmoke(text, path)
}

// LoadSmokeAt is the kind's smoke set at a commit (smoke.ts's loadSmoke with
// `at`), as `ci plan --data-at` reads it, or LoadSmoke's when at is "". A
// commit without the file is an error saying so, and a problem with its
// shape names the file at the commit (`<sha>:<path>`).
func LoadSmokeAt(cfg *config.Loaded, name, at string) ([]SmokeFile, error) {
	if at == "" {
		return LoadSmoke(cfg, name)
	}
	path, err := SmokeFileOf(cfg, name)
	if err != nil {
		return nil, err
	}
	tree, err := source.At(at)
	if err != nil {
		return nil, err
	}
	text, err := tree.Read(path)
	if err != nil {
		return nil, fmt.Errorf("%s holds no %s", at, path)
	}
	return parseSmoke(text, at+":"+path)
}

// SmokeIDs are the smoke set's IDs, without the tag prefix.
func SmokeIDs(k config.Kind, smoke []SmokeFile) []string {
	ids := []string{}
	for _, e := range smoke {
		for _, s := range e.Scenarios {
			ids = append(ids, BareID(k, s.ID))
		}
	}
	return ids
}

// liveScenarios are each file's live test IDs, keyed by its path relative to
// the kind's root, in the order the adapter lists the files; a file with no
// live test is there with none.
func liveScenarios(cfg *config.Loaded, name string, root *string) ([]string, map[string]map[string]bool, error) {
	list, err := ListTestsUnder(cfg, name, source.Current().Tree(), root)
	if err != nil {
		return nil, nil, err
	}
	live := map[string]map[string]bool{}
	for _, f := range list.Files {
		live[f] = map[string]bool{}
	}
	for _, t := range list.Tests {
		if ids, ok := live[t.File]; ok && t.Live {
			ids[t.ID] = true
		}
	}
	return list.Files, live, nil
}

// addHint is how the smoke rule tells a file without a smoke test to get one.
func addHint(k config.Kind) string {
	if k.Smoke.AddHint != nil {
		return *k.Smoke.AddHint
	}
	return "add one to " + value.String(ptrValue(k.Smoke.File))
}

func ptrValue(p *string) any {
	if p == nil {
		return value.Undefined
	}
	return *p
}

func entryProblems(k config.Kind, entry SmokeFile, live map[string]bool) []out.Problem {
	edit := "edit " + value.String(ptrValue(k.Smoke.File))
	if live == nil {
		return []out.Problem{{
			Rule:    "smoke-not-a-file",
			Message: fmt.Sprintf("the smoke list names %s, which is not a feature file", entry.File),
			Fix:     fmt.Sprintf("%s: remove the entry for %s, or correct its path", edit, entry.File),
		}}
	}
	var found []out.Problem
	noWhy := false
	for _, s := range entry.Scenarios {
		if !live[BareID(k, s.ID)] {
			found = append(found, out.Problem{
				Rule:    "smoke-not-live",
				Message: fmt.Sprintf("the smoke list names %s, which is not a live scenario of %s", s.ID, entry.File),
				Fix:     fmt.Sprintf("%s: name a live scenario of %s in place of %s", edit, entry.File, s.ID),
			})
		}
		if value.Trim(s.Why) == "" {
			noWhy = true
		}
	}
	if noWhy {
		found = append(found, out.Problem{
			Rule:    "smoke-no-why",
			Message: fmt.Sprintf("a smoke scenario of %s does not say why it is there", entry.File),
			Fix:     fmt.Sprintf("%s: give each scenario of %s a why", edit, entry.File),
		})
	}
	if len(entry.Scenarios) > 1 && (entry.More == nil || value.Trim(*entry.More) == "") {
		found = append(found, out.Problem{
			Rule:    "smoke-more-no-why",
			Message: fmt.Sprintf("%s has more than one smoke scenario and does not say why", entry.File),
			Fix:     fmt.Sprintf("%s: give %s a more: that says why it has more than one", edit, entry.File),
		})
	}
	return found
}

// SmokeIssues is what breaks the smoke rule, with its rule id: a file with a
// live test and no smoke test (unless the kind's smoke.every_file is false),
// a smoke ID that is not a live test of its file, a missing reason. root,
// when not nil, stands in for a Gherkin kind's own (ListTestsUnder).
func SmokeIssues(cfg *config.Loaded, name string, smoke []SmokeFile, root *string) ([]out.Problem, error) {
	k, err := KindOf(cfg, name)
	if err != nil {
		return nil, err
	}
	files, live, err := liveScenarios(cfg, name, root)
	if err != nil {
		return nil, err
	}
	listed := map[string]bool{}
	for _, e := range smoke {
		if len(e.Scenarios) > 0 {
			listed[e.File] = true
		}
	}
	var found []out.Problem
	for _, file := range files {
		if k.Smoke.EveryFile && len(live[file]) > 0 && !listed[file] {
			hint := addHint(k)
			found = append(found, out.Problem{
				Rule:    "smoke-missing",
				Message: fmt.Sprintf("%s has no smoke scenario (%s)", file, hint),
				Fix:     hint,
			})
		}
	}
	for _, entry := range smoke {
		found = append(found, entryProblems(k, entry, live[entry.File])...)
	}
	return found, nil
}
