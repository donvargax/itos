// Package version is the version command, a slice (ADR-0053) with no domain
// of its own: its CLI and Run are the command line, its Query and Handler
// the application.
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli reads the line and hands Parse the arguments
// after version, with the global --json. Run builds the Query, the one copy
// of the line's values; Handle reads the version and the config's requires
// through Ports, which the slice owns and internal/cli binds to
// internal/version and internal/config. Its --json object goes through
// internal/out.
package version
