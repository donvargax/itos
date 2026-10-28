package ci

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "ci",
	Rank: 170,
	Lines: `  ci plan <from> <to> | --nightly | --whole
                                   print the CI plan; runs nothing
  ci run [<from> <to>] | --nightly [--tests]
                                   run the CI plan; --tests, its run of named tests alone
  ci scope <from> <to>             docs_only=true|false, for GITHUB_OUTPUT
  ci range --head <sha> [--base <sha>]
                                   FROM=<sha>: where a push's range starts
  ci watch [<sha>]                 wait for a commit's CI run, HEAD's by default
`,
	Help: map[string]string{
		"": `Usage: itos ci plan <from> <to> | --nightly | --whole [--data-at <sha>]
       itos ci run [<from> <to>] | --nightly [--tests]
       itos ci scope <from> <to>
       itos ci range --head <sha> [--base <sha>]
       itos ci watch [<sha>]

The forms of a ci.steps entry, and the range each step is given
(ITOS_FROM, ITOS_TO, {from} and {to}): itos help ci run.`,

		"plan": `Usage: itos ci plan <from> <to> | --nightly | --whole [--data-at <sha>]

Prints the plan a CI run would carry out, in cost order, and runs nothing.
Each task check says what the run does with it: run, merged, covered,
nightly (ci.nightly_only leaves it out of a push) or, for a named task's
after: push check in a push, pending (it runs after the push).
With ci.keep_step_order true, a push's plan is ci.steps as written, then the
named tasks' checks in ledger order.
An empty <from> is every commit up to <to>: every test runs, and the checks
of the tasks those commits name.
--data-at reads the ledger, the registry and the smoke set at that commit.
Under a stealth config, given no range, it plans the commits of HEAD on no
remote branch (every one with no remote), the range's from "--remotes".
The code proof's row ends "(code proof)". With the config's proof.code and
no proof step in a push's plan, the last line says why: "Code proof not
scheduled: the range has no start." (an empty or all-zeros start, even when
commits.since gives range checks one), "...the range's changes could not be
read." or "...the range changes no path in proof.code.paths." Neither is
printed for --nightly, and neither is a verdict on the proof.
Under push.mode pr, with ci.range's github provider (and its repository) and
ci.prose's steps, GitHub is asked once which pull request merged the range's
end (commits/<sha>/pulls). When the range is that pull request's head's
commits rebased, after commits of main's own that change only the work
registry and the ledger and name no task or test, each differing from the
head's only in those files, the tasks it names have the same checks in the
ledger at the head and at the end, and the head's newest CI run of
ci.range.github.workflow passed, the range is trusted: the plan is the
prose-only one (the prose steps, the named tasks' static and prose: true
checks, no code proof), and the last line says "Trusted: ...". A merged pull
request that is not trusted says why in a "Not trusted: ..." line. A head the
clone lacks is fetched once from refs/pull/<n>/head. ci scope asks nothing.
ci.jobs names, per job of the CI workflow, the globs of the paths (paths) a
range must touch for the job to run; --json gives, under "jobs", each declared
job's answer for the range, the first reason that holds: no-paths (it declares
none: it runs), whole (the range as typed has an empty or all-zeros start, the
changes cannot be read, or the plan is --whole or --nightly: it runs), config
(the range touches the config file itos read, its path from the repository's
top level: it runs), workflow (the range touches a file under .github/workflows/:
it runs), touched (the range touches one of its paths: it runs) or untouched
(it does not run). A trusted range's jobs are answered by their paths too. A job
ci.jobs does not declare is not in "jobs".

--json: the plan contract, keys only ever added
        {"schema":1,"range","prose","steps","order","tasks","left_out","unknown","not_started","jobs","trusted"?},
        jobs, {"<job>":{"runs","reason"}} for each job ci.jobs declares, {} for none,
        trusted, only for a trusted range, {"pull_request","head","run"}`,

		"run": `Usage: itos ci run [<from> <to>] | --nightly [--tests]

Runs the plan. A range trusted for a merged pull request's green head, as ci
plan says, runs the prose plan and says "Trusted: ..." where a prose-only
range says "Only prose changed: ..."; work only main's run should do (a
deploy, an upload) belongs in a job of its own, since a trusted run, like a
prose one, leaves it out. Unknown task IDs fail first, tasks not started and checks a
prose-only range leaves out are listed, then each step and task check in cost
order, stopping at the first failure; with ci.stop_at_first_failure false,
every one runs and the first failure is the run's. A run that goes on says
"CI failed at" for the first failure and "also failed at" for each later
one, naming it the same way. A named task's after: push
check is listed as pending, to run after the push (itos task, itos work done),
and is neither run nor counted; one ci.nightly_only lists is left out. With no
range, every step and every test. A failing step exits 1, naming its code.
--nightly runs ci.nightly.steps, in the order written, and no task's checks,
going on past a failing step whatever ci.stop_at_first_failure says, and
fails at the end.
With ci.keep_step_order true, a push's run takes ci.steps as written, then the
named tasks' checks in ledger order.
With the config's proof.code, a range that touches proof.code.paths runs the
code proof as a step among the static ones, before the tests: proof.code.check
with {base} the range's start, which reads the results recorded before the
push and runs none of its own. A surviving mutant, an uncovered or a missing
result fails the run, naming each problem its --json lists by its rule and
message, and a check that cannot run fails it too, as does one exiting 0 with
an invalid itos_proof envelope (ci-code-proof-summary) or an explicit judged 0
(ci-code-proof-empty). Itos ci plan lists it. The log says "Code proof" right
before the proof's "$" line (scheduled, not passed), and, with proof.code
and no proof step, one line before the first step says why, as ci plan does;
under --json both go to stderr.
--tests runs the plan's run of named tests alone, merged as the run merges it
(the smoke set, the tests the range's commits name, the task checks
recognized as runs of the kind), and nothing else: no other step, task check
or code proof, and no preamble. It is for a platform job, which runs per push
on its own platform what the ci job judges, the ci job running the rest. Its
exit code is that run's; with no run of named tests (no tests: step, or
nothing selected) it runs nothing, says so, and exits 0. With --nightly it is
the nightly's tests: step alone, and with no range, every test.
With GITHUB_ACTIONS=true, each failure the run reports is also an error
annotation, a workflow command on stderr (up to 10 a run, the first in run
order; GitHub keeps no more): "::error file=<path>,line=<n>::<what> failed:
<reason>" when a failing step's or task check's stdout, else its stderr,
has a line that begins with <path>:<line>: naming a file of the folder (the
path written from the repository's top level), when a code proof problem
names a file and line, else "::error::<what> failed: <reason>". <what> is the
step's command, the task check's "<ID> (<title>)", "the code proof" or "the
tasks named".
A task check, or a run: step, that opts in with an evidence mapping (evidence:
{}, or the inputs, env and tools it declares) runs on the index tree, as itos
task runs it: a record of the same tree, command, timeout, declaration, config
settings, ci.env, itos version and platform answers at once ("cached
evidence"), and a miss runs in a disposable checkout of the index tree and
records its result under .git/itos/evidence, in a checkout that has the
repository's git context. A task check that says snapshot: false runs in the
working folder, nothing looked up or recorded. A run that leaves changes out
says how many tracked paths were not staged.

Each entry of ci.steps and ci.nightly.steps is one of:
  <command>          a shell command; static when a ci.cost.static
                     pattern matches it, else late
  { run: <command>, cost: static|late }
                     a command whose cost: is its class, whatever the
                     patterns say (left out, as for a plain command)
  { tests: <kind> }  the run of tests.<kind>: its run.whole when the
                     range cannot be read (an empty or all-zeros
                     <from>, or no range), when a task check merged
                     into it is recognized as: whole, and on the
                     nightly, else its run.select over the smoke set,
                     the tests the range's footers name and the task
                     checks merged into it, and no run when that
                     selects nothing; a cost: sets its class too
A step's whole: true is accepted and has no effect yet: a tests: step
is narrowed to the range all the same.
Every step and task check the run runs has the range's ends in its
environment as ITOS_FROM and ITOS_TO, full SHAs, which ci.env cannot
change: ITOS_FROM is <from>, or commits.since's commit when <from> is
empty, all zeros or behind it, and is empty when neither gives a start;
--nightly's are commits.since's commit (or empty) and HEAD. {from} and
{to} in a step's text are filled in with the same ends, each as one
quoted shell word (write --from {from}, not '{from}'), and ci plan
prints the step so filled in.

--json: the run's log on stderr; {"schema":1,"ok","failed_at"?,"failures"?,"evidence"?,"proof"?,"unstaged"?} on stdout,
        failures every failure in run order, failed_at the first; evidence, when a check
        or step opted in, lists each one looked up, in run order, as {"command","source",
        "hit","recorded","key","tree","recorded_at"?,"reason"?,"provenance"?}, and unstaged
        counts the tracked paths whose working copy differs from the index; proof,
        once the code proof ran, is {"command","from","to","outcome","summary"}, the
        outcome passed, refused or cannot-run, the summary null unless the provider
        reported a valid itos_proof envelope`,

		"scope": `Usage: itos ci scope <from> <to>

Prints docs_only=true when the range touches only prose (ci.prose.paths).

--json: {"schema":1,"docs_only"}`,

		"range": `Usage: itos ci range --head <sha> [--base <sha>]

Prints FROM=<sha>, where a push's range starts: the pull request's base, else
what the ci.range provider says if it is an ancestor of the head, else the
commit commits.since names if it is one, else empty (run everything). A
provider that fails is not an error. github says the
nearest of the head's first parents, from its parent, with a green run of
ci.range.github.workflow on any branch, asking GitHub's API at GITHUB_API_URL,
else https://api.github.com, about each commit's runs, 100 at most.

--json: {"schema":1,"from"}`,

		"watch": `Usage: itos ci watch [<sha>]

Waits for the CI run of the commit, HEAD's by default, as itos push waits for
the run of the commit it pushed: looks at it every ci.watch.interval seconds,
printing the run's address and each job's result once, as it finishes, until
the run completes or ci.watch.timeout seconds pass. ci.watch.provider says how
it looks: github asks GitHub's API (GITHUB_API_URL, else
https://api.github.com) for the run of ci.watch.github.workflow, with a token
from ci.range.github.token_env, else gh auth token, for the repository
ci.range.github.repository_env names, else the remote's URL. A run not there
yet is waited for. A run a newer push cancelled is no result: the newest run
of the workflow on its branch whose head has the commit is followed in its
place, named when it is, its head fetched from the remote when the clone does
not have it. Exit 0 when the run succeeded; 1 when it did not, naming the
failed jobs; 2 ci.watch.provider none, or not a commit; 3 a refusal from
GitHub's API, or no token, naming both ways to give one; 75 the run still
going, or not there, after the timeout, six looks in a row failing for a
server error, a rate limit or no network, or the run cancelled with no newer
run to follow.

Once the run completed, each job that did not succeed is named with the step
it failed at and what its log says failed: go test's failing tests with their
package and first error lines, godog's failed scenarios with their failing
step and its error, itos ci run's failed steps (the code proof's problems
among them), or, when none of those has lines of its own, the last 20 lines of
the log up to GitHub's first error line. A failure keeps 10 lines and a job 20
failures; what is left out is counted and said. A job's log is asked for once,
never retried, and one that cannot be read changes neither what is said of the
job's step nor the exit code. The tags the remote has at the run's head are
named too, where the run cut a release. This follows the end line, on stderr
whatever -q and --json say.

--json: the progress on stderr; {"schema":1,"ok","commit","ci","run"?,"superseded"?},
        ci one of success, failure, cancelled, timeout, error;
        run {"url","status","conclusion"?,"jobs","tags"?}, the tags of the run's head commit;
        a job {"name","status","conclusion"?,"step"?,"log"?,"failures"?,"more_failures"?},
        step the one the job failed at, log read or unreadable, a failure
        {"kind","name"?,"package"?,"lines","more_lines"?}, kind go-test, scenario, ci-run or tail,
        more_failures the failures left out; the run followed when a newer one was,
        superseded the cancelled run's url`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"plan":  {Flags: []declare.Flag{declare.Switch("--nightly"), declare.Switch("--whole"), declare.Value("--data-at")}},
		"run":   {Flags: []declare.Flag{declare.Switch("--nightly"), declare.Switch("--tests")}},
		"scope": {},
		"watch": {},
		"range": {Flags: []declare.Flag{declare.Value("--head"), declare.Value("--base")}},
	}},
}
