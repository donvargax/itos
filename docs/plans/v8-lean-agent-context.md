# v8: the lean agent context, and the breaking checks that waited

v8 is the next bundle of breaking changes, shipped as release candidates as v7 was (the user's call,
2026-10-10). Its headline is what an agent reads: AGENTS.md, which the harness loads into every
session for every task (28 KB here), shrinks to the safety boundaries and one entry line, and the
commit policy it enumerates is printed by `itos guide commit` instead (q-45, q-46). Around it go
the breaking checks the v7 plan already set aside for the next major.

## Already done, in 7.x

- **The procedures** (slice-119, v7.5.0): `itos guide brief|land|repair|decide|commit|push`, and
  `itos go` and `itos guide work` cut to each role's essentials. `guide commit` prints the commit
  policy from the config, so AGENTS.md can drop it.
- **Crossing a major** (slices 113 and 116): `itos upgrade` applies the schema's config renames and
  removals and moves a consumer across a major, which the v7 plan wanted in place before this one.

## In v8

- **slice-120** (`p1-agents-md-reduced`): the AGENTS.md block `itos init --agent-rules` writes
  keeps two lines, the entry line (start with `itos go` unless your brief names another guide) and
  the one boundary no gate holds yet, staging only your own files (the user, 2026-10-11: the gates
  and the guides cover the rest). Text outside the markers is never touched (q-45). Folds in
  `p1-agent-rules-ci-summary`.
- **`p1-upgrade-refreshes-agent-block`**: the launcher hands `itos upgrade` to the target version
  when it is newer, so each release applies its own steps, a present block's rewrite among them.
  A 7.x launcher has no handoff, so the v7 to v8 crossing stays explicit: slice-120's
  `Upgrading:` footer, and **`p1-stale-agent-block-reported`**, `itos go` and `itos status` saying
  when the block is one another itos wrote.
- **`p1-trim-agents-md`**: this repository's own handwritten AGENTS.md text keeps only what is
  its own.
- **`p1-shim-refuses-hook-skipping`**, merged with `p1-agent-rules-as-checks` (the user,
  2026-10-11): the shim, `itos commit` and `itos push` refuse `--no-verify`, `-n` and a `-c` that
  moves `core.hooksPath`; the shim refuses sweeping stages (`git add -A`, `.`, `-u`, a folder) and
  `itos commit` refuses `-a`; the pre-push hook refuses a push whose remote commit is not an
  ancestor. Then the block's staging line is gated too and the block can keep the entry line
  alone. The server-side guarantee against a force-push past all of these is GitHub's ruleset on
  main, on since 2026-10-11 (`p1-init-recommends-branch-protection` recommends it to every
  project, in any release).
- **Config tightening, one slice**: config check refuses an empty `shell`
  (`p1-shell-not-empty`), an `except_types` type not in `commits.types`
  (`p1-moves-except-types-checked`), `wip_tag` on a command kind (`p1-wip-tag-command-kind`), a
  group label that cannot be a flag (`p1-group-label-flag-word`), `.nan` and `.inf`
  (`p1-config-finite-numbers`) and ledger layouts that read nothing (`p1-ledger-layout-checks`);
  `ledger.id` gets one default (`p1-ledger-id-default`); config check and the commit-msg hook
  judge the questions file (`p1-asks-check`); a `Changes:` footer naming nothing of the last
  release is refused at the commit (`p3-changes-at-commit`).
- **slice-107**: config check refuses a nightly step that runs a done task's checks (q-25, moved
  here from v7).

### From the consumer reports, bundled

Not all breaking, but gaps go-template-itos hit, so they ship with the bundle (the user,
2026-10-10):

- **`p1-push-tag-and-ref`** (issue #37): a sanctioned way to push release tags, several at once and
  atomically, and tags pushed only when asked. Ignoring git's `push.followtags` is the breaking
  part: today any `itos push` from main can publish one tag of a multi-branch release alone.
- **`p1-work-give-back`** (issue #42): an item in progress can go back to `todo`, as
  `itos guide repair` already tells a coordinator to do after a revert.
- **`p1-moves-feature-description`** (issue #43): a `test` or `docs` commit may reword a feature's
  description, which no scenario checks.
- **`p1-init-agent-rules-runs-pin`** (issue #47): `itos init` in a pinned repository runs as the
  pinned itos, so a newer itos on the PATH never writes its own block over the pin's. With
  `p1-upgrade-refreshes-agent-block` the block is always the pinned release's.

### The testing tiers, for speed

Not breaking, but every push after them is faster (the user, 2026-10-11). Issue #41's shape, which
this repository reached by q-41, q-52, q-54 and q-55 but for one tier:

- **`p1-ci-e2e-workflow`** (issue #18): a push's end-to-end step runs in a workflow of its own,
  so the platform jobs can run part of the plan.
- **`p1-testing-tiers-guide`** (issue #41): per push, the platform jobs run the smoke set and the
  named scenarios, every feature on three platforms moving to the nightly (here they run every
  feature on every push since T-072, the slowest part of a run); consumers get a nightly workflow
  beside `ci.steps`, as this repository's `nightly.yml` (`itos ci run --nightly`, the Nightly red
  issue opened, commented and closed); the guides say which tier each gate belongs to, beyond what
  `itos guide push` and `land` already say.

## Not in v8

- **Agent rules as data** (`p1-agent-rules-as-data`): it only adds (a rules file, `itos rule`),
  so it ships as minor releases, before or after v8.
- **`p1-notes-placement`**: it changes what `itos go` prints, so it is breaking, but it waits on
  rules as data; holding the rc window open for that is weeks. The next major.
- **`p1-drop-compat-code`** and its sweep of commands kept for compatibility (`itos guide
coordinate` among them): not yet (the user, 2026-10-10).
- **`p1-flag-defaults-policy`**: held (the user, 2026-10-10).
- **`p1-commit-exit-codes`**: out for now (the user, 2026-10-11); a change of exit code is breaking,
  so it waits for a later major.
- **Guided commits** (`p3-guard-replacement`, docs/plans/guided-commits.md): additive by q-37 and
  q-39, so minor releases when ready; only making the guided refusal the default is a major.
- **`p1-push-fits-foreground`**: dropped. `itos push` waiting for CI stays the default; a project
  with slow CI makes `--no-wait` its own rule, as this one did by q-41.

## Order

1. **Land what is not breaking first.** Once the marker is in, every feat or fix is an rc, so a
   fix a consumer needs now goes out before it.
2. **A `ci` task turns the release candidates on**, as T-119 did for v7: go.mod's module path
   becomes `…/itos/v8`, and `tools/bin/release-version/prerelease` holds `rc`. The next feat or
   fix then cuts `v8.0.0-rc.1`.
3. **rc.1 is slice-120**, the headline; then the other items, each feat a further rc.
4. **Remove the marker** once every item above is done and green: the next push cuts `v8.0.0`.
