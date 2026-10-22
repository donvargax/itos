package port

import (
	"context"
	"fmt"
	"io"
)

// Doc is a document as Load read it: its Name, its Text, and whether it is
// There; one not there has no text.
type Doc struct {
	Name, Text string
	There      bool
}

// Change is one document a transaction writes: its Name, its text before
// (Old) and after (Text), and whether it is Created, not there before, so
// put back by removing it.
type Change struct {
	Name, Old, Text string
	Created         bool
}

// Transaction is what Transact writes and commits: the Changes, in order,
// and the commit's message, Paragraphs, each a paragraph already wrapped.
// Quiet asks the commit for no summary. The commit reads Stdin, its words
// and the hooks' go to Words, and it runs in the environment Env, itos's
// own when nil; each is read when the transaction is made.
type Transaction struct {
	Changes    []Change
	Paragraphs []string
	Quiet      bool
	Stdin      io.Reader
	Words      io.Writer
	Env        []string
}

// Dirty is a document with changes no commit holds, which a commit of it
// would sweep in beside the transaction's: Name as the transaction gave it.
type Dirty struct{ Name string }

func (d *Dirty) Error() string { return d.Name + " has changes no commit holds" }

// Refused is a transaction a step refused, the commit or the staging of a
// created document, with the exit Code of the command that ran it, after
// every document was put back; Restore is why one could not be, nil when
// all are as they were.
type Refused struct {
	Step    string
	Code    int
	Restore error
}

func (r *Refused) Error() string {
	text := fmt.Sprintf("%s failed (exit %d)", r.Step, r.Code)
	if r.Restore != nil {
		text += ": " + r.Restore.Error()
	}
	return text
}

func (r *Refused) Unwrap() error { return r.Restore }

// Held is the lock Tracker.Hold took.
type Held interface {
	// Release gives the lock back, so the next writer may take it.
	Release(ctx context.Context) error
}

// Tracker is the tracker store. Names are as the config names the
// documents, paths from the folder itos runs in. Every method takes ctx
// first, which carries cancellation and nothing else; callers pass
// context.Background() until v9 threads cancellation (ADR-0053). No error is
// wrapped with an exit kind here: what a method returns carries the kind
// its cause gave it, but one, which the adapter's doc names (Reserve's
// repository not located). porttest.TrackerContract holds what every one,
// the fake and the adapter's, must do.
type Tracker interface {
	// Load is the documents names, in their order; one not there is a Doc
	// not There.
	Load(ctx context.Context, names []string) ([]Doc, error)
	// List is the names of the entries directly in the folder dir, sorted;
	// none when there is no such folder.
	List(ctx context.Context, dir string) ([]string, error)
	// Hold makes the caller the one writer from its read to its Transact,
	// until it releases what Hold gave; at once where the commits order the
	// writers.
	Hold(ctx context.Context) (Held, error)
	// Ready is what Transact of the documents names refuses before it
	// writes anything: the hooks that check its commit not ready, an error,
	// else the first of names that is *Dirty; nil where nothing is
	// committed.
	Ready(ctx context.Context, names []string) error
	// Transact writes the changes, as Ready lets it, and commits them alone,
	// whatever else is staged left staged; it gives the commit, "" where it
	// commits nothing. A step that fails after the first write puts every
	// document back as it was, a created one removed: a step refused is a
	// *Refused, any other failure an error.
	Transact(ctx context.Context, t Transaction) (string, error)
	// Reserve reserves count numbers of the series in a row, the first
	// past the higher of floor and the series' counter, and gives the first;
	// the counter is raised past the last in one reservation, never given
	// back, so no other reservation lands inside the run. A series is one or
	// more ASCII letters, digits and hyphens; a series that is not, a count
	// below 1, and a run whose last number would pass the largest int are
	// refused, and reserve nothing.
	Reserve(ctx context.Context, series string, floor, count int) (int, error)
}
