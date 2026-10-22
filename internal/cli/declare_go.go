package cli

import "github.com/donvargax/itos/v8/internal/declare"

var goDeclared = declare.Command{
	Name: "go",
	Rank: 10,
	Lines: `  go                               the coordinator's guide, the repository's own notes, the status
`,
	Help: map[string]string{
		"": `Usage: itos go

Prints the coordinator's guide, the one shipped in itos, the same in every
repository: what the session that coordinates the work needs before any
action, from reading where things stand to handing one item at a time to an
implementing agent, and the map of the procedures it reads before each step
(itos guide brief, land, repair and decide). A session that coordinates starts
with it (! itos go). Then, after a line of ---, the repository's own notes,
when it keeps them: the file guide.orchestrating names (docs/ORCHESTRATING.md
by default), read from the repository's top, or in the git folder's itos/ for
a stealth config. Then, after another line of ---, this clone's own notes,
when it keeps them: .git/itos/notes.md (itos/notes.md in the git common dir,
which every linked worktree shares and git never commits), for what is true on
this machine only, under the heading "# This clone's own notes"; printed once,
as the repository's notes, when guide.orchestrating names that same file.
Last, after another line of ---, where things stand, as itos status prints it
for the person the identity provider names; with no config, or no sound work
registry, a line on stderr says it is left out, and the guide still prints. It
writes nothing; outside a repository the guide prints alone. itos guide
coordinate prints the guide and the repository's notes alone.

--json: {"schema":1,"guide":"coordinate","text","notes"?,"local_notes"?,
"status"?}, notes the file appended, local_notes this clone's own notes' file,
status the object itos status --json prints, but its schema`,
	},
}
