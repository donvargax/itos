// Package pin is the pin command, a slice (ADR-0053) with no domain of its
// own, its engine internal/engine/pin (imported as pinengine): its CLI and
// Run are the command line, its Query and Handler the application (slice
// 47, features/pin.feature).
//
// Command (command.go) declares its name and help, by which internal/cli
// hands Parse the arguments after pin, with the global --json; Parse reads
// the one <version>, the newest by default or with latest (slice 87).
// Handle reads the config itos finds, the stealth one included, and asks
// for the release's checksums.txt, the version's or the newest's with the
// version read from its archive names (asked whatever CI, ITOS_NO_UPDATE or
// the launcher's daily state say, and not written to that state, within 30
// seconds), through Ports, which the slice owns and internal/cli binds to
// the engine. It writes pin.version and that file's SHA-256 in the config,
// changing those values' bytes alone (value.SetScalars), and commits
// nothing: the bump is the project's own commit. Nothing is written when
// the fetch fails (exit 3 for a release the server does not have, 75 for
// one that cannot be reached), when the pin is already that version with
// those checksums (exit 0), or when it is that version with other valid
// checksums (exit 1: the release changed after it was pinned, and
// re-pinning it quietly would defeat the pin). Like git-shim install it is
// the launcher's own command, run by the binary called whatever the pin
// says (internal/launch), since the version pinned may predate it.
package pin
