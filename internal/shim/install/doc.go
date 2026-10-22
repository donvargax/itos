// Package install is where the git shim's link goes and what stands there
// (ADR-0053, step 7 of docs/plans/slices-and-ports.md), which itos git-shim
// install and uninstall and itos init's offer of the shim share
// (internal/cli's gitshim.go and initshim.go): the shim's other half,
// internal/shim, is what the link runs.
//
// Place is the running itos (os.Executable, which the caller gives) and the
// link's path in a
// folder, by default the one holding that itos, made absolute, named git, or
// git.exe on the OS windows (Name). LinksItos is what a git there is: this
// itos, its links followed (the git adapter's IsSelf), or a symbolic link to
// another binary named itos, as an older install's. MakeLink makes the
// symbolic link, replacing the one there when told to, and on windows, where
// symbolic links need a privilege, a hard link where that fails.
// PathStanding places the link's folder and the real git's (the git
// adapter's Real) among the PATH's folders by os.SameFile, and StandingLine
// says where that leaves the shim. Reading the arguments, the messages of a
// refusal and the exit codes are internal/cli's.
//
// It is a package of its own, not internal/shim's, while internal/launch
// imports internal/cli: internal/shim imports internal/launch, so
// internal/cli importing it would close a cycle.
package install
