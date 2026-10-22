// Package guard is the guard command, a slice (ADR-0053): its CLI and Run
// are the command line, its Query and Handler the application, and its
// domain (internal/slices/guard/domain) what the guard judges.
//
// internal/cli's flag spec reads the line and hands Parse the arguments
// after guard, with the folder itos was started in, before a run from a
// subfolder moved to the top. Run reads Claude Code's input, makes that
// folder absolute and builds the Query, the one copy of the line's values;
// Handle asks the domain for the deny's reason, the domain asking whether
// each call's folder is managed through Ports, which the slice owns and
// internal/cli binds to config.Managed. Run never returns an error, since a
// usage error's exit 2 is what Claude Code takes as a block: an input the
// domain cannot read, a folder it cannot tell or an answer it cannot write
// is one stderr line and exit 1. launch.GuardSince, the first itos with the
// command, is the launcher's, which reads it.
package guard
