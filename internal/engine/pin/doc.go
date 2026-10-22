// Package pin is the pin engine (ADR-0053, step 7 of
// docs/plans/slices-and-ports.md): the release a project pins and the
// install script a release's notes write, which itos pin, upgrade, init and
// status share.
//
// ReadConfig reads the config file the caller names (the stealth one
// included) and the pin it holds, as written; Config.Replaced is whether it
// pins a version with other checksums than its checksums.txt has now (the
// release changed after it was pinned, docs/decisions/0025), and
// Config.Moved its text with pin.version and pin.checksums set, every other
// byte as it was (value.SetScalars). Pinned asks internal/release for the
// version's checksums.txt, or the newest's, read from its archive names,
// within Timeout; its errors have their kind: a server that cannot be
// reached or fails kind.Temporary, a release it does not have or cannot be
// pinned kind.Missing. WriteKeepingMode writes a file keeping its
// permissions.
//
// ReadInstallScript reads tools/bin/install-itos (InstallScript) from the
// working directory, the repository's top; ScriptVersion is the version its
// one version= line (ScriptVersionLine) names. VersionArg reads the one
// <version> pin and upgrade take, its errors out.Usage; the exit codes are
// the commands'.
package pin
