package proptest

import (
	"sync"
	"sync/atomic"
	"testing"
)

// fetchAndAdd is a counter whose operation adds its input and answers what
// the counter held before.
func fetchAndAdd(state int, in, out int) (int, bool) { return state + in, out == state }

func adds(ops ...Op[int, int]) []Op[int, int] { return ops }

func add(call, ret int64, out int) Op[int, int] {
	return Op[int, int]{Call: call, Return: ret, In: 1, Out: out}
}

func TestLinearizableAcceptsWhatSomeOrderExplains(t *testing.T) {
	for name, ops := range map[string][]Op[int, int]{
		"no operations":               nil,
		"one after the other":         adds(add(1, 2, 0), add(3, 4, 1)),
		"overlapping, reversed":       adds(add(1, 4, 1), add(2, 3, 0)),
		"touching":                    adds(add(1, 2, 1), add(2, 3, 0)),
		"three that need a backtrack": adds(add(1, 6, 2), add(2, 5, 0), add(3, 4, 1)),
	} {
		if !Linearizable(0, ops, fetchAndAdd) {
			t.Errorf("%s: not linearizable", name)
		}
	}
}

func TestLinearizableRefusesWhatNoOrderExplains(t *testing.T) {
	for name, ops := range map[string][]Op[int, int]{
		"the same answer twice":   adds(add(1, 4, 0), add(2, 3, 0)),
		"a lost update":           adds(add(1, 4, 0), add(2, 3, 0), add(5, 6, 1)),
		"against real time":       adds(add(1, 2, 1), add(3, 4, 0)),
		"an answer from nowhere":  adds(add(1, 2, 5)),
		"one that only fits late": adds(add(1, 2, 0), add(3, 4, 0)),
		"a gap in the answers":    adds(add(1, 6, 0), add(2, 5, 2), add(3, 4, 3)),
	} {
		if Linearizable(0, ops, fetchAndAdd) {
			t.Errorf("%s: linearizable", name)
		}
	}
}

func TestLinearizableStartsFromTheStateGiven(t *testing.T) {
	if !Linearizable(5, adds(add(1, 2, 5), add(3, 4, 6)), fetchAndAdd) {
		t.Error("a history from 5 was refused")
	}
	if Linearizable(0, adds(add(1, 2, 5)), fetchAndAdd) {
		t.Error("a history from 5 was accepted from 0")
	}
}

func TestLinearizableRefusesMoreOperationsThanItCanSearch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("64 operations were searched")
		}
	}()
	Linearizable(0, make([]Op[int, int], 64), fetchAndAdd)
}

// Do runs what it is given, answers what it returned and records every
// operation with a call before its return, the clock never repeating.
func TestHistoryRecordsConcurrentOperations(t *testing.T) {
	var h History[int, int]
	var counter atomic.Int64
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := h.Do(1, func() int { return int(counter.Add(1)) - 1 })
			if got < 0 || got > 7 {
				t.Errorf("Do answered %d", got)
			}
		}()
	}
	wg.Wait()
	ops := h.Ops()
	if len(ops) != 8 {
		t.Fatalf("%d operations recorded, want 8", len(ops))
	}
	seen := map[int64]bool{}
	for _, op := range ops {
		if op.In != 1 || op.Call >= op.Return || seen[op.Call] || seen[op.Return] {
			t.Errorf("operation %+v", op)
		}
		seen[op.Call], seen[op.Return] = true, true
	}
	if !Linearizable(0, ops, fetchAndAdd) {
		t.Error("an atomic counter's history is not linearizable")
	}
	ops[0].Out = -1
	if h.Ops()[0].Out == -1 {
		t.Error("Ops gave the history's own slice")
	}
}

// A point the search has refuted is not searched again: six operations that
// commute, and a seventh no order can place, are refuted in a number of steps
// that grows with the subsets of the six, not their orderings.
func TestLinearizableDoesNotSearchARefutedPointTwice(t *testing.T) {
	ops := make([]Op[int, int], 7)
	for i := range ops {
		ops[i] = Op[int, int]{Call: 1, Return: 2, In: 1}
	}
	ops[6].Out = -1
	steps := 0
	if Linearizable(0, ops, func(state, in, out int) (int, bool) {
		steps++
		return state + in, out >= 0
	}) {
		t.Fatal("an operation no order can place was placed")
	}
	if steps > 1000 {
		t.Errorf("%d steps: refuted points are searched again", steps)
	}
}

// The most it searches is 63 operations, one after the other.
func TestLinearizableSearchesTheMostItAllows(t *testing.T) {
	ops := make([]Op[int, int], 63)
	for i := range ops {
		ops[i] = add(int64(2*i+1), int64(2*i+2), i)
	}
	if !Linearizable(0, ops, fetchAndAdd) {
		t.Error("63 operations in order were refused")
	}
}
