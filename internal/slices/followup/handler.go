package followup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/slices/followup/domain"
	"github.com/donvargax/itos/v8/internal/store/private/port"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is what followup asks: Sub, "" for the list (every thread with
// All) or add, note, close, show or doc; the thread's ID; add's With, Title
// and Note, note's Note and close's last Note; doc's Path as typed ("-" for
// stdout), Typed, the same from the folder itos was started in, and Force.
type Query struct {
	Sub                   string
	All                   bool
	ID, With, Title, Note string
	Path, Typed           string
	Force                 bool
}

// changes is whether the query writes the threads, which holds their lock.
func (q Query) changes() bool {
	switch q.Sub {
	case "add", "note", "close":
		return true
	}
	return q.Sub == "doc" && q.Path != "-"
}

// Ports are what followup reaches the world through, which internal/cli
// binds: the clone's private store, where the threads are; Now, the time a
// change is dated with, the one clock followup reads; and Folder, the
// folder itos runs in, which a doc's Typed path is read from.
type Ports interface {
	port.Store
	Now(ctx context.Context) time.Time
	Folder(ctx context.Context) string
}

// Handler is followup's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Answer is what Handle found or did: no repository, so no threads; a
// refusal, nothing written; the thread asked for or changed; the list's open
// and closed threads; doc's Markdown, the absolute Target it wrote and
// whether git add could commit it (Exposed); and the lock's failure to be
// given back, Released.
type Answer struct {
	NoRepository bool
	Refusal      *out.Problem
	Thread       *domain.Thread
	Open, Closed []domain.Thread
	Markdown     string
	Target       string
	Exposed      bool
	Released     error
}

// Handle answers q. A query that changes the threads holds their lock from
// before it reads them to after it writes them, so another itos changing
// them at once waits rather than one change being lost (bug 16).
func (h Handler) Handle(ctx context.Context, q Query) (a Answer, err error) {
	path, err := h.ports.Path(ctx, domain.FileName)
	var none *port.NoRepository
	if errors.As(err, &none) {
		return Answer{NoRepository: true}, nil
	}
	if err != nil {
		return a, err
	}
	if q.changes() {
		held, err := h.ports.Hold(ctx, domain.FileName)
		if err != nil {
			return a, err
		}
		defer func() { a.Released = held.Release(ctx) }()
	}
	threads, err := h.load(ctx, path)
	if err != nil {
		return a, err
	}
	if q.Sub == "" {
		for _, t := range threads.Threads {
			if t.Status == domain.Open {
				a.Open = append(a.Open, t)
			} else if q.All {
				a.Closed = append(a.Closed, t)
			}
		}
		return a, nil
	}
	if q.Sub == "add" {
		if t := threads.Find(q.ID); t != nil {
			return Answer{Refusal: &out.Problem{
				Rule:    "follow-id-taken",
				Message: fmt.Sprintf("%s is already a thread, with %s: %s", q.ID, t.With, t.Title),
				Fix:     "itos followup note " + q.ID + " adds to it; another id opens a new one",
			}}, nil
		}
		a.Thread = threads.Add(q.ID, q.With, q.Title, q.Note, h.ports.Now(ctx))
		return a, h.save(ctx, threads)
	}
	t := threads.Find(q.ID)
	if t == nil {
		return Answer{Refusal: &out.Problem{
			Rule:    "follow-no-thread",
			Message: fmt.Sprintf("there is no thread %s", q.ID),
			Fix:     "itos followup --all lists the threads; itos followup add " + q.ID + " opens one",
		}}, nil
	}
	a.Thread = t
	switch q.Sub {
	case "note":
		t.Note(q.Note, h.ports.Now(ctx))
	case "close":
		if t.Status == domain.Closed {
			return Answer{Refusal: &out.Problem{
				Rule:    "follow-closed",
				Message: fmt.Sprintf("%s is already closed, since %s", q.ID, domain.Show(t.Closed)),
				Fix:     "itos followup note " + q.ID + " adds to it",
			}}, nil
		}
		t.Close(q.Note, h.ports.Now(ctx))
	case "doc":
		a.Markdown = domain.Markdown(*t)
		if q.Path == "-" {
			return a, nil
		}
		refusal, err := h.doc(ctx, q, &a)
		if refusal != nil || err != nil {
			return Answer{Refusal: refusal, Exposed: a.Exposed}, err
		}
	default:
		return a, nil
	}
	return a, h.save(ctx, threads)
}

// doc writes the thread's Markdown at q's path, readable by the person
// alone (0600, the folders it makes 0700: the thread is private), the
// thread recording where. A file already there is refused unless q.Force; a
// path git add could commit sets a.Exposed (bug 16).
func (h Handler) doc(ctx context.Context, q Query, a *Answer) (*out.Problem, error) {
	target := vcsport.In(h.ports.Folder(ctx), q.Typed)
	info, err := os.Stat(target)
	switch {
	case err == nil && info.IsDir():
		return &out.Problem{
			Rule:    "follow-doc-exists",
			Message: fmt.Sprintf("%s is a folder", q.Path),
			Fix:     "name the file to write, " + filepath.Join(q.Path, q.ID+".md") + " say",
		}, nil
	case err == nil && !q.Force:
		return &out.Problem{
			Rule:    "follow-doc-exists",
			Message: fmt.Sprintf("%s is already there; nothing written", q.Path),
			Fix:     "--force writes over it",
		}, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	err = os.MkdirAll(filepath.Dir(target), 0o700)
	if err == nil {
		err = os.WriteFile(target, []byte(a.Markdown), 0o600)
	}
	// A file --force wrote over keeps its mode through WriteFile; the thread
	// is no less private in it.
	if err == nil {
		err = os.Chmod(target, 0o600)
	}
	if err != nil {
		return nil, err
	}
	a.Exposed = h.ports.NotIgnored(ctx, q.Typed)
	a.Target = target
	a.Thread.Wrote(target)
	return nil, nil
}

// load is the threads in the store; none yet when their file is not there.
func (h Handler) load(ctx context.Context, path string) (domain.File, error) {
	text, ok, err := h.ports.Read(ctx, domain.FileName)
	if err != nil || !ok {
		return domain.File{}, err
	}
	return read(path, text)
}

// read is the threads' file's text as domain.Parse reads it, after it has
// been through value.CheckWritten (bug 152): what value.Parse refuses and a
// null key or null list item, which yaml/v3 skips when it decodes into
// structs. The domain imports nothing of ours (ADR-0053), so the slice asks.
// A second document keeps the domain's own words.
func read(path string, text []byte) (domain.File, error) {
	if err := value.CheckWritten(string(text)); err != nil && !errors.Is(err, value.ErrMultipleDocuments) {
		return domain.File{}, &domain.Invalid{Err: fmt.Errorf("%s: %w", path, err)}
	}
	return domain.Parse(path, text)
}

// save writes the threads back whole.
func (h Handler) save(ctx context.Context, threads domain.File) error {
	text, err := domain.Encode(threads)
	if err == nil {
		err = h.ports.Write(ctx, domain.FileName, text)
	}
	return err
}
