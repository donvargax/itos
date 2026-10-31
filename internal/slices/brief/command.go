package brief

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "brief",
	Rank: 25,
	Lines: `  brief <id> --role <role>         the filled brief an agent starts from; --json its request
`,
	Help: map[string]string{
		"": `Usage: itos brief <id> --role drafter|implementer|reviewer

Prints the brief an agent of the role starts from: the role's template
filled from the item at HEAD. Filled in: the item's id, kind and title, the
repository's top as git prints it and its folder's name as the project, the
scenarios tagged for it and their feature files (a task's checks from its
ledger entry), what it depends on with each status, and what the agent reads:
AGENTS.md, the item's feature files (a task's ledger file), and for each of
its refs the file it names or the doc.go of the folder it names; a ref naming
neither, or a symbolic link, is left out and said so, as is a missing
AGENTS.md. The repository's
own brief lines, the "## What a brief adds here" section of the file
guide.orchestrating names (docs/ORCHESTRATING.md by default), follow as
written. What a person must add stays a marked placeholder: the settled
decisions, the neighbouring scenarios and checks, the item's own questions.
The implementer's template is itos guide brief's; the drafter's and the
reviewer's ship beside it. Everything is read from HEAD's tree, the config
included, never the working tree or the index. It reads, writes nothing,
commits nothing and asks no forge.

Refused, exit 1: an id the registry does not have, a role it does not know,
an idea (promote it first), and for the implementer and the reviewer an item
nothing is tagged for yet (brief the drafter, whose spec commit is null). A
missing --role is a usage error, exit 2.

--json: {"schema":1,"ok":true,"item","role","request","spec":{"commit"},"reads":[{"path","blob","reason","required"}],"omitted":[{"path","reason"}],"brief"},
brief the text output; spec.commit the last commit on HEAD's first-parent
history touching the spec's files, null when it has none; each read's blob
its git blob id at HEAD, required true for AGENTS.md and the spec's files;
request "sha256:" and the hex digest of these LF-terminated lines: the item,
the role, the spec commit's SHA (or "-"), then "<path> <blob>" per read in
order, so the same inputs give the same request and any change a new one.
Refused: {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Value("--role")}},
}
