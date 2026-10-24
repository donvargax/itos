// Package gitshim is the git-shim command, a slice (ADR-0053) with no domain
// of its own: its CLI and Run are the command line, its Query and Handler
// the application (slice 41, features/shim.feature).
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli hands Parse the arguments after git-shim, with the folder
// itos was started in, which --dir is read from as the person typed it.
// install links the itos binary running as git (git.exe on windows) in that
// folder, by default the one holding it, a symbolic link or on windows a
// hard link where that fails, and says where the folder stands on the PATH
// against the real git, since the shim only acts before it; uninstall takes
// the link away. A git there that is this binary is kept, another itos (a
// link to one or a copy of it, shim.LinksItos) replaced, anything else
// refused (exit 1): neither replaces nor removes a git that is not itos. The link's
// place, what a git there links, the link's making and the folder's place
// among the PATH's and the real git's (the git adapter's Real, compared by
// os.SameFile) are internal/shim's, reached through Ports, which the slice
// owns and internal/cli binds for the OS itos runs on.
//
// run is what the link runs in a repository itos manages (internal/shim):
// git commit or git push as itos commit or itos push, with git's arguments,
// which the shim writes after a "--", so none of them is read as an itos
// global flag; Ports binds them to those commands in internal/cli.
package gitshim
