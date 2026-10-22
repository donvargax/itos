// Package declare is what a command declares itself by (ADR-0053, step 5 of
// docs/plans/slices-and-ports.md): its name, its place in itos --help, its
// lines in the Commands list, its --help pages and the spec its flags are
// read by. A command declares one Command in its own file, a slice in its
// own package, and tools/bin/assemble writes internal/cli's command table
// from them, so a lane moving one command touches its own files and the
// generated one alone.
//
// It is a leaf: it imports nothing of ours, nor os or net (the declare-leaf
// rule of .golangci.yml), so a slice and internal/cli both import it with no
// cycle. A declaration is data alone: it holds no function, takes no context
// and reads nothing, so declaring a command costs help and version nothing.
// Where a spec needs the config, the flags its group label or its free-text
// footers add, it names them by More, which internal/cli's parser reads the
// config for (spec.go). The parser is internal/cli's (readLine), whose spec
// and flagSpec are this package's Spec and Flag; v9.0 swaps kong beneath the
// declarations.
package declare
