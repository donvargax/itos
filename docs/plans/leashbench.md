# LeashBench

Status: plan, for review (2026-10-06). Nothing in this plan is built yet.

LeashBench runs real itos work items with the guidance on and with it off, on more than one model and more than one agent harness, under the same environment each time. It answers three questions:

1. Does itos's guidance help a given model or does it make the model worse? Compare the arms of one model.
2. Which model and harness work best inside itos? Compare one arm across models and harnesses.
3. Which rules still prevent a mistake? Look for each rule's mistake in the runs that had no guidance.

LeashBench does not score models in general. Public benchmarks do that (see [Prior work](#prior-work)). LeashBench measures work inside repositories that itos manages, with your own items, rules and gates.

## Why

Agents improve faster than the tools around them. Guidance written for the failures of one model can constrain the next model, and nothing tells you when that happens. The prose rules in `AGENTS.md` and `docs/ORCHESTRATING.md` came from failures of the best model available when they were written, so they were real. Each one still costs context, and none of them expires by itself. LeashBench gives each rule evidence: the rule stays while its mistake still happens without it.

## Prior work

| Benchmark                                                     | What it does                                                                      | What LeashBench takes from it                                                                                |
| ------------------------------------------------------------- | --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| SWE-bench, SWE-bench Verified and variants                    | Replays real GitHub issues and judges them with each project's own tests.         | Tasks are replays of real items, judged by the checks the project already has.                               |
| Terminal-Bench                                                | Runs tasks in containers, so that any agent harness with any model can take them. | Phase 0 checks if its task format can hold a LeashBench task, so that we write only the itos-specific parts. |
| Aider polyglot, LiveCodeBench, SWE-Lancer, METR time horizons | Model scores on generic coding tasks.                                             | Context only. LeashBench does not compete with them.                                                         |

Check the current versions of these benchmarks before phase 0. They change often.

## The parts

### Arms

Each task runs in three arms:

| Arm     | Guidance the agent gets                                                                                                           | Gates (hooks, `itos verify`)                              |
| ------- | --------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| `on`    | All of it: `AGENTS.md`, `itos guide work`, the rules, the full brief.                                                             | On                                                        |
| `gates` | The generated gates block of `AGENTS.md` and a minimal brief: the item, its spec, "commit with itos commit, push with itos push". | On                                                        |
| `off`   | The item and its spec only. No itos guidance.                                                                                     | Off during the run. The judge applies them after the run. |

`on` against `gates` measures the value of the prose. `gates` against `off` measures the value of the gates.

### Tasks

A task is a replay of an item that already landed. Each task holds:

- the commit before the item's first commit, where the run starts;
- the item's spec as it was when the item was handed out: its `@wip` scenarios, or its ledger entry;
- the reference: the commits that landed the item;
- the checks that judge the run (see [Judging](#judging)).

Start with five tasks of different kinds:

1. A bug fix with an `@wip` scenario, for example bug-34.
2. A small feat, for example slice-83.
3. A gate task that changes `tools/bin`, for example T-100.
4. A rename across many files.
5. A change that Windows can break, for example bug-31.

itos is public, so a model can have seen the commits of a task in its training data. Prefer items that landed after the training cutoff of each model you compare. Also use items from your private repositories. Record the landing date of each task, so that analysis can separate tasks a model could have seen.

### Environment

Each run starts from a fixed environment and records it:

- the container image digest;
- the itos version;
- the harness and its version;
- the model ID;
- the network allowlist;
- the task's start commit.

The environment is the devcontainer from the sandbox work: a base image with itos, the git shim and the network allowlist, and one image for each harness. A run has no credentials of yours. It pushes to a local bare repository, never to GitHub.

### Harnesses

An adapter for each harness tells LeashBench three things:

1. How to start the harness with no person present, with a model ID and the task's instructions.
2. Where the harness writes what it did: its transcript or log.
3. How to read tokens, cost and tool calls from that log, where the harness records them.

Start with Claude Code. Then add Codex CLI, Gemini CLI and others. `AGENTS.md` is the instruction file that most harnesses read, so the `on` arm means the same thing in each harness.

### Runs

A run is one task, in one arm, with one model, in one harness. Do each run three times, because a model does not give the same result twice. One full round is tasks × arms × models × harnesses × 3. Measure the cost of one run in phase 0 before you choose how many models and harnesses a round takes.

### Judging

itos judges each run after the agent stops. The agent does not judge itself. A run passes when all of these are true:

- the item's scenarios pass, with `@wip` removed;
- the checks of the item's tasks pass;
- `itos verify` passes over the commits of the run;
- every feature passes.

### What a run records

Each run adds one entry to a results file, `docs/baselines/<date>-<harness>-<model>.yaml` (or a separate repository, see [Open questions](#open-questions)):

- the environment (above);
- the task, the arm and the repeat number;
- pass or fail, and which check failed;
- every gate refusal, by rule ID;
- every rule the run broke, by rule ID (from the judge, and from the gate refusals);
- time, tokens, cost and tool calls, where the harness records them;
- the commits the run made.

### Analysis

A script reads the results files and reports:

- the pass rate for each arm, model and harness, with the three repeats;
- the cost for each arm;
- for each rule, the runs of the `gates` and `off` arms that made its mistake.

How to read the report:

- A rule whose mistake never happens in the `gates` arm is a candidate to suspend (`p1-agent-rules-suspend`).
- When `gates` passes as often as `on` for less cost, the prose makes the model worse.
- When `on` passes more often, the prose earns its context.

### Reminder

A workflow opens a "LeashBench due" issue on the first of each month, as the nightly opens "Nightly red". `itos go` shows open issues at the start of a session, so the coordinator sees the issue and raises it. The issue also says: run LeashBench when a new model comes out. Each results file names its model, so a model ID that has no results file means a run is due.

## Phases

| Phase | Scope                                                                                              | Done when                                                         |
| ----- | -------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| 0     | Claude Code, one model, one task, three arms. Check the Terminal-Bench task format.                | One results file exists, from a script that anyone can run again. |
| 1     | Five tasks, the analysis script, the monthly reminder.                                             | One full round on one model, and its report.                      |
| 2     | More models, and Codex CLI and one more harness.                                                   | One round across two harnesses and three models.                  |
| 3     | Rule IDs from `p1-agent-rules-as-data`. Sightings feed the rules' last-seen dates and suspensions. | A rule suspended or kept because of LeashBench evidence.          |

## Dependencies

- The devcontainer from the sandbox work. Phase 0 needs it.
- `itos verify --json` and gate refusals with rule IDs. Most exist. Decision 35 makes them the contract.
- `p1-agent-rules-as-data`, for stable rule IDs. Until it lands, phase 0 and phase 1 name the lessons of `docs/ORCHESTRATING.md` by title.

## Open questions

- Contamination: do tasks from private repositories go in the same results files as tasks from itos, or in files of their own?
- Budget: how much a monthly round can cost.
- Where results live: in this repository (`docs/baselines/`), or in a repository of their own, because LeashBench runs on more than one project.
- The `gates` arm's minimal brief: write it once and keep it fixed, or else results from different rounds are not comparable.
