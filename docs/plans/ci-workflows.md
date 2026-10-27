# CI workflows by role: a Linux gate, the full suite after it

p1-ci-e2e-workflow, from #18 and #84. The person (2026-10-26): pushes and
pull requests wait on Linux alone; every test on macOS and Windows runs in a
run that follows, which nobody waits on; a red one becomes repair work; the
release waits for it. This is the order, and what it rests on.

Shelved by the person, 2026-10-26, on this plan's own numbers: the split saves about two
minutes a pull request (Windows ends a median two minutes after Linux) and moves every
macOS- or Windows-only break to main after the merge (about six from 10-22 to 10-24). The
cheaper fix comes first: Windows' slowest unit tests (internal/vcs/adapter, 221 s of 325 s)
and the Linux runs that take 10 to 12 minutes. Revisit only if the wait is still too long.

## Where it starts (2026-10-26)

ci.yml holds three jobs the ruleset requires (`ci (ubuntu-latest)`,
`platform (macos-latest)`, `platform (windows-latest)`) and the release,
which needs all three. The platform jobs run the unit tests and the range's
named tests; every feature on three platforms is the nightly's (T-163).
`itos push --ready` watches the head's whole ci.yml run (`ready.go`'s
`finish`), then Codex's review, then turns on auto-merge. `work done` trusts
the merged head's run (slice-181), and main's run of a rebase merge trusts it
too (slice-188, ADR-0057's amendment), so main's run takes a minute.

Measured on the 14 newest green pull-request runs: the run took 5.0 to 12.3
minutes, median 7.2; the Linux job ended at 1.8 to 12.1, median 4.9; Windows
ended last in 13 of 14, 0 to 4.8 minutes after Linux, median about 2. Windows'
unit tests take 325 s, and one package sets that: `internal/vcs/adapter`,
221 s of real git (then `idcounter` 144 s, `cli` 143 s). Codex's verdict lands
2 to 8 minutes after ready, beside CI. In the longest runs Linux's own plan
(the push proof, the named scenarios) is most of the wait, not Windows.

## The design

1. **Two workflows, not jobs itos stops waiting on.** ci.yml keeps the ci job
   alone and is the **gate**: the one required check, what `itos push`,
   `--ready`, `ci watch` and `work done` wait on. A new `full.yml` (name
   "Full suite") is the **after** workflow, started by the same push as a
   run of its own, as itos-cc's cd.yml is: never called from ci.yml (the
   gate's run would stay open until it ended), never chained by
   `workflow_run` (it starts only once CI ends, in the default branch's
   context). Moving the jobs out is what shortens `--ready` with no change
   to itos: it watches one workflow's run. Jobs left in ci.yml but not
   required would still be waited on.
2. **What full.yml runs.** Nightly.yml's platform matrix (Ubuntu, macOS,
   Windows in two shards): itos built natively, the unit tests, every
   feature, the launcher check. Every feature, not the range's named ones:
   nobody waits, so each code merge gets what only the nightly gives today.
   Copied from nightly.yml's job as ci.yml's platform job copies its build
   now; a reusable workflow is left out, since called jobs change their
   names, which `nightly-report` and the nightly's platform step read.
   Its jobs get names of their own (`full (windows-latest, 1/2)`): a
   ruleset matches a required check by name across workflows.
3. **The release moves to full.yml**, on a push to main, needing its
   platform jobs and a `gate` job that runs `tools/bin/itos ci watch
   "$GITHUB_SHA"` (ci.yml's run of that commit, or the run that covers it;
   long finished by then). The release still comes from the commits since
   the last tag, so a red full run holds every item's release until a green
   one, which releases them all.
4. **Each workflow's range starts at its own last green run** (#18's second
   ask). full.yml's scope reads `itos ci range --workflow full.yml`, so a
   prose push after a code merge whose full run was cancelled still runs in
   full. Most pushes to main are bookkeeping (8 of the 9 newest), so the
   prose skip matters. **slice-188's trust must follow the workflow too.**
   `ci-repo scope` trusts the merged head's run of ci.range's workflow;
   after the split that is a Linux-only run, so full.yml would skip macOS
   and Windows on every merged pull request. full.yml's scope trusts only
   the head's full.yml run (an opted-in pull request's, below).
5. **Concurrency.** full.yml has its own group, `full-<ref>`, never ci.yml's
   (one group across two workflows queues the gate behind the suite).
   Cancel in progress on pull_request events (T-314), queue on main
   (itos-cc's T-201: cancelling there let each push of a burst cancel the
   last, and no run finished). GitHub keeps one pending run per group and
   cancels an older pending one; its commits are the next run's, since the
   range starts at the last green. The release keeps its `release` group.
6. **Opting in before the merge.** A pull request labelled `full-suite` runs
   full.yml on its pushes (`opened`, `synchronize`, `reopened`, `labeled`),
   drafts included, since the label is an explicit ask. Nothing requires
   it; the implementer waits on it with `itos ci watch --workflow full.yml`
   before `--ready`. A spec that touches platform-sensitive code
   (`internal/vcs`, `internal/git`, build-tagged files, the launcher, the
   shim) names the label, and every repair of a red full run carries it.
   Main's full run then trusts the head's green full run (point 4).
   `--ready` waiting by itself on an after run at the head waits for
   evidence that an implementer forgot the label's wait.
7. **The nightly** is unchanged but for its step reading the newest
   completed CI run's platform jobs, which reads full.yml's newest
   completed run instead: the backstop that opens the Nightly red issue if a
   red full run is left standing. Its own platform jobs stay: they judge
   main's head on quiet days and catch flakes.

## What breaks after the merge, and who repairs it

The ruleset requires `ci (ubuntu-latest)` alone, so a macOS- or Windows-only
break merges and is found on main by its full run, about 10 to 12 minutes
after the merge (Windows shard 1/2, the slowest, ran 10.3 minutes in the
nightly). `itos status` and `itos go` show it (slices 3 and 4), and
`ci watch --workflow full.yml` in the background tells the coordinator
without polling.

The repair, as for a red nightly today (docs/ORCHESTRATING.md): **a red full
run is the first item**. The coordinator reruns the failed jobs once; green
on the rerun is a flake, recorded as an idea, no repair. Red again: a `fix`
handed to a fresh implementing agent with the run, its failing tests and the
range's commits and their items; it reproduces only those tests (on Linux
when it can; on the platform through the `full-suite` label), fixes, and its
pull request carries the label so the fix is proven before it merges. It is
done when full.yml is green on main. Main's gate stays green throughout, so
"revert at once" (`itos guide repair`, for a red gate) does not apply by
default; the revert threshold is open question 3.

The item that broke it is already closed: `work done` keeps waiting on the
gate alone (open question 1), so "done" means the gate passed, and only the
release means the full suite passed. That revises ADR-0039's "main's run
green" and ADR-0051's "each push's CI runs everything a push can break" for
macOS and Windows; the turn-on records it as an amendment to ADR-0051.
p1-agent-git-ci-contract is untouched: the guard, `--no-wait` and who waits
on the gate stay as they are; the after run is the coordinator's, as the
nightly is.

## The slices, in order

Each is small; 1 to 5 come before the turn-on, 6 and 7 may follow it. All
are additive: a project with one workflow configures nothing new.

1. **Workflows with roles.** `ci.watch.github.workflows`, a list of
   `{workflow, role}`, role `gate`, `after` or `nightly`; `workflow` and
   `nightly_workflow` stay as shorthands for a gate and a nightly. Config
   check refuses two gates, a workflow named twice and both forms at once.
   push, `--ready`, `ci watch` and `work done` keep watching the gate.
   _Before._
2. **A range per workflow.** `--workflow <file>` on `ci range`, `ci plan`
   and `ci run`: the range starts at that workflow's last green run, and
   slice-188's trust reads the head's run of that same workflow. itos-cc
   rewrites its config with yq for this today (cd.yml's scope job). The
   yq copy is the fallback if this slips. _Before._
3. **Status shows each watched workflow.** Its newest run on the branch, a
   run cancelled while pending passed over for the newest one that
   completed otherwise (#17's rule, as `ciwatch.Covering` already follows
   one), and its newest green run while the head has not passed; `go`
   lists a red after run under repair, as it lists a red nightly. A 404
   from a workflow GitHub has not registered yet is "no run yet". _Before._
4. **`ci watch --workflow <file>`**, any watched workflow, with the detail
   it already gives a failed run (jobs, failing tests, tags at the head).
   _Before:_ the repair proof and the coordinator's wake-up rest on it.
5. **The loop in the guides.** `itos guide repair` and `guide brief`: the
   after-red loop above (rerun once, a fix first, the label on the repair,
   done when that workflow is green on main). _Before._
6. **go names the range.** A red after run's failing tests and the commits
   from that workflow's last green run to its head, with the items they
   name, so the coordinator need not read the log and git log. _After._
7. **Status says whether the newest release published** (#84's fourth
   ask): a red full run that held a release back is then visible. _After._

8. **The turn-on**, a `ci` task, last (below).

Left out, for v9 or never: #18's partial plan (`ci run --stage`/`--except`,
`ci.stages`), since full.yml runs the whole suite and the in-process tier
(ADR-0059) and evidence records change what each tier runs; a per-workflow
`ci.range` block, which the flag makes unneeded; `--ready` waiting on after
runs by itself; `work done` waiting on after runs, unless the person
chooses it (question 1).

## Interplay

- **slice-188's trust.** The gate's main run keeps trusting the head's
  ci.yml run. full.yml's never does (point 4); without slice 2 it would
  skip every merged pull request, the worst failure this plan can have, so
  the turn-on's selftest proves full.yml's scope runs in full for a merged
  pull request whose head has no full.yml run.
- **T-314.** Per workflow: each file sets cancel-in-progress for its own
  pull-request runs; neither cancels the other's.
- **slice-153** (deferred). The turn-on adds full.yml in the same merge as
  the config naming it, so a status or `ci watch --workflow` asked in the
  minute before GitHub registers it may meet a 404; slice 3 says "no run
  yet", and `ci watch` exits 3 once, a rerun enough. full.yml's own first
  run asks `ci range` while registered, and a range provider that fails is
  no error anyway (everything runs). push never watches full.yml. Hit at
  most once; slice-153 stays deferred.

## The turn-on

One `ci` task on its own pull request, after slices 1 to 5 are released:

1. full.yml as above; ci.yml loses its platform and release jobs, its
   header saying it is the gate; `ci-repo tests` loses its last caller and
   goes. `ci-repo scope` takes the workflow it trusts.
2. itos.yaml: `ci.watch.github.workflows` names ci.yml (gate), full.yml
   (after), nightly.yml (nightly); the nightly's platform step reads
   full.yml; the `ci-platform` selftest holds full.yml's coverage.
3. The amendment to ADR-0051; docs/ORCHESTRATING.md's red-nightly rule
   names the full run.
4. The ruleset drops the two `platform` checks just before `--ready` of
   this pull request, whose own run has none. Pull requests merging in
   between still run them in ci.yml and `--ready` still waits on them.

**Rollback**: revert the task's commits through a pull request (ci.yml gets
its platform and release jobs back, full.yml and its config entry go), then
re-require the two checks once the revert merged. The itos slices stay; a
config with one workflow uses none of them.

## Cost and saving

- **Per pull request**: Windows' tail, about 2 minutes of a 7-minute run,
  up to 5; twice when GitHub rebases the head and CI runs again. Where
  Linux ends within a minute of Windows (3 of the 14), almost nothing.
  About a quarter of the run, not the 12 minutes: in the three 12-minute
  runs Linux itself ended at 10.3 to 12.1.
- **Release**: 10 to 12 minutes after the merge instead of 2 to 3.
- **Runners**: about 25 runner-minutes per code merge (four jobs, Windows'
  slowest shard about 10); free for a public repository; four of the 20
  concurrent jobs, one of the five macOS.
- **Coverage gained**: every feature on macOS and Windows per code merge,
  where today a push runs the smoke set and its named scenarios there and
  every feature waits for the night.
- **The risk, bluntly.** From 2026-10-22 to 10-24, 12 runs on main were red
  on macOS or Windows alone (about six breaks, one a Windows lock flake,
  p1-idcounter-windows-lock-flake); no pull-request run since pr mode has
  been. Under the split each of those lands on main. With agents merging
  in parallel, others merge on top within the 10 minutes before the full
  run ends and the hour before a repair merges: every later full run stays
  red with the first failure, so a second Windows break hides behind it;
  the release stops for every item; and the range to blame holds several
  items, so the culprit is a judgment from the failing tests. A flake on
  Windows becomes repair work until the rerun rule catches it. A cheaper
  alternative with no red-after-merge: make `internal/vcs/adapter`'s tests
  faster on Windows, or shard Windows' unit tests, which removes most of
  the same 2 minutes.

## Open questions for the person

1. `work done` on the gate alone (recommended: the close stays fast, and a
   red full run is a `fix` of its own), or on the full run too (each close
   waits about 10 minutes, and the red is the item's)?
2. While full.yml is red on main, do pull requests already in flight still
   merge (recommended: yes; the gate is green and the release is held), or
   does the coordinator hold auto-merge until it is green?
3. When does a red full run get reverted instead of repaired: never, after
   a set time without a merged fix (say two hours), or when a second item's
   full run fails a new test?
4. Given the numbers, the split as designed, or first the cheaper
   alternative (Windows' slowest unit tests), keeping all three platforms
   required?
