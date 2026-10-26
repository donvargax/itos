package message

import (
	"context"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
)

// mainLedger is the ledger of the remote's main, read the first time a task
// is asked of it (slice 189).
type mainLedger struct {
	cfg  *config.Loaded
	f    config.Footer
	r    Reading
	read bool
	ids  map[string]bool
}

// has is whether main's ledger holds the task.
func (m *mainLedger) has(id string) bool {
	if !m.read {
		m.read = true
		m.ids = mainTasks(m.cfg, m.f, m.r)
	}
	return m.ids[id]
}

// mainTasks are the tasks of the ledger at the remote's main, as the clone
// last fetched it, which a ledger footer read at the commit may name though
// the commit's own ledger lacks them. Under push.mode pr a commit reaches
// main by the pull request's rebase-merge, so every commit of the merge has
// main's tasks. None under trunk, for a footer read elsewhere, without the
// clone's refs/remotes/<remote>/HEAD (no other ref stands in for it), and
// for a commit already on that main, which is judged by its own ledger.
// Nothing is fetched and no remote is asked. The remote is the current
// branch's upstream's, else origin.
func mainTasks(cfg *config.Loaded, f config.Footer, r Reading) map[string]bool {
	if cfg.Push.Mode != "pr" || !isLedger(f) || cfg.Stealth || r.tree(f) == "" {
		return nil
	}
	ctx := context.Background()
	remote := "origin"
	if branch := cfg.VCS.Branch(ctx); branch != "" {
		remote, _, _ = cfg.VCS.Upstream(ctx, branch)
	}
	prefix := "refs/remotes/" + remote + "/"
	main, ok := cfg.VCS.Symref(ctx, prefix+"HEAD")
	if !ok || !strings.HasPrefix(main, prefix) {
		return nil
	}
	tip, err := cfg.VCS.Ref(ctx, main)
	if err != nil || r.At != "" && cfg.VCS.IsAncestor(ctx, r.At, tip) {
		return nil
	}
	// The layout is the one the commit's own ledger was just read by.
	ids, _ := ledger.IDs(cfg, main)
	return ids
}
