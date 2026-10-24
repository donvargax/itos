package work

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ids"
	"github.com/donvargax/itos/v8/internal/engine/record"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// writing is the registry as a write reads it: the config, the tracker it is
// held and committed through, the registry and its text.
type writing struct {
	cfg      *config.Loaded
	tracker  trackerport.Tracker
	registry domain.Registry
	text     string
}

// read is the registry for a write, when it is there and sound: a command
// writes only a registry work check passes, its problems else the refusal,
// exit 1, with no writing. The Answer's Release, never nil, gives back
// the registry's lock, taken first under a stealth config (the tracker's
// Hold): every write of work's reads the registry here, so the lock held
// from here to the write keeps another itos, in another worktree, from
// reading it before this one's change and writing over it (bug 16). A
// project's registry needs none: its writes are commits, which git's own
// index lock takes in turn.
func (h Handler) read(ctx context.Context, o Out) (Answer, *writing, error) {
	a := Answer{Release: func() {}}
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return a, nil, err
	}
	t := h.ports.Tracker(ctx, cfg)
	if a.Release, err = hold(ctx, t, o.Stderr); err != nil {
		return a, nil, err
	}
	registry, text, found, err := record.Sound(ctx, t, cfg)
	if err != nil {
		return a, nil, err
	}
	if len(found) > 0 {
		a.Refused, a.Code = found, refused
		return a, nil, nil
	}
	return a, &writing{cfg: cfg, tracker: t, registry: registry, text: text}, nil
}

// hold is the tracker's Hold, taken for a command that reads and writes its
// documents: release, never nil and deferred by the caller, gives it back,
// saying on stderr when it cannot.
func hold(ctx context.Context, t trackerport.Tracker, stderr io.Writer) (release func(), err error) {
	release = func() {}
	held, err := t.Hold(ctx)
	if err != nil {
		return release, err
	}
	return func() {
		if err := held.Release(ctx); err != nil {
			fmt.Fprintf(stderr, "itos: the lock cannot be given back: %s\n", err)
		}
	}, nil
}

// take is `work take <id> [--as <handle>]`: the item set in progress for
// the person and the registry committed (work.Take). The person is --as,
// else who the identity provider says, as for work; one that is nobody, or
// not among the people, exits 3. Under a stealth config, or with no one in
// the people (Registry.Nobody: a people file missing or listing nobody), with
// no --as nobody is asked: every item is the session's, and its owner is left
// as it is (bug 46).
func (h Handler) take(ctx context.Context, id, as string, o Out) (Answer, error) {
	a, s, err := h.read(ctx, o)
	if s == nil {
		return a, err
	}
	person, every, problem := whoActs(s.cfg, s.registry, as, h.ports.Identity(ctx, s.cfg))
	if problem != nil {
		a.Refused, a.Code = []out.Problem{*problem}, notAmong
		return a, nil
	}
	change, problem, err := domain.Take(s.registry, s.text, id, person, every)
	return h.commit(ctx, a, s, id, change, asList(problem), err, o)
}

// add is `work add [<idea-id>] --title <t> --why <w> [--kind
// idea|slice|task|bug] [--phase <p>] [--owner <handle>] [--depends-on
// <ids>] [--refs <refs>] [--tags <tags>]`: ideas keep their author's id;
// slices, tasks and bugs mint theirs from the shared counter (ItemID).
func (h Handler) add(ctx context.Context, n domain.New, o Out) (Answer, error) {
	a, s, err := h.read(ctx, o)
	if s == nil {
		return a, err
	}
	if n.Kind != "idea" {
		if n.ID, err = h.ports.ItemID(ctx, s.cfg, n.Kind, ids.Items(s.registry)); err != nil {
			return a, err
		}
	}
	change, found, err := domain.Add(s.cfg, s.registry, s.text, n, ledger.IDPattern(s.cfg))
	return h.commit(ctx, a, s, n.ID, change, found, err, o)
}

// one is a write of one id by edit, queue, drop, defer, resume or
// give-back, through its writer in internal/work. edit replaces the item's
// title, depends_on, refs or tags, or adds a paragraph to its why
// (work.Edit); queue puts it first in the registry's queue, just before or
// after an item it holds, or out of it (work.Queue); drop, defer and resume
// set it dropped, deferred or no longer deferred (work.Drop, work.Defer,
// work.Resume); give-back sets an item in progress to do again, its owner
// judged against work take's person, exit 3 when nobody or not among the
// people (work.GiveBack).
func (h Handler) one(ctx context.Context, q Query, o Out) (Answer, error) {
	id := q.IDs[0]
	a, s, err := h.read(ctx, o)
	if s == nil {
		return a, err
	}
	var change domain.Change
	var found []out.Problem
	switch q.Sub {
	case "edit":
		change, found, err = domain.Edit(s.cfg, s.registry, s.text, id, q.Edits, taskIDs(s.cfg))
	case "queue":
		var problem *out.Problem
		change, problem, err = domain.Queue(s.registry, s.text, id, q.Place)
		found = asList(problem)
	case "drop":
		change, found, err = domain.Drop(s.cfg, s.registry, s.text, id, q.Why)
	case "defer":
		change, found, err = domain.Defer(s.cfg, s.registry, s.text, id, q.Why)
	case "resume":
		change, found, err = domain.Resume(s.cfg, s.registry, s.text, id)
	case "give-back":
		person, every, problem := whoActs(s.cfg, s.registry, q.As, h.ports.Identity(ctx, s.cfg))
		if problem != nil {
			a.Refused, a.Code = []out.Problem{*problem}, notAmong
			return a, nil
		}
		change, found, err = domain.GiveBack(s.cfg, s.registry, s.text, id, person, every, q.Why)
	}
	return h.commit(ctx, a, s, id, change, found, err, o)
}

// commit ends a write of one id: a text the writer could not edit is the
// registry's error, its problems the refusal, exit 1; a change with nothing
// to write is the Answer as it is, and any other is written and committed
// (write), a file with changes no commit holds or a commit refused exit 1.
func (h Handler) commit(ctx context.Context, a Answer, s *writing, id string, change domain.Change, found []out.Problem, err error, o Out) (Answer, error) {
	if err != nil {
		return a, uneditable(s.cfg.Work.Registry, err)
	}
	if len(found) > 0 {
		a.Refused, a.Code = found, refused
		return a, nil
	}
	if !change.Unchanged {
		if a.Commit, found, err = write(ctx, s, change.Text, change.Header, change.Body, o); err != nil {
			return a, err
		}
		if len(found) > 0 {
			a.Refused, a.Code = found, refused
			return a, nil
		}
	}
	a.ID, a.Change = id, &change
	return a, nil
}

// each is a write of several ids (slice 118) by edit, queue, drop, defer,
// resume or give-back, through internal/work's *Each writers: the change
// judged for every id, refused whole (exit 1) when any id is, naming every
// id refused (Each), then written and committed once. work give-back's
// person who is nobody or not among the people refuses it before any id is
// judged, exit 3, as for one id.
func (h Handler) each(ctx context.Context, q Query, o Out) (Answer, error) {
	a, s, err := h.read(ctx, o)
	if s == nil {
		return a, err
	}
	var several domain.Several
	var refusedIDs []string
	var found []out.Problem
	switch q.Sub {
	case "edit":
		several, refusedIDs, found, err = domain.EditEach(s.cfg, s.registry, s.text, q.IDs, q.Edits, taskIDs(s.cfg))
	case "queue":
		several, refusedIDs, found, err = domain.QueueEach(s.registry, s.text, q.IDs, q.Place)
	case "drop":
		several, refusedIDs, found, err = domain.DropEach(s.cfg, s.registry, s.text, q.IDs, q.Why)
	case "defer":
		several, refusedIDs, found, err = domain.DeferEach(s.cfg, s.registry, s.text, q.IDs, q.Why)
	case "resume":
		several, refusedIDs, found, err = domain.ResumeEach(s.cfg, s.registry, s.text, q.IDs)
	case "give-back":
		person, every, problem := whoActs(s.cfg, s.registry, q.As, h.ports.Identity(ctx, s.cfg))
		if problem != nil {
			a.Refused, a.Code = []out.Problem{*problem}, notAmong
			return a, nil
		}
		several, refusedIDs, found, err = domain.GiveBackEach(s.cfg, s.registry, s.text, q.IDs, person, every, q.Why)
	}
	if err != nil {
		return a, uneditable(s.cfg.Work.Registry, err)
	}
	if len(found) > 0 {
		a.Refused, a.Each, a.Code = found, refusedIDs, refused
		return a, nil
	}
	if !several.Unchanged {
		if a.Commit, found, err = write(ctx, s, several.Text, several.Header, several.Body, o); err != nil {
			return a, err
		}
		if len(found) > 0 {
			a.Refused, a.Code = found, refused
			return a, nil
		}
	}
	a.Several = &several
	return a, nil
}

// write is writeAll of the registry alone, written as text.
func write(ctx context.Context, s *writing, text, header, body string, o Out) (string, []out.Problem, error) {
	return writeAll(ctx, s, []record.File{{Path: s.cfg.Work.Registry, Old: s.text, Text: text}}, header, body, o)
}

// writeAll is the record engine's Write (record.Write) through the tracker:
// the files written and, in a project, committed alone, through the hooks,
// git's words and the hooks' on stderr. Its result is the new commit's SHA,
// "" under a stealth config, which commits nothing. A file with changes no
// commit holds, or a commit refused, is a problem, every file as it was;
// the rest are errors. Every write commits the registry alone but work
// promote, which commits a task's ledger file and the questions with it.
func writeAll(ctx context.Context, s *writing, files []record.File, header, body string, o Out) (string, []out.Problem, error) {
	return record.Write(ctx, s.tracker, s.cfg, files, record.Commit{
		Header: header,
		Body:   body,
		Quiet:  o.Quiet,
		// git's words and the hooks' go to stderr: stdout is the command's
		// own result.
		Stdin: os.Stdin,
		Words: o.Stderr,
		Env:   append(message.WithoutFooters(os.Environ()), message.AmendEnv+"=0"),
	})
}

// whoActs is who a command that judges an item's owner acts for, work
// take and work give-back: --as (as), else who the identity provider says,
// as for work (identify, the config's work.identity provider). every is a
// session that owns every item, asked of nobody: a stealth config, or no one
// in the people (Registry.Nobody: a people file missing, unreadable or
// listing nobody),
// with no --as (bug 46). The problem, exit 3, is a person who is nobody or
// not among the people.
func whoActs(cfg *config.Loaded, registry domain.Registry, as string, identify providers.Identity) (person string, every bool, problem *out.Problem) {
	if as == "" && (cfg.Stealth || registry.Nobody()) {
		return "", true, nil
	}
	listedIn := cfg.Work.People.File
	who := domain.Whoami(registry, listedIn, as, identify)
	switch {
	case who.Problem != "":
		return "", false, &out.Problem{Rule: "work-no-person", Message: who.Problem, Fix: "pass --as <handle>"}
	case !who.Listed:
		return "", false, &out.Problem{
			Rule:    "work-no-person",
			Message: fmt.Sprintf("%s is not in %s, so cannot own an item", who.Handle, listedIn),
			Fix:     "add " + who.Handle + " to " + listedIn + ", or pass --as <handle>",
		}
	}
	return who.Handle, false, nil
}

// asList is a writer's one problem as a list: none when it is nil.
func asList(problem *out.Problem) []out.Problem {
	if problem == nil {
		return nil
	}
	return []out.Problem{*problem}
}

// uneditable is a change to the work registry that cannot be made in place,
// its text not as itos edits it: a data file itos refuses, exit 2.
func uneditable(registry string, err error) error {
	return kind.Wrap(kind.Usage, fmt.Errorf("%s cannot be edited in place: %w", registry, err))
}
