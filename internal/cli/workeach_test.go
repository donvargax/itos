package cli

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/work"
)

// eachRegistry is a registry every several-id command can change: a and b
// ideas to do, c and e deferred, d done, the queue holding a.
const eachRegistry = "phases: { 1: null }\nqueue:\n  - a\nitems:\n" +
	"  - { id: a, title: A, phase: 1, owner: null, status: todo, kind: idea }\n" +
	"  - { id: b, title: B, phase: 1, owner: null, status: todo, kind: idea }\n" +
	"  - { id: c, title: C, phase: 1, owner: null, status: todo, kind: idea, deferred: later }\n" +
	"  - { id: d, title: D, phase: 1, owner: null, status: done }\n" +
	"  - { id: e, title: E, phase: 1, owner: null, status: todo, kind: idea, deferred: later }\n"

// eachRun runs a registry command in a fresh repository whose registry is
// the text, and gives its exit code, error, stdout and stderr.
func eachRun(t *testing.T, registry string, run func([]string, Out) (int, error), json bool, args ...string) (int, error, string, string) {
	t.Helper()
	registryWriterRepo(t, registry)
	var stdout, stderr strings.Builder
	code, err := run(args, Out{Stdout: &stdout, Stderr: &stderr, JSON: json})
	return code, err, stdout.String(), stderr.String()
}

// Slice 118: each command that takes several ids, run with one id and with
// several, through every way it ends.
func TestSeveralIDCommands(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func([]string, Out) (int, error)
		// one and several change the registry; refused and refusedSeveral
		// are refused; same changes nothing; usage are usage errors.
		one, several, refused, refusedSeveral, same []string
		says                                        string
		usage                                       [][]string
	}{
		{"edit", workEdit, []string{"a", "--refs", "x.md"}, []string{"a", "b", "--refs", "x.md"},
			[]string{"z", "--refs", "x.md"}, []string{"a", "z", "--refs", "x.md"}, []string{"a", "--refs", ""}, "a's refs changed\nb's refs changed\n",
			[][]string{{"a", "--bogus", "x"}, {"a"}, {"a", "b", "--title", "T"}, {"--refs", "x.md"}, {"a", "a", "--refs", "x.md"}}},
		{"drop", workDrop, []string{"a", "--why", "Gone."}, []string{"a", "b", "--why", "Gone."},
			[]string{"d", "--why", "Gone."}, []string{"a", "d", "--why", "Gone."}, nil, "a is dropped\nb is dropped\n",
			[][]string{{"a", "--bogus", "x"}, {"a"}}},
		{"defer", workDefer, []string{"a", "--why", "Later."}, []string{"a", "b", "--why", "Later."},
			[]string{"d", "--why", "Later."}, []string{"a", "d", "--why", "Later."}, nil, "a is deferred\nb is deferred\n",
			[][]string{{"a", "--bogus", "x"}, {"a"}}},
		{"resume", workResume, []string{"c"}, []string{"c", "e"}, []string{"a"}, []string{"c", "a"}, nil, "c is resumed\ne is resumed\n",
			[][]string{{"c", "--bogus", "x"}}},
		{"queue", workQueue, []string{"b", "--before", "a"}, []string{"b", "c", "--top"}, []string{"d", "--top"}, []string{"b", "d", "--top"},
			[]string{"a", "--top"}, "b and c are first in the queue, in that order\n",
			[][]string{{"a", "--bogus", "x"}, {"a", "--top", "--remove"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, args := range c.usage {
				if code, err, _, _ := eachRun(t, eachRegistry, c.run, false, args...); code != 0 || err == nil || ExitCode(err) != ExitUsage {
					t.Errorf("%q: (%d, %v), want a usage error", args, code, err)
				}
			}
			for _, args := range [][]string{c.one, c.several} {
				if code, err, stdout, stderr := eachRun(t, eachRegistry, c.run, false, args...); code != 0 || err != nil || !strings.Contains(stdout, "committed ") {
					t.Errorf("%q: (%d, %v) %q %q", args, code, err, stdout, stderr)
				}
				// A registry that is not sound, one that cannot be edited
				// in place and a commit a hook refuses each end it.
				if code, err, _, _ := eachRun(t, "phases: { 1: null }\nitems:\n  - { id: a, title: A, phase: 1, status: todo, depends_on: [nope] }\n", c.run, false, args...); code != ExitPolicy || err != nil {
					t.Errorf("%q, a registry not sound: (%d, %v)", args, code, err)
				}
				flow := "{ phases: { 1: null }, queue: [a], items: [" + strings.Join(itemsOf(eachRegistry), ", ") + "] }\n"
				if code, err, _, _ := eachRun(t, flow, c.run, false, args...); code != 0 || err == nil || !strings.Contains(err.Error(), "cannot be edited in place") {
					t.Errorf("%q, a flow registry: (%d, %v)", args, code, err)
				}
				registryWriterRepo(t, eachRegistry)
				refusingHook(t)
				if code, err := c.run(args, Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}); code != ExitPolicy || err != nil {
					t.Errorf("%q, a commit refused: (%d, %v)", args, code, err)
				}
			}
			for _, args := range [][]string{c.refused, c.refusedSeveral} {
				if code, err, _, stderr := eachRun(t, eachRegistry, c.run, false, args...); code != ExitPolicy || err != nil || stderr == "" {
					t.Errorf("%q: (%d, %v) %q, want refused", args, code, err, stderr)
				}
				if text, _ := os.ReadFile("registry.yaml"); string(text) != eachRegistry {
					t.Errorf("%q wrote the registry:\n%s", args, text)
				}
			}
			if c.same != nil {
				if code, err, stdout, _ := eachRun(t, eachRegistry, c.run, false, c.same...); code != 0 || err != nil || !strings.Contains(stdout, "nothing to change") {
					t.Errorf("%q: (%d, %v) %q", c.same, code, err, stdout)
				}
			}
			code, err, stdout, _ := eachRun(t, eachRegistry, c.run, false, c.several...)
			if code != 0 || err != nil || !strings.HasPrefix(stdout, c.says) || !strings.Contains(stdout, "docs: "+c.name+" ") {
				t.Errorf("%q: %q", c.several, stdout)
			}
		})
	}
}

// itemsOf is the registry text's item lines, as flow mappings.
func itemsOf(text string) []string {
	var items []string
	for _, line := range strings.Split(text, "\n") {
		if item, ok := strings.CutPrefix(line, "  - { "); ok {
			items = append(items, "{ "+item)
		}
	}
	return items
}

// --json for several ids gives the items in the order given and the commit,
// a refusal the ids refused; -q prints nothing; a change already made
// commits nothing.
func TestWriteEachReports(t *testing.T) {
	code, err, stdout, _ := eachRun(t, eachRegistry, workQueue, true, "c", "b", "--after", "a")
	var got struct {
		OK     bool             `json:"ok"`
		Items  []map[string]any `json:"items"`
		Queue  []string         `json:"queue"`
		Commit *string          `json:"commit"`
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || !got.OK || len(got.Items) != 2 ||
		got.Items[0]["id"] != "c" || strings.Join(got.Queue, " ") != "a c b" || got.Commit == nil {
		t.Errorf("queue c b --after a --json: (%d, %v) %s", code, err, stdout)
	}
	code, err, stdout, _ = eachRun(t, eachRegistry, workResume, true, "c", "a", "z")
	var refused struct {
		OK       bool          `json:"ok"`
		Refused  []string      `json:"refused"`
		Problems []out.Problem `json:"problems"`
	}
	if code != ExitPolicy || err != nil || json.Unmarshal([]byte(stdout), &refused) != nil || refused.OK ||
		strings.Join(refused.Refused, " ") != "a z" || len(refused.Problems) != 2 {
		t.Errorf("resume c a z --json: (%d, %v) %s", code, err, stdout)
	}
	registryWriterRepo(t, eachRegistry)
	var quiet strings.Builder
	if code, err := workDrop([]string{"a", "b", "--why", "Gone."}, Out{Stdout: &quiet, Stderr: &quiet, Quiet: true}); code != 0 || err != nil || strings.Contains(quiet.String(), "dropped") {
		t.Errorf("drop -q: (%d, %v) %q", code, err, quiet.String())
	}
	code, err, stdout, _ = eachRun(t, eachRegistry, workQueue, true, "b", "c", "--remove")
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Commit != nil {
		t.Errorf("queue b c --remove of items not queued --json: (%d, %v) %s", code, err, stdout)
	}
	code, err, stdout, _ = eachRun(t, eachRegistry, workQueue, false, "b", "c", "--remove")
	if code != 0 || err != nil || stdout != "b and c are already out of the queue\nnothing to change\n" {
		t.Errorf("queue b c --remove: (%d, %v) %q", code, err, stdout)
	}
	code, err, stdout, _ = eachRun(t, eachRegistry, workDrop, false, "a", "b", "--why", "Gone.")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "a is dropped\nb is dropped\ncommitted ") {
		t.Errorf("drop a b: %q", stdout)
	}
	code, err, stdout, _ = eachRun(t, "phases: { 1: null }\nitems:\n  - { id: a, title: A, phase: 1, status: dropped }\n  - { id: b, title: B, phase: 1, status: dropped }\n",
		workDrop, false, "a", "b", "--why", "Gone.")
	if code != 0 || err != nil || stdout != "a is already dropped\nb is already dropped\nnothing to change\n" {
		t.Errorf("drop a b already dropped: (%d, %v) %q", code, err, stdout)
	}
	code, err, stdout, _ = eachRun(t, "phases: { 1: null }\nitems:\n  - { id: a, title: A, phase: 1, status: dropped }\n", workDrop, false, "a", "--why", "Gone.")
	if code != 0 || err != nil || stdout != "a is already dropped; nothing to change\n" {
		t.Errorf("drop a already dropped: (%d, %v) %q", code, err, stdout)
	}
	code, err, stdout, _ = eachRun(t, eachRegistry, workEdit, false, "a", "--title", "A, renamed", "--depends-on", "", "--tags", "")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "a's title changed: committed ") {
		t.Errorf("edit a --title: (%d, %v) %q", code, err, stdout)
	}
	code, err, stdout, _ = eachRun(t, eachRegistry, workQueue, false, "a", "b", "--after", "a")
	if code != ExitPolicy || err != nil {
		t.Errorf("queue a b --after a: (%d, %v) %q", code, err, stdout)
	}
}

// writeEach's own ends: a writer's error is the registry's that cannot be
// edited, and a config that does not load ends it before.
func TestWriteEachErrors(t *testing.T) {
	registryWriterRepo(t, eachRegistry)
	failing := func(*config.Loaded, work.Registry, string) (work.Several, []string, []out.Problem, error) {
		return work.Several{}, nil, nil, errors.New("no edit")
	}
	if code, err := writeEach(Out{}, "edit", failing, nil, nil); code != 0 || err == nil || !strings.Contains(err.Error(), "cannot be edited in place: no edit") {
		t.Errorf("a failing writer: (%d, %v)", code, err)
	}
	t.Setenv("ITOS_CONFIG", "nope.yaml")
	if code, err := writeEach(Out{}, "edit", failing, nil, nil); code != 0 || err == nil {
		t.Errorf("no config: (%d, %v)", code, err)
	}
}
