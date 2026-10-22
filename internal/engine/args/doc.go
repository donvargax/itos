// Package args is the argument-parsing engine (ADR-0053, step 9 of
// docs/plans/slices-and-ports.md): how a command reads the arguments after
// its name, after Main's readLine has judged them against the command's
// spec and written every flag --flag value, which every command shares and
// a slice's Parse calls.
//
// Split is a command's subcommand and the arguments after it, First a
// list's first element, Positional the arguments that are neither flags nor
// a valued flag's value, FlagValue the value after a flag. Sub reads a
// subcommand's arguments stricter, as a person may write them (--flag=value
// too), and refuses what the subcommand does not take with out.Usage. It
// reads no file and reaches no port.
package args
