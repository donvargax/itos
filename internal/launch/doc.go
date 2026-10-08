// Package launch is the launcher every run of itos passes through first: it
// picks the version of itos to run and, when that is not the binary that was
// called, fetches that version's release into a cache, checks it and runs it
// with the same arguments, handing back its exit code
// (docs/decisions/0024-a-global-itos-is-a-launcher-that-runs-the-version-a-repository-pins.md,
// features/pin.feature). The binary that was called is never rewritten.
//
// # Choosing the version
//
// cmd/itos calls Main, and only when it hands the run back cli.Main; both
// get the arguments Args gives: a first argument --version is the command
// version, so a pinned release, even one older than --version, is handed
// "version", which every release answers; a --version anywhere else stays
// what it was.
//
// The version to run is ITOS_VERSION when it is set, else the config's
// pin.version. A binary whose own version is the one to run runs itself, so
// the version the launcher runs, which it tells by ITOS_VERSION, never
// launches again. readConfig reads the config where the command line does
// (--config, else config.Locate under --root, or with neither under the top
// config.Top finds from a subfolder: ITOS_CONFIG, itos.yaml or the stealth
// config), from the working tree, and only its pin, so a config written for
// a newer itos still reaches the version it pins. choose tells three states
// apart:
//
//   - pinned: the pin runs, after notice (below).
//   - absent: no file there at all, or the stealth config with no pin key
//     (one person's itos in a repository that does not use it): the newest
//     release the launcher knows of runs (newest, below).
//   - unpinned: a project's config with no pin, one it cannot read, or one
//     whose pin fails config.PinVersion and config.PinChecksums: the binary
//     that was called runs, and reports what is wrong with the config, if
//     anything.
//
// # Fetching a release
//
// ensure finds <cache>/<version>/itos (ITOS_CACHE, else itos/ under
// os.UserCacheDir), cached when present and, under a pin, when the
// checksums.txt beside it hashes to pin.checksums. Else fetch gets
// <base>/download/v<version>/checksums.txt (internal/release), holds it to
// the pin (an ITOS_VERSION the config does not pin trusts it as fetched),
// takes the platform archive's line (itos-<version>-<os>-<arch>.tar.gz, .zip
// on windows), fetches and checks the archive, extracts the binary at its top
// and moves it with that checksums.txt into the cache as one folder, written
// beside it first, so nothing unverified is ever left in the cache. Two
// first runs of a release fetch it at once (bug 44): the folder goes into
// place only where there is none, so a run that finds the release cached by
// another meanwhile runs that one and never removes what the other is about
// to exec (nor could it on windows, where a running itos.exe can be neither
// removed nor replaced); only a folder checked against another checksums.txt
// is moved aside for it. A release server that cannot be reached or fails
// exits 75, a failure that may pass when run again; anything else that cannot
// be fetched or does not match exits 3, a missing environment; each with one
// line naming what failed, and nothing runs in its place. run is syscall.Exec on unix, so the
// version run owns the process, its signals and its exit code, after writing
// the process's coverage counters to GOCOVERDIR when it is set, since an exec
// runs no exit hook to write them (T-124); elsewhere a child whose exit code
// is passed back.
//
// # Keeping to the newest release
//
// update.go hangs off two branches of choose (features/update.feature,
// features/stealth.feature). absent calls newest, which runs the newest of
// the binary's own version and the stable releases the cache holds
// (cachedVersions: the <version> folders whose binary is there, pre-releases
// left out, as GitHub's latest release is never one), after installing the
// release the server announces when the cache lacks it, checked against the
// announced checksums.txt. pinned calls notice before the pin runs, even
// when the pin is this binary, and before it, when the command is version,
// pinLine: one stderr line naming the pin and this binary's version,
// unthrottled, since the version line on stdout is the pin's.
//
// Both read announced: the newest version the server named, kept with the
// time it was had in <cache>/state/latest ("<unix seconds> <version>"), and
// asked for again (<base>/latest/download/checksums.txt, its version read
// from the archive names, within askTimeout, three seconds) only when that
// is an hour old (answerHolds) and the run may ask (CI and ITOS_NO_UPDATE
// both unset). A question with no answer is written as asked, so an offline
// machine pays the timeout once an hour. notice compares with the pin
// (version.Compare) the newest of the announced version, the stable releases
// the cache holds and this binary's version when it is a release (bug 48: the
// answer alone can be behind what the cache already holds), and says it once
// a day per repository, keyed by the
// config's absolute path in <cache>/state/notice-<hash>, written before the
// line is said so a cache it cannot write to stays silent rather than saying
// it every run. Nothing here is ever an error: every failure falls through
// to what the cache has. A binary built without a version
// (version.Unstamped) runs itself where there is no config.
//
// # What the launcher leaves to the binary called
//
// binaryCommand leaves to the binary that was called the commands that must
// not run a pinned version: git-shim install and uninstall, which link it as
// git; pin, which moves the pin and may be newer than the version pinned;
// upgrade, which moves it too; and init, which writes the config and its pin
// where there is none. The git shim's own runs (git-shim run) are launched
// as any other, so in a pinned repository git commit is the pinned itos's
// when it has the shim (Handed tells internal/shim the version).
//
// guard claude-code is handed on as any other run, but not to an itos older
// than it (cli.GuardSince), which has no such command: its usage error's
// exit 2 would make Claude Code block the tool, so unguarded catches it, by
// any of choose's branches, and answerNothing reads stdin to its end (not a
// terminal's), says why in one stderr line and exits 0 with nothing on
// stdout, fetching nothing. An ITOS_VERSION that is no version is handed on,
// to fail as for any command.
package launch
