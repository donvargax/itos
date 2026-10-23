# Evidence records

ADR-0060 decides it; this is the design and the order, reviewed in five rounds by the reviewer (2026-10-23). Work items: p1-evidence-local (A), p1-evidence-ci (B), p1-evidence-red-first (C), p1-evidence-pre-push (the follow-up in A5). Open questions settled by the person: CI artifacts kept 90 days, an expired one asking an opted-in work done for fresh CI; only deterministic checks opt in, their author declaring inputs; A comes after bug-111, bug-113 and bug-112; making B and C the default is a v9 question.

## Shared record and opt-in contract

- All work items use one record shape: canonical key, complete manifest,
  verdict, raw exit code, bounded output tail, start/end time, runner identity
  and provenance. The key binds the whole tree the check actually runs on,
  command/mode/timeout, policy/config and toolchain identities, OS/architecture,
  and hashes of declared untracked inputs. CI uses the commit tree. Once
  bug-112 lands, commit-msg checks run from a disposable snapshot of the exact
  index tree, so their key is that index tree; until then, commit-msg runs are
  uncached. Untracked files affect the key only when declared. Current `check.Key` only
  normalizes command and timeout (`internal/check/check.go:49-60,82-121`).
- Proposed config keys, absent by default (today's behavior): `ci.evidence:
  true` enables CI record publication; `work.evidence.require_ci: true` makes
  `work done` require matching CI evidence; `work.evidence.require_red_first:
  true` requires slice red-first evidence. This repository sets them in CI
  config commits after the features they enable. Other consumers get no new
  completion refusal or red-first requirement unless they opt in.
- An opted-in `done_when` check or structured `ci.steps` entry declares
  `evidence: { inputs: [...paths...], env: [...names...] }`. Presence opts in;
  hash listed paths and environment values. The whole tree covers tracked
  files; declared paths cover untracked/ignored inputs. Opted-in `ci.steps` are
  in scope; arbitrary workflow steps outside `ci.steps` are not. If a command
  reads undeclared data, a cache hit is unsound: it is the author's
  responsibility to disclose network, clock, credentials and other external
  dependencies. itos cannot infer them reliably from arbitrary shell text.
  Authors must not opt in such checks. `deps-check` and `schema-contract` use
  network inputs (`itos.yaml:214-224`); task checks can ask GitHub
  (`.github/workflows/ci.yml:86-91`). `after: push` is never reusable
  (`internal/check/check.go:82-90`).
- CI skips checks merged, covered, nightly-only or pending; records are written
  only for actual executions (`internal/ci/ci.go:238-246`). Go's unit-test
  package cache already makes unchanged inputs fast (`tools/bin/go-unit-tests:8-14`);
  preserve normal `-count=1` behavior for scenarios.
- `itos task` and `ci run` invoked by hand in a working tree with any
  uncommitted tracked change still run normally but are uncached. With a clean
  tracked tree, key by `HEAD`'s tree plus declared untracked inputs. This avoids
  claiming the commit tree represents modified working files.
- Local records live in `.git/itos/evidence/` under the git common dir, are
  never committed and never prove CI or completion. Reuse `internal/lock.Hold`,
  not a new lease: its 10-second `lock.Wait`, exclusive acquisition, cross-OS
  behavior and manual stale-lock guidance are established
  (`internal/lock/lock.go:14-18,24-57,60-69`; `internal/lock/doc.go:19-26`).
  If Hold cannot acquire within its bound, run uncached. “Fail closed” means
  never trust the record, not return Fail. A stale lock left by a killed
  process suppresses caching until manually removed, but never changes check
  semantics.
- Write to a unique temp directory and rename to the absent final key directory;
  never overwrite. On any rename failure on any OS, inspect the destination
  and validate its completion marker and manifest. Use it only if valid. A
  corrupt or markerless final record is a miss: quarantine it by renaming to a
  unique name containing key, reason and timestamp, then rerun and record. If
  rename/quarantine cannot complete, run uncached. Readers ignore incomplete
  temp directories.
- Text hits say “cached evidence” and show command, verdict/exit, time and
  source. `--json` adds stable `evidence` fields (`source`, `key`,
  `recorded_at`, provenance), retaining current fields. Misses say “running”;
  failed results are reusable, and changed keys rerun.

## Slice ordering rule

Every `feat` step below is preceded by its own `test` commit with new `@wip`
scenarios; no feature implementation precedes its red-first spec. Scenarios
pinning behavior that already passes go live (not `@wip`) in the `feat`
commit itself, where new live scenarios are allowed; a `test` commit may add
only `@wip` scenarios under this repository's moves rule (`itos.yaml:151`).

## Work item A — local record store and lookup engine

Goal: unchanged eligible local checks rerun instantly without changing which
checks run or their verdict semantics.

1. **Manifest and opt-in (feat, minor).** Add `evidence` to ledger checks and
   structured `ci.steps`; bind tree, command, policy/toolchain, platform,
   environment and declared untracked files. Scenarios: unchanged input hits;
   tracked/untracked input, command, policy, toolchain or OS change misses;
   undeclared checks do not cache; `fails` mode replays its correct verdict.
2. **Store and concurrency (feat, minor).** Use common-dir store and
   `lock.Hold`; lock contention/timeout runs uncached. Scenarios: waiting writer
   reads a complete record; stale lock never fails the check; all rename errors
   validate a possible destination; corrupt/markerless final is quarantined;
   incomplete temp is ignored.
3. **Command lookup integration (feat, minor).** Cache clean-tree `itos task`
   and `ci run` executions only; dirty tracked working trees execute uncached.
   Scenarios: miss runs and repeat hits; cached failure preserves result;
   shared check runs once; a dirty tracked file bypasses lookup; `after: push`
   stays pending.
4. **Commit-msg snapshot lookup (feat, minor; depends on bug-112).** After
   bug-112 materializes the exact staged tree in a disposable directory with
   private Git metadata, key commit-msg records by that index tree and run
   selected checks there. Until bug-112 lands, commit-msg hook runs are
   uncached and continue today's working-tree behavior. A4 scenario: stage a
   change, make a further unstaged edit to a tracked file, verify the check
   runs on the staged snapshot and cannot see the unstaged edit, then run again
   with the same index and hit the record.
5. **Pre-push snapshot lookup (follow-up item, not A).** Pre-push checks should
   run in a disposable snapshot of each pushed commit's tree, using the
   bug-112 materialization mechanism, and key by that commit tree. Until that
   follow-up lands, pre-push checks run uncached. Do not key a working-tree
   pre-push execution as though it ran on the pushed commit. It also carries
   step 6 of docs/plans/in-process-tier.md, moved ahead (the person,
   2026-10-23): a config key makes the pre-push hook run the done_when
   checks of the tasks a push names, on those snapshots, each failure naming
   task, command, exit and output; opted-in checks answer from their
   records, so an agent that already ran one pays nothing. CI still runs
   them.

## Work item B — CI records and `work done`

Goal: consumers can opt in to CI evidence; absent keys preserve today's
consumer-visible behavior.

1. **CI driver writer (feat, minor).** Extend `internal/ci` to collect records
   for opted-in checks and `ci.steps` that actually execute, keyed by the
   shared manifest, and return a bundle for publication. Scenarios: pass/fail
   produces manifest and output; merged/covered/nightly/pending creates no
   phantom pass; unopted checks behave as before.
2. **Completion lookup spec and feature (test, then feat; feat is minor).** The
   `feat` commit adds live scenarios pinning today's passing behavior:
   `work-done-task-check` refuses before CI lookup (`internal/slices/work/done.go:341-371`),
   absent key preserves behavior, and `ci.watch.provider: none` remains
   unwatched. Add new artifact requirements as `@wip` scenarios. Then implement
   lookup when `work.evidence.require_ci` is true: validate the passing run's
   artifact against head/tree, policy and required checks. Keep `failingTask`
   exactly as the pre-CI refusal, then the existing `landedCI` watch
   (`done.go:65-84,341-371,390-429`). Thus a failing task still refuses first.
   `ci.watch.provider: none` keeps today's behavior: report unwatched, require
   no artifact, proceed as today (`done.go:398-402`). If the key is absent,
   do not retrieve evidence or change completion behavior; if on, missing,
   expired or mismatched evidence refuses/asks for fresh CI. Local evidence
   never substitutes. A failed workflow's artifact is not accepted (a `@wip`
   scenario here, since B3 is workflow wiring). IDs are minted for these
   scenarios; the already-passing pinning cases go live in the `feat` commit.
3. **Workflow publication (ci).** Workflow-only slice: publish the
   `internal/ci` bundle as an artifact tied to expected completed run/head when
   `ci.evidence` is enabled. Only Linux's `ci` job runs task checks today
   (`.github/workflows/ci.yml:84-91`); macOS/Windows unit and named-test jobs
   record no check results in v1 (`:110-127,175-186`). Choose artifacts over
   notes for current permissions and push/fetch cost: CI has `contents: read`
   and full checkout (`ci.yml:8-10,30-33`); notes need explicit ref fetch/push
   and write permission. Both stores have the same push-controlled trust.
   No scenarios: a `ci` commit carries none. It is verified by a gate
   self-test under `tools/selftest` (enabled uploads an exact-run bundle,
   disabled publishes none), in the same `ci` commit.
4. **Enable CI publication (ci).** In a separate `ci` config step after B1's
   writer and B3's workflow publisher, set `ci.evidence: true` in this
   repository's `itos.yaml`.
5. **Enable completion evidence (ci).** In a separate `ci` config step after
   B2's consumer feature and B3's publisher, set
   `work.evidence.require_ci: true` in this repository's `itos.yaml`.

## Work item C — red-first records

Goal: opt-in CI proves new slice specs red against the parent implementation;
local attempts remain advisory.

1. **Red-first spec (test).** Add item-linked `@wip` scenarios in a test
   commit before C2, for the runner and record only: Then failure,
   `And`/`But` classification, guard pass, setup/undefined/build failure,
   unexpected pass.
2. **Runner and record (feat, minor; its own `@wip` spec in C1).** A command,
   `itos tests red-first <test-commit>`, produces the record for one `test`
   commit that adds or changes item-linked `@wip` IDs; C3 calls it and never
   re-implements the checkout, build or run. Check
   out the test commit's feature files, separately check out/build its
   immediate parent, and run those scenarios against that binary via `ITOS_BIN`
   (`features/steps_test.go:568-580`). Scheduling the per-commit job is CI
   wiring, not this feat. Parent build failure cannot yield a passing record.
   Every non-guard scenario must fail on a Then step, not merely exit nonzero;
   `And`/`But` inherit the keyword of the nearest preceding Given, When or
   Then. Undefined/setup/Given/When/cleanup failures, skipped scenarios and
   unexpected passes invalidate proof. Record scenario events, trees, IDs,
   binary/build and tools in the shared record. `@wip @guard` marks a declared
   guard which must pass.
3. **Close gate (feat, minor).** A `test` commit before it adds its `@wip`
   scenarios: local red does not satisfy close, several qualifying test
   commits, and `require_red_first` absent (today's behavior) and on. With
   `work.evidence.require_red_first: true`, `work done` requires matching
   trusted red evidence for the slice's test commit plus later green CI
   evidence; the key absent preserves today's behavior.
4. **Per-commit job (ci).** Select each qualifying test commit in the pushed
   range and run `itos tests red-first` on it, publishing the trusted red
   record. Parent build failure fails the job and creates no passing record.
5. **Enable red-first requirement (ci).** In a separate `ci` config step after
   C3's gate and C4's job, set
   `work.evidence.require_red_first: true` in this repository's `itos.yaml`.
   With the key absent, no consumer must run or provide red evidence.

## Compatibility, ADR and settled questions

All consumer-visible requirements are opt-in in v8. This repository turns on
the keys in the `ci` steps above; other consumers retain today's behavior when
keys are absent. Existing live scenarios and their contracts remain intact;
new opt-in requirements are specified by added scenarios. No existing live
scenario needs rewriting or removal for v8. Making these requirements default
is a v9 question, not part of this work.

### The decision (ADR-0060)

Memoize only opted-in checks, keyed by the whole tree actually executed,
command, policy/toolchain and declared untracked inputs. One record shape serves
local and CI runs. The local store is advisory; CI artifacts are accepted only
for the expected completed run/head. `ci.evidence` controls publication;
`work.evidence.require_ci` and `work.evidence.require_red_first` absent preserve
today's completion rules. When enabled, CI evidence supplements (does not
replace) `failingTask` and the CI watch; provider `none` keeps today's behavior.
Red-first runs qualifying test-commit scenarios against the immediate parent;
Then failures count, with `And`/`But` inheriting the nearest preceding
Given/When/Then keyword, and `@wip @guard` scenarios must pass. Network use
cannot be inferred reliably: check authors must not opt in network-dependent
checks. Lock or record failure means run uncached, never fail the check. Once
lookup is implemented, agents may run checks freely; this relaxes ADR-0056/0057
only for local feedback, not CI acceptance or release proof.

Settled by the person (2026-10-23):

1. **Artifact retention.** 90 days; an opted-in `work done` that finds the
   artifact expired asks for a fresh CI run on the head, never passing
   without one.
2. **Initial opt-in set.** Deterministic checks only, whose inputs are the
   tree plus declared files and environment; network or clock checks never
   opt in. The author's declaration is the trust boundary.
3. **Default.** The keys stay off by default in v8; making either
   requirement the default is a v9 question.
