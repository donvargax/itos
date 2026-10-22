package cli

// itos ci watch [<sha>] (HEAD's by default), the end of itos push and work
// done wait for a commit's CI run through internal/engine/ciwatch, whose doc
// says how; the wrappers here bind it to this clone and map Out to its
// writers.

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// watched is how a watch ended (ciwatch.Watched): its exit code, its outcome
// (success, failure, cancelled, timeout or error), the run as last seen, nil
// when none was, and the address of the commit's cancelled run when a newer
// run was followed in its place.
type watched struct {
	code       int
	outcome    string
	run        *providers.Run
	superseded string
}

// fields are the watch's keys in a --json object (ciwatch.Watched.Fields).
func (w watched) fields() []out.Field {
	return ciwatch.Watched{Code: w.code, Outcome: w.outcome, Run: w.run, Superseded: w.superseded}.Fields()
}

// watchedOf is the engine's result as a watched.
func watchedOf(w ciwatch.Watched) watched {
	return watched{code: w.Code, outcome: w.Outcome, run: w.Run, superseded: w.Superseded}
}

// watchOut is o as the engine's writers and flags.
func watchOut(o Out) ciwatch.Out {
	return ciwatch.Out{Stdout: o.Stdout, Stderr: o.Stderr, JSON: o.JSON, Quiet: o.Quiet}
}

// watcher is ci.watch's provider for the config at the folder itos runs in,
// for a commit pushed to remote: ok is false when the provider is none.
// With no config there is nothing to watch with, as with none.
func watcher(cfg *config.Loaded, remote string, o Out) (providers.Watcher, bool, error) {
	return ciwatch.Watcher(context.Background(), cfg, cfg.VCS, remote, providers.WatchSetup{
		Env:     os.Getenv,
		GhToken: providers.GhToken,
	})
}

// watchRun waits for the commit's own run (ciwatch.Watch with no branch).
func watchRun(cfg *config.Loaded, wr providers.Watcher, sha, remote string, o Out) watched {
	return watchCovered(cfg, wr, sha, remote, "", o)
}

// watchCovered is ciwatch.Watch in this clone: with a branch, a commit that
// may have no run of its own is judged by the run on it that covers it (bug
// 49).
func watchCovered(cfg *config.Loaded, wr providers.Watcher, sha, remote, branch string, o Out) watched {
	return watchedOf(ciwatch.Watch(context.Background(), cfg, wr, vcs.Git{}, sha, remote, branch, watchOut(o)))
}

// covering is ciwatch.Covering in this clone.
func covering(runs providers.Runs, branch, sha, remote string, cancelled *providers.Run, conclusion string) (providers.Run, bool, error) {
	return ciwatch.Covering(context.Background(), runs, vcs.Git{}, branch, sha, remote, cancelled, conclusion)
}

// hasCommit is whether the repository has the commit rev names.
func hasCommit(ctx context.Context, v vcsport.VCS, rev string) bool {
	_, err := v.Resolve(ctx, rev)
	return err == nil
}

// ended is ciwatch.Ended: the watch of a run that completed.
func ended(run providers.Run, o Out, end io.Writer) watched {
	return watchedOf(ciwatch.Ended(run, watchOut(o), end))
}

// ciWatch is `ci watch [<sha>]`: the run of the commit, HEAD's by default,
// waited for as itos push waits for its own.
func ciWatch(args []string, o Out) (int, error) {
	if len(args) > 1 {
		return 0, usage("ci watch takes at most one commit (%s)", args[1])
	}
	rev := "HEAD"
	if len(args) == 1 {
		rev = args[0]
	}
	ctx, v := context.Background(), vcs.Git{}
	if _, err := v.Locate(ctx, ""); err != nil {
		fmt.Fprintln(o.Stderr, "itos: ci watch runs in a git repository, and this is none")
		return ExitMissing, nil
	}
	sha, err := v.Resolve(ctx, rev)
	if err != nil {
		return 0, usage("ci watch: %s is not a commit of this repository", rev)
	}
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	remote := "origin"
	if branch := v.Branch(ctx); branch != "" {
		remote, _, _ = v.Upstream(ctx, branch)
	}
	wr, ok, err := watcher(cfg, remote, o)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s\n", err)
		return watchReport(o, watched{code: ExitMissing, outcome: "error"}, sha)
	}
	if !ok {
		return 0, config.Invalid(cfg.Path, "ci.watch.provider is none, so there is no CI run to watch: set ci.watch.provider to github")
	}
	return watchReport(o, watchRun(cfg, wr, sha, remote, o), sha)
}

// watchReport ends a ci watch: under --json its object on stdout.
func watchReport(o Out, w watched, sha string) (int, error) {
	if !o.JSON {
		return w.code, nil
	}
	fields := append([]out.Field{{Key: "ok", Value: w.code == 0}, {Key: "commit", Value: sha}}, w.fields()...)
	return w.code, out.Emit(o.Stdout, fields...)
}
