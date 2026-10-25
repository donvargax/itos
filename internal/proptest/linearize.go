package proptest

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Op is one operation of a concurrent history: its input and its output, and
// the times it was invoked and returned on the history's clock.
type Op[I, O any] struct {
	Call, Return int64
	In           I
	Out          O
}

// History records the operations of goroutines running at once. Do is safe
// for concurrent use.
type History[I, O any] struct {
	mu    sync.Mutex
	clock atomic.Int64
	ops   []Op[I, O]
}

// Do runs call, an operation with input in, and records it with its output.
func (h *History[I, O]) Do(in I, call func() O) O {
	start := h.clock.Add(1)
	out := call()
	end := h.clock.Add(1)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ops = append(h.ops, Op[I, O]{Call: start, Return: end, In: in, Out: out})
	return out
}

// Ops are the operations recorded, in the order they returned.
func (h *History[I, O]) Ops() []Op[I, O] {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Op[I, O](nil), h.ops...)
}

// Linearizable is whether the operations can be put in a total order that
// keeps real time (an operation that returned before another was invoked
// stays before it) and that the sequential model accepts: step is given the
// model's state and an operation's input and output, and gives the state
// after it and whether the model could give that output there. It is the
// Wing and Gong search over the operations, memoizing the (done, state)
// pairs it has refuted, for the few operations of a property's history: at
// most 63.
func Linearizable[S comparable, I, O any](initial S, ops []Op[I, O], step func(state S, in I, out O) (S, bool)) bool {
	if len(ops) > 63 {
		panic(fmt.Sprintf("proptest: %d operations to linearize, at most 63", len(ops)))
	}
	type point struct {
		done  uint64
		state S
	}
	refuted := map[point]bool{}
	all := uint64(1)<<len(ops) - 1
	var search func(done uint64, state S) bool
	search = func(done uint64, state S) bool {
		if done == all {
			return true
		}
		if refuted[point{done, state}] {
			return false
		}
		for i, op := range ops {
			if done>>i&1 == 1 || !first(ops, done, i) {
				continue
			}
			if next, ok := step(state, op.In, op.Out); ok && search(done|1<<i, next) {
				return true
			}
		}
		refuted[point{done, state}] = true
		return false
	}
	return search(0, initial)
}

// first is whether no operation left to order returned before ops[i] began.
func first[I, O any](ops []Op[I, O], done uint64, i int) bool {
	for j, other := range ops {
		if j != i && done>>j&1 == 0 && other.Return < ops[i].Call {
			return false
		}
	}
	return true
}
