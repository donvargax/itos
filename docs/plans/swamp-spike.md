# The Swamp spike

`p1-swamp-factory-spike`: run one real itos item, bug-160, through Swamp's
`@swamp/software-factory`, with itos judging every gate, and decide from what
happens whether to build beside Swamp rather than finish v9's engine. The
questions it answers are in the idea's `why`; this is how it runs.

## Where it runs

In this repository's devcontainer (`.devcontainer/`, started with the
devcontainer CLI), never on the host: Swamp is a Deno program and its extensions
are TypeScript. The container runs Claude Code with `bypassPermissions`, so no
classifier stops the run.

The container needs a clone, not a worktree (`.devcontainer/README.md`), and
the spike changes the container's allowlist, so it gets a clone of its own,
whose `.devcontainer` changes are never committed:

1. `git clone https://github.com/donvargax/itos.git ~/tmp/llms-safe/itos-swamp-spike`
2. In it, on the host: `tools/bin/itos hook install`,
   `git config hook.repo-pre-commit.event pre-commit`,
   `git config hook.repo-pre-commit.command tools/bin/pre-commit`.
3. In it, on the host, the spike's container changes:
   - `ALLOWED_NAMES` in `.devcontainer/itos-firewall.sh` gains `swamp-club.com`,
     `jsr.io` and `deno.land`. A name Swamp needs beyond those shows as a refused
     lookup; add it and rebuild, and list it in the verdict.
   - `ITOS_VERSION` in the Dockerfile moves from 6.3.0 to 8.45.3,
     `ITOS_CHECKSUMS_SHA256` to
     `d91adcab38594aabbaabc828a6a82b027580374d3e55ea3968d7dee75e8dfaea`.
   - `remoteEnv` in `devcontainer.json` gains
     `"SWAMP_API_KEY": "${localEnv:SWAMP_API_KEY:}"`.
4. On the host, the credentials as environment variables: `CLAUDE_CODE_OAUTH_TOKEN`
   (from `claude setup-token`) and `SWAMP_API_KEY` (a personal key from
   swamp-club.com). GitHub: `gh auth login` inside, once; the volume keeps it.
5. `devcontainer up --workspace-folder ~/tmp/llms-safe/itos-swamp-spike`, then
   `devcontainer exec --workspace-folder ~/tmp/llms-safe/itos-swamp-spike zsh`,
   and `claude` in it, pointed at this file.

## Phases

**1. Install and read.** Swamp from `curl -fsSL https://swamp-club.com/install.sh | sh`,
inside; telemetry off; `swamp auth whoami` answers. software-factory installed
into the clone's Swamp repository, kept out of what the item commits. Read the
extension's source for its lifecycle schema and `examples/`, which its page does
not show, and Swamp's extension permissions: whether an extension may run a
process (`itos`). If it may not, stop: the verdict is that itos cannot judge
from inside Swamp.

**2. The glue.** Our own extension, a model `itos` whose methods run
`tools/bin/itos` with `--json` and return its output as typed data: `next`,
`brief`, `verify`, `review_status`, `ci_watch`. It talks to Swamp only through
the extension API, so the COPYING-EXCEPTION leaves it ours; it lives in the
spike clone, outside what bug-160 commits, and nothing of Swamp's goes into
itos (ADR-0063).

**3. The lifecycle.** Stages spec, spec-review, implement, land and done, with
spec-changes back to spec and repair back to implement. Every gate is a
`workflow-succeeded` over a workflow that runs `itos next bug-160 --json` and
asserts the state the transition needs, so itos decides and Swamp sequences.
`record_dispatch` before every agent step.

**4. The run.** bug-160 from `needs-spec` to merged and closed. The drafter,
reviewer and implementer are Claude Code subagents on the same briefs and
`itos attempt submit` the pilot driver uses.
Count agents, tokens, wall time and every time the person had to step in;
the baseline is bug-157's driven runs (25 agents and 43 minutes to land, then
16 agents and 92 minutes through a repair round).

**5. The stress tests**, one per question:

- Authority: delete Swamp's datastore mid-run and resume. Resuming from
  `itos next` alone makes Swamp a cache; failing to makes it a second authority.
- Cycles: a spec-changes round and a repair round, through `maxCycles` and an
  override.
- Extensions: an agent writes a model from scratch for an API it has not seen
  (GitHub's check runs), and we judge what it wrote.
- Events: `swamp serve` with a webhook moves the item when a run finishes.

**6. The verdict**, as `docs/plans/swamp-spike-verdict.md`: each question
answered, with the evidence; the account terms (every command needs a
swamp-club.com credential, 14 days offline); the Deno exposure (its runtime's
fixes stop about October 2027, and what Swamp has said by then); and the
recommendation. A pivot becomes a decision record; no pivot sends the ideas
worth taking (dispatch before a step, overrides that accumulate, a diagram
from config) into v9.

## Stops

Any of these ends the spike with a no: an extension cannot run `itos`; the
lifecycle cannot advance without Swamp's store deciding; the account terms
change for the worse during the spike.

About two days of agent work: phases 1 to 3 in one session, 4 and 5 in a
second.
