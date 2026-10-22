// Package guide is the guide command (slice 64, features/guide.feature), a
// slice (ADR-0053): it prints the guides embedded in the binary
// (internal/guide) as written, and the procedures a guide's map names
// (slice 119). Its CLI and Run are the command line, its Query and Handler
// the application, over the Ports it owns, which internal/cli binds: the
// repository, the trees and the working tree, where the config is found
// and read.
//
// Command (command.go) declares its name and help; every error Parse gives
// is a usage error naming the guides, then the procedures, before anything
// is read. The coordinator's guide (coordinate, as itos go prints it) is
// followed by the repository's own notes, and the commit procedure by the
// commit policy the config sets, both internal/engine/guides'; the config
// is read for those two alone, so it needs no config, and a config that
// cannot be read is a line on stderr. It writes nothing.
package guide
