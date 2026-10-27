package work

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/proptest"
	"github.com/donvargax/itos/v8/internal/providers"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
	"pgregory.net/rapid"
)

// trustConfig is a config with a ledger of T-<n> tasks in tasks/phase-<n>.yaml
// and the registry work-items.yaml.
const trustConfig = `version: 1
ledger:
  files: "tasks/phase-{group}.yaml"
  id: "T-\\d+"
work:
  registry: work-items.yaml
`

const trustItem = "T-1"

// The paths a commit of main may change outside the registry and the ledger:
// code, a document, a file beside the ledger whose name is no ledger file's,
// and one in a folder of its own. They sort as git lists them.
var trustCodePaths = []string{"docs/note.md", "main.go", "src/lib.go", "tasks/notes.md"}

// trustCheck is one done_when check of a task: its command and its timeout in
// seconds, 0 for none.
type trustCheck struct {
	run     string
	timeout int
}

var (
	trustCommands = []string{"true", "go vet ./...", "go test ./...", "make lint"}
	trustTimeouts = []int{0, 30, 120}
)

// What work done's trust of a merged pull request's head run promises
// (slice 181, bug 137) is a rule over what main gained since the head: the
// run judges the landing commit exactly when no path outside the work
// registry and the ledger changed, and the item's own task has the checks it
// had at the head, command, timeout and order. A run of random commits to the
// VCS's fake, each a registry edit, an edit to the other task's checks, an
// edit to the item's task's checks (one added, removed, moved or given
// another command or timeout), an edit of its why alone, or a code file, is
// held after every step to a model of exactly that: the set of paths changed
// outside the bookkeeping, and the checks of the item's task now and at the
// head. The model is the rule over the operations, never a second diff.
func TestPropertyWorkDoneTrust(t *testing.T) {
	root := t.TempDir()
	proptest.Check(t, func(rt *rapid.T) {
		w := newTrustWorld(t, rt, root)
		rt.Repeat(w.actions())
	}, proptest.Checks(1000))
}

// trustWorld is one run: a history in the VCS's fake of the merged head and
// the commits main gained since, with the trees the ledger is read from, and
// the model.
type trustWorld struct {
	rt    *rapid.T
	vcs   *vcsfake.VCS
	trees treefake.Trees
	cfg   *config.Loaded
	// files are the files of the newest commit.
	files map[string]string
	// head is the merged pull request's head; tip is main's newest commit,
	// the landing commit.
	head, tip string
	step      int

	// The model: the paths changed since the head outside the registry and
	// the ledger, and the checks of the item's task at the head and now.
	outside     map[string]bool
	headChecks  []trustCheck
	itemChecks  []trustCheck
	otherChecks []trustCheck
	itemWhy     int
}

func newTrustWorld(t *testing.T, rt *rapid.T, root string) *trustWorld {
	common := root + "/.git"
	w := &trustWorld{
		rt:      rt,
		vcs:     &vcsfake.VCS{Root: root, Located: map[string]vcsport.Location{"": {Top: root, GitDir: common, Common: common}}},
		trees:   treefake.Trees{Common: common, Files: map[string]map[string]string{}},
		files:   map[string]string{},
		outside: map[string]bool{},
	}
	w.itemChecks = drawChecks(rt, "the item's checks", 3)
	w.otherChecks = drawChecks(rt, "the other task's checks", 2)
	w.files["itos.yaml"] = trustConfig
	w.files["work-items.yaml"] = "# registry\n"
	w.files["main.go"] = "package main\n"
	w.writeLedgers()
	w.commit()
	w.files["main.go"] = "package main // the item's work\n"
	w.commit()
	w.head = w.tip
	w.commit()
	w.headChecks = slices.Clone(w.itemChecks)
	w.trees.Files["worktree"] = maps.Clone(w.files)
	src, err := w.trees.At(rt.Context(), "worktree")
	if err != nil {
		rt.Fatal(err)
	}
	if w.cfg, err = config.FromText(w.vcs, w.trees, src, "itos.yaml", trustConfig, false); err != nil {
		rt.Fatal(err)
	}
	return w
}

func drawCheck(rt *rapid.T) trustCheck {
	return trustCheck{rapid.SampledFrom(trustCommands).Draw(rt, "command"), rapid.SampledFrom(trustTimeouts).Draw(rt, "timeout")}
}

func drawChecks(rt *rapid.T, label string, most int) []trustCheck {
	return rapid.SliceOfN(rapid.Custom(drawCheck), 0, most).Draw(rt, label)
}

func (w *trustWorld) actions() map[string]func(*rapid.T) {
	return map[string]func(*rapid.T){
		"a registry edit": func(*rapid.T) {
			w.step++
			w.files["work-items.yaml"] += fmt.Sprintf("# edit %d\n", w.step)
			w.commit()
		},
		"an edit to the other task's checks": func(rt *rapid.T) {
			w.otherChecks = editChecks(rt, w.otherChecks)
			w.writeLedgers()
			w.commit()
		},
		"an edit to the item's task's checks": func(rt *rapid.T) {
			w.itemChecks = editChecks(rt, w.itemChecks)
			w.writeLedgers()
			w.commit()
		},
		"an edit of the item's task's why": func(*rapid.T) {
			w.itemWhy++
			w.writeLedgers()
			w.commit()
		},
		"a code file": func(rt *rapid.T) {
			w.step++
			path := rapid.SampledFrom(trustCodePaths).Draw(rt, "path")
			w.files[path] = fmt.Sprintf("edit %d\n", w.step)
			w.outside[path] = true
			w.commit()
		},
		"": func(*rapid.T) { w.check() },
	}
}

// editChecks is the checks after one edit: a check added at some place,
// removed, moved to another place, or given another command or timeout.
func editChecks(rt *rapid.T, checks []trustCheck) []trustCheck {
	checks = slices.Clone(checks)
	kinds := []string{"add"}
	if len(checks) > 0 {
		kinds = append(kinds, "remove", "change a command", "change a timeout")
	}
	if len(checks) > 1 {
		kinds = append(kinds, "move")
	}
	kind := rapid.SampledFrom(kinds).Draw(rt, "edit")
	if kind == "add" {
		return slices.Insert(checks, rapid.IntRange(0, len(checks)).Draw(rt, "at"), drawCheck(rt))
	}
	at := rapid.IntRange(0, len(checks)-1).Draw(rt, "at")
	switch kind {
	case "remove":
		return slices.Delete(checks, at, at+1)
	case "change a command":
		checks[at].run = rapid.SampledFrom(trustCommands).Draw(rt, "command")
	case "change a timeout":
		checks[at].timeout = rapid.SampledFrom(trustTimeouts).Draw(rt, "timeout")
	case "move":
		moved := checks[at]
		checks = slices.Delete(checks, at, at+1)
		checks = slices.Insert(checks, rapid.IntRange(0, len(checks)).Draw(rt, "to"), moved)
	}
	return checks
}

// check asks work done's trust whether the head's passing run judges the
// landing commit and holds the answer, and the line that says why not, to the
// model.
func (w *trustWorld) check() {
	w.rt.Helper()
	paths := slices.Sorted(maps.Keys(w.outside))
	want := len(paths) == 0 && slices.Equal(w.headChecks, w.itemChecks)

	m := &mergedHead{url: "https://github.example/o/r/pull/7", head: w.head, remote: "origin", on: "origin/main", number: 7}
	watcher := look(map[string]providers.Run{w.head: passes("u-head", w.head)})
	var said strings.Builder
	run, ok := trustedRun(w.rt.Context(), w.cfg, trustItem, m, doneCommit{sha: w.tip}, watcher, Out{Stdout: &said, Stderr: &said})
	line := said.String()
	describe := fmt.Sprintf("the head %s, main's newest %s, %d paths changed outside the bookkeeping %v, the item's checks %v at the head and %v now",
		short(w.head), short(w.tip), len(paths), paths, w.headChecks, w.itemChecks)
	if ok != want {
		w.rt.Fatalf("trustedRun trusted the head's run = %v, the model says %v: %s\nsaid: %s", ok, want, describe, line)
	}
	switch {
	case ok:
		if run.Run.URL != "u-head" || !strings.Contains(line, "so the head's CI run judges "+short(w.tip)) {
			w.rt.Fatalf("trusted, but gave the run %q and said %q: %s", run.Run.URL, line, describe)
		}
	case len(paths) > 0:
		more := ""
		if len(paths) > 1 {
			more = fmt.Sprintf(" and %d more", len(paths)-1)
		}
		if !strings.Contains(line, " differs from it in "+paths[0]+more+", so origin/main's CI run judges ") {
			w.rt.Fatalf("not trusted for %v, but said %q: %s", paths, line, describe)
		}
	default:
		if !strings.Contains(line, "checks in the ledger differ from the head's") {
			w.rt.Fatalf("not trusted for the item's checks, but said %q: %s", line, describe)
		}
	}
}

// writeLedgers writes the two tasks' ledger files from the model: the item's
// task in phase 1, the other in phase 2.
func (w *trustWorld) writeLedgers() {
	w.files["tasks/phase-1.yaml"] = ledgerEntry(trustItem, fmt.Sprintf("Why %d.", w.itemWhy), w.itemChecks)
	w.files["tasks/phase-2.yaml"] = ledgerEntry("T-2", "Another task.", w.otherChecks)
}

func ledgerEntry(id, why string, checks []trustCheck) string {
	text := fmt.Sprintf("- id: %s\n  type: chore\n  title: A task\n  why: %s\n", id, why)
	if len(checks) == 0 {
		return text
	}
	text += "  done_when:\n"
	for _, c := range checks {
		text += fmt.Sprintf("    - run: %q\n", c.run)
		if c.timeout > 0 {
			text += fmt.Sprintf("      timeout: %d\n", c.timeout)
		}
	}
	return text
}

// commit makes the files as they are a commit on the history, an empty one
// when an edit left them as they were, and main's newest.
func (w *trustWorld) commit() {
	c := vcsfake.Commit{Name: fmt.Sprintf("c%d", len(w.vcs.History)+1), Tree: maps.Clone(w.files)}
	if n := len(w.vcs.History); n > 0 {
		c.Parents = []string{w.vcs.History[n-1].Name}
	}
	w.vcs.History = append(w.vcs.History, c)
	w.trees.Files[c.Name] = c.Tree
	w.tip = c.Name
}
