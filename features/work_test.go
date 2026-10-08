// The steps of the commands that write the work registry (work.feature,
// slices 52 to 54) and the ledger (task add, task.feature, slice 55): a
// registry of items with owners, dependencies and ideas, committed, so a
// command's own commit holds only its change; a scenario tagged for an
// item; a ledger file deleted and a commit-msg hook that refuses, for a
// commit that cannot be made (bug 13); a pre-commit hook that rewrites the
// Markdown files it commits, as a formatter does (bug 17); and what the registry reads after
// (an item's status, owner, kind, title, dependencies and why), a ledger
// file's task, what the last commit holds, and what git status reports; an
// idea whose refs are a flow list over several lines or a block list, and
// the refs an item has after (bug 14); a command that has to succeed before
// the run a scenario is about, a queue written into the registry, and the
// registry's queue after (slice 66); the folder work.decisions names for
// itos decision record's decision records (slice 71); an item with a why, a task
// with none and a ledger task with one, and an item's why gone after (slice
// 76); a queue that no longer names an item and the last commit's body
// (slice 78); the last commit's body within a line length (bug 22); a
// registry commit made while work done waits for CI (bug 34); the tags the
// config declares, an item with tags and the tags an item has after (slice
// 97); an item deferred with its reason, and one not deferred (slice 98);
// an item with an owner that depends on another (bug 37).
package features

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

func initializeWorkSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the work registry has the item "([^"]*)" owned by nobody with the status "([^"]*)"$`, func(id, status string) error {
		return w.registryItem(id, "null", status, "", "")
	})
	sc.Step(`^the work registry has the item "([^"]*)" owned by "([^"]*)" with the status "([^"]*)"$`, func(id, owner, status string) error {
		return w.registryItem(id, owner, status, "", "")
	})
	sc.Step(`^the work registry has the item "([^"]*)" owned by nobody with the status "([^"]*)", depending on "([^"]*)"$`, func(id, status, dep string) error {
		return w.registryItem(id, "null", status, dep, "")
	})
	sc.Step(`^the work registry has the item "([^"]*)" owned by "([^"]*)" with the status "([^"]*)", depending on "([^"]*)"$`, func(id, owner, status, dep string) error {
		return w.registryItem(id, owner, status, dep, "")
	})
	sc.Step(`^the work registry has the item "([^"]*)" owned by "([^"]*)" with the status "([^"]*)" and the why "([^"]*)"$`, func(id, owner, status, why string) error {
		return w.registryItemWhy(id, owner, status, "", "", why, "")
	})
	sc.Step(`^the work registry has the item "([^"]*)" owned by nobody with the status "([^"]*)" and the tags "([^"]*)"$`, func(id, status, tags string) error {
		return w.registryItemWhy(id, "null", status, "", "", "", tags)
	})
	sc.Step(`^the work registry has the item "([^"]*)" owned by "([^"]*)" with the status "([^"]*)" and the tags "([^"]*)"$`, func(id, owner, status, tags string) error {
		return w.registryItemWhy(id, owner, status, "", "", "", tags)
	})
	sc.Step(`^the config's work\.tags is "([^"]*)"$`, func(tags string) error {
		w.config.workTags = strings.Split(tags, ", ")
		return w.writeConfig()
	})
	sc.Step(`^the work registry has the task "([^"]*)" owned by nobody with the status "([^"]*)" and no why$`, func(id, status string) error {
		return w.registryItem(id, "null", status, "", "task")
	})
	sc.Step(`^the ledger's task "([^"]*)" has the why "([^"]*)"$`, w.ledgerTaskWhy)
	sc.Step(`^the ledger's task "([^"]*)" has no checks and the why "([^"]*)"$`, w.ledgerTaskWithoutChecks)
	sc.Step(`^the work registry has the idea "([^"]*)" owned by nobody$`, func(id string) error {
		return w.registryItem(id, "null", "todo", "", "idea")
	})
	sc.Step(`^the work registry has the idea "([^"]*)" owned by nobody, its refs a flow list over several lines$`, func(id string) error {
		return w.registryIdeaWithRefs(id, "\n      [\n        features/x.feature,\n        docs/y.md,\n      ]\n")
	})
	sc.Step(`^the work registry has the idea "([^"]*)" owned by nobody, its refs a block list of "([^"]*)" and "([^"]*)"$`, func(id, a, b string) error {
		return w.registryIdeaWithRefs(id, fmt.Sprintf("\n      - %s\n      - %s\n", a, b))
	})
	sc.Step(`^a feature file with the scenario "([^"]*)" tagged "([^"]*)"$`, w.taggedScenario)
	sc.Step(`^itos has run "([^"]*)"$`, func(line string) error { return w.hasRunLine(w.dir, line) })
	sc.Step(`^the registry's queue names "([^"]*)"$`, w.registryQueueNames)
	sc.Step(`^while work done waits for CI, a commit takes "([^"]*)" for "([^"]*)"$`, w.takenWhileWaiting)
	sc.Step(`^work\.decisions is "([^"]*)"$`, func(path string) error {
		return w.configSets("work.decisions", path)
	})

	sc.Step(`^the registry's item "([^"]*)" has the status "([^"]*)" and the owner "([^"]*)"$`, w.registryItemIs)
	sc.Step(`^the registry's item "([^"]*)" has the status "([^"]*)" and no why$`, w.registryItemWithoutWhy)
	sc.Step(`^the work registry beside the config gives the item "([^"]*)" the status "([^"]*)"$`, func(id, status string) error {
		return w.registryItemIs(id, status, "")
	})
	sc.Step(`^the registry has no item "([^"]*)"$`, w.registryLacks)
	sc.Step(`^the registry is unchanged$`, w.registryUnchanged)
	sc.Step(`^the registry's item "([^"]*)" is a (slice|task) whose why starts with "([^"]*)"$`, w.registryItemKind)
	sc.Step(`^the registry's item "([^"]*)" depends on "([^"]*)"$`, w.registryItemDependsOn)
	sc.Step(`^the registry's item "([^"]*)" is an? (idea|slice|task) titled "([^"]*)" with the status "([^"]*)"$`, w.registryItemMade)
	sc.Step(`^the registry's item "([^"]*)" is titled "([^"]*)"$`, func(id, title string) error {
		return w.registryItemField(id, "title", title)
	})
	sc.Step(`^the registry's item "([^"]*)" has a why ending with "([^"]*)"$`, w.registryWhyEnds)
	sc.Step(`^the registry's item "([^"]*)" has the refs "([^"]*)"$`, func(id, refs string) error {
		return w.registryItemList(id, "refs", refs, ",")
	})
	sc.Step(`^the registry's item "([^"]*)" has no refs$`, func(id string) error { return w.registryItemList(id, "refs", "", ",") })
	sc.Step(`^the registry's item "([^"]*)" has the tags "([^"]*)"$`, func(id, tags string) error {
		return w.registryItemList(id, "tags", tags, ", ")
	})
	sc.Step(`^the registry's item "([^"]*)" has no tags$`, func(id string) error { return w.registryItemList(id, "tags", "", ", ") })
	sc.Step(`^the registry's item "([^"]*)" is deferred with the reason "([^"]*)"$`, w.registryItemDeferred)
	sc.Step(`^the registry's item "([^"]*)" is not deferred$`, func(id string) error { return w.registryItemDeferred(id, "") })
	sc.Step(`^the registry's queue is "([^"]*)"$`, w.registryQueueIs)
	sc.Step(`^the registry's queue is empty$`, func() error { return w.registryQueueIs("") })
	sc.Step(`^the registry's queue does not name "([^"]*)"$`, w.registryQueueLacks)
	sc.Step(`^the last commit's header is "([^"]*)"$`, w.lastHeaderIs)
	sc.Step(`^the last commit's header is not "([^"]*)"$`, w.lastHeaderIsNot)
	sc.Step(`^the last commit's body says "([^"]*)"$`, w.lastBodySays)
	sc.Step(`^the last commit's body has no line longer than (\d+) characters$`, w.lastBodyFits)
	sc.Step(`^the last commit touches only "([^"]*)"$`, func(path string) error { return w.lastTouchesOnly(path) })
	sc.Step(`^the last commit touches only "([^"]*)" and "([^"]*)"$`, func(a, b string) error { return w.lastTouchesOnly(a, b) })
	sc.Step(`^the ledger file "([^"]*)" has the task "([^"]*)" with the check "([^"]*)"$`, w.ledgerFileHasTask)
	sc.Step(`^"([^"]*)" is still staged$`, w.stillStaged)

	sc.Step(`^the ledger file "([^"]*)" is deleted and the deletion not committed$`, w.ledgerFileDeleted)
	sc.Step(`^a commit-msg hook that refuses every commit$`, w.refusingCommitMsgHook)
	sc.Step(`^a pre-commit hook that appends a line to each staged Markdown file and stages it again$`, w.formattingPreCommitHook)
	sc.Step(`^the ledger has no task "([^"]*)"$`, w.ledgerLacks)
	sc.Step(`^git reports no change to the working tree or the index$`, w.gitStatusClean)

	sc.Step(`^itos runs decision add with a question whose commit body would wrap to start a line with "([^"]*)"$`, w.askAddWrappingTo)
}

// askAddWrappingTo runs decision add with a question that ends in the text, its
// words before the text built so that a plain wrap of the commit body at the
// lint's body limit, as itos's own commits were wrapped before bug 18,
// breaks just before it. decision add's body is `Ask q-<n> ("<question>"), with
// itos decision add.`, so the body's prefix and the words before the text fill
// the first line to the limit exactly, and the text would start the second.
// The question's id is the next free one of the questions file.
func (w *world) askAddWrappingTo(text string) error {
	id, err := w.nextQuestion()
	if err != nil {
		return err
	}
	prefix := fmt.Sprintf("Ask %s (\"", id)
	return w.itos("decision", "add", wordsOf(bodyLimit-len(prefix))+" "+text)
}

// nextQuestion is the id decision add gives the next question: one past the
// highest the questions file, beside the registry, holds; q-1 with none.
func (w *world) nextQuestion() (string, error) {
	text, err := os.ReadFile(filepath.Join(w.dir, w.data(filepath.Join(filepath.Dir(startingRegistry), "asks.yaml"))))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	highest := 0
	for _, m := range regexp.MustCompile(`\bq-(\d+)\b`).FindAllStringSubmatch(string(text), -1) {
		if n, _ := strconv.Atoi(m[1]); n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("q-%d", highest+1), nil
}

// One more item of phase 1, a group nobody owns, in the registry where the
// scenario keeps itos's data: its owner (null for nobody), status, one
// dependency when given, a kind when given and a why when given. An owner is added to the
// people, so the registry stays sound, and in a project the registry and the
// people are committed past the hooks, so a command that commits the
// registry commits only its own change; a change already staged stays
// staged.
func (w *world) registryItem(id, owner, status, dep, kind string) error {
	return w.registryItemWhy(id, owner, status, dep, kind, "", "")
}

// registryItemWhy is registryItem with a why ("" for none), one quoted line
// after the item's dependencies (slice 76), and its tags, a list of the
// names given as "a, b" ("" for none, no key) after it (slice 97).
func (w *world) registryItemWhy(id, owner, status, dep, kind, why, tags string) error {
	deps := "[]"
	if dep != "" {
		deps = "[" + dep + "]"
	}
	line := fmt.Sprintf("  - { id: %s, title: %s, phase: 1, owner: %s, status: %s", id, id, owner, status)
	if kind != "" {
		line += ", kind: " + kind
	}
	line += ", depends_on: " + deps
	if why != "" {
		line += fmt.Sprintf(", why: %q", why)
	}
	if tags != "" {
		line += ", tags: [" + tags + "]"
	}
	w.registryLines = append(w.registryLines, line+" }\n")
	if err := w.writeRegistryLines(); err != nil {
		return err
	}
	if owner != "null" {
		people := filepath.Join(w.dir, w.data("people.yaml"))
		text, err := os.ReadFile(people)
		if err != nil {
			return err
		}
		if !slices.Contains(strings.Split(string(text), "\n"), "- "+owner) {
			if err := os.WriteFile(people, append(text, []byte("- "+owner+"\n")...), 0o644); err != nil {
				return err
			}
		}
	}
	if w.dataDir != "" {
		return nil
	}
	return w.commitLeavingStaged("docs: a registry")
}

// takenWhileWaiting has the fake GitHub, when it is first asked for the
// watched run, commit in the clone where itos runs a take of the item for
// the person, before it answers: the item's owner and status doing in the
// registry, the person added to the people file when it does not list them,
// in one commit past the hooks, as a take made in another session and
// pulled in lands while work done waits (bug 34).
func (w *world) takenWhileWaiting(id, owner string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	if w.dataDir != "" {
		return errors.New("a stealth registry is committed by nothing: this needs a project's")
	}
	registry, people := w.data(startingRegistry), w.data("people.yaml")
	item := regexp.MustCompile(`(?m)^(  - \{ id: ` + regexp.QuoteMeta(id) + `, .*?owner: )[^,]*(, status: )[^,]*`)
	w.github.firstLook = func() error {
		text, err := os.ReadFile(filepath.Join(w.dir, registry))
		if err != nil {
			return err
		}
		if !item.Match(text) {
			return fmt.Errorf("the registry has no item %q to take:\n%s", id, text)
		}
		taken := item.ReplaceAll(text, []byte("${1}"+owner+"${2}doing"))
		if err := os.WriteFile(filepath.Join(w.dir, registry), taken, 0o644); err != nil {
			return err
		}
		listed, err := os.ReadFile(filepath.Join(w.dir, people))
		if err != nil {
			return err
		}
		if !slices.Contains(strings.Split(string(listed), "\n"), "- "+owner) {
			if err := os.WriteFile(filepath.Join(w.dir, people), append(listed, []byte("- "+owner+"\n")...), 0o644); err != nil {
				return err
			}
		}
		if err := w.git("add", "--", registry, people); err != nil {
			return err
		}
		return w.git("commit", "-q", "--no-verify", "-m", "docs: take "+id)
	}
	return nil
}

// Every file committed as commit commits them, but for those already staged,
// which stay staged and out of the commit: a change the scenario staged
// first (its Background's) is for the commit it makes, not the registry's
// (slice 63).
func (w *world) commitLeavingStaged(message string) error {
	listed, err := w.gitOutput("diff", "--cached", "--name-only")
	if err != nil {
		return err
	}
	staged := strings.Fields(listed)
	if len(staged) == 0 {
		return w.commit(message)
	}
	if err := w.git("add", "-A"); err != nil {
		return err
	}
	if err := w.git(append([]string{"reset", "-q", "--"}, staged...)...); err != nil {
		return err
	}
	if err := w.git("commit", "-q", "--no-verify", "-m", message); err != nil {
		return err
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	w.commits = append(w.commits, sha)
	return w.git(append([]string{"add", "--"}, staged...)...)
}

// The registry's lines written where the scenario keeps itos's data, its
// one phase before them, and its queue after them when a step wrote one.
func (w *world) writeRegistryLines() error {
	text := "phases: { 1: null }\nitems:\n" + strings.Join(w.registryLines, "")
	if w.registryQueue != nil {
		text += "queue: [" + strings.Join(w.registryQueue, ", ") + "]\n"
	}
	w.registryText = text
	return w.write(w.data(startingRegistry), text)
}

// The registry is as the work steps last wrote it, in the working tree and,
// where the registry is committed, at HEAD: the command wrote nothing and
// committed nothing (slice 79).
func (w *world) registryUnchanged() error {
	path := w.data(startingRegistry)
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return err
	}
	if string(text) != w.registryText {
		return fmt.Errorf("the registry changed:\n%s\n%s", text, w.report())
	}
	if w.dataDir != "" {
		return nil
	}
	committed, err := w.gitOutput("show", "HEAD:"+filepath.ToSlash(path))
	if err != nil {
		return err
	}
	if committed != w.registryText {
		return fmt.Errorf("the registry at HEAD changed:\n%s\n%s", committed, w.report())
	}
	return nil
}

// The registry's queue names one more id, whether an item has it or not,
// committed as registryItem commits.
func (w *world) registryQueueNames(id string) error {
	w.registryQueue = append(w.registryQueue, id)
	if err := w.writeRegistryLines(); err != nil {
		return err
	}
	if w.dataDir != "" {
		return nil
	}
	return w.commitLeavingStaged("docs: a queue")
}

// The registry's queue, read as registryQueueIs reads it, does not name the
// id (slice 78: work drop takes the item out of it).
func (w *world) registryQueueLacks(id string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, w.data(startingRegistry)))
	if err != nil {
		return err
	}
	var registry struct {
		Queue []any `yaml:"queue"`
	}
	if err := yaml.Unmarshal(text, &registry); err != nil {
		return fmt.Errorf("the registry does not read: %w\n%s", err, text)
	}
	for _, q := range registry.Queue {
		if fmt.Sprint(q) == id {
			return fmt.Errorf("the registry's queue still names %s\n%s\n%s", id, text, w.report())
		}
	}
	return nil
}

// The registry's queue is the comma-separated ids, in order; none ("") is no
// queue key, null or an empty list.
func (w *world) registryQueueIs(ids string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, w.data(startingRegistry)))
	if err != nil {
		return err
	}
	var registry struct {
		Queue any `yaml:"queue"`
	}
	if err := yaml.Unmarshal(text, &registry); err != nil {
		return fmt.Errorf("the registry does not read: %w\n%s", err, text)
	}
	got := []string{}
	if list, ok := registry.Queue.([]any); ok {
		for _, id := range list {
			got = append(got, fmt.Sprint(id))
		}
	} else if registry.Queue != nil {
		return fmt.Errorf("the registry's queue is %v, not a list\n%s\n%s", registry.Queue, text, w.report())
	}
	want := []string{}
	if ids != "" {
		want = strings.Split(ids, ",")
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("the registry's queue is %q, not %q\n%s\n%s", got, want, text, w.report())
	}
	return nil
}

// One more idea of phase 1, todo and owned by nobody, written as a block
// mapping whose last key is refs, its value the text after the colon (bug
// 14: a flow list the formatter wrapped over several lines, or a block
// list), committed as registryItem commits.
func (w *world) registryIdeaWithRefs(id, refs string) error {
	w.registryLines = append(w.registryLines, fmt.Sprintf(
		"  - id: %s\n    title: %s\n    phase: 1\n    owner: null\n    status: todo\n    kind: idea\n    depends_on: []\n    refs:%s",
		id, id, refs))
	if err := w.writeRegistryLines(); err != nil {
		return err
	}
	if w.dataDir != "" {
		return nil
	}
	return w.commit("docs: a registry")
}

// The item's list under key (refs, tags) is the names given, split at sep,
// in order; none ("") is no such key, null or an empty list.
func (w *world) registryItemList(id, key, names, sep string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	got := []string{}
	if list, ok := item[key].([]any); ok {
		for _, r := range list {
			got = append(got, fmt.Sprint(r))
		}
	} else if item[key] != nil {
		return fmt.Errorf("%s's %s are %v, not a list\n%s", id, key, item[key], w.report())
	}
	want := []string{}
	if names != "" {
		want = strings.Split(names, sep)
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("%s's %s are %q, not %q\n%s", id, key, got, want, w.report())
	}
	return nil
}

// The registry's items, read from where the scenario keeps itos's data.
func (w *world) registryItems() ([]map[string]any, error) {
	text, err := os.ReadFile(filepath.Join(w.dir, w.data(startingRegistry)))
	if err != nil {
		return nil, err
	}
	var registry struct {
		Items []map[string]any `yaml:"items"`
	}
	if err := yaml.Unmarshal(text, &registry); err != nil {
		return nil, fmt.Errorf("the registry does not read: %w\n%s", err, text)
	}
	return registry.Items, nil
}

// The registry's item with the id, an error naming the registry when it has
// none.
func (w *world) registryItemOf(id string) (map[string]any, error) {
	items, err := w.registryItems()
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if fmt.Sprint(item["id"]) == id {
			return item, nil
		}
	}
	return nil, fmt.Errorf("the registry has no item %q\n%s", id, w.report())
}

// The item has the status and, when one is given, the owner (nobody for
// none).
func (w *world) registryItemIs(id, status, owner string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	if item["status"] != status {
		return fmt.Errorf("%s's status is %v, not %s\n%s", id, item["status"], status, w.report())
	}
	// nobody is an owner of null, as the Given steps write it.
	if owner == "nobody" && item["owner"] != nil || owner != "" && owner != "nobody" && item["owner"] != owner {
		return fmt.Errorf("%s's owner is %v, not %s\n%s", id, item["owner"], owner, w.report())
	}
	return nil
}

func (w *world) registryLacks(id string) error {
	items, err := w.registryItems()
	if err != nil {
		return err
	}
	for _, item := range items {
		if fmt.Sprint(item["id"]) == id {
			return fmt.Errorf("the registry still has the item %q\n%s", id, w.report())
		}
	}
	return nil
}

func (w *world) registryItemKind(id, kind, start string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	if item["kind"] != kind {
		return fmt.Errorf("%s's kind is %v, not %s\n%s", id, item["kind"], kind, w.report())
	}
	why, _ := item["why"].(string)
	if !strings.HasPrefix(why, start) {
		return fmt.Errorf("%s's why is %q, which does not start with %q\n%s", id, why, start, w.report())
	}
	return nil
}

func (w *world) registryItemDependsOn(id, dep string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	deps, _ := item["depends_on"].([]any)
	for _, d := range deps {
		if fmt.Sprint(d) == dep {
			return nil
		}
	}
	return fmt.Errorf("%s depends on %v, not %s\n%s", id, deps, dep, w.report())
}

func (w *world) lastHeaderIs(header string) error {
	out, err := w.gitOutput("log", "-1", "--format=%s")
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(out); got != header {
		return fmt.Errorf("the last commit's header is %q, not %q\n%s", got, header, w.report())
	}
	return nil
}

func (w *world) lastHeaderIsNot(header string) error {
	out, err := w.gitOutput("log", "-1", "--format=%s")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == header {
		return fmt.Errorf("the last commit's header is %q\n%s", header, w.report())
	}
	return nil
}

// The last commit's body holds the text, its words compared whatever the
// lines it is wrapped over (slice 78: work drop's reason is the body).
func (w *world) lastBodySays(text string) error {
	out, err := w.gitOutput("log", "-1", "--format=%b")
	if err != nil {
		return err
	}
	if !strings.Contains(strings.Join(strings.Fields(out), " "), strings.Join(strings.Fields(text), " ")) {
		return fmt.Errorf("the last commit's body does not say %q:\n%s\n%s", text, out, w.report())
	}
	return nil
}

// No line of the last commit's body is longer than n characters (bug 22:
// ask record named a long title's record file on one line over the lint's
// limit).
func (w *world) lastBodyFits(n int) error {
	out, err := w.gitOutput("log", "-1", "--format=%b")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		if len([]rune(line)) > n {
			return fmt.Errorf("a line of the last commit's body is %d characters long, over %d:\n%s\n%s",
				len([]rune(line)), n, out, w.report())
		}
	}
	return nil
}

// The last commit touches the paths and nothing else, in any order.
func (w *world) lastTouchesOnly(paths ...string) error {
	out, err := w.gitOutput("diff-tree", "--no-commit-id", "--name-only", "-r", "--root", "HEAD")
	if err != nil {
		return err
	}
	got, want := strings.Fields(out), slices.Clone(paths)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return fmt.Errorf("the last commit touches %v, not only %v\n%s", got, paths, w.report())
	}
	return nil
}

// The ledger file, read where the scenario keeps itos's data, has the task,
// one of whose checks runs the command (run:).
func (w *world) ledgerFileHasTask(file, id, command string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, w.data(file)))
	if err != nil {
		return err
	}
	var tasks []struct {
		ID       string `yaml:"id"`
		DoneWhen []struct {
			Run string `yaml:"run"`
		} `yaml:"done_when"`
	}
	if err := yaml.Unmarshal(text, &tasks); err != nil {
		return fmt.Errorf("%s does not read: %w\n%s", file, err, text)
	}
	for _, t := range tasks {
		if t.ID != id {
			continue
		}
		for _, c := range t.DoneWhen {
			if c.Run == command {
				return nil
			}
		}
		return fmt.Errorf("%s's task %s has no check that runs %q\n%s\n%s", file, id, command, text, w.report())
	}
	return fmt.Errorf("%s has no task %s\n%s\n%s", file, id, text, w.report())
}

func (w *world) stillStaged(path string) error {
	out, err := w.gitOutput("diff", "--cached", "--name-only")
	if err != nil {
		return err
	}
	if !slices.Contains(strings.Fields(out), path) {
		return fmt.Errorf("%s is not staged; the staged files are %q\n%s", path, out, w.report())
	}
	return nil
}

// A feature file of one scenario, its tag line the scenario's ID and the
// tags, committed, so a command reading the scenarios at HEAD finds it; the
// config gains the kind tests.scenario, which reads features/.
func (w *world) taggedScenario(id, tags string) error {
	w.config.scenarios = true
	if err := w.writeConfig(); err != nil {
		return err
	}
	text := fmt.Sprintf("Feature: Tagged\n\n  %s %s\n  Scenario: %s runs\n    When it runs\n", id, tags, id)
	if err := w.write(filepath.Join("features", "tagged.feature"), text); err != nil {
		return err
	}
	return w.commit("test: a tagged scenario")
}

// The item is of the kind, titled so, with the status.
func (w *world) registryItemMade(id, kind, title, status string) error {
	if err := w.registryItemField(id, "kind", kind); err != nil {
		return err
	}
	if err := w.registryItemField(id, "title", title); err != nil {
		return err
	}
	return w.registryItemField(id, "status", status)
}

// The item's key reads as the text.
func (w *world) registryItemField(id, key, want string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(item[key]); got != want {
		return fmt.Errorf("%s's %s is %q, not %q\n%s", id, key, got, want, w.report())
	}
	return nil
}

// The item is deferred with the reason, its last line break aside (a folded
// text ends with one); none ("") is no deferred key, null or an empty one.
func (w *world) registryItemDeferred(id, reason string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	got := ""
	if item["deferred"] != nil {
		got = strings.TrimRight(fmt.Sprint(item["deferred"]), "\n")
	}
	if got != reason {
		return fmt.Errorf("%s's deferred is %q, not %q\n%s", id, got, reason, w.report())
	}
	return nil
}

// The item's why ends with the text, its last line break aside.
func (w *world) registryWhyEnds(id, end string) error {
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	why, _ := item["why"].(string)
	if !strings.HasSuffix(strings.TrimRight(why, "\n"), end) {
		return fmt.Errorf("%s's why is %q, which does not end with %q\n%s", id, why, end, w.report())
	}
	return nil
}

// The ledger file, where the scenario keeps itos's data, removed from the
// working tree: git sees it deleted, the deletion neither staged nor
// committed.
func (w *world) ledgerFileDeleted(file string) error {
	return os.Remove(filepath.Join(w.dir, w.data(file)))
}

// A commit-msg hook in git's own hooks folder that says it refuses and exits
// 1, so every commit that runs the hooks fails.
func (w *world) refusingCommitMsgHook() error {
	return w.gitHook("commit-msg", "#!/bin/sh\necho 'the hook refuses every commit' >&2\nexit 1\n")
}

// A pre-commit hook in git's own hooks folder that does what a formatter
// run on the staged files does (vp staged, bug 17): each staged Markdown
// file gains a line in the working tree and is staged again, so the commit
// and the working tree hold the hook's version.
func (w *world) formattingPreCommitHook() error {
	return w.gitHook("pre-commit", `#!/bin/sh
git diff --cached --name-only --diff-filter=ACM -- '*.md' | while IFS= read -r file; do
	echo 'A line the hook added.' >>"$file"
	git add -- "$file" || exit 1
done
`)
}

// gitHook writes the hook, by its name, in git's own hooks folder.
func (w *world) gitHook(name, script string) error {
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
	return os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755)
}

// No file of the ledger, every one ledger.files names in its folder, has the
// task.
func (w *world) ledgerLacks(id string) error {
	files, err := filepath.Glob(filepath.Join(w.dir, w.data(strings.ReplaceAll(w.ledgerFilesPattern(), "{group}", "*"))))
	if err != nil {
		return err
	}
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var tasks []map[string]any
		if err := yaml.Unmarshal(text, &tasks); err != nil {
			return fmt.Errorf("%s does not read: %w\n%s", file, err, text)
		}
		for _, t := range tasks {
			if fmt.Sprint(t["id"]) == id {
				return fmt.Errorf("%s still has the task %s\n%s\n%s", file, id, text, w.report())
			}
		}
	}
	return nil
}

// git status lists nothing: no file changed, staged or untracked.
func (w *world) gitStatusClean() error {
	out, err := w.gitOutput("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("git status reports:\n%s\n%s", out, w.report())
	}
	return nil
}

// The ledger's task, added after the others when the ledger lacks it, has
// the why, written and committed as registryItem commits (slice 76: a
// task's why lives in its ledger entry).
func (w *world) ledgerTaskWhy(task, why string) error {
	i := slices.IndexFunc(w.ledger, func(t ledgerTask) bool { return t.id == task })
	if i < 0 {
		w.ledger = append(w.ledger, ledgerTask{id: task})
		i = len(w.ledger) - 1
	}
	w.ledger[i].why = why
	if err := w.write(w.data(w.ledgerPath()), w.ledgerText()); err != nil {
		return err
	}
	if w.dataDir != "" {
		return nil
	}
	return w.commitLeavingStaged("docs: a ledger")
}

// The ledger reads as the steps wrote it with the task's checks gone, its
// why kept: the rest of its text as it was, in the working tree and, where
// the ledger is committed, at HEAD (slice 101: work done takes a task's
// checks out of the ledger in the close commit).
func (w *world) ledgerTaskWithoutChecks(task, why string) error {
	i := slices.IndexFunc(w.ledger, func(t ledgerTask) bool { return t.id == task })
	if i < 0 {
		return fmt.Errorf("the steps wrote no task %s", task)
	}
	if w.ledger[i].why != why {
		return fmt.Errorf("the steps gave %s the why %q, not %q", task, w.ledger[i].why, why)
	}
	without := slices.Clone(w.ledger)
	without[i].checks = nil
	want := (&world{ledger: without}).ledgerText()
	path := w.data(w.ledgerPath())
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return err
	}
	if string(text) != want {
		return fmt.Errorf("the ledger reads:\n%s\nnot:\n%s\n%s", text, want, w.report())
	}
	if w.dataDir != "" {
		return nil
	}
	committed, err := w.gitOutput("show", "HEAD:"+filepath.ToSlash(path))
	if err != nil {
		return err
	}
	if committed != want {
		return fmt.Errorf("the ledger at HEAD reads:\n%s\nnot:\n%s\n%s", committed, want, w.report())
	}
	return nil
}

// The item has the status and no why: no why key, or an empty one (slice
// 76: work done drops the why of the item it closes).
func (w *world) registryItemWithoutWhy(id, status string) error {
	if err := w.registryItemIs(id, status, ""); err != nil {
		return err
	}
	item, err := w.registryItemOf(id)
	if err != nil {
		return err
	}
	if why, has := item["why"]; has && why != nil && why != "" {
		return fmt.Errorf("%s still has the why %q\n%s", id, why, w.report())
	}
	return nil
}
