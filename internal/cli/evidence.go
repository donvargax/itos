package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/message"
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
// checks by hand, in the folder the config was read in: itos task (mode
// task), whose note on unstaged changes names --working-tree, or ci run
// (mode ci), which binds ci.env to the key as it applies it to the
// commands. goos is the OS the lock waits by, as Out.OS names it.
func newEvidence(cfg *config.Loaded, mode, goos string) *evidence.Engine {
	dir := cfg.Folder()
	git := vcs.Git{Dir: dir}
	e := &evidence.Engine{
		Cfg:     cfg,
		Git:     evidenceGit{git},
		Store:   private.Store{OS: goos, VCS: git},
		Binding: evidence.ThisBinary(),
		Work:    dir,
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

// hookGit is the evidence engine's view of the repository for the commit-msg
// hook: the tree the hook captured is the index tree, whatever the index has
// become since, and a checkout's environment is less the footers and the
// amend itos commit handed the hook, which are this commit's and no commit a
// check makes.
type hookGit struct {
	evidence.Git
	tree string
}

// IndexTree is the tree the hook captured.
func (g hookGit) IndexTree(context.Context) (string, error) { return g.tree, nil }

// Snapshot is the repository's, less the commit's footers in its environment.
func (g hookGit) Snapshot(ctx context.Context, tree string) (evidence.Checkout, error) {
	co, err := g.Git.Snapshot(ctx, tree)
	co.Env = message.WithoutFooters(co.Env)
	return co, err
}

// newHookEvidence is the engine for the commit-msg hook's checks, on the
// tree it captured: keyed as itos task keys the same check, its misses run
// in a checkout of their own and a check it cannot run on the tree is
// refused, never run in the working folder.
func newHookEvidence(cfg *config.Loaded, tree, goos string) *evidence.Engine {
	e := newEvidence(cfg, "hook", goos)
	e.Git, e.Hint, e.Hook = hookGit{Git: e.Git, tree: tree}, "", true
	return e
}

// doneGit is the evidence engine's view of the repository for work done,
// which judges landed work: HEAD's tree stands where the index's does, for
// the key and for the checkout a miss runs in, and the working folder's
// unstaged changes are not counted, since none reaches the run.
type doneGit struct{ evidenceGit }

// IndexTree is HEAD's tree.
func (g doneGit) IndexTree(ctx context.Context) (string, error) { return g.Git.RevTree(ctx, "HEAD") }

// IndexPaths are the paths of HEAD's tree.
func (g doneGit) IndexPaths(ctx context.Context) ([]string, error) {
	return g.Git.TreePaths(ctx, "HEAD")
}

// UnstagedPaths are none: work done says nothing of a change HEAD leaves out.
func (doneGit) UnstagedPaths(context.Context) ([]string, error) { return nil, nil }

// newDoneEvidence is the engine for work done's checks, on HEAD's tree:
// keyed as itos task keys the same check on a clean index, so a hand run,
// the commit-msg hook and work done pay once. A check whose checkout cannot
// be made runs uncached in the working folder, as a hand run's does.
func newDoneEvidence(cfg *config.Loaded, goos string) *evidence.Engine {
	e := newEvidence(cfg, "done", goos)
	e.Git, e.Hint = doneGit{evidenceGit{vcs.Git{Dir: cfg.Folder()}}}, ""
	return e
}
