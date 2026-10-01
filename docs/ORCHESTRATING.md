# Orchestrating

For the session that coordinates the work: it hands slices to subagents,
checks their results and keeps `main` green. **If you were given a slice, a
fix or a task to implement, this file is not for you:** follow `AGENTS.md`,
do the work yourself and don't start subagents.

## How the user wants the loop run

- **Keep going until the queue is empty.** Launch the next agent when the
  current one hands back, slice after slice, until the phase is done or the
  user stops you. Stop only for a decision you genuinely cannot make
  yourself; make the routine calls and say which you made in the report.
- **Ask with a recommendation.** A real choice goes to the user as a short
  recommendation plus its options, the recommended one first. Before asking
  design questions about a slice, say in a line what the slice is for: a
  question without its purpose cannot be answered well.
- **Split before starting.** A slice that mixes a cheap job with an
  expensive one is two slices: the smaller one lands sooner, and less is lost
  if the other is interrupted.
- **Mind the spend.** An agent costs a lot of tokens per slice, often
  hundreds of thousands, and a spend limit can cut one off mid-slice. Say so
  when proposing many more agents.
- **Expect the plan to move while agents run.** The user refines `PLAN.md`,
  the feature files and the ledger mid-run; that is "Doc work while a
  subagent is running" below, not an interruption.

## The loop

**Before the loop, and again after every landing: read the last nightly.**
`gh run list --workflow nightly.yml --limit 1` (its failing scenarios:
`gh run view <id> --log-failed`, or the open "Nightly red" issue). The whole
E2E suite runs only there, so a change that reaches scenarios no push names
shows up the next morning. A red nightly is the first item, a `fix` handed to
an agent before any new slice; it is not left for whoever looks next.

1. Pick the next slice from `docs/HANDOFF.md` ("Next"), among what
   `vp run work` proposes for the person you work for: an item another person
   owns in `docs/work-items.yaml` is theirs, and one whose dependencies are not
   done waits. If it is not specified in `@wip` scenarios or a task yet,
   specify it first. An idea (`kind: idea`, listed apart by `vp run work`) is
   such an item: specify it, then change its kind to `slice` or `task` in the
   same `docs` commit (and its id, once it is a numbered slice or a T- ID).
   An item with `deferred:` waits until its reason goes; the user lifts it,
   not the coordinator.
2. Start **one** implementing subagent with the brief below. One at a time:
   slices share code, the ledger and the registry, and two agents in one
   checkout overwrite each other. **More work means a longer queue, not more
   agents at once** — when the user asks for several things, or for enough to
   keep you busy, that is a queue to work through one agent after another. Go
   parallel only if they say so in as many words, and then:
   - give each agent a worktree of its own (`isolation: "worktree"`), because
     agents sharing a checkout share its index and overwrite each other's
     staged work and registry edits;
   - tell each one to symlink `node_modules` to the main checkout's instead of
     running `vp install`, because a second install can load two copies of a
     tool and fail every suite (`ln -s <main checkout>/node_modules
node_modules`; the worktrees live under `.claude/worktrees/`, which the
     unit tests, the linter and the formatter leave out);
   - one E2E run per checkout at a time, because runs in one checkout share
     its build output and report folders (each checkout serves on a port of
     its own, so separate checkouts don't collide);
   - pull with `--no-autostash` before every push;
   - and commit from a worktree of your own while they run.
3. When it reports, check the result (below). Relay the report to the user
   with what to look at in `vp dev`.
4. The user reviews; that review is final. What they ask for next is
   **specified here and handed to an agent**, not built here: feedback that
   arrives mid-session is a slice like any other, however small it sounds
   when it is described. The test, and it is the whole test: **if it needs a
   scenario, it is a slice and it goes to a subagent.** What the coordinator
   does itself is the work that changes no scenario — this file and the other
   docs, `PLAN.md`, the ledger and the registry, the briefs, landing and
   pushing an agent's work, and the `docs/HANDOFF.md` rewrite at the end of
   the session.

Work goes straight to `main`: no branches, no pull requests. Every push runs
CI (`.github/workflows/ci.yml`).

## When a slice does not reach green

Revert it on `main` at once, before writing anything up: a red `main` blocks
everyone who pushes after it, and the write-up can wait.

1. One `revert:` commit undoing the slice, with a `Task:` footer naming the
   task whose work it undoes (the hooks reject `git revert`'s default message,
   so write it as any other).
2. If a live scenario was what blocked it and `main` cannot be green without
   it, set that scenario `@wip` with its reason as a comment above it.
3. In a `docs` commit, record the attempt in the item's `why` in
   `docs/work-items.yaml` (or the task's in `tasks/`): the commits, the
   mechanism, what was found and what is left, so the next agent, or the
   other owner's, starts from it. The item goes back to its owner as `todo`;
   its scenarios stay `@wip` as the spec.

**No archive branch.** The reverted commits stay reachable through the revert
and the registry; a branch is one more thing to clean up.

## Brief for an implementing subagent

Fill in the slice, its scenarios or task and the reads specific to it; keep
the rest.

> Implement phase <p> slice <n> of <project>, in the repository at
> <absolute path>: <one line>. The scenarios are <ids> in <feature file>, all
> `@wip` (or: the task is <T-…> in `tasks/phase-<p>.yaml`).
>
> Read `AGENTS.md`, `PLAN.md` (<the sections this slice rests on>), the
> earlier slices' commits (<which; `vp run changelog -- --scenario <id>` or
> `git log --grep` finds them>), what the last one left missing (<the ideas
> and `todo` items in `docs/work-items.yaml`, from `vp run work`>),
> `docs/ARCHITECTURE.md`, `e2e/features/README.md` and `tasks/README.md`
> first, and follow `AGENTS.md` — in particular "The gates run themselves":
> just commit and react to what a gate reports.
>
> <What PLAN settled, in short — decisions the agent must build to, not
> re-decide. The gaps the previous slice left, from the registry. Any live
> scenario this slice will make untrue, and that the `feat` corrects it and
> says so.>
>
> Check the scenarios' labels and names against what you build, correcting
> only a name, never what a scenario checks. If a scenario can't show the
> behaviour it names, stop and propose the change, as `AGENTS.md` says.
>
> Before pushing, run what this slice can reach beyond the scenarios it
> names: `vp run e2e --grep "<the slice's and its neighbours' ids>"`<, and
> the project's own checks that no gate runs>.
>
> Other people push to `main` while you work. Commit, then
> `git pull --rebase --no-autostash origin main`, then push in a separate
> command — always in that order, because a stale push is rejected only
> after the pre-push hook has run. Never force-push.
>
> Do the work yourself; don't start subagents. Don't add `Co-Authored-By` or
> any other attribution lines. Each commit's body says why, and is the
> changelog; the reasons that outlast it go where `AGENTS.md` says. Push at
> checkpoints if the slice is long. Finish as `AGENTS.md` says: push to
> `main`, get CI green, and wait for the CI result before you report.
> `gh run list --commit` needs the full 40-character SHA.
>
> Report: the commits, the CI run URL, the scenarios turned green, <the
> slice's own questions>, what the next slice will find missing (the
> `kind: idea` items its last `docs` commit added), and any scenario text
> corrected and why.

Everything else an implementing agent needs is in `AGENTS.md`. Don't restate
its rules in the brief — repeating them invites the agent to treat them as
the interesting part and to "prove" each one by running it. The lines above
that look like rules are there because each one is a mistake agents make
when the brief leaves it out. Add a line to the template here when a slice
teaches you a new one, stated as the rule and its reason.

## Lessons

- **A checkpoint push that carries a task's footer runs that task's
  checks**, so a checkpoint pushed before the task's `done_when` is met is red
  by design. Expect it, and read the final push's run.
- **Never chain a push after a rebase in one command.** A
  `git pull --rebase && git push` chain can push a rebase that stopped on a
  conflict, so `main` takes part of the branch and the rest follows in a
  second push. Rebase, read the result, then push in a separate command
  behind a guard:
  `g=$(git rev-parse --git-dir); [ ! -d "$g/rebase-merge" ] && [ ! -d "$g/rebase-apply" ] && [ -z "$(git diff --name-only --diff-filter=U)" ] && git push origin HEAD:main`.
  Check `git rebase --continue`'s exit before any amend (an amend after a
  failed continue folds conflict markers into the previous commit), and stop
  an interactive rebase on a commit's SHA, not its subject, which may repeat.
  A landing that rewrites nothing on `main` is done in a worktree of its own
  while an agent works in the checkout.
- **Specify before starting, and keep slices small.** Agents implement
  scenarios quickly; the specs are the bottleneck, so write the next slices
  while the current one runs. When a slice mixes a cheap job with an
  expensive one, split it before an agent starts: a smaller slice pushes
  sooner and risks less if it is interrupted.
- **Check a spec's words against the product.** A scenario that names a label
  the product does not use, compares more than the behaviour it is about, or
  sets things up so the outcome holds either way, sends an agent after the
  wrong thing or proves nothing. Before committing a spec, look its names up
  in the code and the interface, and ask whether the scenario can fail
  before the work.
- **Agents hand back before CI finishes.** A report often says "CI not
  confirmed". Watch the run yourself with a `Monitor` on its id (from
  `gh run list --commit <full sha>`), and treat the slice as done only when
  it is green. A slice that goes red in CI usually does so on a live scenario
  its own footer did not name: the brief's "run what this slice can reach"
  line is the answer to that.
- **An agent's run link is a claim, not evidence.** An agent can report a
  green run that does not exist, or one for another commit. Before relaying a
  result, look the run up from the pushed head
  (`gh run list --commit $(git rev-parse origin/main)`) and read its
  conclusion; watch it yourself if it is still going.
- **Resume the agent that made a red run** with `SendMessage`, the log's
  failure and the rule "fix the cause in the product, not the step". It has
  the context; a fresh agent would re-read everything.
- **Stop a finished agent that keeps waking.** An agent whose CI watch is
  still armed re-sends its report and could, in principle, commit again while
  the next one works. `TaskStop` it once its work is pushed and green.
- **The spend limit can cut an agent off mid-slice.** Its work survives if it
  pushed first, which is why the brief asks for checkpoints. Read its commits
  (`vp run changelog -- --scenario <id>`) instead of resuming it for a
  report.
- **Never pipe a command whose exit code matters** (`… | tail`,
  `…; echo EXIT=$?` after a pipe): the pipeline, and a background task
  running it, reports the last command's status. Write the output to a file
  and read it. A rejected push is as often the pre-push hook failing as the
  remote having moved; read which before saying so.

## Doc work while a subagent is running

The user keeps refining `PLAN.md`, the feature files and the ledger while
implementation is in flight, so this is the normal case, not an
interruption. A subagent without a worktree of its own works in **this same
working tree, with the same index**, so:

- **Don't stage or commit while an agent is running.** Not even with explicit
  paths. `git commit` takes everything in the index, and the agent's staged
  files are in it too; and two pre-commit hooks at once break `vp staged`'s
  backup and restore of the index, which can reset the tree to HEAD and leave
  both sessions' work only in that backup. Explicit `git add` paths protect
  against sweeping the agent's _unstaged_ files and nothing else.
- **Keep your edits in the scratchpad until the agent reports**: new files as
  copies, changes to existing files as a patch (`git diff > …`), plus a note
  of what to commit and why. Apply and commit them once the agent has pushed
  and handed back; then push them yourself.
- **Don't touch what the agent is writing.** `docs/HANDOFF.md` is yours, not
  the agent's: rewrite it at the end of the session so it says only what the
  next one should do. A `refactor` or `perf` agent may not touch feature
  files at all, so those, `PLAN.md` and `tasks/*.yaml` are usually safe to
  prepare — but prepare them, don't commit them.
- **If the tree is lost anyway**, the hook's backup is still in the object
  store: `git fsck --no-reflog --unreachable`, and the newest `WIP on main`
  commit is the working tree (its second parent, `index on main`, is what was
  staged). Restore each session's paths with `git checkout <wip> -- <paths>`
  after checking nobody has recreated them, and tell the agent.

## Checking a result

The gates already cover most of it (`AGENTS.md`, "The gates run
themselves"): the commit hooks enforce format, lint, types, the unit tests the
change reaches, the audit, and commit footers and scope; the pre-push hook
runs the unit tests the pushed commits reach; CI re-checks every pushed
commit against the commit rules, so a commit that skipped the hooks turns it
red, and runs the plan `itos.yaml`'s `ci` states (the whole unit suite with
coverage, the build, the audit, the checks of every task the pushed commits
name, and one E2E run over the smoke set and what the commits name); the
whole E2E suite runs nightly. Don't re-run what these cover. Check only:

- CI is green for the last pushed commit:
  `gh run list --commit <full sha> --json conclusion --jq '.[0].conclusion'`.
- **And the next nightly is green for what the slice reaches.** A push runs
  the smoke set and what its commits name; the whole suite runs nightly. A
  slice is done when the nightly after it is green too, or red only on what
  it did not touch; a red on a scenario it reaches is that slice's, the next
  morning, before new work.
- Nothing is left unpushed (`git status -sb` shows no "ahead").
- No `@slice-<n>` scenario is still `@wip` without a reason in the report.
- The slice's commits say why in their bodies
  (`vp run changelog -- --scenario <id>`), and the reasons that outlast them
  are where they belong (a comment in the feature file, a `why`,
  `docs/ARCHITECTURE.md`), not only in a commit.
- The project's own checks that no gate runs (`AGENTS.md`, "What no gate does
  for you") ran, if the slice reaches them.

## When CI is red outside a slice

If CI on `main` is red and no implementing subagent is working on it (for
example after a push by someone else, or a failure that only shows on the
runner), start a temporary subagent to fix it:

> CI on `main` of <project> is red: <run URL>. Read `AGENTS.md` first. Read
> the failure with `gh run view <id> --log-failed`, fix the cause (not the
> check), and commit through the hooks with the type and footer the rules
> require. Do the work yourself; don't start subagents. Push to `main` and
> get CI green. Report the cause, the commits and the green run URL.

If the failure is in the spec or a gate rather than the code, stop and ask
the user instead of changing the gate.
