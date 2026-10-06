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

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/git"
	"github.com/donvargax/itos/v5/internal/kind"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/providers"
	"github.com/donvargax/itos/v5/internal/value"
)

// watched is how a watch ended: its exit code, its outcome (success,
// failure, timeout or error) and the run as last seen, nil when none was.
type watched struct {
	code    int
	outcome string
	run     *providers.Run
}

// fields are the watch's keys in a --json object: "ci", its outcome, and
// "run" when there was one.
func (w watched) fields() []out.Field {
	fields := []out.Field{{Key: "ci", Value: w.outcome}}
	if w.run != nil {
		run := *w.run
		if run.Jobs == nil {
			run.Jobs = []providers.Job{}
		}
		fields = append(fields, out.Field{Key: "run", Value: run})
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
func watcher(cfg *config.Loaded, remote string, o Out) (providers.Watch, bool, error) {
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
// and the run's end to stdout when it succeeded, else stderr.
func watchRun(cfg *config.Loaded, look providers.Watch, sha string, o Out) watched {
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
	failed := 0 // the looks in a row that failed for a temporary reason
	for {
		run, found, err := look(sha)
		switch {
		case kind.Of(err) == kind.Temporary:
			lastErr = err
			if failed++; failed >= giveUp {
				fmt.Fprintf(o.Stderr, "itos: %d looks in a row at the CI run of %s failed, the last: %s; %s\n", failed, sha, err, again)
				return watched{code: ExitTemporary, outcome: "error", run: last}
			}
		case err != nil:
			fmt.Fprintf(o.Stderr, "itos: %s; %s\n", err, again)
			return watched{code: ExitMissing, outcome: "error", run: last}
		case !found:
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
			if run.Done() {
				return ended(run, o, end)
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
	look, ok, err := watcher(cfg, remote, o)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s\n", err)
		return watchReport(o, watched{code: ExitMissing, outcome: "error"}, sha)
	}
	if !ok {
		return 0, config.Invalid(cfg.Path, "ci.watch.provider is none, so there is no CI run to watch: set ci.watch.provider to github")
	}
	return watchReport(o, watchRun(cfg, look, sha, o), sha)
}

// watchReport ends a ci watch: under --json its object on stdout.
func watchReport(o Out, w watched, sha string) (int, error) {
	if !o.JSON {
		return w.code, nil
	}
	fields := append([]out.Field{{Key: "ok", Value: w.code == 0}, {Key: "commit", Value: sha}}, w.fields()...)
	return w.code, out.Emit(o.Stdout, fields...)
}
