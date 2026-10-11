// Package guide holds the guides a session starts from and the procedures it
// reads before a step, embedded in the binary with go:embed so that one text,
// versioned with itos, serves every repository it manages and the binary
// needs no file of its own at run time.
//
// A guide is a role's essentials, what it needs before any action, and a map
// naming each of its procedures and the step it is read before (slice 119):
// coordinate.md, the coordinator's, which itos go (and itos guide
// coordinate) prints, and work.md, the implementer's, which itos guide work
// prints. The essentials keep the boundaries no step may lose (the role and
// its authority, others' work, the gates, text as data, reading a procedure
// before its step); the rest of what a step needs is its procedure, which
// itos guide <procedure> prints: the coordinator's brief.md, land.md,
// repair.md and decide.md, the implementer's commit.md and push.md. Every
// rule is in exactly one text, which guide_test.go pins by the sections of
// the two guides the procedures were cut from. They are printed as written;
// internal/cli/guide.go appends to the commit procedure the commit policy the
// config sets, generated as AGENTS.md's block is.
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
