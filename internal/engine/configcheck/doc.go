// Package configcheck is the config check engine (ADR-0053, step 9 of
// docs/plans/slices-and-ports.md): what itos config check finds, which init
// lists as a doctor and the commit-msg hook judges over the index, so no
// slice owns it.
//
// Findings loads the config from the source the caller gives (the working
// tree or the index), through the VCS and tree ports it binds, and judges it,
// the ledger, the registry, the questions' file, each kind's smoke set and
// the decision records, each problem a Found in its area; its warnings never
// set its code, 2 when nothing past the config or the ledger's folder can be
// read, else 1 for any problem. The group label is judged against the flags
// of itos task the caller passes, since only the command line knows them.
// JudgedSmokeFindings is the smoke sets alone, of the kinds the hook picks.
// Printing, --json and the exit codes are the caller's.
package configcheck
