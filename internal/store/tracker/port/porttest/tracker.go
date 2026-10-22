package porttest

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// Tracker is the tracker store in memory: the documents of a working tree,
// and of the last commit, which a transaction commits to unless Stealth,
// where nothing is committed and Hold orders the writers. Unready is the
// error of the hooks not ready, nil when they are; Refuse refuses every
// commit, its words "refused". The zero Tracker is an empty project; a
// Tracker is safe for concurrent use.
type Tracker struct {
	Stealth bool
	Unready error
	Refuse  bool

	mu      sync.Mutex
	disk    map[string]string
	head    map[string]string
	commits int
	held    bool
}

var _ port.Tracker = &Tracker{}

// Edit sets the document name's text in the working tree, behind the
// tracker's back: a change no commit holds.
func (s *Tracker) Edit(name, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disk == nil {
		s.disk = map[string]string{}
	}
	s.disk[name] = text
}

// Load is the working tree's documents.
func (s *Tracker) Load(_ context.Context, names []string) ([]port.Doc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	docs := make([]port.Doc, len(names))
	for i, name := range names {
		text, there := s.disk[name]
		docs[i] = port.Doc{Name: name, Text: text, There: there}
	}
	return docs, nil
}

// List is the names directly in dir: its documents' and the first folder of
// those deeper.
func (s *Tracker) List(_ context.Context, dir string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir = filepath.Clean(dir)
	seen := map[string]bool{}
	for name := range s.disk {
		rel, err := filepath.Rel(dir, name)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		first, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
		seen[first] = true
	}
	return slices.Sorted(maps.Keys(seen)), nil
}

// Hold takes the one writer's place under Stealth, an error at once when it
// is taken: the fake has no other process to wait for. Else it is at once.
func (s *Tracker) Hold(context.Context) (port.Held, error) {
	if !s.Stealth {
		return free{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held {
		return nil, errors.New("the tracker is held by another writer")
	}
	s.held = true
	return &held{s}, nil
}

// Ready is Unready, else the first of names whose working tree's text, or
// whether it is there, is not the last commit's; nil under Stealth.
func (s *Tracker) Ready(_ context.Context, names []string) error {
	if s.Stealth {
		return nil
	}
	if s.Unready != nil {
		return s.Unready
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, name := range names {
		text, there := s.disk[name]
		committed, tracked := s.head[name]
		if there != tracked || text != committed {
			return &port.Dirty{Name: name}
		}
	}
	return nil
}

// Transact writes the changes to the working tree and, unless Stealth,
// commits them, its words the commit's number and first paragraph unless
// Quiet; a refused commit puts the working tree back.
func (s *Tracker) Transact(ctx context.Context, t port.Transaction) (string, error) {
	names := make([]string, len(t.Changes))
	for i, c := range t.Changes {
		names[i] = c.Name
	}
	if err := s.Ready(ctx, names); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disk == nil {
		s.disk = map[string]string{}
	}
	for _, c := range t.Changes {
		s.disk[c.Name] = c.Text
	}
	if s.Stealth {
		return "", nil
	}
	if s.Refuse {
		for _, c := range t.Changes {
			s.disk[c.Name] = c.Old
			if c.Created {
				delete(s.disk, c.Name)
			}
		}
		say(t, "refused\n")
		return "", &port.Refused{Step: "the commit of " + and(names), Code: 1}
	}
	if s.head == nil {
		s.head = map[string]string{}
	}
	for _, c := range t.Changes {
		s.head[c.Name] = c.Text
	}
	s.commits++
	if !t.Quiet && len(t.Paragraphs) > 0 {
		say(t, fmt.Sprintf("[%d] %s\n", s.commits, t.Paragraphs[0]))
	}
	return fmt.Sprintf("%040d", s.commits), nil
}

// say writes the words to the transaction's Words, when it has one.
func say(t port.Transaction, words string) {
	if t.Words != nil {
		fmt.Fprint(t.Words, words)
	}
}

// and is the names joined as a list is said: "a", "a and b", "a, b and c".
func and(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// free is the Held of a Hold that orders nothing.
type free struct{}

func (free) Release(context.Context) error { return nil }

// held is the writer's place the fake's Hold took under Stealth.
type held struct{ s *Tracker }

// Release gives the place back; one not held is an error.
func (h *held) Release(context.Context) error {
	h.s.mu.Lock()
	defer h.s.mu.Unlock()
	if !h.s.held {
		return errors.New("the tracker is not held")
	}
	h.s.held = false
	return nil
}
