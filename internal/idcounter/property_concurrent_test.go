package idcounter

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/donvargax/itos/v8/internal/proptest"
	"pgregory.net/rapid"
)

// claimIn and claimOut are a Claim as a concurrent history records it: the
// repository's highest id and how many numbers the caller asked for, and the
// first number it was given or why it was given none.
type claimIn struct{ floor, count int }

type claimOut struct {
	first int
	err   error
}

// plan is what each goroutine does: the claims it makes, one after the other.
type plan [][]claimIn

func drawPlan(rt *rapid.T, workers int) plan {
	claims := rapid.Custom(func(rt *rapid.T) claimIn {
		return claimIn{floor: rapid.IntRange(0, 12).Draw(rt, "floor"), count: rapid.IntRange(1, 3).Draw(rt, "numbers")}
	})
	plans := make(plan, workers)
	for i := range plans {
		plans[i] = rapid.SliceOfN(claims, 1, 3).Draw(rt, "claims of worker "+strconv.Itoa(i))
	}
	return plans
}

// counterSpec is the sequential counter the concurrent history must be
// explained by: a claim answers the number after the higher of the floor it
// was given and the counter, and leaves the counter at the last it gave. An
// answer that is an error is not a counter's.
func counterSpec(counter int, in claimIn, out claimOut) (int, bool) {
	if out.err != nil || out.first != max(in.floor, counter)+1 {
		return counter, false
	}
	return out.first + in.count - 1, true
}

// run is the plans' claims made at once, one goroutine per store, through
// Claim, and the history they made.
func run(category string, stores []Store, plans plan) *proptest.History[claimIn, claimOut] {
	var history proptest.History[claimIn, claimOut]
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	for i, store := range stores {
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			for _, in := range plans[i] {
				history.Do(in, func() claimOut {
					first, err := Claim(store, category, in.floor, in.count)
					return claimOut{first, err}
				})
			}
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	return &history
}

// holds is what the concurrent history of a category must satisfy, whatever
// the claims met: no number twice, the history explained by the sequential
// counter, from the counter's value when it began, and the counter at the
// last number any claim was given. A claim that failed was given no number
// and takes no part in the order; it is a failure of the property unless
// allowed says that error is one the counter may give under contention.
func holds(rt *rapid.T, history *proptest.History[claimIn, claimOut], start, stored int, allowed func(error) bool) {
	rt.Helper()
	var ok []proptest.Op[claimIn, claimOut]
	given := map[int]bool{}
	last := start
	for _, op := range history.Ops() {
		if op.Out.err != nil {
			if allowed == nil || !allowed(op.Out.err) {
				rt.Fatalf("Claim(floor %d, %d) failed: %v", op.In.floor, op.In.count, op.Out.err)
			}
			if op.Out.first != 0 {
				rt.Fatalf("a failed Claim gave %d", op.Out.first)
			}
			continue
		}
		ok = append(ok, op)
		for n := op.Out.first; n < op.Out.first+op.In.count; n++ {
			if given[n] {
				rt.Fatalf("%d handed out twice, history %v", n, describe(history.Ops()))
			}
			given[n] = true
			last = max(last, n)
		}
	}
	if !proptest.Linearizable(start, ok, counterSpec) {
		rt.Fatalf("no order of the claims explains what they were given: %v", describe(ok))
	}
	if stored < last {
		rt.Fatalf("the counter holds %d, behind %d, a number handed out", stored, last)
	}
	if stored != last {
		rt.Fatalf("the counter holds %d, the last number handed out is %d", stored, last)
	}
}

func describe(ops []proptest.Op[claimIn, claimOut]) string {
	var lines []string
	for _, op := range slices.SortedFunc(slices.Values(ops), func(a, b proptest.Op[claimIn, claimOut]) int { return int(a.Call - b.Call) }) {
		lines = append(lines, "["+strconv.FormatInt(op.Call, 10)+","+strconv.FormatInt(op.Return, 10)+"] Claim(floor "+
			strconv.Itoa(op.In.floor)+", "+strconv.Itoa(op.In.count)+") = "+strconv.Itoa(op.Out.first)+errText(op.Out.err))
	}
	return "\n\t" + strings.Join(lines, "\n\t")
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return " " + err.Error()
}

// The file counter's lock holds under claims at once from worktrees that
// share one git folder: every claim answers, in an order that respects
// real time, what a sequential counter would, from a counter that already
// holds some numbers.
func TestPropertyConcurrentFileCounterIsLinearizable(t *testing.T) {
	proptest.Check(t, func(rt *rapid.T) {
		common := t.TempDir()
		stores := make([]Store, rapid.IntRange(2, 4).Draw(rt, "worktrees"))
		for i := range stores {
			stores[i] = Store{Root: filepath.Join(common, "worktree-"+strconv.Itoa(i)), CommonDir: common, OS: runtime.GOOS}
		}
		start := rapid.IntRange(0, 9).Draw(rt, "counter at the start")
		if start > 0 {
			if err := os.MkdirAll(filepath.Join(common, "itos"), 0o700); err != nil {
				rt.Fatal(err)
			}
			text := "counters:\n  slice: " + strconv.Itoa(start) + "\n"
			if err := os.WriteFile(filepath.Join(common, "itos", localFile), []byte(text), 0o600); err != nil {
				rt.Fatal(err)
			}
		}
		history := run("slice", stores, drawPlan(rt, len(stores)))
		counters, err := readCounters(filepath.Join(common, "itos", localFile))
		if err != nil {
			rt.Fatal(err)
		}
		holds(rt, history, start, counters["slice"], nil)
	}, proptest.Checks(60))
}
