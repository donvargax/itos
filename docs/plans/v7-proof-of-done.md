# v7: proof of done, through itos-cc

A proposal for the user to mark up (2026-10-07). It turns q-13 and the design recorded on
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

**Open.** Recommended: `itos work done <id> --without-proof --why '…'` is allowed, records the
reason in the close commit, and `itos status` lists every item closed that way until the person
clears it, so it is visible rather than forbidden. Alternative: refused outright, the only way out
being an exception in `itos-cc.yaml` with its reason.

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

1. Before v7: `p1-previous-release-selftest-red` (red since T-106, unrun), T-113, then the two
   upgrade items.
2. v7, each item specified and landed in turn, all held back from release until the last lands,
   so consumers get one major: the role protocol, the proof rule and `work done`, the CI sample,
   expiring checks, the standing rules, debt, then the rest of the bundle.
3. **Open:** how to hold the release. Recommended: each v7 commit lands as it is ready, its
   breaking footer naming what it changes, and the release cut once, when the last lands; that
   needs the release job to wait for a marker (a `v7` tag the person pushes) rather than cutting
   one major per breaking commit.
