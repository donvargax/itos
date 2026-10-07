package cli

// Waiting for a commit's CI run (slice 51, features/watch.feature): itos push
// after it pushed, and itos ci watch [<sha>] for any commit, HEAD's by
// default. With ci.watch's provider (internal/providers/watch.go) the run is
// looked at every ci.watch.interval seconds until it completes, each job's
// result printed once, as it finishes, and the run's address; the exit is
// the run's result, 0 when it succeeded and 1 when it did not, naming the
// jobs that failed. A provider that cannot look exits 3; a run still going
// after ci.watch.timeout seconds, and a look that fails for a server error, a
// rate limit or no network giveUp times in a row, exit 75 (slice 86), a
// failure that may pass when run again; each names itos ci watch <sha> to
// wait again.
//
// A run a newer push cancelled is no verdict (bug 41): with cancel-in-progress
// on CI's concurrency group, the newer push's run checks the cancelled run's
// commits too. The watch follows the newest run of the workflow on the
// cancelled run's branch whose head has the commit as an ancestor, saying
// which, and exits with its result; the newer head is fetched from the remote
// when the clone does not have it. A cancelled run with no newer run to follow
// exits 75, outcome cancelled: a rerun may pass.
//
// work done judges a commit that may have no run of its own (bug 49): pushed
// with registry-only commits after it, CI ran once, for the push's head. By
// the same rule, the newest run on the branch whose head has the commit as an
// ancestor judges it, and the watch says which run it follows.

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/kind"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/providers"
	"github.com/donvargax/itos/v6/internal/value"
)

// watched is how a watch ended: its exit code, its outcome (success,
// failure, cancelled, timeout or error), the run as last seen, nil when none
// was, and the address of the commit's cancelled run when a newer run was
// followed in its place.
type watched struct {
	code       int
	outcome    string
	run        *providers.Run
	superseded string
}

// fields are the watch's keys in a --json object: "ci", its outcome, "run"
// when there was one, and "superseded" when it was a newer run followed.
func (w watched) fields() []out.Field {
	fields := []out.Field{{Key: "ci", Value: w.outcome}}
	if w.run != nil {
		run := *w.run
		if run.Jobs == nil {
			run.Jobs = []providers.Job{}
		}
		fields = append(fields, out.Field{Key: "run", Value: run})
	}
	if w.superseded != "" {
		fields = append(fields, out.Field{Key: "superseded", Value: w.superseded})
	}
	return fields
}

// sleep is how a watch waits between two looks; a test makes it instant.
var sleep = time.Sleep

// giveUp is how many looks in a row may fail for a server error, a rate
// limit or no network (kind.Temporary) before the watch stops retrying and
// exits 75: a minute of them at the default ci.watch.interval, so a blip is
// looked past and an outage is handed to whoever runs it again.
const giveUp = 6

// watcher is ci.watch's provider for the config at the folder itos runs in,
// for a commit pushed to remote: ok is false when the provider is none.
// With no config there is nothing to watch with, as with none.
func watcher(cfg *config.Loaded, remote string, o Out) (providers.Watcher, bool, error) {
	remoteURL, _ := git.Output("remote", "get-url", remote)
	return providers.WatchProvider(cfg, providers.WatchSetup{
		Env:       os.Getenv,
		RemoteURL: strings.TrimSpace(remoteURL),
		GhToken:   providers.GhToken,
	})
}

// watchRun looks at the commit's run until it completes or the timeout
// passes. Each job's result and the run's address go to progress as they
// come (stdout, or stderr under --json; nowhere under -q but a failure),
// and the run's end to stdout when it succeeded, else stderr. A cancelled
// run is followed by the newer run that has the commit, found on remote's
// branch, the commit looked at from then on that run's head; the timeout
// counts from the start of the watch.
func watchRun(cfg *config.Loaded, wr providers.Watcher, sha, remote string, o Out) watched {
	return watchCovered(cfg, wr, sha, remote, "", o)
}

// watchCovered is watchRun for a commit that may have no run of its own (bug
// 49): while it has none, the newest run on the branch whose head has it as
// an ancestor (covering) is followed in its place, as a cancelled run's
// successor is. With no branch it is watchRun.
func watchCovered(cfg *config.Loaded, wr providers.Watcher, sha, remote, branch string, o Out) watched {
	progress, end := o.Stdout, o.Stdout
	if o.JSON {
		progress, end = o.Stderr, o.Stderr
	}
	if o.Quiet {
		progress = io.Discard
	}
	interval := time.Duration(*cfg.CI.Watch.Interval * float64(time.Second))
	timeout := time.Duration(*cfg.CI.Watch.Timeout * float64(time.Second))
	deadline := time.Now().Add(timeout)
	again := fmt.Sprintf("itos ci watch %s waits for it again", sha)
	printed := map[string]bool{}
	shown := "" // the run's address, once printed
	var last *providers.Run
	var lastErr error
	failed := 0      // the looks in a row that failed for a temporary reason
	target := sha    // the commit whose run is looked at: sha, or the head of a newer run followed
	superseded := "" // the address of sha's cancelled run, once a newer run is followed
	// failing is a look that failed: looked past when temporary, until giveUp
	// in a row; ended otherwise.
	failing := func(err error) (watched, bool) {
		if kind.Of(err) == kind.Temporary {
			lastErr = err
			if failed++; failed >= giveUp {
				fmt.Fprintf(o.Stderr, "itos: %d looks in a row at the CI run of %s failed, the last: %s; %s\n", failed, sha, err, again)
				return watched{code: ExitTemporary, outcome: "error", run: last, superseded: superseded}, true
			}
			return watched{}, false
		}
		fmt.Fprintf(o.Stderr, "itos: %s; %s\n", err, again)
		return watched{code: ExitMissing, outcome: "error", run: last, superseded: superseded}, true
	}
	for {
		run, found, err := wr.Look(target)
		switch {
		case err != nil:
			if w, end := failing(err); end {
				return w
			}
		case !found:
			if branch != "" && target == sha {
				next, ok, err := covering(wr.Runs, branch, sha, remote, nil)
				if err != nil {
					if w, end := failing(err); end {
						return w
					}
					break
				}
				if ok {
					fmt.Fprintf(progress, "No CI run of %s; following %s, the run of %s, which has it\n",
						short(sha), next.URL, short(next.HeadSHA))
					target, printed = next.HeadSHA, map[string]bool{}
					continue
				}
			}
			failed = 0
		default:
			lastErr, failed = nil, 0
			if run.URL != "" && run.URL != shown {
				fmt.Fprintf(progress, "CI run: %s\n", run.URL)
				shown = run.URL
			}
			last = &run
			for _, j := range run.Jobs {
				if j.Done() && !printed[j.Name] {
					printed[j.Name] = true
					fmt.Fprintf(progress, "  %s: %s\n", j.Name, j.Conclusion)
				}
			}
			if !run.Done() {
				break
			}
			if run.Conclusion != "cancelled" {
				w := ended(run, o, end)
				w.superseded = superseded
				return w
			}
			next, ok, err := covering(wr.Runs, run.Branch, sha, remote, &run)
			switch {
			case err != nil:
				if w, end := failing(err); end {
					return w
				}
			case !ok:
				fmt.Fprintf(o.Stderr, "CI cancelled: %s; no newer run on %s has %s, so it has no result: rerun the workflow, and %s\n",
					run.URL, run.Branch, short(sha), again)
				return watched{code: ExitTemporary, outcome: "cancelled", run: &run, superseded: superseded}
			default:
				if superseded == "" {
					superseded = run.URL
				}
				fmt.Fprintf(progress, "CI run %s was cancelled; following %s, the newer run of %s, which has %s\n",
					run.URL, next.URL, short(next.HeadSHA), short(sha))
				target, printed = next.HeadSHA, map[string]bool{}
				continue
			}
		}
		left := time.Until(deadline)
		if left <= 0 {
			what := "has not finished"
			if last == nil {
				what = "has not appeared"
			}
			fmt.Fprintf(o.Stderr, "itos: the CI run of %s %s after %s seconds (ci.watch.timeout); %s\n",
				sha, what, value.Number(*cfg.CI.Watch.Timeout), again)
			if lastErr != nil {
				fmt.Fprintf(o.Stderr, "itos: the last look failed: %s\n", lastErr)
			}
			return watched{code: ExitTemporary, outcome: "timeout", run: last}
		}
		sleep(min(interval, left))
	}
}

// covering is the run that covers sha: the newest of the workflow's runs on
// the branch whose head is sha or has it as an ancestor; found is false when
// there is none. With cancelled, sha's run that a newer push cancelled (bug
// 41), only a run newer than it counts, the one that superseded it; without,
// any run counts, for a commit that has no run of its own (bug 49). A head the
// clone does not have is fetched from remote once, since the newer push may be
// someone else's; one still missing after it is passed over.
func covering(runs providers.Runs, branch, sha, remote string, cancelled *providers.Run) (providers.Run, bool, error) {
	if runs == nil || branch == "" {
		return providers.Run{}, false, nil
	}
	listed, err := runs(branch)
	if err != nil {
		return providers.Run{}, false, err
	}
	fetched := false
	for _, r := range listed {
		if cancelled != nil && (r.ID == cancelled.ID || (r.Created != "" && cancelled.Created != "" && r.Created < cancelled.Created)) {
			break
		}
		if r.HeadSHA == "" {
			continue
		}
		if r.HeadSHA != sha && !git.HasCommit(r.HeadSHA) && !fetched {
			fetched = true
			fetchQuietly(remote)
		}
		if r.HeadSHA == sha || git.Succeeds("merge-base", "--is-ancestor", sha, r.HeadSHA) {
			return r, true, nil
		}
	}
	return providers.Run{}, false, nil
}

// fetchQuietly fetches the remote's branches, never prompting for
// credentials, within lsRemoteTimeout; a fetch that fails leaves the clone as
// it was.
func fetchQuietly(remote string) {
	if remote == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), lsRemoteTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, git.Bin(), "fetch", "-q", remote)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	_ = cmd.Run()
}

// ended is the watch of a run that completed: 0 when it succeeded, else 1,
// naming the jobs that did not succeed.
func ended(run providers.Run, o Out, end io.Writer) watched {
	if run.Conclusion == "success" {
		if !o.Quiet {
			fmt.Fprintf(end, "CI passed: %s\n", run.URL)
		}
		return watched{code: 0, outcome: "success", run: &run}
	}
	var failed []string
	for _, j := range run.Jobs {
		if j.Done() && j.Conclusion != "success" && j.Conclusion != "skipped" && j.Conclusion != "neutral" {
			failed = append(failed, j.Name)
		}
	}
	line := fmt.Sprintf("CI %s: %s", run.Conclusion, run.URL)
	if len(failed) > 0 {
		line += "; failed jobs: " + strings.Join(failed, ", ")
	}
	fmt.Fprintln(o.Stderr, line)
	return watched{code: ExitPolicy, outcome: "failure", run: &run}
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
	if !git.Succeeds("rev-parse", "--git-dir") {
		fmt.Fprintln(o.Stderr, "itos: ci watch runs in a git repository, and this is none")
		return ExitMissing, nil
	}
	sha, err := git.Output("rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil || strings.TrimSpace(sha) == "" {
		return 0, usage("ci watch: %s is not a commit of this repository", rev)
	}
	sha = strings.TrimSpace(sha)
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	remote := "origin"
	if branch := git.Branch(); branch != "" {
		remote, _, _ = git.Upstream(branch)
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
