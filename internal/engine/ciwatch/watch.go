package ciwatch

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// The watch's exit codes, internal/cli's of the same names.
const (
	exitPolicy    = 1
	exitMissing   = 3
	exitTemporary = 75
)

// Watched is how a watch ended: its exit code, its outcome (success,
// failure, cancelled, timeout or error; bound for a WatchReady whose Ready
// bound passed first), the run as last seen, nil when none was, not finished on
// a failure for a WatchReady that stopped at a failed job, and the
// address of the commit's cancelled run when a newer run was followed in its
// place.
type Watched struct {
	Code       int
	Outcome    string
	Run        *providers.Run
	Superseded string
}

// Fields are the watch's keys in a --json object: "ci", its outcome, "run"
// when there was one, and "superseded" when it was a newer run followed.
func (w Watched) Fields() []out.Field {
	fields := []out.Field{{Key: "ci", Value: w.Outcome}}
	if w.Run != nil {
		run := *w.Run
		if run.Jobs == nil {
			run.Jobs = []providers.Job{}
		}
		fields = append(fields, out.Field{Key: "run", Value: run})
	}
	if w.Superseded != "" {
		fields = append(fields, out.Field{Key: "superseded", Value: w.Superseded})
	}
	return fields
}

// Out is where a watch writes: its progress to Stdout, or Stderr under
// JSON, nowhere under Quiet; its end likewise, a success's line not under
// Quiet; its failures to Stderr.
type Out struct {
	Stdout, Stderr io.Writer
	JSON, Quiet    bool
}

// sleep is how a watch waits between two looks; a test makes it instant.
var sleep = time.Sleep

// giveUp is how many looks in a row may fail for a server error, a rate
// limit or no network (kind.Temporary) before the watch stops retrying and
// exits 75: a minute of them at the default ci.watch.interval, so a blip is
// looked past and an outage is handed to whoever runs it again.
const giveUp = 6

// FetchTimeout is how long the quiet fetch of a head the clone lacks may
// take.
const FetchTimeout = 20 * time.Second

// Watcher is ci.watch's provider for cfg, for a commit pushed to remote,
// whose address v gives: ok is false when the provider is none. setup is
// the rest of what the provider reads, its RemoteURL set here.
func Watcher(ctx context.Context, cfg *config.Loaded, v vcsport.VCS, remote string, setup providers.WatchSetup) (providers.Watcher, bool, error) {
	remoteURL, _ := v.RemoteURL(ctx, remote)
	setup.RemoteURL = strings.TrimSpace(remoteURL)
	return providers.WatchProvider(cfg, setup)
}

// Watch looks at the commit's run until it completes or the timeout
// passes. Each job's result and the run's address go to progress as they
// come (stdout, or stderr under --json; nowhere under -q but a failure),
// and the run's end to stdout when it succeeded, else stderr. A cancelled
// run is followed by the newer run that has the commit, found on remote's
// branch, the commit looked at from then on that run's head; the timeout
// counts from the start of the watch. With a branch, a commit that has no
// run of its own (bug 49) is judged, while it has none, by the newest run on
// it whose head has the commit as an ancestor (Covering), followed as a
// cancelled run's successor is.
func Watch(ctx context.Context, cfg *config.Loaded, wr providers.Watcher, v vcsport.VCS, sha, remote, branch string, o Out) Watched {
	return watch(ctx, cfg, wr, v, sha, remote, branch, o, nil)
}

// Ready is what itos push --ready adds to a watch (slices 179 and 200): Left,
// when set, is how long push.pr.ready_wait leaves the whole call, which ends
// the watch with the outcome bound, code 75, at the first look after it has
// passed (and none of the watch's own timeout line) unless ci.watch.timeout
// passed before it.
type Ready struct {
	Left func() time.Duration
}

// WatchReady is Watch for itos push --ready (slice 179), which marks a draft
// ready: the draft's last push ran CI with every job skipped, and GitHub may
// record that run as a success, so a look that finds a run all of whose jobs
// were skipped, or that lists none, is taken for no run yet and made again
// each ci.watch.interval within ci.watch.timeout, until the run the marking
// starts on the same head shows. The timeout counts from the start of the
// run's latest attempt (providers.Run.Started) when GitHub gives it (slice
// 200), so that a rerun of --ready continues it, else from the run's creation,
// else from the watch's start. A look that finds a job concluded failure, the run
// not finished, ends the watch at once as a red run's end, before either limit
// is judged (slice-225), the run as GitHub gave it, with no conclusion.
func WatchReady(ctx context.Context, cfg *config.Loaded, wr providers.Watcher, v vcsport.VCS, sha, remote string, o Out, ready Ready) Watched {
	return watch(ctx, cfg, wr, v, sha, remote, "", o, &ready)
}

// DraftsRun is whether a run is only a draft's: it lists no job, or every job
// it lists was skipped. One cancelled before its jobs started is not (bug-160):
// it is the end of the run that was to check the head.
func DraftsRun(run providers.Run) bool {
	return !run.CancelledUnstarted() &&
		!slices.ContainsFunc(run.Jobs, func(j providers.Job) bool { return j.Conclusion != "skipped" })
}

// Looks is the rule a wait for a run keeps of the looks that fail: Failed is how
// many in a row have, Last the last that did, and Pause how long it asked to be
// left, a rate limit's wait.
type Looks struct {
	Failed int
	Last   error
	Pause  time.Duration
}

// Failing judges a look at sha's run that failed: a temporary failure is looked
// past until giveUp in a row, then ended with 75; any other ends it with 3. Ended,
// it says so on stderr.
func (l *Looks) Failing(err error, sha string, stderr io.Writer) (code int, ended bool) {
	if kind.Of(err) == kind.Temporary {
		l.Last, l.Pause = err, providers.Wait(err)
		if l.Failed++; l.Failed >= giveUp {
			fmt.Fprintf(stderr, "itos: %d looks in a row at the CI run of %s failed, the last: %s; %s\n", l.Failed, sha, err, waitsAgain(sha))
			return exitTemporary, true
		}
		return 0, false
	}
	fmt.Fprintf(stderr, "itos: %s; %s\n", err, waitsAgain(sha))
	return exitMissing, true
}

// waitsAgain is the watch's advice for a commit whose run it stopped looking at.
func waitsAgain(sha string) string {
	return fmt.Sprintf("itos ci watch %s waits for it again", sha)
}

// TimedOut ends a watch of sha's run at ci.watch.timeout, saying so on stderr:
// last is the run as last seen, nil for none, and lastErr the look that failed
// last, nil when none has since a run was seen.
func TimedOut(cfg *config.Loaded, sha string, last *providers.Run, lastErr error, stderr io.Writer) Watched {
	what := "has not finished"
	if last == nil {
		what = "has not appeared"
	}
	fmt.Fprintf(stderr, "itos: the CI run of %s %s after %s seconds (ci.watch.timeout); %s\n",
		sha, what, value.Number(*cfg.CI.Watch.Timeout), waitsAgain(sha))
	if lastErr != nil {
		fmt.Fprintf(stderr, "itos: the last look failed: %s\n", lastErr)
	}
	return Watched{Code: exitTemporary, Outcome: "timeout", Run: last}
}

func watch(ctx context.Context, cfg *config.Loaded, wr providers.Watcher, v vcsport.VCS, sha, remote, branch string, o Out, ready *Ready) Watched {
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
	printed := map[string]bool{}
	shown := "" // the run's address, once printed
	var last *providers.Run
	var looks Looks
	target := sha    // the commit whose run is looked at: sha, or the head of a newer run followed
	superseded := "" // the address of sha's cancelled run, once a newer run is followed
	failing := func(err error) (Watched, bool) {
		code, ended := looks.Failing(err, sha, o.Stderr)
		if !ended {
			return Watched{}, false
		}
		return Watched{Code: code, Outcome: "error", Run: last, Superseded: superseded}, true
	}
	for {
		looks.Pause = 0
		run, found, err := wr.Look(target)
		if ready != nil && found && DraftsRun(run) {
			found = false
		}
		switch {
		case err != nil:
			if w, end := failing(err); end {
				return w
			}
		case !found:
			if branch != "" && target == sha {
				next, ok, err := Covering(ctx, wr.Runs, v, branch, sha, remote, nil, "")
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
			looks.Failed = 0
		default:
			looks.Last, looks.Failed = nil, 0
			if started, ok := StartOf(run); ready != nil && ok {
				deadline = started.Add(timeout)
			}
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
			if !run.Done() && (ready == nil || !slices.ContainsFunc(run.Jobs, concludedFailure)) {
				break
			}
			if run.Conclusion != "cancelled" {
				w := Ended(Detail(ctx, wr, v, remote, run), o, end)
				w.Superseded = superseded
				return w
			}
			next, ok, err := Covering(ctx, wr.Runs, v, run.Branch, sha, remote, &run, "")
			switch {
			case err != nil:
				if w, end := failing(err); end {
					return w
				}
			case !ok:
				fmt.Fprintf(o.Stderr, "CI cancelled: %s; no newer run on %s has %s, so it has no result: rerun the workflow, and %s\n",
					run.URL, run.Branch, short(sha), waitsAgain(sha))
				return Watched{Code: exitTemporary, Outcome: "cancelled", Run: &run, Superseded: superseded}
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
		var bound time.Duration
		if ready != nil && ready.Left != nil {
			if bound = ready.Left(); bound <= 0 && (left > 0 || bound < left) {
				return Watched{Code: exitTemporary, Outcome: "bound", Run: last}
			}
		}
		if left <= 0 {
			return TimedOut(cfg, sha, last, looks.Last, o.Stderr)
		}
		wait := min(max(interval, looks.Pause), left)
		if ready != nil && ready.Left != nil {
			wait = min(wait, bound)
		}
		if wait > interval {
			fmt.Fprintf(progress, "GitHub's rate limit: looking again in %s\n", wait.Round(time.Second))
		}
		sleep(wait)
	}
}

// StartOf is when the run's latest attempt started, as GitHub writes it, else
// when the run was created; none for a run with neither, or neither that can be
// read.
func StartOf(run providers.Run) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, run.Started)
	if err != nil {
		t, err = time.Parse(time.RFC3339, run.Created)
	}
	return t, err == nil
}

// Covering is the run that covers sha: the newest of the workflow's runs on
// the branch whose head is sha or has it as an ancestor; found is false when
// there is none. With cancelled, sha's run that a newer push cancelled (bug
// 41), only a run of a newer head than the cancelled run's counts, the one that
// superseded it (bug-160: RunOf has read any other of that head already, and a
// draft's taken for its successor would be followed back to it for ever);
// without, any run counts, for a commit that has no run of its own (bug 49).
// With a conclusion, only a run that ended so counts: work done's look for a run
// that passed (slice 127). A head the
// clone does not have is fetched from remote once, since the newer push may be
// someone else's; one still missing after it is passed over.
func Covering(ctx context.Context, runs providers.Runs, v vcsport.VCS, branch, sha, remote string, cancelled *providers.Run, conclusion string) (providers.Run, bool, error) {
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
		if r.HeadSHA == "" || (conclusion != "" && r.Conclusion != conclusion) ||
			(cancelled != nil && r.HeadSHA == cancelled.HeadSHA) {
			continue
		}
		if r.HeadSHA != sha && !fetched && !hasCommit(ctx, v, r.HeadSHA) {
			fetched = true
			fetchQuietly(ctx, v, remote)
		}
		if r.HeadSHA == sha || v.IsAncestor(ctx, sha, r.HeadSHA) {
			return r, true, nil
		}
	}
	return providers.Run{}, false, nil
}

// hasCommit is whether the repository has the commit rev names.
func hasCommit(ctx context.Context, v vcsport.VCS, rev string) bool {
	_, err := v.Resolve(ctx, rev)
	return err == nil
}

// fetchQuietly fetches the remote's branches, never prompting for
// credentials, within FetchTimeout; a fetch that fails leaves the clone as
// it was.
func fetchQuietly(ctx context.Context, v vcsport.VCS, remote string) {
	if remote == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, FetchTimeout)
	defer cancel()
	_, _ = v.Fetch(ctx, remote, "", vcsport.Talk{NoPrompt: true})
}

// Ended is the watch of a run that completed, or that WatchReady stopped at
// while it goes on: 0 when it succeeded, else 1, naming the jobs that did not
// succeed and, once Detail has been to them, what failed in each, and saying of
// a run stopped at that it has not finished; the end goes to end, a failure's to
// o.Stderr. The tags the run's head carries are named beside its address.
func Ended(run providers.Run, o Out, end io.Writer) Watched {
	tagged := ""
	if len(run.Tags) > 0 {
		tagged = "; tags: " + strings.Join(run.Tags, ", ")
	}
	if run.Conclusion == "success" {
		if !o.Quiet {
			fmt.Fprintf(end, "CI passed: %s%s\n", run.URL, tagged)
		}
		return Watched{Code: 0, Outcome: "success", Run: &run}
	}
	var failed []string
	for _, j := range run.Jobs {
		if failedJob(j) {
			failed = append(failed, j.Name)
		}
	}
	said, going := run.Conclusion, ""
	if !run.Done() {
		said, going = "failure", "; the run has not finished"
	}
	line := fmt.Sprintf("CI %s: %s", said, run.URL)
	if len(failed) > 0 {
		line += "; failed jobs: " + strings.Join(failed, ", ")
	}
	fmt.Fprintln(o.Stderr, line+going+tagged)
	describe(o.Stderr, run)
	return Watched{Code: exitPolicy, Outcome: "failure", Run: &run}
}

// failedJob is whether the job finished and did not succeed, skipped and
// neutral ones aside.
func failedJob(j providers.Job) bool {
	return j.Done() && j.Conclusion != "success" && j.Conclusion != "skipped" && j.Conclusion != "neutral"
}

// concludedFailure is whether the job ended in failure: what stops WatchReady on
// a run still going. A cancelled job does not (a newer push cancels a run's jobs
// first, bug 41), and timed_out, or a job still running with a failed step, is
// left to the run's end.
func concludedFailure(j providers.Job) bool {
	return j.Done() && j.Conclusion == "failure"
}

// short is a commit's abbreviation as the TypeScript's slice(0, 7) writes it.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
