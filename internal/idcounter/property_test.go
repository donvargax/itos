package idcounter

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/nextid"
	"github.com/donvargax/itos/v8/internal/proptest"
	"pgregory.net/rapid"
)

// categories are the series a run claims in: the counters of two must never
// meet. A category's ids are <category>-<n>, as the engine writes them.
var categories = []string{"slice", "ID-A"}

// The counter's model is the set of numbers handed out and the counter. A
// run of random operations, on a counter claimed on a remote (the VCS port's
// fake, with another clone landing claims between ours, a push the remote
// refuses, the remote out of reach) and on the file counter (with no remote
// or under a stealth config; another worktree claiming; the file unreadable),
// is checked after every step against the promises of ADR-0045: no number is
// handed out twice, every one is above the repository's highest id and the
// counter, claims are runs of consecutive numbers with no gap but the ones
// failures leave, the counter is never behind a number handed out, and a
// failed claim hands out nothing and never lowers the counter.
func TestPropertyIDCounterIsHeldToItsModel(t *testing.T) {
	proptest.Check(t, func(rt *rapid.T) {
		w := newCounterWorld(t, rt)
		rt.Repeat(w.actions())
	}, proptest.Checks(200))
}

// counterWorld is one run: the counter under test, reached through store, and
// the model.
type counterWorld struct {
	rt     *rapid.T
	mode   string // "remote", "no-remote" or "stealth"
	store  Store
	remote *remoteFake // the remote, in remote mode
	other  Store       // another worktree of the same clone, in the other modes
	common string      // the git common dir the file counter lives in

	issued  map[string]map[int]string // the numbers handed out through the counter, to whom
	written map[string]map[int]bool   // the numbers a person wrote into the repository by hand
	counter map[string]int            // what the counter holds, as the model has it
	repo    []string                  // the ids the repository holds

	down   bool  // the remote is out of reach
	lose   []int // the claims that land on the remote before each of the next pushes
	refuse bool  // the next push after those is refused outright

	saved   []byte // the file counter's contents while it is made unreadable
	hadFile bool
	broken  bool
}

func newCounterWorld(t *testing.T, rt *rapid.T) *counterWorld {
	dir, err := os.MkdirTemp(t.TempDir(), "run")
	if err != nil {
		rt.Fatal(err)
	}
	w := &counterWorld{
		rt:      rt,
		mode:    rapid.SampledFrom([]string{"remote", "no-remote", "stealth"}).Draw(rt, "mode"),
		common:  filepath.Join(dir, "common"),
		issued:  map[string]map[int]string{},
		written: map[string]map[int]bool{},
		counter: map[string]int{},
	}
	for _, category := range categories {
		w.issued[category], w.written[category] = map[int]string{}, map[int]bool{}
	}
	switch w.mode {
	case "remote":
		w.remote = newRemoteFake()
		w.store = Store{Root: dir, CommonDir: w.common, Remote: origin, VCS: w.remote, OS: runtime.GOOS}
	case "no-remote":
		w.store = Store{Root: dir, CommonDir: w.common, OS: runtime.GOOS}
	case "stealth":
		// A stealth config never reaches the remote it has: a nil VCS makes any reach panic.
		w.store = Store{Root: dir, CommonDir: w.common, Remote: origin, LocalOnly: true, OS: runtime.GOOS}
	}
	w.other = w.store
	w.other.Root = filepath.Join(dir, "another-worktree")
	file := "counters:\n"
	for _, category := range categories {
		start := rapid.IntRange(0, 12).Draw(rt, "counter of "+category+" at the start")
		w.counter[category] = start
		file += "  " + category + ": " + strconv.Itoa(start) + "\n"
		for _, n := range rapid.SliceOfN(rapid.IntRange(1, 15), 0, 3).Draw(rt, "ids of "+category+" at the start") {
			w.write(category, n)
		}
		if w.remote != nil && start > 0 {
			if err := w.remote.advance(rt.Context(), category, start); err != nil {
				rt.Fatal(err)
			}
		}
	}
	if w.remote == nil {
		if err := os.MkdirAll(filepath.Join(w.common, "itos"), 0o700); err != nil {
			rt.Fatal(err)
		}
		if err := os.WriteFile(w.counterFile(), []byte(file), 0o600); err != nil {
			rt.Fatal(err)
		}
	}
	return w
}

func (w *counterWorld) counterFile() string { return filepath.Join(w.common, "itos", localFile) }

// write is a person writing the id <category>-<n> into the repository.
func (w *counterWorld) write(category string, n int) {
	w.written[category][n] = true
	w.repo = append(w.repo, category+"-"+strconv.Itoa(n))
}

// floor is the repository's highest number of the category, as the callers
// of the counter read it: one less than the number nextid would give next.
func (w *counterWorld) floor(category string) int {
	next := nextid.Next(category+"-", w.repo, 1, nil)
	n, err := strconv.Atoi(strings.TrimPrefix(next, category+"-"))
	if err != nil {
		w.rt.Fatalf("nextid gave %q: %v", next, err)
	}
	return n - 1
}

// stored is the counter on disk or on the remote.
func (w *counterWorld) stored(category string) int {
	w.rt.Helper()
	if w.remote != nil {
		n, err := w.remote.stored(w.rt.Context(), category)
		if err != nil {
			w.rt.Fatal(err)
		}
		return n
	}
	counters, err := readCounters(w.counterFile())
	if err != nil {
		w.rt.Fatal(err)
	}
	return counters[category]
}

func (w *counterWorld) actions() map[string]func(*rapid.T) {
	actions := map[string]func(*rapid.T){
		"Mint": func(rt *rapid.T) { w.claim(rt, 1) },
		"Claim": func(rt *rapid.T) {
			w.claim(rt, rapid.IntRange(1, 4).Draw(rt, "numbers"))
		},
		"someone writes an id by hand": func(rt *rapid.T) {
			category := rapid.SampledFrom(categories).Draw(rt, "category")
			w.write(category, rapid.IntRange(1, w.counter[category]+8).Draw(rt, "number"))
		},
		"another clone claims": func(rt *rapid.T) {
			w.otherClaims(rapid.SampledFrom(categories).Draw(rt, "category"), rapid.IntRange(1, 3).Draw(rt, "numbers"))
		},
		"": func(rt *rapid.T) { w.check() },
	}
	if w.remote != nil {
		actions["the remote goes down"] = func(*rapid.T) { w.down, w.remote.Remote[origin].Down = true, true }
		actions["the remote comes back"] = func(*rapid.T) { w.down, w.remote.Remote[origin].Down = false, false }
		actions["claims land before the next pushes"] = func(rt *rapid.T) {
			w.lose = rapid.SliceOfN(rapid.IntRange(1, 3), 1, 18).Draw(rt, "claims")
		}
		actions["the remote refuses the next push"] = func(*rapid.T) { w.refuse = true }
	} else {
		actions["the counter file becomes unreadable"] = func(*rapid.T) { w.breakFile() }
		actions["the counter file is readable again"] = func(*rapid.T) { w.repairFile() }
	}
	return actions
}

// claim hands out count numbers of a category through Mint or Claim and holds
// the answer to the model.
func (w *counterWorld) claim(rt *rapid.T, count int) {
	rt.Helper()
	category := rapid.SampledFrom(categories).Draw(rt, "category")
	floor := w.floor(category)
	before := w.counter[category]
	if w.remote != nil {
		w.remote.beforePush = func() error {
			if len(w.lose) > 0 {
				w.otherClaims(category, w.lose[0])
				w.lose = w.lose[1:]
				return nil
			}
			if w.refuse {
				w.refuse = false
				return refuse()
			}
			return nil
		}
	}
	lose, refused := w.lose, w.refuse
	var first int
	var err error
	if count == 1 && rapid.Bool().Draw(rt, "through Mint") {
		first, err = Mint(w.store, category, floor)
	} else {
		first, err = Claim(w.store, category, floor, count)
	}
	w.lose, w.refuse = nil, false
	seen, fails := w.expect(before, lose, refused)
	if fails {
		if err == nil || first != 0 {
			rt.Fatalf("Claim(%s, floor %d, %d) = (%d, %v) with the counter at %d, want an error and no number", category, floor, count, first, err, before)
		}
		if !w.broken {
			got := w.stored(category)
			if got < w.counter[category] {
				rt.Fatalf("a failed claim lowered the counter of %s to %d from %d", category, got, w.counter[category])
			}
			w.counter[category] = got
		}
		return
	}
	if err != nil {
		rt.Fatalf("Claim(%s, floor %d, %d) failed: %v", category, floor, count, err)
	}
	if first <= floor || first <= seen {
		rt.Fatalf("Claim(%s, floor %d, %d) gave %d, not above the repository's highest %d and the counter %d", category, floor, count, first, floor, seen)
	}
	if want := max(floor, seen) + 1; first != want {
		rt.Fatalf("Claim(%s, floor %d, %d) gave %d, want the number after the higher of the two, %d (a gap only failures leave)", category, floor, count, first, want)
	}
	for n := first; n < first+count; n++ {
		w.take(category, n, "this claim")
	}
	w.counter[category] = first + count - 1
	if rapid.Bool().Draw(rt, "written to the repository") {
		for n := first; n < first+count; n++ {
			w.repo = append(w.repo, category+"-"+strconv.Itoa(n))
		}
	}
}

// expect is what a claim on a counter at `before` meets: the counter it sees
// when it succeeds, after the claims that land before its pushes, and
// whether it fails instead: the remote out of reach, a push refused outright,
// or the sixteenth push lost to a claim that landed first.
func (w *counterWorld) expect(before int, lose []int, refused bool) (seen int, fails bool) {
	if w.broken || w.down {
		return before, true
	}
	seen = before
	if w.remote == nil {
		return seen, false
	}
	for attempt := 0; attempt < 16; attempt++ {
		if attempt < len(lose) {
			seen += lose[attempt]
			continue
		}
		return seen, refused
	}
	return seen, true
}

// take records a number handed out, to whom, and refuses a number handed out
// twice, or, when it was handed to this repository, one written into it by
// hand: another clone's repository holds ids of its own.
func (w *counterWorld) take(category string, n int, to string) {
	w.rt.Helper()
	if prior, twice := w.issued[category][n]; twice {
		w.rt.Fatalf("%s-%d handed out to %s, and before to %s", category, n, to, prior)
	}
	if w.written[category][n] && to == "this claim" {
		w.rt.Fatalf("%s-%d handed out to %s, and a person wrote it into the repository", category, n, to)
	}
	w.issued[category][n] = to
}

// otherClaims is another clone (or another worktree of this one) claiming
// count numbers of a category, as it would with a repository of its own that
// holds no id: the numbers after the counter.
func (w *counterWorld) otherClaims(category string, count int) {
	w.rt.Helper()
	if w.broken {
		return
	}
	now := w.stored(category)
	if w.remote != nil {
		if err := w.remote.advance(w.rt.Context(), category, now+count); err != nil {
			w.rt.Fatal(err)
		}
	} else if first, err := Claim(w.other, category, 0, count); err != nil || first != now+1 {
		w.rt.Fatalf("another worktree's Claim(%s, 0, %d) = (%d, %v), want %d", category, count, first, err, now+1)
	}
	for n := now + 1; n <= now+count; n++ {
		w.take(category, n, "another clone")
	}
	w.counter[category] = now + count
}

// breakFile makes the file counter's path a folder, which no OS lets a file
// be read from or written over, keeping what the file held.
func (w *counterWorld) breakFile() {
	w.rt.Helper()
	if w.broken {
		return
	}
	data, err := os.ReadFile(w.counterFile())
	w.saved, w.hadFile = data, err == nil
	if err := errors.Join(os.Remove(w.counterFile()), os.Mkdir(w.counterFile(), 0o700)); err != nil && w.hadFile {
		w.rt.Fatal(err)
	}
	w.broken = true
}

func (w *counterWorld) repairFile() {
	w.rt.Helper()
	if !w.broken {
		return
	}
	if err := os.Remove(w.counterFile()); err != nil {
		w.rt.Fatal(err)
	}
	if w.hadFile {
		if err := os.WriteFile(w.counterFile(), w.saved, 0o600); err != nil {
			w.rt.Fatal(err)
		}
	}
	w.broken = false
}

// check holds the counter to the model after a step: it holds exactly what
// the model says (the claims that failed left no gap in these runs), is never
// behind a number handed out, and is where ADR-0045 puts it and nowhere else.
func (w *counterWorld) check() {
	w.rt.Helper()
	if w.mode == "remote" {
		if _, err := os.Stat(filepath.Join(w.common, "itos")); !errors.Is(err, fs.ErrNotExist) {
			w.rt.Fatalf("a counter on the remote left a file counter in the git folder: %v", err)
		}
	}
	if w.broken {
		return
	}
	for _, category := range categories {
		got := w.stored(category)
		highest := slices.Max(append(slices.Collect(maps.Keys(w.issued[category])), 0))
		if got < highest {
			w.rt.Fatalf("the counter of %s holds %d, behind %d, a number handed out", category, got, highest)
		}
		if got != w.counter[category] {
			w.rt.Fatalf("the counter of %s holds %d, the model has %d", category, got, w.counter[category])
		}
	}
}
