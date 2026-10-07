// Package out is how a command reports under --json (itos --help): one object
// on stdout, "schema": 1 first and its keys in the order written (Emit; Go's
// maps would sort them), and the problems a check reports, each a sentence
// with a stable rule id and, where one exists, a fix an agent can act on.
//
// A key given twice keeps its first place and takes its last value, as
// JavaScript's { schema: 1, ...value } does, so a command adapter's listing
// that itos tests list --json prints key for key keeps its order. Logs go to
// stderr while --json holds stdout for the one object.
package out
