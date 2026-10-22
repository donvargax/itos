package porttest

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// recorder runs TrackerContract as testing does, each case on its own
// goroutine, a Fatalf ending it, and keeps what each case reported.
type recorder struct {
	name     string
	failures map[string][]string
}

func (r *recorder) Helper() {}

func (r *recorder) Context() context.Context { return context.Background() }

func (r *recorder) Errorf(format string, args ...any) {
	r.failures[r.name] = append(r.failures[r.name], fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

func (r *recorder) Run(name string, f func(t *recorder)) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(&recorder{name: name, failures: r.failures})
	}()
	<-done
	return len(r.failures[name]) == 0
}

// broken is the fake broken for one fault.
type broken struct {
	*Tracker
	fault string
	holds int
}

var errBroken = errors.New("broken")

func (b *broken) Load(ctx context.Context, names []string) ([]port.Doc, error) {
	docs, err := b.Tracker.Load(ctx, names)
	for i := range docs {
		switch {
		case b.fault == "phantom docs":
			docs[i].There = true
		case b.fault == "texts lost":
			docs[i].Text = ""
		case b.fault == "stealth loses" && b.Stealth:
			docs[i] = port.Doc{Name: docs[i].Name}
		}
	}
	return docs, err
}

func (b *broken) List(ctx context.Context, dir string) ([]string, error) {
	names, err := b.Tracker.List(ctx, dir)
	switch {
	case b.fault == "missing folder fails" && dir == "gone":
		return nil, errBroken
	case b.fault == "empty folder phantom" && len(names) == 0:
		return []string{"phase-0.yaml"}, nil
	case b.fault == "list unsorted":
		slices.Reverse(names)
	}
	return names, err
}

func (b *broken) Hold(ctx context.Context) (port.Held, error) {
	switch {
	case b.fault == "hold fails":
		return nil, errBroken
	case b.fault == "project holds once" && !b.Stealth:
		if b.holds++; b.holds > 1 {
			return nil, errBroken
		}
	}
	h, err := b.Tracker.Hold(ctx)
	if err != nil && b.fault == "held twice" {
		return free{}, nil
	}
	if err != nil {
		return nil, err
	}
	b.holds++
	return &brokenHeld{h, b, b.holds, false}, nil
}

func (b *broken) Ready(ctx context.Context, names []string) error {
	if b.fault == "unready ignored" {
		was := b.Unready
		b.Unready = nil
		defer func() { b.Unready = was }()
	}
	err := b.Tracker.Ready(ctx, names)
	var dirty *port.Dirty
	switch {
	case b.fault == "never dirty" && errors.As(err, &dirty):
		return nil
	case b.fault == "stealth checks hooks" && b.Stealth && b.Unready != nil:
		return b.Unready
	case b.fault == "dirty beside" && err == nil && len(names) == 1:
		if b.Tracker.Ready(ctx, slices.Collect(func(yield func(string) bool) {
			for name := range b.disk {
				if !yield(name) {
					return
				}
			}
		})) != nil {
			return &port.Dirty{Name: names[0]}
		}
	}
	return err
}

func (b *broken) Transact(ctx context.Context, t port.Transaction) (string, error) {
	names := make([]string, len(t.Changes))
	for i, c := range t.Changes {
		names[i] = c.Name
	}
	var dirty *port.Dirty
	switch {
	case b.fault == "transact fails":
		return "", errBroken
	case b.fault == "no words":
		t.Words = nil
	case b.fault == "never quiet":
		t.Quiet = false
	case b.fault == "transact ignores dirty" && errors.As(b.Tracker.Ready(ctx, names), &dirty):
		for _, c := range t.Changes {
			b.Edit(c.Name, c.Text)
		}
		return "", nil
	case b.fault == "transact ignores unready" && b.Unready != nil:
		was := b.Unready
		b.Unready = nil
		defer func() { b.Unready = was }()
	case b.fault == "stealth commits" && b.Stealth:
		_, err := b.Tracker.Transact(ctx, t)
		return "1", err
	case b.fault == "stealth speaks" && b.Stealth && t.Words != nil:
		fmt.Fprint(t.Words, "written\n")
	case b.fault == "stealth refuses" && b.Stealth && b.Refuse:
		return "", &port.Refused{Step: "the commit of " + and(names), Code: 1}
	}
	sha, err := b.Tracker.Transact(ctx, t)
	var refused *port.Refused
	switch {
	case b.fault == "one sha" && sha != "":
		return "1", nil
	case b.fault == "commit not held" && err == nil && t.Quiet:
		b.mu.Lock()
		for _, name := range names {
			delete(b.head, name)
		}
		b.mu.Unlock()
	case b.fault == "refusal unrestored" && errors.As(err, &refused):
		for _, c := range t.Changes {
			b.Edit(c.Name, c.Text)
		}
	case b.fault == "refusal plain" && errors.As(err, &refused):
		return "", errBroken
	}
	return sha, err
}

func (b *broken) Reserve(ctx context.Context, series string, floor, count int) (int, error) {
	switch {
	case b.fault == "floor ignored":
		floor = 0
	case b.fault == "counter ignored":
		b.mu.Lock()
		delete(b.counters, series)
		b.mu.Unlock()
	case b.fault == "series shared" && validSeries(series):
		series = "slice"
	case b.fault == "any series" && !validSeries(series):
		return 1, nil
	case b.fault == "any count" && count < 1:
		return 1, nil
	}
	first, err := b.Tracker.Reserve(ctx, series, floor, count)
	switch {
	case b.fault == "runs overlap" && err == nil:
		b.mu.Lock()
		b.counters[series] = first
		b.mu.Unlock()
	case b.fault == "refusal reserves" && err != nil && validSeries(series):
		b.mu.Lock()
		b.counters[series]++
		b.mu.Unlock()
	case b.fault == "overflow wraps" && err != nil && validSeries(series) && count >= 1:
		return floor + 1, nil
	}
	return first, err
}

// brokenHeld is a Hold broken's Hold took, its nth.
type brokenHeld struct {
	port.Held
	b        *broken
	n        int
	released bool
}

func (h *brokenHeld) Release(ctx context.Context) error {
	switch {
	case h.b.fault == "project release fails" && !h.b.Stealth,
		h.b.fault == "release fails" && h.b.Stealth,
		h.b.fault == "last release fails" && h.b.Stealth && h.n == 2:
		return errBroken
	case h.b.fault == "release keeps" && h.b.Stealth:
		if h.released {
			return errBroken
		}
		h.released = true
		return nil
	}
	return h.Held.Release(ctx)
}

// The contract fails a tracker broken for any one of its cases, each case
// reporting what it asserts and no other case reporting anything: a
// contract that cannot fail proves nothing.
func TestTrackerContractFailsEachBrokenTracker(t *testing.T) {
	for _, c := range []struct {
		fault string
		want  map[string][]string
	}{
		{"", nil},
		{"phantom docs", map[string][]string{"loads": {"Load of documents never written"}, "hooks not ready": {"a transaction the hooks were not ready for left"}, "refused": {"a refused commit left"}}},
		{"texts lost", map[string][]string{"loads": {"Load after a transaction"}, "dirty": {"a transaction refused as dirty left"}, "refused": {"a refused commit left"}, "stealth": {"a transaction under stealth left"}}},
		{"stealth loses", map[string][]string{"stealth": {"a transaction under stealth left"}}},
		{"missing folder fails", map[string][]string{"lists": {"List of no folder"}}},
		{"empty folder phantom", map[string][]string{"lists": {"List of no folder", "List of an empty folder"}}},
		{"list unsorted", map[string][]string{"lists": {"List: "}}},
		{"transact fails", map[string][]string{"loads": {"Transact: "}, "lists": {"Transact: "}, "commits": {"Transact: "}, "dirty": {"Transact: "}, "refused": {"Transact: "}, "stealth": {"Transact under stealth"}}},
		{"no words", map[string][]string{"commits": {"a commit not quiet says nothing"}, "refused": {"a refused commit's words"}}},
		{"one sha", map[string][]string{"commits": {"a second commit"}}},
		{"never quiet", map[string][]string{"commits": {"a quiet commit says"}}},
		{"commit not held", map[string][]string{"commits": {"Ready of a document as committed"}}},
		{"never dirty", map[string][]string{"dirty": {"Ready: "}}},
		{"dirty beside", map[string][]string{"dirty": {"Ready of a clean document beside dirty ones"}}},
		{"transact ignores dirty", map[string][]string{"dirty": {"Transact over a dirty document", "a transaction refused as dirty left"}}},
		{"unready ignored", map[string][]string{"hooks not ready": {"Ready with the hooks not ready"}}},
		{"transact ignores unready", map[string][]string{"hooks not ready": {"Transact with the hooks not ready", "a transaction the hooks were not ready for left"}}},
		{"refusal unrestored", map[string][]string{"refused": {"a refused commit left", "Ready after a refused commit"}}},
		{"refusal plain", map[string][]string{"refused": {"a refused commit: "}}},
		{"stealth commits", map[string][]string{"stealth": {"Transact under stealth"}}},
		{"stealth speaks", map[string][]string{"stealth": {"a transaction under stealth says"}}},
		{"stealth checks hooks", map[string][]string{"stealth": {"Ready under stealth"}}},
		{"stealth refuses", map[string][]string{"stealth": {"a transaction under stealth no commit could take"}}},
		{"hold fails", map[string][]string{"holds in a project": {"Hold: "}, "holds under stealth": {"Hold: "}}},
		{"project holds once", map[string][]string{"holds in a project": {"a second Hold where commits order the writers"}}},
		{"project release fails", map[string][]string{"holds in a project": {"Release where commits order the writers"}}},
		{"held twice", map[string][]string{"holds under stealth": {"a held tracker was taken again"}}},
		{"release fails", map[string][]string{"holds under stealth": {"Release: "}}},
		{"release keeps", map[string][]string{"holds under stealth": {"a tracker given back cannot be taken"}}},
		{"last release fails", map[string][]string{"holds under stealth": {"Release after a retake"}}},
		{"floor ignored", map[string][]string{"reserves": {"Reserve past the floor", "Reserve past the counter", "Reserve past the largest number", "Reserve of the largest number", "Reserve past a counter at the largest number"}}},
		{"counter ignored", map[string][]string{"reserves": {"Reserve past the counter", "Reserve after a run", "Reserve past a counter at the largest number"}}},
		{"series shared", map[string][]string{"reserves": {"Reserve of a series of its own", "Reserve of a run", "Reserve after a run", "Reserve after a refusal"}}},
		{"runs overlap", map[string][]string{"reserves": {"Reserve after a run"}}},
		{"any series", map[string][]string{"reserves": {"Reserve of the series \"\"", "Reserve of the series \"a/b\"", "Reserve of the series \"a b\""}}},
		{"any count", map[string][]string{"reserves": {"Reserve of 0 numbers", "Reserve of -1 numbers"}}},
		{"refusal reserves", map[string][]string{"reserves": {"Reserve after a refusal"}}},
		{"overflow wraps", map[string][]string{"reserves": {"Reserve past the largest number", "Reserve past a counter at the largest number"}}},
	} {
		r := &recorder{failures: map[string][]string{}}
		TrackerContract(r, func(_ *recorder, stealth bool) Fixture {
			f := fixture(stealth)
			f.Tracker = &broken{Tracker: f.Tracker.(*Tracker), fault: c.fault}
			return f
		})
		for name, got := range r.failures {
			want := c.want[name]
			ok := len(got) == len(want)
			for i := 0; ok && i < len(got); i++ {
				ok = strings.HasPrefix(got[i], want[i])
			}
			if !ok {
				t.Errorf("%q: case %q reported %q; want %q", c.fault, name, got, want)
			}
		}
		for name := range c.want {
			if _, ok := r.failures[name]; !ok {
				t.Errorf("%q: case %q reported nothing; want %q", c.fault, name, c.want[name])
			}
		}
	}
}

// The names are said as a list is.
func TestAndSaysTheNames(t *testing.T) {
	for names, want := range map[string]string{"": "", "a": "a", "a b": "a and b", "a b c": "a, b and c"} {
		if got := and(strings.Fields(names)); got != want {
			t.Errorf("and(%q) = %q; want %q", names, got, want)
		}
	}
}
