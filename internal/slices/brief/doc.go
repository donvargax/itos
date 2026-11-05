// Package brief is the brief command (slice-204, features/brief.feature), a
// slice (ADR-0053): itos brief <id> --role drafter|implementer|reviewer
// prints the brief an agent of the role starts from, the role's template
// (internal/guide: for a slice or a bug, the implementer's is the brief
// procedure's own, the drafter's and the reviewer's ship beside it and are
// no guide names; a task's text for each role is its own, slice-220, the
// item's kind as next reads it, ledger.KindOf, choosing)
// filled from the item, and with --json the request a deterministic
// coordinator binds an attempt to (docs/plans/v9-coordinator.md, the LLM
// steps' contracts): the spec commit, the reads with their blobs, what was
// left out, and a digest of them all. Its CLI and Run are the command
// line, its Query and Handler the application, over the Ports it owns,
// which internal/cli binds: the repository, the config at HEAD, a commit's
// tree entries, the last first-parent commit touching paths and next's
// reading of an item's branch.
//
// Everything is read from one commit's tree, HEAD's or the one --at names
// (slice-207, which binds an agent's result to the brief at that commit,
// its base), the config included, never the working tree or the index, so
// the brief is a function of one commit and names it and what it holds:
// the registry (internal/work), the
// scenarios tagged for the item (tests.TaggedFiles), a task's ledger entry
// (internal/ledger), the "## What a brief adds here" section of the file
// guide.orchestrating names, and the blobs of the reads. The reads are
// AGENTS.md, the item's feature files (a task's ledger file), both
// required, and for each of the item's refs the file it names or the
// doc.go of the folder it names; a ref naming neither, or a symbolic link,
// which an agent's checkout would resolve to something the recorded blob
// is not, is omitted and said so, as is a missing AGENTS.md. The spec
// commit is the last commit on
// that commit's first-parent history touching the spec's files, none when nothing
// is tagged yet, which only the drafter is briefed on. The request is
// "sha256:" and the digest of LF-terminated lines, the item, the role, the
// spec commit's SHA or "-", then "<path> <blob>" per read in order, so
// another tool can recompute it; the attempt number and the base revision
// are not in it. What a person must add stays a marked placeholder, never
// invented. It reads, writes nothing, commits nothing and asks no forge.
//
// Under push.mode pr an item's spec lives on its branch while a driver stays on
// main, so a reviewer's or an implementer's brief with no --at is computed at
// origin/<id> when next's reading finds the spec there (slice-226, branchSpec),
// by Handle answering as --at would: that commit is the base, and attempt
// submit recomputes the request at the base, which is why everything moves and
// not the spec alone. The reading is next's, reached through the SpecCommit
// port, as slices import no slice, and asked under HEAD's config. A done or
// dropped item reads no ref, whatever its kind: next has no such exception for
// scenarios, as it stops at done before any brief, and here a stale branch
// would be briefed in place of main's spec.
//
// A drafter's brief names the commit its spec goes in under (slice-221,
// chooseSpecCommit), a function of the config at that commit and the spec's
// files, or the file it would have, by the path rules itos commit check-paths
// judges with (internal/scope) and the footers the hook calls missing
// (internal/message): the flag the config has and the type it lets carry them,
// or a stop in place of the commit's sentences. It is in the text and not the
// request, as the repository's own brief lines are: the base an attempt binds
// fixes the config.
package brief
