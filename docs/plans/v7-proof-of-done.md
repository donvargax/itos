# v7: proof of done, through itos-cc

A proposal for the user to mark up (2026-10-07); settled where it says so. It turns q-13 and the design recorded on
`p1-nightly-late-done-checks` into what v7 builds. Each **Open** line is a call still to make; the
recommendation comes first.

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

## The config (shape to settle in the spec)

```yaml
proof:
  provider:
    itos-cc: { version: "0.3.0", checksums: "<sha256>" }
  by_type:
    refactor: [code]
    perf: [code]
    test: [code]
    feat: [scenarios, code]
    fix: [scenarios, code]
    build: [gate]
    ci: [gate]
    docs: [review]
    chore: [review]
```

`config check` refuses a type that changes code or gates with no proof named. A repository with no
`proof` key works as today: itos stays usable in a language itos-cc does not measure.

## Standing rules (q-13 part c)

- **No test reads a policy file as text** (`p1-no-policy-text-tests`): a lint over the tests and
  self-tests; a test of a gate runs the gate.
- **Every gate program's tests run in CI** (`p1-tools-bin-unit-tests-in-ci`): today they run only
  when a task names them.
- **Every command, path, scenario and decision the docs name resolves**: a docs-reference check,
  replacing the doc-mentions-X greps.
- **Open:** a coverage threshold (q-6). Recommended: none. `--fail-uncovered` on the changed
  functions already refuses untested new code, and a global threshold only rewards tests that
  touch lines without checking them. Alternative: a floor that only rises.

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

## Order

1. Finish 6.x: T-117 (the previous-release self-test, red since T-106), T-113, then
   `p1-upgrade-applies-config-steps` and `p1-upgrade-on-major`, so consumers cross the major
   cleanly. Once the module path moves to `/v7`, main cuts no 6.x patch (T-089's rule: no v7 tag
   from a `/v6` go.mod, and no v6 tag from a `/v7` one).
2. **Pre-releases** (the user's call, 2026-10-07; T-118): a pre-release marker,
   `tools/bin/release-version/prerelease` (T-119: a file of the release tool's own, since
   `itos.yaml` holds only keys itos reads), and while it says `rc`, the commits that would cut
   7.0.0 cut `7.0.0-rc.1`, `rc.2` and so on, marked pre-release and never latest.
   `go install …@latest`, the update notice and a bare `itos pin` keep naming 6.x; a repository
   opts in with `itos pin 7.0.0-rc.<n>` (itos-cc and code-quality first).
3. Turn the marker on, move the module path to `/v7`, and land **the itos-cc integration first**:
   the role protocol, the proof rule and `work done`, CI's `mutation sample`, expiring checks.
   That is `v7.0.0-rc.1` (T-119 did the first two). Its push must carry a breaking change: until
   rc.1 is cut, a feat or a fix without one computes a 6.x version, which the `/v7` path refuses
   (`p1-release-major-from-module-path`).
4. The rest of the bundle, each item landing as it is ready, each a further rc: the standing rules,
   debt and claims, exit codes, the hook-skipping refusal, config tightening, slice-95, rule 22,
   dropping compatibility code.
5. Remove the marker: the next push cuts `v7.0.0`.
