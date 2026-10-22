// Package gocmd is the go command (slice 64, slice 67, slice 68,
// features/guide.feature), a slice (ADR-0053): the coordinator's guide, the
// same as itos guide coordinate, then where things stand, so a session
// starts from one command. Its CLI and Run are the command line, its
// Handler the application, over the Ports it owns, which internal/cli
// binds: the repository, the trees and the working tree, where the config
// is found and read, and Standing, itos status's computation.
//
// Command (command.go) declares its name and help; it takes no arguments,
// and any is a usage error. The guide is followed by the repository's own
// notes and this clone's own (internal/engine/guides' Compose), then, after
// a second line of ---, by where things stand, under --json as the status
// key; a config not there or not read, or a registry with problems, is one
// line on stderr instead, said before the notes' warnings. It writes
// nothing; outside a repository the guide prints alone.
package gocmd
