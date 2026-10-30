// Package brief is the brief command (slice-204, features/brief.feature), a
// slice (ADR-0053): itos brief <id> --role drafter|implementer|reviewer
// prints the brief an agent of the role starts from, the role's template
// (internal/guide: the implementer's is the brief procedure's own, the
// drafter's and the reviewer's ship beside it and are no guide names)
// filled from the item, and with --json the request a deterministic
// coordinator binds an attempt to (docs/plans/v9-coordinator.md, the LLM
// steps' contracts): the spec commit, the reads with their blobs, what was
// left out, and a digest of them all. Its CLI and Run are the command
// line, its Query and Handler the application, over the Ports it owns,
// which internal/cli binds: the repository, the config at HEAD, a commit's
// tree entries and the last first-parent commit touching paths.
//
// Everything is read from HEAD's tree, the config included, never the
// working tree or the index, so the brief is a function of one commit and
// names what it holds: the registry (internal/work), the
// scenarios tagged for the item (tests.TaggedFiles), a task's ledger entry
// (internal/ledger), the "## What a brief adds here" section of the file
// guide.orchestrating names, and the blobs of the reads. The reads are
// AGENTS.md, the item's feature files (a task's ledger file), both
// required, and for each of the item's refs the file it names or the
// doc.go of the folder it names; a ref naming neither, or a symbolic link,
// which an agent's checkout would resolve to something the recorded blob
// is not, is omitted and said so, as is a missing AGENTS.md. The spec
// commit is the last commit on
// HEAD's first-parent history touching the spec's files, none when nothing
// is tagged yet, which only the drafter is briefed on. The request is
// "sha256:" and the digest of LF-terminated lines, the item, the role, the
// spec commit's SHA or "-", then "<path> <blob>" per read in order, so
// another tool can recompute it; the attempt number and the base revision
// are not in it. What a person must add stays a marked placeholder, never
// invented. It reads, writes nothing, commits nothing and asks no forge.
package brief
