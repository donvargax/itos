// The steps of task promotion (work.feature, slice 138): a committed idea
// with every kind of metadata a promotion keeps (an owner, a done
// dependency, refs, a declared tag, a place in the queue), an item that
// depends on it and a question that names it; the defects a promotion
// refuses before it reserves an id; a hook that refuses the promotion's
// commit; the fixture moved under a stealth config; the repository's state,
// the id counter's included, saved and compared; and what the promotion left
// in the ledger, the registry and the questions. A generic step reads a
// value of the JSON output by its dotted path.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

// The task-promotion fixture's idea as it was committed, and what the
// steps learn of its promotion.
type promotionFixture struct {
	idea, title, why, owner string
	dependsOn, refs, tags   []string
	queueAt                 int      // the idea's place in the queue, from 0
	dependants              []string // the items whose depends_on name the idea
	questions               []string // the questions that name the idea
	head                    string   // HEAD once the fixture is committed
	newID                   string   // the promoted item's id, read once from the JSON
	saved                   *promotionState
	hookRecord              string // where the rejecting hook records that it ran
}

// The repository as a promotion could change it: HEAD, the index, what git
// status says, every file of the working tree and of itos's folder in the
// git folder, and the id counter's file ("" when there is none).
type promotionState struct {
	head, index, status, counter string
	files                        map[string]string
}

// asksPath is where the scratch repository's questions are: work.asks's
// default, beside the registry.
var asksPath = filepath.Join(filepath.Dir(startingRegistry), "asks.yaml")

func initializePromoteSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a committed task-promotion fixture for the idea "([^"]*)"$`, w.taskPromotionFixture)
	sc.Step(`^the promotion fixture has "([^"]*)"$`, w.promotionDefect)
	sc.Step(`^the promotion fixture uses a stealth config$`, w.promotionStealth)
	sc.Step(`^a configured hook that records and rejects promotion commits$`, w.rejectingPromotionHook)
	sc.Step(`^an unrelated change to "([^"]*)" is staged$`, w.unrelatedStagedChange)
	sc.Step(`^the promotion state is saved for comparison$`, w.savePromotionState)

	sc.Step(`^its JSON's "([^"]*)" is (.+)$`, w.jsonValueIs)
	sc.Step(`^the promoted ledger task has type "([^"]*)" and the idea's title and why$`, func(kind string) error {
		if w.promotion == nil {
			return errors.New("no task-promotion fixture")
		}
		return w.promotedLedgerTaskIs(kind, w.promotion.title, w.promotion.why)
	})
	sc.Step(`^the promoted ledger task has title "([^"]*)" and why "([^"]*)"$`, func(title, why string) error {
		return w.promotedLedgerTaskIs("", title, why)
	})
	sc.Step(`^the promoted ledger task's checks equal:$`, w.promotedChecksEqual)
	sc.Step(`^the promoted item preserves the idea's phase owner dependencies refs tags and queue position$`, w.promotedItemPreserved)
	sc.Step(`^every dependency and question naming "([^"]*)" names the promoted task instead$`, w.promotedReferences)
	sc.Step(`^the promoted registry item has no why copy$`, w.promotedItemHasNoWhy)
	sc.Step(`^a commit message naming the promoted task passes its configured ledger-footer check$`, w.promotedTaskFooterPasses)
	sc.Step(`^the promotion commit contains the ledger registry and rewritten questions together$`, w.promotionCommitTogether)
	sc.Step(`^the promotion files index HEAD and ID counter match the saved state$`, func() error {
		return w.promotionStateMatches(true)
	})
	sc.Step(`^the promotion files index and HEAD match the saved state$`, func() error {
		return w.promotionStateMatches(false)
	})
	sc.Step(`^the rejecting promotion hook ran$`, w.rejectingPromotionHookRan)
	sc.Step(`^the configured stealth ledger contains the promoted task$`, w.stealthLedgerHasPromoted)
	sc.Step(`^HEAD and the working tree are unchanged by the promotion$`, w.headAndTreeUnchanged)
	sc.Step(`^the promoted slice has no ledger entry$`, func() error {
		id, err := w.promotedID()
		if err != nil {
			return err
		}
		return w.ledgerLacks(id)
	})
}

// The fixture: the config declares the tag archive, the registry holds a
// done task (T-001, the Background's), an idea queued before the one given,
// the idea itself (owned by someone, whom the people list, depending on
// T-001, with refs, the tag and a why) and an idea depending on it, the
// queue naming all three ideas; committed past the hooks, then a question
// naming the idea, added by itos decision add.
func (w *world) taskPromotionFixture(idea string) error {
	if len(w.ledger) != 1 || w.ledger[0].id != "T-001" {
		return errors.New(`the fixture needs the Background's ledger of the one task "T-001"`)
	}
	p := &promotionFixture{
		idea: idea, title: "Archive the reviewed inputs", why: "Reviewed inputs are lost between runs.",
		owner: "someone", dependsOn: []string{"T-001"}, refs: []string{"docs/archive.md"}, tags: []string{"archive"},
		queueAt: 1, dependants: []string{"p1-later"},
	}
	w.config.workTags = []string{"archive"}
	if err := w.writeConfig(); err != nil {
		return err
	}
	registry := fmt.Sprintf(`phases: { 1: null }
items:
  - { id: T-001, title: Tidy, phase: 1, owner: null, status: done, kind: task, depends_on: [] }
  - { id: p1-first, title: First idea, phase: 1, owner: null, status: todo, kind: idea, depends_on: [], why: Comes first. }
  - id: %[1]s
    title: %[2]s
    phase: 1
    owner: %[3]s
    status: todo
    kind: idea
    depends_on: [%[4]s]
    refs: [%[5]s]
    tags: [%[6]s]
    why: >
      %[7]s
  - { id: p1-later, title: Later idea, phase: 1, owner: null, status: todo, kind: idea, depends_on: [%[1]s], why: Comes after. }
queue: [p1-first, %[1]s, p1-later]
`, idea, p.title, p.owner, strings.Join(p.dependsOn, ", "), strings.Join(p.refs, ", "), strings.Join(p.tags, ", "), p.why)
	if err := w.write(w.data(startingRegistry), registry); err != nil {
		return err
	}
	w.registryText = registry
	if err := w.commit("docs: the promotion fixture"); err != nil {
		return err
	}
	if err := w.hasRunLine(w.dir, "decision add 'How are the inputs archived?' --item "+idea); err != nil {
		return err
	}
	asks, err := w.questionsNaming(idea)
	if err != nil {
		return err
	}
	if len(asks) != 1 {
		return fmt.Errorf("decision add left %d questions naming %s, not one", len(asks), idea)
	}
	p.questions = asks
	if p.head, err = w.head(); err != nil {
		return err
	}
	w.promotion = p
	return nil
}

// One defect, committed past the hooks but for the ledger edit, which is
// left unstaged.
func (w *world) promotionDefect(problem string) error {
	p := w.promotion
	if p == nil {
		return errors.New("no task-promotion fixture")
	}
	registry := filepath.Join(w.dir, w.data(startingRegistry))
	edit := func(path, old, new string) error {
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Count(string(text), old) != 1 {
			return fmt.Errorf("%s does not hold %q once:\n%s", path, old, text)
		}
		return os.WriteFile(path, []byte(strings.Replace(string(text), old, new, 1)), 0o644)
	}
	switch problem {
	case "an undeclared commit type":
		// The When's --type undeclared, which commits.types does not list.
		text, err := os.ReadFile(filepath.Join(w.dir, w.data("itos.yaml")))
		if err != nil {
			return err
		}
		if strings.Contains(string(text), "undeclared") {
			return fmt.Errorf("the config declares undeclared:\n%s", text)
		}
		return nil
	case "an idea phase outside the ledger pattern":
		// The default pattern, [^/]+, takes any phase a registry can list.
		if err := w.configSets("ledger.group.pattern", `\d+`); err != nil {
			return err
		}
		if err := edit(registry, "phases: { 1: null }", "phases: { 1: null, two: null }"); err != nil {
			return err
		}
		if err := edit(registry, "  - id: "+p.idea+"\n    title: "+p.title+"\n    phase: 1\n",
			"  - id: "+p.idea+"\n    title: "+p.title+"\n    phase: two\n"); err != nil {
			return err
		}
	case "an unknown dependency":
		if err := edit(registry, "    depends_on: ["+strings.Join(p.dependsOn, ", ")+"]\n",
			"    depends_on: ["+strings.Join(append(slices.Clone(p.dependsOn), "p1-missing"), ", ")+"]\n"); err != nil {
			return err
		}
	case "an unreadable linked questions file":
		path := filepath.Join(w.dir, w.data(asksPath))
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, append(text, []byte("    colour: red\n")...), 0o644); err != nil {
			return err
		}
	case "an unstaged edit to the target ledger":
		path := filepath.Join(w.dir, w.data(w.ledgerPath()))
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(text, []byte("# An edit no commit holds.\n")...), 0o644)
	default:
		return fmt.Errorf("no such promotion defect: %q", problem)
	}
	if err := w.commit("docs: a defect"); err != nil {
		return err
	}
	var err error
	p.head, err = w.head()
	return err
}

// The fixture's config, people, ledger, registry and questions moved into
// the git folder, the stealth config, their removal from the root committed.
func (w *world) promotionStealth() error {
	p := w.promotion
	if p == nil {
		return errors.New("no task-promotion fixture")
	}
	if w.dataDir != "" {
		return errors.New("the config is already kept in the git folder")
	}
	for _, path := range []string{"itos.yaml", "people.yaml", w.ledgerPath(), startingRegistry, asksPath} {
		text, err := os.ReadFile(filepath.Join(w.dir, path))
		if err != nil {
			return err
		}
		if err := w.write(filepath.Join(stealthDir, path), string(text)); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(w.dir, path)); err != nil {
			return err
		}
	}
	w.dataDir = stealthDir
	if err := w.commit("docs: keep itos in the git folder"); err != nil {
		return err
	}
	var err error
	p.head, err = w.head()
	return err
}

// A commit-msg hook declared in the git config, beside itos's, that records
// it ran and refuses a commit whose header is a promotion's.
func (w *world) rejectingPromotionHook() error {
	if w.promotion == nil {
		return errors.New("no task-promotion fixture")
	}
	w.promotion.hookRecord = filepath.Join(w.support, "promotion-hook-ran")
	dir := filepath.Join(w.support, "promotion-hook")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "reject-promotion")
	script := "#!/bin/sh\ncase \"$(head -n 1 \"$1\")\" in\n\"docs: promote \"*)\n" +
		"  echo ran >> " + quote(filepath.ToSlash(w.promotion.hookRecord)) + "\n" +
		"  echo 'the hook refuses the promotion' >&2\n  exit 1 ;;\nesac\n"
	if err := w.writeProgram(path, script); err != nil {
		return err
	}
	named := filepath.ToSlash(path)
	if strings.ContainsAny(named, " '\"") {
		named = quote(named)
	}
	if err := w.git("config", "--local", "hook.reject-promotion.event", "commit-msg"); err != nil {
		return err
	}
	return w.git("config", "--local", "hook.reject-promotion.command", named)
}

func (w *world) rejectingPromotionHookRan() error {
	if w.promotion == nil || w.promotion.hookRecord == "" {
		return errors.New("no rejecting promotion hook")
	}
	text, err := os.ReadFile(w.promotion.hookRecord)
	if err != nil || !strings.Contains(string(text), "ran") {
		return fmt.Errorf("the rejecting promotion hook did not run: %v\n%s", err, w.report())
	}
	return nil
}

// The file, new or changed, staged.
func (w *world) unrelatedStagedChange(path string) error {
	if err := w.write(path, "# Notes\n\nAn unrelated change.\n"); err != nil {
		return err
	}
	return w.git("add", "--", path)
}

func (w *world) savePromotionState() error {
	if w.promotion == nil {
		return errors.New("no task-promotion fixture")
	}
	state, err := w.promotionStateNow()
	if err != nil {
		return err
	}
	w.promotion.saved = state
	return nil
}

// The counter's file: the one a clone with no remote, or a stealth config,
// keeps in itos's folder of the git common dir.
var counterFile = filepath.Join(".git", "itos", "ids.yaml")

func (w *world) promotionStateNow() (*promotionState, error) {
	s := &promotionState{files: map[string]string{}}
	var err error
	if s.head, err = w.head(); err != nil {
		return nil, err
	}
	if s.index, err = w.gitOutput("ls-files", "--stage"); err != nil {
		return nil, err
	}
	if s.status, err = w.gitOutput("status", "--porcelain", "--untracked-files=all"); err != nil {
		return nil, err
	}
	if counter, err := os.ReadFile(filepath.Join(w.dir, counterFile)); err == nil {
		s.counter = string(counter)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	err = filepath.WalkDir(w.dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(w.dir, path)
		if err != nil {
			return err
		}
		inGit := rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator))
		ours := rel == stealthDir || strings.HasPrefix(rel, stealthDir+string(filepath.Separator))
		switch {
		case entry.IsDir() && inGit && rel != ".git" && !ours:
			return filepath.SkipDir
		case entry.IsDir(), inGit && !ours, rel == counterFile:
			return nil
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s.files[filepath.ToSlash(rel)] = string(text)
		return nil
	})
	return s, err
}

// The repository is as saved: HEAD, the index, git status and every file;
// and the id counter too when counter is true.
func (w *world) promotionStateMatches(counter bool) error {
	if w.promotion == nil || w.promotion.saved == nil {
		return errors.New("no promotion state saved")
	}
	saved := w.promotion.saved
	now, err := w.promotionStateNow()
	if err != nil {
		return err
	}
	var differ []string
	if now.head != saved.head {
		differ = append(differ, fmt.Sprintf("HEAD is %s, not %s", now.head, saved.head))
	}
	if now.index != saved.index {
		differ = append(differ, fmt.Sprintf("the index is\n%s\nnot\n%s", now.index, saved.index))
	}
	if now.status != saved.status {
		differ = append(differ, fmt.Sprintf("git status is\n%s\nnot\n%s", now.status, saved.status))
	}
	if counter && now.counter != saved.counter {
		differ = append(differ, fmt.Sprintf("the id counter is\n%s\nnot\n%s", now.counter, saved.counter))
	}
	differ = append(differ, filesDiffer(saved.files, now.files)...)
	if len(differ) > 0 {
		return fmt.Errorf("the repository changed: %s\n%s", strings.Join(differ, "; "), w.report())
	}
	return nil
}

// filesDiffer names each file that is in one map and not the other, or
// whose text differs.
func filesDiffer(before, after map[string]string) []string {
	var differ []string
	for path, text := range before {
		if now, ok := after[path]; !ok {
			differ = append(differ, path+" is gone")
		} else if now != text {
			differ = append(differ, fmt.Sprintf("%s is\n%s\nnot\n%s", path, now, text))
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			differ = append(differ, path+" is new")
		}
	}
	slices.Sort(differ)
	return differ
}

// Under a stealth config, HEAD is the fixture's and nothing outside the git
// folder changed.
func (w *world) headAndTreeUnchanged() error {
	if w.promotion == nil || w.promotion.saved == nil {
		return errors.New("no promotion state saved")
	}
	saved := w.promotion.saved
	now, err := w.promotionStateNow()
	if err != nil {
		return err
	}
	var differ []string
	if now.head != saved.head {
		differ = append(differ, fmt.Sprintf("HEAD is %s, not %s", now.head, saved.head))
	}
	if now.status != saved.status {
		differ = append(differ, fmt.Sprintf("git status is\n%s\nnot\n%s", now.status, saved.status))
	}
	tree := func(files map[string]string) map[string]string {
		kept := map[string]string{}
		for path, text := range files {
			if !strings.HasPrefix(path, ".git/") {
				kept[path] = text
			}
		}
		return kept
	}
	differ = append(differ, filesDiffer(tree(saved.files), tree(now.files))...)
	if len(differ) > 0 {
		return fmt.Errorf("the promotion changed the project: %s\n%s", strings.Join(differ, "; "), w.report())
	}
	return nil
}

// Standard output is one JSON object whose value at the dotted path is the
// JSON given: a quoted text, a number, true, false, null, a list or an
// object.
func (w *world) jsonValueIs(path, want string) error {
	var got any
	if err := json.Unmarshal([]byte(w.stdout), &got); err != nil {
		return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	for _, key := range strings.Split(path, ".") {
		object, ok := got.(map[string]any)
		if !ok {
			return fmt.Errorf("the JSON has no %s: %v is not an object\n%s", path, got, w.report())
		}
		if got, ok = object[key]; !ok {
			return fmt.Errorf("the JSON has no %s\n%s", path, w.report())
		}
	}
	var expected any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		return fmt.Errorf("the scenario's %s is not JSON: %v", want, err)
	}
	if !reflect.DeepEqual(got, expected) {
		return fmt.Errorf("the JSON's %s is %s, not %s\n%s", path, jsonText(got), want, w.report())
	}
	return nil
}

func jsonText(v any) string {
	text, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(text)
}

// The promoted item's id, as the promotion's JSON output gave it.
func (w *world) promotedID() (string, error) {
	p := w.promotion
	if p == nil {
		return "", errors.New("no task-promotion fixture")
	}
	if p.newID != "" {
		return p.newID, nil
	}
	var result struct {
		Item struct {
			ID string `json:"id"`
		} `json:"item"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &result); err != nil || result.Item.ID == "" {
		return "", fmt.Errorf("the output names no promoted item: %v\n%s", err, w.report())
	}
	p.newID = result.Item.ID
	return p.newID, nil
}

// The promoted task in the ledger, any of its files, as YAML reads it.
func (w *world) promotedLedgerTask() (map[string]any, error) {
	id, err := w.promotedID()
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(w.dir, w.data(strings.ReplaceAll(w.ledgerFilesPattern(), "{group}", "*"))))
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var tasks []map[string]any
		if err := yaml.Unmarshal(text, &tasks); err != nil {
			return nil, fmt.Errorf("%s does not read: %w\n%s", file, err, text)
		}
		for _, t := range tasks {
			if fmt.Sprint(t["id"]) == id {
				return t, nil
			}
		}
	}
	return nil, fmt.Errorf("no ledger file has the task %s\n%s", id, w.report())
}

// The promoted ledger task has the type (any when ""), the title and the why,
// its why's folding and final newline aside.
func (w *world) promotedLedgerTaskIs(kind, title, why string) error {
	task, err := w.promotedLedgerTask()
	if err != nil {
		return err
	}
	var differ []string
	if kind != "" && task["type"] != kind {
		differ = append(differ, fmt.Sprintf("its type is %v, not %s", task["type"], kind))
	}
	if task["title"] != title {
		differ = append(differ, fmt.Sprintf("its title is %v, not %s", task["title"], title))
	}
	if got, _ := task["why"].(string); strings.TrimSpace(got) != why {
		differ = append(differ, fmt.Sprintf("its why is %q, not %q", got, why))
	}
	if len(differ) > 0 {
		return fmt.Errorf("the promoted task %s: %s\n%s", task["id"], strings.Join(differ, "; "), w.report())
	}
	return nil
}

// The promoted ledger task's done_when, as JSON, equals the doc string's.
func (w *world) promotedChecksEqual(doc *godog.DocString) error {
	task, err := w.promotedLedgerTask()
	if err != nil {
		return err
	}
	var got, want any
	if err := json.Unmarshal([]byte(jsonText(task["done_when"])), &got); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(doc.Content), &want); err != nil {
		return fmt.Errorf("the scenario's checks are not JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("the promoted task's checks are %s, not %s\n%s", jsonText(got), doc.Content, w.report())
	}
	return nil
}

// The registry's item under the promoted id is a task in the idea's phase,
// with its owner, dependencies, refs and tags, at the idea's place in the
// queue.
func (w *world) promotedItemPreserved() error {
	p := w.promotion
	id, err := w.promotedID()
	if err != nil {
		return err
	}
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	var differ []string
	if item["kind"] != "task" {
		differ = append(differ, fmt.Sprintf("its kind is %v, not task", item["kind"]))
	}
	if fmt.Sprint(item["phase"]) != "1" {
		differ = append(differ, fmt.Sprintf("its phase is %v, not 1", item["phase"]))
	}
	if item["owner"] != p.owner {
		differ = append(differ, fmt.Sprintf("its owner is %v, not %s", item["owner"], p.owner))
	}
	for key, want := range map[string][]string{"depends_on": p.dependsOn, "refs": p.refs, "tags": p.tags} {
		if got := texts(item[key]); !slices.Equal(got, want) {
			differ = append(differ, fmt.Sprintf("its %s is %q, not %q", key, got, want))
		}
	}
	queue, err := w.queueRead()
	if err != nil {
		return err
	}
	if at := slices.Index(queue, id); at != p.queueAt {
		differ = append(differ, fmt.Sprintf("it is at %d in the queue %q, not %d", at, queue, p.queueAt))
	}
	if len(differ) > 0 {
		return fmt.Errorf("the promoted item %s: %s\n%s", id, strings.Join(differ, "; "), w.report())
	}
	return nil
}

// texts is a YAML list read as text, nil for none.
func texts(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, x := range list {
		out = append(out, fmt.Sprint(x))
	}
	return out
}

// The registry's queue, as text.
func (w *world) queueRead() ([]string, error) {
	text, err := os.ReadFile(filepath.Join(w.dir, w.data(startingRegistry)))
	if err != nil {
		return nil, err
	}
	var registry struct {
		Queue []any `yaml:"queue"`
	}
	if err := yaml.Unmarshal(text, &registry); err != nil {
		return nil, fmt.Errorf("the registry does not read: %w\n%s", err, text)
	}
	return texts(registry.Queue), nil
}

// The questions whose item is the id.
func (w *world) questionsNaming(id string) ([]string, error) {
	text, err := os.ReadFile(filepath.Join(w.dir, w.data(asksPath)))
	if err != nil {
		return nil, err
	}
	var asks struct {
		Questions []struct {
			ID   string `yaml:"id"`
			Item string `yaml:"item"`
		} `yaml:"questions"`
	}
	if err := yaml.Unmarshal(text, &asks); err != nil {
		return nil, fmt.Errorf("the questions do not read: %w\n%s", err, text)
	}
	var named []string
	for _, q := range asks.Questions {
		if q.Item == id {
			named = append(named, q.ID)
		}
	}
	return named, nil
}

// No item depends on the idea, and every item and question that named it
// names the promoted id.
func (w *world) promotedReferences(idea string) error {
	p := w.promotion
	if p == nil || p.idea != idea {
		return fmt.Errorf("no task-promotion fixture for %s", idea)
	}
	id, err := w.promotedID()
	if err != nil {
		return err
	}
	items, err := w.registryItems()
	if err != nil {
		return err
	}
	var differ []string
	for _, item := range items {
		deps := texts(item["depends_on"])
		if slices.Contains(deps, idea) {
			differ = append(differ, fmt.Sprintf("%v still depends on %s", item["id"], idea))
		}
		if slices.Contains(p.dependants, fmt.Sprint(item["id"])) && !slices.Contains(deps, id) {
			differ = append(differ, fmt.Sprintf("%v does not depend on %s", item["id"], id))
		}
	}
	if old, err := w.questionsNaming(idea); err != nil {
		return err
	} else if len(old) > 0 {
		differ = append(differ, fmt.Sprintf("%s still name %s", strings.Join(old, ", "), idea))
	}
	if named, err := w.questionsNaming(id); err != nil {
		return err
	} else if !slices.Equal(named, p.questions) {
		differ = append(differ, fmt.Sprintf("the questions naming %s are %q, not %q", id, named, p.questions))
	}
	if len(differ) > 0 {
		return fmt.Errorf("%s\n%s", strings.Join(differ, "; "), w.report())
	}
	return nil
}

// The registry's item under the promoted id has no why: the ledger's entry
// holds it.
func (w *world) promotedItemHasNoWhy() error {
	id, err := w.promotedID()
	if err != nil {
		return err
	}
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	if why, ok := item["why"]; ok {
		return fmt.Errorf("the registry's %s has the why %q\n%s", id, why, w.report())
	}
	return nil
}

// itos's commit-msg hook passes a commit whose ledger footer names the
// promoted task.
func (w *world) promotedTaskFooterPasses() error {
	id, err := w.promotedID()
	if err != nil {
		return err
	}
	footer := w.config.ledgerFooter
	if footer == "" {
		footer = "Task"
	}
	if err := w.commitMsgHook(fmt.Sprintf("chore: use %s\n\n%s: %s\n", id, footer, id)); err != nil {
		return err
	}
	return w.exitsWith(0)
}

// The promotion is one commit past the fixture, which touches the ledger
// file, the registry and the questions, and nothing else.
func (w *world) promotionCommitTogether() error {
	p := w.promotion
	if p == nil {
		return errors.New("no task-promotion fixture")
	}
	count, err := w.gitOutput("rev-list", "--count", p.head+"..HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(count) != "1" {
		return fmt.Errorf("the promotion made %s commits, not one\n%s", strings.TrimSpace(count), w.report())
	}
	touched, err := w.gitOutput("show", "--name-only", "--format=", "HEAD")
	if err != nil {
		return err
	}
	got := strings.Fields(touched)
	want := []string{filepath.ToSlash(w.data(w.ledgerPath())), filepath.ToSlash(w.data(startingRegistry)), filepath.ToSlash(w.data(asksPath))}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return fmt.Errorf("the promotion commit touches %q, not %q\n%s", got, want, w.report())
	}
	return nil
}

// The stealth config's ledger, in the git folder, has the promoted task.
func (w *world) stealthLedgerHasPromoted() error {
	if w.dataDir != stealthDir {
		return errors.New("the scenario keeps no config in the git folder")
	}
	task, err := w.promotedLedgerTask()
	if err != nil {
		return err
	}
	if task["type"] != "build" {
		return fmt.Errorf("the promoted task's type is %v, not build\n%s", task["type"], w.report())
	}
	return nil
}
