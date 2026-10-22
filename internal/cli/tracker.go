package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/store/tracker/adapter"
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
