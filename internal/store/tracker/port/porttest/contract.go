package porttest

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// Fixture is a tracker TrackerContract runs against, and what it does
// behind the tracker's back: Edit sets a document's text in the working
// tree, a change no commit holds; Refuse makes every later commit refused,
// as a commit-msg hook that says "refused" and exits 1; Unready makes the
// hooks that check a commit not ready.
type Fixture struct {
	Tracker port.Tracker
	Edit    func(name, text string)
	Refuse  func()
	Unready func()
}

// T is what TrackerContract asks of the test it runs in: *testing.T is one,
// and so is the recorder that proves each assertion fires.
type T[C any] interface {
	Helper()
	Context() context.Context
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Run(name string, f func(t C)) bool
}

// The documents TrackerContract writes.
var (
	registry = "registry.yaml"
	first    = filepath.Join("tasks", "phase-1.yaml")
	second   = filepath.Join("tasks", "phase-2.yaml")
)

// TrackerContract holds what every tracker does, the fake and the
// adapter's alike. open stands the test in a project with no document, its
// folder tasks there and empty, whose commits the tracker makes, or under
// a stealth config, which commits nothing, when stealth; its lock's wait
// short. The disk's own (modes, a write cut short, a git that cannot run)
// stays the adapter's tests'.
func TrackerContract[C T[C]](t C, open func(t C, stealth bool) Fixture) {
	t.Helper()
	t.Run("loads", func(t C) {
		s := open(t, false).Tracker
		docs, err := s.Load(t.Context(), []string{registry, first})
		if err != nil || !slices.Equal(docs, []port.Doc{{Name: registry}, {Name: first}}) {
			t.Fatalf("Load of documents never written: %v, %v", docs, err)
		}
		if sha, err := s.Transact(t.Context(), created(first)); err != nil || sha == "" {
			t.Fatalf("Transact: %q, %v", sha, err)
		}
		docs, err = s.Load(t.Context(), []string{registry, first})
		if want := []port.Doc{{Name: registry}, {Name: first, Text: first + "\n", There: true}}; err != nil || !slices.Equal(docs, want) {
			t.Errorf("Load after a transaction: %v, %v; want %v", docs, err, want)
		}
	})
	t.Run("lists", func(t C) {
		s := open(t, false).Tracker
		if names, err := s.List(t.Context(), "gone"); len(names) != 0 || err != nil {
			t.Errorf("List of no folder: %q, %v", names, err)
		}
		if names, err := s.List(t.Context(), "tasks"); len(names) != 0 || err != nil {
			t.Errorf("List of an empty folder: %q, %v", names, err)
		}
		if sha, err := s.Transact(t.Context(), created(second, registry, first)); err != nil || sha == "" {
			t.Fatalf("Transact: %q, %v", sha, err)
		}
		want := []string{filepath.Base(first), filepath.Base(second)}
		if names, err := s.List(t.Context(), "tasks"); err != nil || !slices.Equal(names, want) {
			t.Errorf("List: %q, %v; want %q", names, err, want)
		}
	})
	t.Run("commits", func(t C) {
		s := open(t, false).Tracker
		var words strings.Builder
		add := created(registry)
		add.Words = &words
		sha, err := s.Transact(t.Context(), add)
		if err != nil || sha == "" {
			t.Fatalf("Transact: %q, %v", sha, err)
		}
		if words.Len() == 0 {
			t.Errorf("a commit not quiet says nothing")
		}
		words.Reset()
		again, err := s.Transact(t.Context(), port.Transaction{
			Changes:    []port.Change{{Name: registry, Old: registry + "\n", Text: "items: []\n"}},
			Paragraphs: []string{"docs: edit the registry", "Edit it."}, Quiet: true, Words: &words,
		})
		if err != nil || again == "" || again == sha {
			t.Errorf("a second commit: %q after %q, %v", again, sha, err)
		}
		if words.Len() != 0 {
			t.Errorf("a quiet commit says %q", words.String())
		}
		if err := s.Ready(t.Context(), []string{registry}); err != nil {
			t.Errorf("Ready of a document as committed: %v", err)
		}
	})
	t.Run("dirty", func(t C) {
		f := open(t, false)
		if _, err := f.Tracker.Transact(t.Context(), created(registry, first)); err != nil {
			t.Fatalf("Transact: %v", err)
		}
		f.Edit(first, "edited\n")
		f.Edit(second, "new\n")
		if err := f.Tracker.Ready(t.Context(), []string{registry}); err != nil {
			t.Errorf("Ready of a clean document beside dirty ones: %v", err)
		}
		var dirty *port.Dirty
		if err := f.Tracker.Ready(t.Context(), []string{registry, second, first}); !errors.As(err, &dirty) || dirty.Name != second {
			t.Errorf("Ready: %v; want %s dirty", err, second)
		}
		_, err := f.Tracker.Transact(t.Context(), port.Transaction{Changes: []port.Change{
			{Name: registry, Old: registry + "\n", Text: "items: []\n"},
			{Name: first, Old: "edited\n", Text: "x\n"},
		}, Paragraphs: []string{"docs: edit"}})
		if !errors.As(err, &dirty) || dirty.Name != first {
			t.Errorf("Transact over a dirty document: %v; want %s dirty", err, first)
		}
		docs, _ := f.Tracker.Load(t.Context(), []string{registry, first})
		if want := []port.Doc{{Name: registry, Text: registry + "\n", There: true}, {Name: first, Text: "edited\n", There: true}}; !slices.Equal(docs, want) {
			t.Errorf("a transaction refused as dirty left %v; want %v", docs, want)
		}
	})
	t.Run("hooks not ready", func(t C) {
		f := open(t, false)
		f.Edit(second, "new\n")
		f.Unready()
		var dirty *port.Dirty
		if err := f.Tracker.Ready(t.Context(), []string{second}); err == nil || errors.As(err, &dirty) {
			t.Errorf("Ready with the hooks not ready: %v", err)
		}
		if _, err := f.Tracker.Transact(t.Context(), created(registry)); err == nil || errors.As(err, &dirty) {
			t.Errorf("Transact with the hooks not ready: %v", err)
		}
		if docs, _ := f.Tracker.Load(t.Context(), []string{registry}); !slices.Equal(docs, []port.Doc{{Name: registry}}) {
			t.Errorf("a transaction the hooks were not ready for left %v", docs)
		}
	})
	t.Run("refused", func(t C) {
		f := open(t, false)
		if _, err := f.Tracker.Transact(t.Context(), created(registry)); err != nil {
			t.Fatalf("Transact: %v", err)
		}
		f.Refuse()
		var words strings.Builder
		sha, err := f.Tracker.Transact(t.Context(), port.Transaction{Changes: []port.Change{
			{Name: registry, Old: registry + "\n", Text: "items: []\n"},
			{Name: first, Text: first + "\n", Created: true},
		}, Paragraphs: []string{"docs: add"}, Words: &words})
		var refused *port.Refused
		if step := "the commit of " + registry + " and " + first; sha != "" || !errors.As(err, &refused) ||
			refused.Step != step || refused.Code != 1 || refused.Restore != nil {
			t.Errorf("a refused commit: %q, %v; want %s refused, exit 1, all put back", sha, err, step)
		}
		if !strings.Contains(words.String(), "refused") {
			t.Errorf("a refused commit's words: %q", words.String())
		}
		docs, _ := f.Tracker.Load(t.Context(), []string{registry, first})
		if want := []port.Doc{{Name: registry, Text: registry + "\n", There: true}, {Name: first}}; !slices.Equal(docs, want) {
			t.Errorf("a refused commit left %v; want %v", docs, want)
		}
		if err := f.Tracker.Ready(t.Context(), []string{registry, first}); err != nil {
			t.Errorf("Ready after a refused commit: %v", err)
		}
	})
	t.Run("stealth", func(t C) {
		f := open(t, true)
		var words strings.Builder
		add := created(registry)
		add.Words = &words
		if sha, err := f.Tracker.Transact(t.Context(), add); sha != "" || err != nil {
			t.Fatalf("Transact under stealth: %q, %v", sha, err)
		}
		if words.Len() != 0 {
			t.Errorf("a transaction under stealth says %q", words.String())
		}
		if docs, _ := f.Tracker.Load(t.Context(), []string{registry}); !slices.Equal(docs, []port.Doc{{Name: registry, Text: registry + "\n", There: true}}) {
			t.Errorf("a transaction under stealth left %v", docs)
		}
		f.Edit(first, "edited\n")
		f.Unready()
		f.Refuse()
		if err := f.Tracker.Ready(t.Context(), []string{first}); err != nil {
			t.Errorf("Ready under stealth: %v", err)
		}
		change := port.Transaction{Changes: []port.Change{{Name: first, Old: "edited\n", Text: "x\n"}}, Paragraphs: []string{"docs: edit"}}
		if sha, err := f.Tracker.Transact(t.Context(), change); sha != "" || err != nil {
			t.Errorf("a transaction under stealth no commit could take: %q, %v", sha, err)
		}
	})
	t.Run("holds in a project", func(t C) {
		s := open(t, false).Tracker
		one, err := s.Hold(t.Context())
		if err != nil {
			t.Fatalf("Hold: %v", err)
		}
		other, err := s.Hold(t.Context())
		if err != nil {
			t.Fatalf("a second Hold where commits order the writers: %v", err)
		}
		if err := errors.Join(one.Release(t.Context()), other.Release(t.Context())); err != nil {
			t.Errorf("Release where commits order the writers: %v", err)
		}
	})
	t.Run("holds under stealth", func(t C) {
		s := open(t, true).Tracker
		held, err := s.Hold(t.Context())
		if err != nil {
			t.Fatalf("Hold: %v", err)
		}
		if again, err := s.Hold(t.Context()); err == nil {
			again.Release(t.Context())
			t.Fatalf("a held tracker was taken again")
		}
		if err := held.Release(t.Context()); err != nil {
			t.Fatalf("Release: %v", err)
		}
		after, err := s.Hold(t.Context())
		if err != nil {
			t.Fatalf("a tracker given back cannot be taken: %v", err)
		}
		if err := after.Release(t.Context()); err != nil {
			t.Errorf("Release after a retake: %v", err)
		}
	})
}

// created is the transaction that creates the documents, each its name a
// line.
func created(names ...string) port.Transaction {
	t := port.Transaction{Paragraphs: []string{"docs: add " + strings.Join(names, " "), "Add them."}}
	for _, name := range names {
		t.Changes = append(t.Changes, port.Change{Name: name, Text: name + "\n", Created: true})
	}
	return t
}
