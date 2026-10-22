// Package config is the config command, a slice (ADR-0053) with no domain
// of its own: its CLI and Run are the command line, its Query and Handler
// the application, over internal/config (imported as itosconfig), which
// finds, loads and describes the config, and internal/engine/configcheck,
// which judges it and its data.
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli reads the line and hands Parse the arguments after config;
// every error Parse gives is a usage error, and --ledger is read from the
// folder itos was started in. config check is configcheck.Findings over the
// working tree, with the flags of itos task the TaskFlags port gives, which
// internal/cli binds to the flag spec it holds; --print-defaults prints
// config.DefaultsFor of the config, or of none when it does not load, and
// checks nothing. config get refuses a key the schema does not have before
// any config is read. Handle reaches the world through Ports, which the
// slice owns and internal/cli binds: the repository, the trees and the
// working tree.
package config
