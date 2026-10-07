// Package adr is the architecture decision records itos decision record
// writes (features/ask.feature): an answered question written as a record
// in MADR 4's format (adr/madr 4.0.0, its bare-minimal template), the
// maintained one under the adr organisation. A record is a Markdown file
// NNNN-slug.md (FileName, Slug): YAML frontmatter holding its status and
// date, which no section repeats; the title as "# Title", with no number;
// then the sections Context and Problem Statement, Considered Options,
// Decision Outcome and its Consequences, and More Information when it
// supersedes another (Text). A record superseded says so in its status,
// "superseded by ADR-NNNN", as MADR's template spells it (SetStatus
// replaces the frontmatter's status line, adds one, or adds frontmatter to a
// record with none). The folder is work.decisions, docs/decisions by
// default, and a new record's number is one past the highest any name there
// starts with (Next).
//
// The folder's README.md holds, between itos's markers, the index of the
// live records (Index): every record but those whose status says
// superseded, deprecated or rejected, so a record with no frontmatter, which
// MADR allows, is listed. The rest of the README is kept.
//
// A record is read by its structure, never its bytes: the status is the
// frontmatter's, parsed as YAML (Record.Status; Record.SupersededByNumber
// reads the number in any case), the title the first "# " heading after it,
// outside a code block, so a record a person or a formatter rewrote reads
// the same. Everything written here is in the form a Markdown formatter
// leaves alone.
//
// Config check holds the folder to two rules (Problems, area decisions): no
// two records share a number (decisions-number-twice, naming both files),
// and a record superseded names, in its status, a number a record in the
// folder has (decisions-superseded-by-missing). The index is not held to the
// folder, since itos decision record writes it whole at every record and
// nothing regenerates it on demand.
//
// The functions here are text in, text out, but for List and Next, which
// read the folder, List through internal/source so that the commit-msg hook
// reads the records as staged (a folder that is not there holds none);
// internal/cli/ask.go writes and commits.
package adr
