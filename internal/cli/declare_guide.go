package cli

import "github.com/donvargax/itos/v8/internal/declare"

var guideDeclared = declare.Command{
	Name: "guide",
	Rank: 20,
	Lines: `  guide <name>                     a guide (coordinate, work) or the procedure read before a step
`,
	Help: map[string]string{
		"": `Usage: itos guide coordinate|work
       itos guide brief|land|repair|decide|commit|push

Prints a guide or a procedure shipped in itos, as written, the same in every
repository. A guide is a role's essentials and the map of its procedures:
coordinate, the coordinator's, as itos go prints it, with the repository's own
notes after it but not the status; work, the implementer's, which a
coordinator asks each agent it starts to run first. A procedure is what a role
reads before one step, as its guide's map says: the coordinator's brief (how
to specify an item, then the brief to fill in), land (pushing, waiting on CI
and checking a result), repair (a red main branch, a problem itos itself
causes) and decide (asking the person, the queue, decisions and lessons); the
implementer's commit (how to commit through itos, then, where there is a
config, the commit types, footers and paths it sets) and push (the push routine and what the gates do).
A procedure takes no argument. It needs no config and writes nothing; a config
that cannot be read is a line on stderr, and commit prints without its rules.
A name it has no guide or procedure by is a usage error (exit 2) naming the
guides, then the procedures.

--json: {"schema":1,"guide","text","notes"?}, guide the name printed, notes
the file appended after the coordinator's guide`,
	},
}
