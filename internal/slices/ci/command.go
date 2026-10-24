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
       itos ci watch [<sha>]`,

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

--json: the plan contract, keys only ever added
        {"schema":1,"range","prose","steps","order","tasks","left_out","unknown","not_started"}`,

		"run": `Usage: itos ci run [<from> <to>] | --nightly [--tests]

Runs the plan: unknown task IDs fail first, tasks not started and checks a
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
message, and a check that cannot run fails it too. Itos ci plan lists it.
--tests runs the plan's run of named tests alone, merged as the run merges it
(the smoke set, the tests the range's commits name, the task checks
recognized as runs of the kind), and nothing else: no other step, task check
or code proof, and no preamble. It is for a platform job, which runs per push
on its own platform what the ci job judges, the ci job running the rest. Its
exit code is that run's; with no run of named tests (no tests: step, or
nothing selected) it runs nothing, says so, and exits 0. With --nightly it is
the nightly's tests: step alone, and with no range, every test.
A task check, or a run: step, that opts in with an evidence mapping (evidence:
{}, or the inputs, env and tools it declares) runs on the index tree, as itos
task runs it: a record of the same tree, command, timeout, declaration, config
settings, ci.env, itos version and platform answers at once ("cached
evidence"), and a miss runs in a disposable checkout of the index tree and
records its result under .git/itos/evidence. A run that leaves changes out
says how many tracked paths were not staged.

--json: the run's log on stderr; {"schema":1,"ok","failed_at"?,"failures"?,"evidence"?,"unstaged"?} on stdout,
        failures every failure in run order, failed_at the first; evidence, when a check
        or step opted in, lists each one looked up, in run order, as {"command","source",
        "hit","recorded","key","tree","recorded_at"?,"reason"?,"provenance"?}, and unstaged
        counts the tracked paths whose working copy differs from the index`,

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
