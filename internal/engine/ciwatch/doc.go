// Package ciwatch waits for a commit's CI run (slice 51,
// features/watch.feature): itos push after it pushed, itos ci watch [<sha>]
// for any commit, and work done for the commit it judges. Watcher builds
// ci.watch's provider (internal/providers/watch.go); Watch looks at the run
// every ci.watch.interval seconds until it completes, each job's result
// printed once, as it finishes, and the run's address; its exit is the run's
// result, 0 when it succeeded and 1 when it did not, naming the jobs that
// failed. A provider that cannot look exits 3; a run still going after
// ci.watch.timeout seconds, and a look that fails for a server error, a rate
// limit or no network giveUp times in a row, exit 75 (slice 86), a failure
// that may pass when run again; each names itos ci watch <sha> to wait again.
// A rate limit is GitHub's 403 or 429 (bug 40): the look after it waits as
// long as GitHub asked (providers.Wait), when longer than the interval,
// though never past ci.watch.timeout. Between two looks it sleeps (sleep, a
// variable a unit test makes instant).
//
// The run's address and each job's result, keyed by its name, go to stdout
// (stderr under --json, nowhere under -q); the end goes to stdout for a
// success, else to stderr with the jobs that did not succeed, skipped and
// neutral ones aside. The run is done once it completed with a conclusion:
// GitHub can say completed a moment before it records one.
//
// A run a newer push cancelled is no verdict (bug 41): with cancel-in-progress
// on CI's concurrency group, the newer push's run checks the cancelled run's
// commits too. Covering lists the workflow's runs on the cancelled run's
// branch (providers.Watcher.Runs), newest first, and takes the first newer
// than it whose head has the commit as an ancestor, after one quiet fetch of
// the remote when a head is not in the clone; the watch says which and looks
// at that head's run from then on, under the same deadline, ending with its
// result. A cancelled run with no newer run to follow exits 75, outcome
// cancelled: a rerun may pass.
//
// work done judges a commit that may have no run of its own (bug 49): pushed
// with registry-only commits after it, CI ran once, for the push's head. By
// the same rule, given the branch, the newest run on it whose head has the
// commit as an ancestor judges it, and the watch says which run it follows.
//
// itos push --ready (slice 179) watches with WatchReady: a draft's last push
// ran CI with every job skipped, and GitHub may record that run as a success,
// so a run all of whose jobs were skipped, or that lists none, is taken for no
// run yet and looked for again each interval within the timeout, until the run
// that marking the pull request ready starts shows.
//
// A run that completed with any conclusion but cancelled is given Detail
// before its end is said (slice 172, features/watch-failures.feature); a
// cancelled run is followed or ends 75 as above, and has nothing failed to
// name. Detail is the remote's tags at its head commit
// (git ls-remote, an annotated tag by its ^{} commit), and, for each job that
// did not succeed, the step it failed at and what its log names, the log asked
// of providers.Watcher.Log once and never retried, none for a run that
// succeeded. readLog (logs.go) cleans the log as tools/bin/nightly-report
// does, and names its go test failures (a parent with a failed subtest never
// one of its own), godog's failed scenarios (merged with their go test
// subtest), itos ci run's failed steps and, when none has lines of its own, a
// tail; a failure keeps 10 lines, a job 20 failures, a tail 20 lines, the
// rest counted. An unreadable log is said, and never changes the exit code;
// its job's failures are then the annotations of its check run
// (providers.Watcher.Annotations, slice 197, asked once, none for a log that
// was read): those of level failure, each of kind annotation named
// path:line, bounded as a log's. Annotations that cannot be read, or none of
// level failure, leave the job with its step alone.
//
// The outcome is a Watched (the exit code, success, failure, cancelled,
// timeout or error, the run as last seen, the cancelled run's address when a
// newer one was followed), whose Fields are the ci, run and superseded keys
// the commands' --json add. The exit codes, 0, 1, 3 and 75, are internal/cli's
// ExitPolicy, ExitMissing and ExitTemporary, which the caller returns as they
// are. The repository is the VCS port the caller binds.
package ciwatch
