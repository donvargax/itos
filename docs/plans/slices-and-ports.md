# Slices and ports

ADR-0053 decides it; this is the order. itos takes the layout itos-cc reached
(its `docs/architecture.md`, ADR-0047, ADR-0051, ADR-0053): each command a
vertical slice under `internal/slices/<command>` with its CLI, `Query`,
`Handler` and `Ports`; what several commands share in engines no slice owns;
the outside world through ports, each with an in-memory fake in `porttest`
and a contract suite both run. Every port method takes a `context.Context`
first. Refactors only, in v8.x: the scenarios, the contract-only corpus
(ADR-0052) and previous-release are the proof, and what slips through is
fixed forward. Features wait until the layout is done.

## Where it starts (2026-10-21)

`internal/cli` holds every command, its UI and the dispatch: 52 files,
18.4k lines, importing 26 of the other 32 packages. No depguard, no ports,
no fakes, almost no context. About 120 git call sites (14 raw
`exec.Command(git.Bin())`), the tracker files read in many places and
written through `writeCommitted`, 10 `runtime.GOOS` branches in logic.

## The slices

work (14 subcommands), task (with add, next-id), decision, followup, draft,
commit, hook and verify (sharing the commit judge, an engine), push, ci (with
watch), tests, config, version, init, upgrade (owning pin), git-shim, guard,
guide (owning go, which reads status's summary through an engine), status.
The UI and assembly (help, the flag spec, extensions) stay in `internal/cli`.

## The order

1. **The gates**, from itos-cc: pinned golangci-lint with depguard,
   `tools/bin/architecture-check` (siblings, outside, linked) and its
   fixtures, no mocking library. Rules allow today's tree and tighten as each
   package moves.
2. **Global state out**: the package-global current source, the folder the
   person stood in, `tests.Warnings`, the reserved ID, the cached
   `core.commentChar`, the `init()` that re-enters `cli.Main`.
3. **Pilots**: guard, version, then followup, the first to use the tracker
   store.
4. **The corpus lane** (in parallel, its own worktree): ADR-0052's
   conversion, one task per corpus file, help.yaml first.
5. **A per-command declaration**, after 6 and 7 and before 9 (the user,
   2026-10-21): each slice declares its name, help line, help text and flag
   spec in its own package, in the hand-written parser's spec format, and a
   generator (as itos-cc's `tools/bin/assemble`) writes the command table
   from them, checked current in CI. So agents in parallel lanes change
   their own slice, never `commands.go`, `spec.go` or `help.go`. v9.0 swaps
   the parser beneath the declarations for kong.
6. **The ports**: the tree reader (`source`, about 40 call sites), then git
   by operation group (locate and resolve; ancestry and ranges; trees and
   changed paths; working-tree state; messages and footers; commit; remote;
   hooks), then the tracker store (read at a tree, load, transact, lock,
   reserve IDs, the clone's private folder). config's locating moves out
   first: config imports git and source today.
7. **The engines** out of `internal/cli`: record (`workwrite.go`), ids
   (`idmint.go`, `nextid.go`, `idcounter`), ci watch, the commit judge, the
   status summary, agent rules, pin, shim install. `launch` and `shim` stop
   importing `internal/cli`.
8. **The OS as a value** at the boundary.
9. **The rest of the slices**, least coupled first, more lanes in parallel
   once 5, 6 and 7 are done; status, then work in three or four pieces, last.

Each move carries its itos-cc exceptions with its files.
`docs/ARCHITECTURE.md` and the package doc comments follow each task. The
inventory behind this plan is the coordinator's, not committed; its facts
are above.

## v9.0

kong for flags and help (p1-cli-kong), and itos-cc's exit codes: 80 a usage
error, 81 an unknown command, 82 a requirement not met, 100 internal, 130
interrupted, 75 kept, each `--json` problem carrying its own `exit`. Designed
during the moves, landed together as one breaking release.
