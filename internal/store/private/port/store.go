package port

import "context"

// Store is the clone's private store: files itos keeps for the clone alone,
// each by its name, never committed. Every method takes ctx first, which
// carries cancellation and nothing else; callers pass context.Background()
// until v9 threads cancellation (ADR-0053). Outside a git repository there
// is no store, and every method but NotIgnored fails with *NoRepository.
// porttest.StoreContract holds what every one, the fake and the adapter's,
// must do.
type Store interface {
	// Path is where the file name is, absolute, whether or not it is there,
	// for a message to name it.
	Path(ctx context.Context, name string) (string, error)
	// Read is the file name's whole text, the caller's own, and whether it
	// is there: one not there is no text and no error.
	Read(ctx context.Context, name string) ([]byte, bool, error)
	// Write replaces the file name's text whole with text, readable by its
	// owner alone, so a reader never sees half of it and a crash leaves the
	// old text or the new; the store's folder is made when it is not there.
	Write(ctx context.Context, name string, text []byte) error
	// Hold takes the lock of the file name, which no other holder of it,
	// in this process or another, may take until it is given back; one held
	// past the store's wait is an error naming it. It orders writers that
	// read, change and write a file whole; reads and writes do not ask it.
	Hold(ctx context.Context, name string) (Held, error)
	// NotIgnored is whether path, absolute or from the folder itos runs in,
	// is in the clone's work tree and git does not ignore it, so one git add
	// can commit it. A path outside the work tree, or with no repository
	// at all, is not.
	NotIgnored(ctx context.Context, path string) bool
}

// Held is a lock Store.Hold took.
type Held interface {
	// Release gives the lock back, so the next holder may take it; giving
	// it back again is an error.
	Release(ctx context.Context) error
}

// NoRepository is a store asked for where there is none: the folder itos
// runs in is in no git repository.
type NoRepository struct{}

func (*NoRepository) Error() string { return "no git repository" }
