package adapter

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/proptest"
	"github.com/donvargax/itos/v8/internal/vcs/port"
	"pgregory.net/rapid"
)

// touchedPaths are the files a history draws from: flat and in a folder, one
// with a space, none a prefix of another (a path is never a file and a folder).
var touchedPaths = []string{"a", "b", "c", "d/e", "d/f", "g h"}

// Touched is held to a model of the history: its commits, each with the
// parents it names and the content of every path it holds, and nothing of
// what git prints. For a range From..To (From "" for every commit up to To,
// Since leaving a commit and its ancestors out) the model counts the paths
// changed by the commits reachable from To and from neither From nor Since,
// each commit against its first parent alone (a root commit against nothing):
// a path it holds that the parent lacks or holds with other content, or the
// parent holds and it lacks. A rename is both its paths, as Touched asks git
// for no rename detection; a merge counts only when merges is set, against its
// first parent, and the commits of the history it merged in are in the range
// all the same. The paths are compared as a set, each once; their order is
// git's.
//
// Rapid draws the history operation by operation: a commit on the current
// branch (adding, changing, deleting or renaming files, or changing none), a
// new branch, a switch of branch, a branch with a root commit of its own, and
// a merge of another branch into the current one, a path the two sides
// disagree on resolved to either side, to a new text or to nothing. Each is
// made on a scratch repository too, as a commit written whole by git
// fast-import, and after every operation Touched is asked for ranges drawn
// between the commits and for the one a merge ends, its first parent to the
// merge (what push judges a commit by), both ways.
func TestPropertyTouched(t *testing.T) {
	base := alone(t)
	proptest.Check(t, func(rt *rapid.T) {
		w := newTouchedWorld(t, rt, base)
		rt.Repeat(map[string]func(*rapid.T){
			"commit": w.commit,
			"branch": w.branch,
			"switch": w.switchBranch,
			"root":   w.root,
			"merge":  w.merge,
			"":       w.check,
		})
	}, proptest.Checks(touchedChecks))
}

// touchedChecks keeps the property, every operation running a few git
// commands, within the 15 seconds this package's tests may take on Linux.
const touchedChecks = 30

// touchedCommit is one commit of the model.
type touchedCommit struct {
	parents []int // indexes of its parents, the first the one it is judged against
	files   map[string]string
}

type touchedWorld struct {
	t       *testing.T
	rt      *rapid.T
	dir     string
	commits []touchedCommit
	shas    []string       // the scratch repository's name for each commit
	tips    map[string]int // each branch's tip
	names   []string       // the branches, in the order made
	current string
	stamp   int
}

func newTouchedWorld(t *testing.T, rt *rapid.T, base string) *touchedWorld {
	t.Helper()
	dir, err := os.MkdirTemp(base, "run")
	if err != nil {
		rt.Fatal(err)
	}
	if out, err := gitCmd(base, "", "init", "-q", "-b", "main", dir).CombinedOutput(); err != nil {
		rt.Fatalf("git init: %s", out)
	}
	w := &touchedWorld{t: t, rt: rt, dir: dir, tips: map[string]int{}}
	w.current = "main"
	w.names = []string{"main"}
	w.tips["main"] = w.write("main", nil, w.someFiles(rt, "the first commit's"))
	return w
}

// someFiles draws a few paths with a text each.
func (w *touchedWorld) someFiles(rt *rapid.T, label string) map[string]string {
	files := map[string]string{}
	for _, p := range rapid.SliceOfNDistinct(rapid.SampledFrom(touchedPaths), 0, 3, rapid.ID[string]).Draw(rt, label+" paths") {
		files[p] = w.draw(rt, "text of "+p)
	}
	return files
}

// draw is a text, from few so that two sides often agree.
func (w *touchedWorld) draw(rt *rapid.T, label string) string {
	return rapid.SampledFrom([]string{"1\n", "2\n", "3\n"}).Draw(rt, label)
}

// write makes the commit with these parents and files in the model and in
// the repository, on the branch, and returns its index.
func (w *touchedWorld) write(branch string, parents []int, files map[string]string) int {
	w.t.Helper()
	w.stamp++
	var in bytes.Buffer
	msg := "c" + strconv.Itoa(len(w.commits))
	fmt.Fprintf(&in, "commit refs/heads/%s\nmark :1\ncommitter t <t@t> %d +0000\ndata %d\n%s\n", branch, 1700000000+w.stamp, len(msg), msg)
	for i, p := range parents {
		verb := "merge"
		if i == 0 {
			verb = "from"
		}
		fmt.Fprintf(&in, "%s %s\n", verb, w.shas[p])
	}
	in.WriteString("deleteall\n")
	for _, p := range slices.Sorted(maps.Keys(files)) {
		fmt.Fprintf(&in, "M 100644 inline %s\ndata %d\n%s\n", p, len(files[p]), files[p])
	}
	in.WriteString("get-mark :1\n")
	cmd := gitCmd(w.dir, "", "fast-import", "--quiet")
	cmd.Stdin = &in
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		w.t.Fatalf("git fast-import: %v: %s", err, stderr.String())
	}
	w.shas = append(w.shas, strings.TrimSpace(out.String()))
	w.commits = append(w.commits, touchedCommit{parents: parents, files: files})
	return len(w.commits) - 1
}

func (w *touchedWorld) commit(rt *rapid.T) {
	w.rt = rt
	files := maps.Clone(w.commits[w.tips[w.current]].files)
	for range rapid.IntRange(0, 3).Draw(rt, "changes") {
		kind := rapid.SampledFrom([]string{"put", "delete", "rename"}).Draw(rt, "change")
		p := rapid.SampledFrom(touchedPaths).Draw(rt, "path")
		switch _, held := files[p]; {
		case kind == "put":
			files[p] = w.draw(rt, "text of "+p)
		case !held:
		case kind == "delete":
			delete(files, p)
		default:
			to := rapid.SampledFrom(touchedPaths).Draw(rt, "renamed to")
			if _, taken := files[to]; !taken {
				files[to] = files[p]
				delete(files, p)
			}
		}
	}
	rt.Logf("commit on %s with %v", w.current, files)
	w.tips[w.current] = w.write(w.current, []int{w.tips[w.current]}, files)
}

func (w *touchedWorld) branch(rt *rapid.T) {
	w.rt = rt
	from := rapid.SampledFrom(w.names).Draw(rt, "branch from")
	name := "b" + strconv.Itoa(len(w.names))
	w.names = append(w.names, name)
	w.tips[name] = w.tips[from]
	w.current = name
	if out, err := gitCmd(w.dir, "", "update-ref", "refs/heads/"+name, w.shas[w.tips[name]]).CombinedOutput(); err != nil {
		rt.Fatalf("git update-ref: %s", out)
	}
	rt.Logf("branch %s from %s, now current", name, from)
}

func (w *touchedWorld) switchBranch(rt *rapid.T) {
	w.rt = rt
	w.current = rapid.SampledFrom(w.names).Draw(rt, "switch to")
}

func (w *touchedWorld) root(rt *rapid.T) {
	w.rt = rt
	name := "b" + strconv.Itoa(len(w.names))
	files := w.someFiles(rt, "the root's")
	w.names = append(w.names, name)
	w.tips[name] = w.write(name, nil, files)
	w.current = name
	rt.Logf("root commit on %s, now current", name)
}

// merge makes a commit of the other branch into the current one, its files
// those the two sides agree on and, for a path they disagree on, one of
// either side's, a new text or none.
func (w *touchedWorld) merge(rt *rapid.T) {
	w.rt = rt
	other := rapid.SampledFrom(w.names).Draw(rt, "merge in")
	ours, theirs := w.tips[w.current], w.tips[other]
	if ours == theirs {
		rt.Skip()
	}
	files := map[string]string{}
	for _, p := range touchedPaths {
		o, hasO := w.commits[ours].files[p]
		th, hasT := w.commits[theirs].files[p]
		resolution := "ours"
		if o != th || hasO != hasT {
			resolution = rapid.SampledFrom([]string{"ours", "theirs", "new", "none"}).Draw(rt, "resolution of "+p)
		}
		switch resolution {
		case "ours":
			if hasO {
				files[p] = o
			}
		case "theirs":
			if hasT {
				files[p] = th
			}
		case "new":
			files[p] = w.draw(rt, "new text of "+p)
		}
	}
	rt.Logf("merge %s (c%d) into %s (c%d) with %v", other, theirs, w.current, ours, files)
	merged := w.write(w.current, []int{ours, theirs}, files)
	w.tips[w.current] = merged
	w.askRange(rt, port.Range{From: w.shas[ours], To: w.shas[merged]}, ours, merged, -1)
}

// reach is the commit and its ancestors, as a set of indexes.
func (w *touchedWorld) reach(c int) map[int]bool {
	seen := map[int]bool{}
	var walk func(int)
	walk = func(c int) {
		if seen[c] {
			return
		}
		seen[c] = true
		for _, p := range w.commits[c].parents {
			walk(p)
		}
	}
	walk(c)
	return seen
}

// changed are the paths the commit changes against its first parent.
func (w *touchedWorld) changed(c int) []string {
	own := w.commits[c]
	var against map[string]string
	if len(own.parents) > 0 {
		against = w.commits[own.parents[0]].files
	}
	var paths []string
	for _, p := range touchedPaths {
		was, hadIt := against[p]
		now, has := own.files[p]
		if hadIt != has || was != now {
			paths = append(paths, p)
		}
	}
	return paths
}

// expect is what the model says Touched gives for the range of commit
// indexes (from and since -1 for none).
func (w *touchedWorld) expect(from, to, since int, merges bool) []string {
	out := w.reach(to)
	for _, left := range []int{from, since} {
		if left >= 0 {
			for c := range w.reach(left) {
				delete(out, c)
			}
		}
	}
	set := map[string]bool{}
	for c := range out {
		if len(w.commits[c].parents) > 1 && !merges {
			continue
		}
		for _, p := range w.changed(c) {
			set[p] = true
		}
	}
	return slices.Sorted(maps.Keys(set))
}

// check asks Touched, both ways, for a few ranges drawn between the commits.
func (w *touchedWorld) check(rt *rapid.T) {
	w.rt = rt
	last := len(w.commits) - 1
	for range 3 {
		from := rapid.IntRange(-1, last).Draw(rt, "from")
		to := rapid.IntRange(0, last).Draw(rt, "to")
		since := -1
		if rapid.IntRange(0, 3).Draw(rt, "with since") == 0 {
			since = rapid.IntRange(0, last).Draw(rt, "since")
		}
		r := port.Range{To: w.shas[to]}
		if from >= 0 {
			r.From = w.shas[from]
		}
		if since >= 0 {
			r.Since = w.shas[since]
		}
		w.askRange(rt, r, from, to, since)
	}
}

// askRange compares Touched of the range, with and without merges, to the
// model's, the range named by the indexes of its commits (from and since -1
// for none).
func (w *touchedWorld) askRange(rt *rapid.T, r port.Range, from, to, since int) {
	rt.Helper()
	for _, merges := range []bool{true, false} {
		got, err := (Git{Platform: runtime.GOOS, Dir: w.dir}).Touched(rt.Context(), r, merges)
		if err != nil {
			rt.Fatalf("Touched: %v", err)
		}
		want := w.expect(from, to, since, merges)
		if !slices.Equal(slices.Sorted(slices.Values(got)), want) {
			rt.Fatalf("Touched(c%d..c%d since c%d, merges %v) = %q; the model has %q\nthe history, a commit's parents and paths: %s", from, to, since, merges, got, want, w.describe())
		}
	}
}

// describe is the history, commit by commit.
func (w *touchedWorld) describe() string {
	var b strings.Builder
	for i, c := range w.commits {
		fmt.Fprintf(&b, "\n  c%d parents %v holds", i, c.parents)
		for _, p := range slices.Sorted(maps.Keys(c.files)) {
			fmt.Fprintf(&b, " %s=%s", p, strings.TrimSpace(c.files[p]))
		}
	}
	return b.String()
}
