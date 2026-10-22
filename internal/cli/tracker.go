package cli

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/store/tracker/adapter"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// tracker is the tracker store the config's documents are in: the files of
// the working tree, committed through git, whose hooks must be ready to
// check the commit, unless the config is the stealth one; under it, the
// registry's lock orders the writers. Nothing is located here: HooksReady
// locates when Ready asks it.
func tracker(cfg *config.Loaded, goos string) adapter.Files {
	return adapter.Files{
		VCS:        vcs.Git{},
		OS:         goos,
		Stealth:    cfg.Stealth,
		Lock:       cfg.Work.Registry,
		HooksReady: func(context.Context) error { return hooksReady("commit-msg") },
	}
}

// holdTracker is the tracker's Hold, taken for a command that reads and
// writes its documents: release, never nil and deferred by the caller,
// gives it back, saying on stderr when it cannot.
func holdTracker(t trackerport.Tracker, o Out) (release func(), err error) {
	release = func() {}
	h, err := t.Hold(context.Background())
	if err != nil {
		return release, err
	}
	return func() {
		if err := h.Release(context.Background()); err != nil {
			fmt.Fprintf(o.Stderr, "itos: the lock cannot be given back: %s\n", err)
		}
	}, nil
}

// trackedText is the text of the document name, which the command found
// there: one gone since is an error naming it.
func trackedText(t trackerport.Tracker, name string) (string, error) {
	docs, err := t.Load(context.Background(), []string{name})
	if err == nil && !docs[0].There {
		err = &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if err != nil {
		return "", err
	}
	return docs[0].Text, nil
}
