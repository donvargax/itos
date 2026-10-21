package porttest

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/store/private/port"
)

// Fixture is a store StoreContract runs against: Store over a clone, or
// over none, and three paths, Kept in the clone's work tree and not
// ignored, Ignored in it and ignored, and Outside out of it.
type Fixture struct {
	Store                  port.Store
	Kept, Ignored, Outside string
}

// T is what StoreContract asks of the test it runs in: *testing.T is one,
// and so is the recorder that proves each assertion fires.
type T[C any] interface {
	Helper()
	Context() context.Context
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Run(name string, f func(t C)) bool
}

// StoreContract holds what every private store does, the fake and the
// adapter's alike. open lays out a fixture: a clone whose work tree holds
// Kept and ignores Ignored when repository, else a folder in no repository,
// its store empty, its lock wait short. The disk's own (modes, a cut-short
// write) stays the adapter's tests'.
func StoreContract[C T[C]](t C, open func(t C, repository bool) Fixture) {
	t.Helper()
	t.Run("paths", func(t C) {
		s := open(t, true).Store
		first, err := s.Path(t.Context(), "threads.yaml")
		if err != nil || !filepath.IsAbs(first) || filepath.Base(first) != "threads.yaml" {
			t.Fatalf("Path: %q, %v; want an absolute path ending in threads.yaml", first, err)
		}
		other, err := s.Path(t.Context(), "drafts.yaml")
		if err != nil || filepath.Dir(other) != filepath.Dir(first) {
			t.Errorf("two names are not in one folder: %q, %q, %v", first, other, err)
		}
		if again, err := s.Path(t.Context(), "threads.yaml"); again != first || err != nil {
			t.Errorf("Path moved: %q then %q, %v", first, again, err)
		}
	})
	t.Run("reads and writes", func(t C) {
		s := open(t, true).Store
		if text, ok, err := s.Read(t.Context(), "threads.yaml"); ok || err != nil || len(text) != 0 {
			t.Fatalf("a file never written: %q, %v, %v", text, ok, err)
		}
		given := []byte("threads: []\n")
		if err := s.Write(t.Context(), "threads.yaml", given); err != nil {
			t.Fatalf("Write: %v", err)
		}
		given[0] = 'X'
		text, ok, err := s.Read(t.Context(), "threads.yaml")
		if !ok || err != nil || string(text) != "threads: []\n" {
			t.Fatalf("read back: %q, %v, %v", text, ok, err)
		}
		text[0] = 'Y'
		if err := s.Write(t.Context(), "threads.yaml", []byte("threads:\n  - id: a\n")); err != nil {
			t.Fatalf("a second Write: %v", err)
		}
		if again, _, err := s.Read(t.Context(), "threads.yaml"); err != nil || !bytes.Equal(again, []byte("threads:\n  - id: a\n")) {
			t.Errorf("a second write is not what is read: %q, %v", again, err)
		}
		if other, ok, err := s.Read(t.Context(), "drafts.yaml"); ok || err != nil || len(other) != 0 {
			t.Errorf("one name's write reads under another: %q, %v, %v", other, ok, err)
		}
	})
	t.Run("locks", func(t C) {
		s := open(t, true).Store
		held, err := s.Hold(t.Context(), "threads.yaml")
		if err != nil {
			t.Fatalf("Hold: %v", err)
		}
		path, _ := s.Path(t.Context(), "threads.yaml")
		if again, err := s.Hold(t.Context(), "threads.yaml"); err == nil {
			again.Release(t.Context())
			t.Fatalf("a held lock was taken again")
		} else if !strings.Contains(err.Error(), path+".lock") {
			t.Errorf("a held lock's refusal does not name %s.lock: %v", path, err)
		}
		other, err := s.Hold(t.Context(), "drafts.yaml")
		if err != nil {
			t.Fatalf("one name's lock holds another's: %v", err)
		}
		if err := other.Release(t.Context()); err != nil {
			t.Fatalf("Release of the other name: %v", err)
		}
		if err := s.Write(t.Context(), "threads.yaml", []byte("threads: []\n")); err != nil {
			t.Errorf("a write under the lock: %v", err)
		}
		if err := held.Release(t.Context()); err != nil {
			t.Fatalf("Release: %v", err)
		}
		if err := held.Release(t.Context()); err == nil {
			t.Errorf("a lock given back twice")
		}
		after, err := s.Hold(t.Context(), "threads.yaml")
		if err != nil {
			t.Fatalf("a lock given back cannot be taken: %v", err)
		}
		if err := after.Release(t.Context()); err != nil {
			t.Fatalf("Release after a retake: %v", err)
		}
	})
	t.Run("ignored", func(t C) {
		f := open(t, true)
		for p, want := range map[string]bool{f.Kept: true, f.Ignored: false, f.Outside: false} {
			if got := f.Store.NotIgnored(t.Context(), p); got != want {
				t.Errorf("NotIgnored(%s) is %v", p, got)
			}
		}
	})
	t.Run("no repository", func(t C) {
		f := open(t, false)
		var none *port.NoRepository
		if _, err := f.Store.Path(t.Context(), "threads.yaml"); !errors.As(err, &none) {
			t.Errorf("Path: %v", err)
		}
		if _, _, err := f.Store.Read(t.Context(), "threads.yaml"); !errors.As(err, &none) {
			t.Errorf("Read: %v", err)
		}
		if err := f.Store.Write(t.Context(), "threads.yaml", nil); !errors.As(err, &none) {
			t.Errorf("Write: %v", err)
		}
		if _, err := f.Store.Hold(t.Context(), "threads.yaml"); !errors.As(err, &none) {
			t.Errorf("Hold: %v", err)
		}
		if f.Store.NotIgnored(t.Context(), f.Kept) {
			t.Errorf("NotIgnored(%s) with no repository", f.Kept)
		}
	})
}
