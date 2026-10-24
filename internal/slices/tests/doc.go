// Package tests is the tests command, a slice (ADR-0053) with no domain of
// its own: its CLI and Run are the command line, its Query and Handler the
// application, over internal/tests (imported as namedtests), which knows
// the kinds, their adapters, the smoke set and the moves rule.
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli reads the line and hands Parse the arguments after tests,
// with the folder the person stood in, which smoke check's --features is
// read from; every error Parse gives is a usage error. tests list, tests
// smoke check, ids and run, and tests moves are calls into internal/tests,
// reading only the working tree but for list's --at and the moves rule's
// index and HEAD; list's --at commit is checked to be one (the refs
// engine) before the config is read, list --tsv (refused beside --json,
// before anything is read) projects its live tests through namedtests.TSV
// from the folder itos runs in, and smoke run runs the kind's select
// template over the smoke IDs through the Shell port, its exit the
// runner's. tests next-id reads the next free tag (namedtests.NextTag),
// counting the registry's ids (the ids engine's Registry); of an area
// (namedtests.Area) it then claims what it prints through the ids engine's
// Claim and the tracker's Reserve, which item ids are minted through too,
// the area's stem its series; a bare stem that would be an area with ID- in
// front (namedtests.Mistaken: SEC for ID-SEC) is a usage error naming the
// command that claims, the counter untouched. Handle reaches the world through Ports,
// which the slice owns and internal/cli binds: the VCS, the config as the
// working tree has it, the tracker store and the config's shell with
// itos's streams.
package tests
