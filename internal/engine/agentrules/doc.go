// Package agentrules is itos's rules for agents, an engine (ADR-0053, step 7
// of docs/plans/slices-and-ports.md): what init, status, upgrade and guide
// share of them. The block (Block, slice 59, slice 120) is a constant
// between <!-- itos:begin --> and <!-- itos:end -->, holding the staging
// boundary under a heading, in Markdown a formatter leaves alone:
// one line a paragraph, a blank line between blocks. A file's text is read
// LF, whether it used CRLF said beside it, and written back with the line
// endings it had (ReadText, WriteText).
//
// Split cuts AGENTS.md around the markers, refusing a lone or misordered
// marker or a second block (kind.Usage); blockSpan finds a block's markers
// while skipping another's. A block found other than Block is stale:
// Stale names the files holding one, as init's rerun decides it (itos
// status, slice 124), StaleLine is what both say of it, and Refreshed is
// each such file with that block, and nothing else, rewritten in this itos's
// words (itos upgrade, slice 126). Neither writes, nor fails: a file that
// cannot be read, or whose markers cannot be placed, is left for init to
// report.
//
// Under a stealth config (slice 80) nothing tracked may change, so the
// rules are three files git never looks at: the block in <git common
// dir>/itos/AGENTS.md, a CLAUDE.local.md whose block imports @AGENTS.md when
// the project has one and then that file, by the path the VCS names the
// common dir by, and an AGENTS.override.md holding a copy of AGENTS.md
// between <!-- itos:agents-md:begin --> and <!-- itos:agents-md:end -->,
// then the block. ReadStealth reads them, locating the repository through
// the VCS port only then; an AGENTS.override.md is itos's, and gets the
// copy, when it is missing, holds the copy's markers or holds nothing
// outside the rules block, and any other is the person's and only gains the
// block, as a CLAUDE.local.md always does. Plan computes each file's next
// text, so a rerun compares texts.
//
// CommitPolicy renders the loaded config (commits.types, the header lint,
// each footer's source and required_for with in_place_of as an alternative,
// commits.scopes with its $sets expanded) as itos guide commit prints it
// (slice 119), every config value through Code, a one-line code span fenced
// past any backtick it holds.
//
// The offer, writing the block and CLAUDE.md's import, and refusing a
// tracked file under a stealth config, are init's (internal/cli,
// initrules.go and initrulesstealth.go).
package agentrules
