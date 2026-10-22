package cli

// itos status (slice 67, features/status.feature): where the work stands,
// for the person a session works for, from what already records it, in
// place of a handoff file rewritten by hand. The remote branch's head is
// asked of the remote (git ls-remote), else read as last fetched; its CI run
// is looked at once through ci.watch's provider, never waited for, a run
// still going reported as going; then (slice 70) the newest release, the
// highest tag vX.Y.Z by the release cut's rule, asked of the remote as the
// head is, else as last fetched, and the commits since it the next release would carry, the feat,
// fix and breaking ones, read from the commits as fetched here; after the
// head's CI run (slice 72), the last nightly's, looked at once through the
// same provider when it names a nightly, and left out when it names none;
// before it, while the head's run is going or has not passed (slice 73), the
// commit main last proved, as ci.range's provider names it, read once; then the
// person's items in progress, the next ones they can start in the queue's
// order (a handful), the open questions of itos decision, and (slice 96) the
// drafts waiting for itos draft promote. After the release (slice 116), the
// version of itos the repository runs, its pin else its install script's as
// itos upgrade reads it, and the newest stable release of itos, asked of the
// release server once, naming itos upgrade only for a newer major: only a new
// major is something to act on. Beside it (slice 124), a line naming itos
// init --agent-rules when AGENTS.md, or under a stealth config the copies
// init writes there, holds a block of itos's rules for agents other than the
// one this itos writes, as init's rerun decides it (agentrules.Stale,
// internal/engine/agentrules): a project crossing majors with an older itos on its PATH
// gets no block rewritten by itself. The head's line names the repository's
// visibility (slice 135), public, private or internal, asked once of
// ci.watch's provider, and nothing of it when the provider cannot tell. It
// reads, never writes. What cannot be reached is
// one line naming it, and the rest still prints, exit 0; a status that
// cannot be computed at all (no config, no registry, one with problems) is
// exit 2 or 1, as itos work's. itos go prints it after the guides
// (guide.go), and there skips one that cannot be computed with a line.
// What it reads and prints is internal/engine/status's; reading the
// arguments and the exit codes are this file's.

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/engine/status"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// standing is where things stand (status.Standing), as status and itos go
// print it.
type standing struct{ *status.Standing }

// print is status.Standing.Print.
func (st *standing) print(w io.Writer) { st.Print(w) }

// fields is status.Standing.Fields.
func (st *standing) fields() []out.Field { return st.Fields() }

// statusCommand is `status [--as <handle>]`.
func statusCommand(args []string, o Out) (int, error) {
	pos, flags, _, err := subArgs("status", "", args, []string{"--as"}, nil)
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usage("status takes no arguments: %s", strings.Join(pos, " "))
	}
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	st, found, code, err := statusOf(cfg, flags["--as"], o)
	if err != nil || code != 0 {
		if len(found) > 0 {
			return refuseWork(found, code, o)
		}
		return code, err
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, st.fields()...)
	}
	st.print(o.Stdout)
	return 0, nil
}

// statusOf is status.Of in this clone: where things stand for the person
// --as names (as, "" when not given), else work.identity's provider's, the
// newest release and the rules for agents read through the real git, the
// questions through the config's tracker and the drafts from the git common
// dir. A registry that is not there or not sound is its problems, exit 1;
// --as not among the people is exit 3, said on stderr.
func statusOf(cfg *config.Loaded, as string, o Out) (*standing, []out.Problem, int, error) {
	st, found, missing, err := status.Of(context.Background(), cfg, as, status.In{
		Identity: providers.IdentityProvider(cfg),
		Stderr:   o.Stderr,
		Git:      vcs.Git{},
		Setup:    providers.WatchSetup{Env: os.Getenv, GhToken: providers.GhToken},
		Questions: func() (ask.File, error) {
			asks, _, _, err := loadAsks(cfg, tracker(cfg, o.OS))
			return asks, err
		},
		Drafts: func() (draft.File, bool, error) {
			file := draftsFile()
			if file == "" {
				return draft.File{}, false, nil
			}
			drafts, err := draft.Load(file)
			return drafts, true, err
		},
	})
	switch {
	case err != nil:
		return nil, nil, 0, err
	case len(found) > 0:
		return nil, found, ExitPolicy, nil
	case missing:
		return nil, nil, ExitMissing, nil
	}
	return &standing{st}, nil, 0, nil
}
