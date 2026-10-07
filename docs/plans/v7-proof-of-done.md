# v7: proof of done, through itos-cc

The plan for v7 (2026-10-07). It turns q-13 and the design recorded on
`p1-nightly-late-done-checks` into what v7 builds. Every call it needed is settled, each marked
with the user's call.

## The problem

A task is closed by checks a model writes for itself, and models will do anything to finish: a
`grep` that the edit happened, or a test that reads a file and asserts it contains a string. Those
prove an edit happened once, not that anything holds, and kept as standing checks they cost every
night and caught nothing real (the evidence is on `p1-nightly-late-done-checks`). They are useful
while a task is open, as a sweep for what a rename or a removal left behind.

## The rule

**A task's proof is computed from its diff and its type, never written by the model.** Core itos
owns the rule; a provider measures; itos checks the provider and never trusts its files.

| What the item's commits changed                                         | The proof `work done` requires                                                                                                                                                            |
| ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Go code (`refactor`, `perf`, `test`, and the code of a `feat` or `fix`) | `itos-cc mutation check --since <base> --fail-uncovered`: every function the item changed has cached results, no surviving mutant that is not excepted with a reason, no uncovered mutant |
| Behaviour (`feat`, `fix`)                                               | its scenarios, live, red first, as today                                                                                                                                                  |
| Gates (`ci`, `build`: workflows, `itos.yaml`, hooks, `tools/bin`)       | the gate's own tests run (a `tools/bin` program's unit tests, its self-test), never a test reading a policy file as text; `actionlint` for a workflow                                     |
| Prose (`docs`, `chore`)                                                 | the person's review; the standing rules below                                                                                                                                             |

`<base>` is the parent of the item's first commit (`itos work show` already knows the commits).

**The checks a coordinator writes in a task's `done_when` stay, as the agent's progress:** they
run while the task is open and gate `work done`, then `work done` removes them from the ledger
(git history keeps them). Nothing reruns a done task's checks; the nightly step that did goes.

## itos-cc as the provider (v0.3.0)

- itos calls it through one role protocol (`p3-role-protocol`): a command the config names, its
  version pinned with its checksums as itos pins its own, read by `--json` and exit code only
  (0 pass, 1 a survivor, an uncovered mutant or a failing baseline, 2 usage, 3 environment).
  itos never reads itos-cc's cache or `itos-cc.yaml`.
- **At work done:** `mutation check --since <base> --fail-uncovered --json`, which runs nothing
  slow: it refuses missing or stale results (a function or its tests changed since). The agent runs
  `mutation run --since <base>` before; the brief says so.
- **Coverage from the features** (the user's call, 2026-10-07): itos's behaviour is tested by
  features that run the built binary as another process, which Go's in-process coverage cannot
  see, so `--fail-uncovered` would call nearly everything uncovered. The fix is Go's integration
  coverage: itos-cc sets `GOCOVERDIR` and merges what the binary writes (itos-cc#14), and the
  features build the itos under test with `-cover` when it is set (T-121). This repository adopts
  the proof only with both in place (T-120), never through an interim without coverage.
- **In CI:** `mutation sample --since <range start> --json`, the spot check that a forged or stale
  cache fails, with the flags the results were recorded with (`--all-tests` where they were).
- **Equivalent mutants** are excepted in `itos-cc.yaml` under `mutation.exceptions`, each with its
  reason, the person reviewing them; a stale one fails (`mutation.exception-stale`).
- **Debt** (`p3-debt-role`, `p3-debt-claims`): itos-cc's committed baseline; a new violation the
  baseline lacks is refused at commit; every entry is claimed by an open item (`pays:`), and an
  item cannot close while it claims one. Entries older than a start point are exempt, so adopting
  it costs nothing up front. Opt-in per repository.

## The config (as slice-100 specifies it)

```yaml
proof:
  code:
    paths: ["{cmd,internal}/**/*.go"] # what counts as code
    check: "tools/bin/pinned itos-cc mutation check --since {base} --fail-uncovered --json"
```

When an item's commits touch `proof.code.paths`, `work done` runs `proof.code.check` with `{base}`,
the parent of the item's first commit, after the CI judgement: exit 0 closes, exit 1 refuses naming
what the check's `--json` lists, anything else refuses with exit 3. `config check` refuses a check
without `{base}`. With no `proof` key, itos works as today, so it stays usable in a language itos-cc
does not measure. The other rows of the table (gates, prose) come after rc.1.

## Standing rules (q-13 part c)

- **No test reads a policy file as text** (`p1-no-policy-text-tests`): a lint over the tests and
  self-tests; a test of a gate runs the gate.
- **Every gate program's tests run in CI** (`p1-tools-bin-unit-tests-in-ci`): today they run only
  when a task names them.
- **Every command, path, scenario and decision the docs name resolves**: a docs-reference check,
  replacing the doc-mentions-X greps.
- **Settled (the user's call, 2026-10-07): no coverage threshold, global or as a floor.** Both
  reward tests that touch lines without checking them; `--fail-uncovered` on the changed functions
  refuses untested new code, and mutants judge whether a test notices a change (q-6).

## Closing without the proof

**Settled (the user's call, 2026-10-07): nothing closes an item past a failing proof.** Models
will do anything to finish, and a bypass any session can type is one they will use. The only way
past a surviving mutant is an exception in `itos-cc.yaml` with its reason, which the person
reviews and which fails once stale; otherwise the agent stops and asks.

## The rest of v7 (breaking, so together)

- Exit codes: `p1-failure-exit-codes`, `p1-commit-exit-codes`.
- `p1-shim-refuses-hook-skipping`: `itos commit`, `itos push` and the shim refuse `--no-verify`,
  `-n` and a `-c` that moves the hooks path.
- Config tightening, one slice: `p1-shell-not-empty`, `p1-moves-except-types-checked`,
  `p1-wip-tag-command-kind`, `p1-group-label-flag-word`, `p1-config-finite-numbers`,
  `p1-check-timeout-positive`, `p1-ledger-layout-checks`, `p1-ledger-id-grouping`,
  `p1-ledger-id-default`, `p1-asks-check`, and `p3-changes-at-commit`.
- Already deferred to v7: slice-95 (AGENTS.md's block sends each session to its guide) and
  `p1-init-plugin-rule-22`.
- `p1-drop-compat-code`.

**Before v7, not in it**, so consumers cross the major cleanly: `p1-upgrade-applies-config-steps`
and `p1-upgrade-on-major`.

**Not in v7:** agent rules as data (its own major), and kong (its spike decides whether it breaks
anything).

## Order, and where it stands (2026-10-07)

1. **6.x is finished:** v6.5.1 is its last release (bug-50 orders release candidates right, so the
   launcher every repository runs reads them). The two upgrade items are deferred (the user: the
   itos-cc integration as soon as it can be).
2. **Pre-releases are built** (T-118, decision 42) and **on** (T-119): go.mod is `…/itos/v7` and
   `tools/bin/release-version/prerelease` holds `rc` (a file of the release tool's own, so the
   config schema keeps only keys itos reads). Since T-123, a module path ahead of the last
   release cuts its major for any releasable commit: **the next feat or fix on main cuts
   v7.0.0-rc.1.** So nothing releasable lands before slice-100: rc.1 is the itos-cc integration.
3. **rc.1, the itos-cc integration — on hold (the user, 2026-10-07)** while an issue itos-cc found
   with end-to-end tests and coverage is worked out:
   - **slice-100** (deferred): the proof rule and `work done`, as specified in
     features/work.feature and config.feature. It lands with its help text, a README section on
     `proof`, docs/ARCHITECTURE.md's line and the package docs.
   - **T-121**: the features build the itos under test with `-cover` when `GOCOVERDIR` is set, so
     itos-cc sees what they cover (its half of itos-cc#14).
   - **T-120** (deferred): this repository turns the proof on. It needs itos-cc **v0.4.0** (the
     first release carrying #14, cut 2026-10-07) or a later one, pinned through
     `tools/bin/pinned`, which T-120 teaches itos-cc; `pinned` takes no version younger than 7
     days (T-067), so **not before 2026-10-14**. CI gains `mutation sample` over the pushed range;
     the coordinator's notes tell agents to run `itos-cc mutation run --since <base>
--fail-uncovered` before `work done`.
4. **Then the rest of the bundle**, each a further rc: slice-101 (a task's checks expire at
   done), the standing rules, debt and claims, exit codes, the hook-skipping refusal, config
   tightening, slice-95, rule 22, dropping compatibility code.
5. **Remove the marker file:** the next push cuts `v7.0.0`.
