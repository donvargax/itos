package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	private "github.com/donvargax/itos/v8/internal/store/private/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// evidenceGit is the evidence engine's view of the repository: the VCS's
// adapter, whose task snapshot is bug-112's checkout.
type evidenceGit struct{ vcs.Git }

// Snapshot is TaskSnapshot, as the engine's Checkout.
func (g evidenceGit) Snapshot(ctx context.Context, tree string) (evidence.Checkout, error) {
	s, err := g.Git.TaskSnapshot(ctx, tree)
	return evidence.Checkout{Dir: s.Dir, Env: s.Env, Remove: s.Remove}, err
}

// newEvidence is the engine for one invocation of a command that runs
// checks by hand: itos task (mode task), whose note on unstaged changes
// names --working-tree, or ci run (mode ci), which binds ci.env to the key
// as it applies it to the commands. goos is the OS the lock waits by, as Out.OS
// names it.
func newEvidence(cfg *config.Loaded, mode, goos string) *evidence.Engine {
	e := &evidence.Engine{
		Cfg:     cfg,
		Git:     evidenceGit{vcs.Git{}},
		Store:   private.Store{OS: goos, VCS: vcs.Git{}},
		Binding: evidence.ThisBinary(),
		Mode:    mode,
	}
	if mode == "ci" {
		e.CIEnv = cfg.CI.Env
		if e.CIEnv == nil {
			e.CIEnv = map[string]string{}
		}
	} else {
		e.Hint = " itos task --working-tree runs the working folder."
	}
	return e
}
