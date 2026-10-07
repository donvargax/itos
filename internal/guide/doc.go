// Package guide holds the guides a session starts from, embedded in the
// binary with go:embed so that one text, versioned with itos, serves every
// repository it manages and the binary needs no file of its own at run time:
// coordinate.md, the coordinator's, which itos go (and itos guide
// coordinate) prints, and work.md, the implementer's, which itos guide work
// prints. They are printed as written.
//
// They are generic by rule: what one repository learns that holds for no
// other stays in that repository's own notes, guide.orchestrating in the
// config (docs/ORCHESTRATING.md by default), which itos go appends after a
// line of ---, and what holds on one machine only in the clone's own
// notes.md in itos's folder of the git common dir, which itos go alone
// appends after another. internal/cli/guide.go reads both; a config or notes
// that cannot be read are a warning on stderr, never a failure, since the
// guide is what a session starts from.
package guide
