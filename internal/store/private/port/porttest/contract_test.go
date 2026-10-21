package porttest

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/private/port"
)

// recorder runs StoreContract as testing does, each case on its own
// goroutine, a Fatalf ending it, and keeps what each case reported.
type recorder struct {
	ctx      context.Context
	name     string
	failures map[string][]string
}

func (r *recorder) Helper() {}

func (r *recorder) Context() context.Context { return r.ctx }

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
		f(&recorder{ctx: r.ctx, name: name, failures: r.failures})
	}()
	<-done
	return len(r.failures[name]) == 0
}

// broken is the fake broken for one fault; with no repository it is the
// fake's own but for the fault "plain errors".
type broken struct {
	*Store
	fault  string
	paths  int
	writes int
	kept   map[string][]byte
	holds  map[string]int
}

var errBroken = errors.New("broken")

func (b *broken) Path(ctx context.Context, name string) (string, error) {
	switch {
	case b.Folder == "" && b.fault == "plain errors":
		return "", errBroken
	case b.Folder == "":
	case b.fault == "relative path":
		return name, nil
	case b.fault == "names apart":
		return filepath.Join(b.Folder, name, name), nil
	case b.fault == "path moves":
		if b.paths++; b.paths > 2 {
			return filepath.Join(b.Folder, "moved", name), nil
		}
	}
	return b.Store.Path(ctx, name)
}

func (b *broken) Read(ctx context.Context, name string) ([]byte, bool, error) {
	switch {
	case b.Folder == "" && b.fault == "plain errors":
		return nil, false, errBroken
	case b.Folder == "":
	case b.fault == "phantom file":
		text, _, err := b.Store.Read(ctx, name)
		return text, err == nil, err
	case b.fault == "kept slices":
		text, ok := b.kept[name]
		return text, ok, nil
	case b.fault == "one file":
		name = "threads.yaml"
	}
	return b.Store.Read(ctx, name)
}

func (b *broken) Write(ctx context.Context, name string, text []byte) error {
	switch {
	case b.Folder == "" && b.fault == "plain errors":
		return errBroken
	case b.Folder == "":
	case b.fault == "write fails":
		return errBroken
	case b.fault == "kept slices":
		if b.kept == nil {
			b.kept = map[string][]byte{}
		}
		b.kept[name] = text
		return nil
	case b.fault == "second write fails":
		if b.writes++; b.writes == 2 {
			return errBroken
		}
	case b.fault == "second write lost":
		if _, ok, _ := b.Store.Read(ctx, name); ok {
			return nil
		}
	case b.fault == "one file":
		name = "threads.yaml"
	case b.fault == "write under lock fails":
		b.mu.Lock()
		held := b.held[name]
		b.mu.Unlock()
		if held {
			return errBroken
		}
	}
	return b.Store.Write(ctx, name, text)
}

func (b *broken) Hold(ctx context.Context, name string) (port.Held, error) {
	switch {
	case b.Folder == "" && b.fault == "plain errors":
		return nil, errBroken
	case b.Folder == "":
		return b.Store.Hold(ctx, name)
	case b.fault == "hold fails":
		return nil, errBroken
	case b.fault == "one lock":
		return b.Store.Hold(ctx, "threads.yaml")
	}
	h, err := b.Store.Hold(ctx, name)
	switch {
	case err != nil && b.fault == "hold twice":
		return &brokenHeld{b: b}, nil
	case err != nil && b.fault == "refusal unnamed":
		return nil, errBroken
	case err != nil:
		return nil, err
	}
	if b.holds == nil {
		b.holds = map[string]int{}
	}
	b.holds[name]++
	return &brokenHeld{h, b, name, b.holds[name], false}, nil
}

func (b *broken) NotIgnored(ctx context.Context, path string) bool {
	switch b.fault {
	case "all exposed":
		return true
	case "none exposed":
		return false
	}
	return b.Store.NotIgnored(ctx, path)
}

// brokenHeld is a lock broken's Hold took, the nth of its name; one with
// no Held is a lock granted while held.
type brokenHeld struct {
	port.Held
	b        *broken
	name     string
	n        int
	released bool
}

func (h *brokenHeld) Release(ctx context.Context) error {
	switch {
	case h.Held == nil:
		return nil
	case h.b.fault == "release fails",
		h.b.fault == "release of held fails" && h.name == "threads.yaml" && h.n == 1,
		h.b.fault == "last release fails" && h.name == "threads.yaml" && h.n == 2:
		return errBroken
	case h.b.fault == "release twice":
		h.Held.Release(ctx)
		return nil
	case h.b.fault == "release keeps":
		if h.released {
			return errBroken
		}
		h.released = true
		return nil
	}
	return h.Held.Release(ctx)
}

// The contract fails a store broken for any one of its cases, each case
// reporting what it asserts and no other case reporting anything: a contract
// that cannot fail proves nothing.
func TestStoreContractFailsEachBrokenStore(t *testing.T) {
	root, elsewhere := t.TempDir(), t.TempDir()
	for _, c := range []struct {
		fault string
		want  map[string][]string
	}{
		{"", nil},
		{"relative path", map[string][]string{"paths": {"Path: "}}},
		{"names apart", map[string][]string{"paths": {"two names are not in one folder"}, "locks": {"a held lock's refusal does not name"}}},
		{"path moves", map[string][]string{"paths": {"Path moved"}}},
		{"phantom file", map[string][]string{"reads and writes": {"a file never written"}}},
		{"write fails", map[string][]string{"reads and writes": {"Write: "}, "locks": {"a write under the lock"}}},
		{"kept slices", map[string][]string{"reads and writes": {"read back"}}},
		{"second write fails", map[string][]string{"reads and writes": {"a second Write: "}}},
		{"second write lost", map[string][]string{"reads and writes": {"a second write is not what is read"}}},
		{"one file", map[string][]string{"reads and writes": {"one name's write reads under another"}}},
		{"hold fails", map[string][]string{"locks": {"Hold: "}}},
		{"hold twice", map[string][]string{"locks": {"a held lock was taken again"}}},
		{"refusal unnamed", map[string][]string{"locks": {"a held lock's refusal does not name"}}},
		{"one lock", map[string][]string{"locks": {"one name's lock holds another's"}}},
		{"release fails", map[string][]string{"locks": {"Release of the other name: "}}},
		{"write under lock fails", map[string][]string{"locks": {"a write under the lock"}}},
		{"release of held fails", map[string][]string{"locks": {"Release: "}}},
		{"release twice", map[string][]string{"locks": {"a lock given back twice"}}},
		{"release keeps", map[string][]string{"locks": {"a lock given back cannot be taken"}}},
		{"last release fails", map[string][]string{"locks": {"Release after a retake: "}}},
		{"all exposed", map[string][]string{"ignored": {"NotIgnored(", "NotIgnored("}, "no repository": {"NotIgnored("}}},
		{"none exposed", map[string][]string{"ignored": {"NotIgnored("}}},
		{"exposed with no repository", map[string][]string{"no repository": {"NotIgnored("}}},
		{"plain errors", map[string][]string{"no repository": {"Path: ", "Read: ", "Write: ", "Hold: "}}},
	} {
		r := &recorder{ctx: t.Context(), failures: map[string][]string{}}
		StoreContract(r, func(_ *recorder, repository bool) Fixture {
			f := Fixture{
				Kept:    filepath.Join(root, "kept.md"),
				Ignored: filepath.Join(root, "ignored.md"),
				Outside: filepath.Join(elsewhere, "outside.md"),
			}
			s := &Store{Exposed: []string{f.Kept}}
			if repository {
				s.Folder = filepath.Join(root, ".git", "itos")
			} else if c.fault != "exposed with no repository" {
				s.Exposed = nil
			}
			f.Store = &broken{Store: s, fault: c.fault}
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
