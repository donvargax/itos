# Specifying and briefing an item

Read before you specify an item and hand it to an implementing agent (the
loop's steps 2 and 3 in `itos go`).

## Specifying

- **Specify the item if it is not yet**: `@wip` scenarios for a behaviour, or
  a task with checks for anything else. An idea becomes work with
  `itos work promote <idea> --kind slice`, or `--kind task --type <type>
  --check <command>`, which writes the task's ledger entry too (itos mints
  the ID), and `--title <title>` when what it has become is no longer what
  its title says. Leave it `todo`: the agent takes it with `itos work take`.
- **Every decision goes into the spec before the agent starts**: a slice's
  in its scenarios and the comments above them, a task's in its ledger
  entry's why. The registry is an index: only an idea, which has no spec
  yet, keeps its reasons in its why there (`itos work edit <id> --note '…'`),
  and `itos work done` drops a closed item's why. A brief is read once and
  lost with the agent; a decision only the brief holds is lost too.
- **Check a spec's words against the product.** A scenario that names a
  label the product does not use, asserts more than its behaviour, or holds
  either way sends the agent after the wrong thing. Ask whether it can fail
  before the work.
- **A check a spec adds goes at the cheapest tier that catches what it
  catches**: the hooks for what the change reaches, in seconds; each push's
  CI for what a push can break; the nightly for what is slow, wide or random,
  which nothing waits on. Deterministic where a tier gates, random where it
  explores: fixed seeds locally and per push, fresh ones nightly. Every
  nightly find becomes a fixed regression at the push tier (a test that kills
  the mutant, a failing seed committed). So a race detector and fuzzing run
  nightly, a secrets scan at pre-push and in CI, govulncheck nightly. Why:
  ADR-0051 in itos's own repository.
- **Split before starting.** A cheap job beside an expensive one is two
  items: the small one lands sooner, and less is lost if the other stops.
- **Specify the next item while the current one runs**: the specs, not the
  agents, are the bottleneck.
- **Red first.** The agent commits the steps alone first, in a `test` commit
  made with `itos commit --item <id>`, its scenarios still `@wip`; runs them
  and sees each fail at the step that checks the behaviour; then builds.
  Git shows the order, and anyone can check that commit out and see red.

## The brief

Fill in the item, its scenarios and its reads; keep the rest. Each rule in it
is there because agents make that mistake without it; add the next one an
item teaches you to the repository's own notes, as a lesson
(`itos guide decide`).

> Implement <item> of <project>, in the repository at <absolute path>:
> <one line>. The scenarios are <ids> in <feature file>, all `@wip` (or: the
> task is <id>). Take it with `itos work take <item>` and push that first.
>
> Run `itos guide work` first, then `itos work show <item>`, and read
> <the repository's agent instructions and the docs this item rests on>.
>
> <What is settled: the decisions to build to, not to re-decide. Any live
> scenario this item makes untrue, and that the commit must say so.>
>
> Red first: commit the steps the scenarios need alone, in a `test` commit
> made with `itos commit --item <item>`, the scenarios still `@wip`; run them
> with `@wip` removed locally and see each fail at the step that checks the
> behaviour; then build. Check the scenarios' names against what you build,
> correcting only a name, never what a scenario checks; if one cannot show
> its behaviour, stop and propose the change.
>
> Before pushing, run what the change can reach beyond what it names:
> <the neighbouring scenarios and checks; for an item that moves a rule from
> one command to another, the scenarios of both commands>.
>
> Others push to main while you work: commit with `itos commit`, push with
> `itos push`, never force (a branch main moved past is brought up with `itos
> push --onto-main`). Do the work yourself; start no agents. Each
> commit's body says why, and its footers stand alone in its last
> paragraph, the one place git reads them. Prefix your scratch files with <item>: sessions share
> the folder. Push, wait for CI with `itos push` in the
> background, and close the item with `itos work done <item>` once it is
> green.
>
> Report: the commits, the CI run's address, each scenario's failing step
> before the work, the scenarios turned green, <the item's own questions>,
> the ideas you added, and any scenario text corrected and why.
