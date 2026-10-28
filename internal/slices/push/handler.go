package push

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/engine/gitrun"
	"github.com/donvargax/itos/v8/internal/engine/hooks"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is what push asks: NoWait, --no-wait; Ready, --ready, which marks a
// draft pull request ready and waits for it (slice 179); OntoMain,
// --onto-main, which brings an item's branch up to main (slice-193); Tags and
// Refs, what --tag and
// --ref named, in the order given (slice 131); Threads, push threads, which
// lists the open review threads of the branch's pull request, or with
// PullNumber (--pr) of that pull request (slice 201); Thread, Reply and
// Resolve, push thread <id> --reply <text> --resolve, which answers one, or
// shows it with neither Reply nor Resolve (slices 191 and 201); and Origin, the folder the person stood in, relative to the
// repository's top, where git runs; "" when itos runs where it was started.
type Query struct {
	NoWait     bool
	Ready      bool
	OntoMain   bool
	Tags, Refs []string
	Threads    bool
	Thread     string
	PullNumber int
	Reply      string
	Resolve    bool
	Origin     string
}

// Ports are what push reaches the world through, which internal/cli binds:
// the repository (the VCS port), its git config and hooks folder
// (hooks.Ports, which hold itos's pre-push hook, and gitrun.Config, the
// notes' rewriting);
// ConfigFound, whether there is a config file, itos being set up; Stealth,
// whether the config is the stealth one; LoadConfig, the config, loaded from
// the working tree; and Watcher, ci.watch's provider for cfg, for a commit
// pushed to remote, ok false when it is none. For push.mode pr (pr.go),
// RemoteHead, the branch the remote's HEAD names; Gh, gh run with the
// arguments, its stdout, providers.ErrNoGh when it is not installed; and
// GitHubRepository, the repository every gh call names, "" for none.
type Ports interface {
	vcsport.VCS
	hooks.Ports
	gitrun.Config
	ConfigFound(ctx context.Context) bool
	Stealth(ctx context.Context) bool
	LoadConfig(ctx context.Context) (*config.Loaded, error)
	Watcher(ctx context.Context, cfg *config.Loaded, remote string) (providers.Watcher, bool, error)
	RemoteHead(ctx context.Context, remote string, talk vcsport.Talk) (string, error)
	Gh(ctx context.Context, args ...string) (string, error)
	GitHubRepository(ctx context.Context, cfg *config.Loaded, remote string) string
}

// Handler is push's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is where what Handle runs writes as it works, git's own output and
// words, the lines itos says before it stops and the CI run's progress, and
// the global flags that shape it: --json and -q.
type Out struct {
	Stdout, Stderr io.Writer
	JSON, Quiet    bool
}

// Answer is how the push ended: Code, its exit; Outcome, what --json names
// it, "" when nothing is left to report; Lines, what it says, on stdout
// for a success, else on stderr; Remote and Branch, the upstream's; Tags
// and Refs, what --tag and --ref named; Commit, the commit pushed;
// PullRequest, the pull request of an item's branch under push.mode pr; Pull,
// Threads and Thread, what push threads and push thread report (threads.go),
// Thread a *ThreadResult for an answer or a *ThreadView for a shown thread;
// and Watched, the CI run waited for after the push, if one was.
type Answer struct {
	Code           int
	Outcome        string
	Lines          []string
	Remote, Branch string
	Tags, Refs     []string
	Commit         string
	PullRequest    *PullRequest
	Pull           *PullRef
	Threads        []ThreadInfo
	Thread         any
	Watched        *ciwatch.Watched
}

// Handle answers q, writing where o says as it runs: the plain push, or
// with --tag or --ref, what they name (named.go).
func (h Handler) Handle(ctx context.Context, q Query, o Out) (Answer, error) {
	if hooks.PathSet() {
		return Answer{}, out.Usage("push refuses core.hooksPath set by git -c: it moves the hooks git runs, skipping the " +
			"repository's own; push without it")
	}
	r := pushRun{ports: h.ports, o: o, origin: q.Origin, git: o.Stdout, noWait: q.NoWait, markReady: q.Ready, ontoMain: q.OntoMain, began: now()}
	if o.JSON {
		r.git = o.Stderr
	}
	v := h.ports
	if _, err := v.Locate(ctx, ""); err != nil {
		fmt.Fprintln(o.Stderr, "itos: push runs in a git repository, and this is none")
		return Answer{Code: missing}, nil
	}
	if q.Threads || q.Thread != "" {
		a := r.reviewThreads(ctx, q)
		if o.JSON && a.Code != 0 {
			for _, l := range a.Lines {
				fmt.Fprintln(o.Stderr, l)
			}
		}
		return a, nil
	}
	// Where itos is set up, a push git would make without itos's pre-push
	// hook is refused before anything is fetched (slice 91).
	if v.ConfigFound(ctx) {
		if err := hooks.Ready(ctx, v, "pre-push"); err != nil {
			if o.JSON {
				fmt.Fprintf(o.Stderr, "itos: %s\n", err)
			}
			return r.report(missing, "hooks-not-run", "itos: "+err.Error()), nil
		}
	}
	pr := r.prMode(ctx)
	if q.OntoMain && !pr {
		return r.report(policy, "onto-main-without-pr",
			"itos push: --onto-main brings an item's branch up to main, and push.mode is not pr in this config; nothing was fetched or pushed.",
			"Push without --onto-main, or set push.mode: pr."), nil
	}
	if q.Ready && !(pr && r.cfg.Push.PR.Draft) {
		return r.report(policy, "ready-without-draft",
			"itos push: --ready is for push.mode pr with push.pr.draft true, which this config does not set; nothing was fetched or pushed.",
			"Push without --ready, or set push.mode: pr and push.pr.draft: true."), nil
	}
	if len(q.Tags)+len(q.Refs) > 0 {
		if pr && len(q.Refs) > 0 {
			if a, done := r.judgeRefs(ctx, q.Refs); done {
				return a, nil
			}
		}
		return r.pushNamed(ctx, q.Tags, q.Refs), nil
	}
	if a, done := r.ready(ctx); done {
		return a, nil
	}
	branch := v.Branch(ctx)
	if branch == "" {
		return r.report(policy, "detached", "itos push: HEAD is not on a branch; check out the branch to push first"), nil
	}
	remote, ref, set := v.Upstream(ctx, branch)
	r.remote, r.branch = remote, strings.TrimPrefix(ref, "refs/heads/")
	if _, err := v.Resolve(ctx, "HEAD"); err != nil {
		return r.report(0, "nothing-to-push", fmt.Sprintf("Nothing to push: %s has no commit yet.", branch)), nil
	}
	if !set && !hasRemote(ctx, v, remote) {
		return r.report(missing, "no-remote", fmt.Sprintf("itos push: %s has no upstream and there is no remote %s to push to", branch, remote)), nil
	}
	onMain := false
	if pr {
		main, a, done := r.defaultBranch(ctx)
		if done {
			return a, nil
		}
		if branch != main {
			return r.pushItem(ctx, branch, main), nil
		}
		if q.OntoMain {
			return r.report(policy, "onto-main-on-main",
				fmt.Sprintf("itos push: --onto-main brings an item's branch up to main, and HEAD is on %s, which is never rewritten; nothing was fetched or pushed.", main),
				"Run itos push --onto-main from the item's branch."), nil
		}
		if q.Ready {
			return r.report(policy, "ready-on-main",
				fmt.Sprintf("itos push: --ready marks an item's pull request ready, and HEAD is on %s, which has none; nothing was fetched or pushed.", main),
				"Run itos push --ready from the item's branch."), nil
		}
		onMain = true
	} else if !set {
		if a, done := r.trunkDefault(ctx, branch); done {
			return a, nil
		}
		ref = "refs/heads/" + r.branch
	}
	onto, exists, failure, failed := r.fetch(ctx, remote, ref)
	if failed {
		return r.report(remoteExit(failure), "fetch-failed", r.remoteLines("fetching "+r.upstream()+" failed", failure)...), nil
	}
	if exists && !v.IsAncestor(ctx, onto, "HEAD") {
		if a, done := r.rebase(ctx, onto); done {
			return a, nil
		}
	}
	if a, failed := r.resolveHead(ctx); failed {
		return a, nil
	}
	if exists {
		if ahead, err := v.Commits(ctx, vcsport.Range{From: onto, To: r.sha}); err == nil && len(ahead) == 0 {
			return r.report(0, "nothing-to-push", fmt.Sprintf("Nothing to push: %s is at %s.", branch, r.upstream())), nil
		}
	}
	if onMain {
		if oldest, adds := r.addsWork(ctx, onto, r.sha); adds {
			return r.workOnMain(ctx, oldest), nil
		}
	}
	return r.push(ctx, remote, ref, onto), nil
}

// resolveHead sets sha to HEAD's commit once the rebase is over: failed, with
// the exit reported, when it cannot be told.
func (r *pushRun) resolveHead(ctx context.Context) (Answer, bool) {
	sha, err := r.ports.Resolve(ctx, "HEAD")
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot resolve HEAD after the rebase: %s\n", err)
		return Answer{Code: missing}, true
	}
	r.sha = sha
	return Answer{}, false
}
