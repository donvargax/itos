package porttest

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sync"

	"github.com/donvargax/itos/v8/internal/store/private/port"
)

// Store is the private store in memory, keeping its files and locks as the
// adapter keeps them on disk, at Folder, an absolute folder it never
// touches; Folder "" is no repository. NotIgnored is true for the paths in
// Exposed alone. The zero Store is no repository; a Store is safe for
// concurrent use.
type Store struct {
	Folder  string
	Exposed []string

	mu    sync.Mutex
	files map[string][]byte
	held  map[string]bool
}

var _ port.Store = &Store{}

// Path is name in Folder.
func (s *Store) Path(_ context.Context, name string) (string, error) {
	if s.Folder == "" {
		return "", &port.NoRepository{}
	}
	return filepath.Join(s.Folder, name), nil
}

// Read is the text Write last gave name, a copy, and whether there is one.
func (s *Store) Read(ctx context.Context, name string) ([]byte, bool, error) {
	if _, err := s.Path(ctx, name); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	text, ok := s.files[name]
	return slices.Clone(text), ok, nil
}

// Write keeps a copy of text as name's.
func (s *Store) Write(ctx context.Context, name string, text []byte) error {
	if _, err := s.Path(ctx, name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files == nil {
		s.files = map[string][]byte{}
	}
	s.files[name] = append([]byte{}, text...)
	return nil
}

// Hold takes name's lock, an error at once when it is held: the fake has no
// other process to wait for.
func (s *Store) Hold(ctx context.Context, name string) (port.Held, error) {
	file, err := s.Path(ctx, name)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held[name] {
		return nil, errors.New(file + ".lock is held by another itos")
	}
	if s.held == nil {
		s.held = map[string]bool{}
	}
	s.held[name] = true
	return &held{s, name}, nil
}

// NotIgnored is whether path is one of Exposed.
func (s *Store) NotIgnored(_ context.Context, path string) bool {
	return slices.Contains(s.Exposed, path)
}

// held is a lock the fake's Hold took.
type held struct {
	store *Store
	name  string
}

// Release gives name's lock back; one not held is an error.
func (h *held) Release(context.Context) error {
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	if !h.store.held[h.name] {
		return errors.New(h.name + ".lock is not held")
	}
	delete(h.store.held, h.name)
	return nil
}
