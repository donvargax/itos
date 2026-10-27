// Package followup is the followup command, a slice (ADR-0053): the
// person's private threads with people (slice 61, features/follow.feature).
// Its CLI and Run are the command line, its Query and Handler the
// application, and its domain (internal/slices/followup/domain) the threads
// and their file.
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli reads the line and hands Parse the arguments
// after followup, with the folder itos was started in, which a relative doc
// path is read from; every error Parse gives is a usage error. Handle reaches
// the world through Ports, which the slice owns and internal/cli binds: the
// clone's private store (internal/store/private/port, its adapter behind it)
// and the clock, read once a run. The threads are domain.FileName in the
// store; no config is read and nothing committed, so any git repository will
// do, and none is exit 3. A query that changes them holds the file's lock
// from before the read to after the write (bug 16). followup doc writes the
// Markdown where the person typed, 0600 in folders made 0700, refuses a file
// already there without --force, records the absolute path in the thread,
// and warns when the store says git add could commit it; doc - prints the
// Markdown and writes nothing. A refusal is a problem, exit 1. A file the
// domain refuses is domain.Invalid, which internal/cli reports as a usage
// error's exit 2; the slice reads the file through value.CheckWritten before
// the domain parses it, so a null key or list item is refused too (bug 152).
package followup
