package brief

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "brief",
	Rank: 25,
	Lines: `  brief <id> --role <role>         the filled brief an agent starts from; --json its request
`,
	Help: map[string]string{
		"": `Usage: itos brief <id> --role drafter|implementer|reviewer [--at <rev>]

Prints the brief an agent of the role starts from: the role's template
filled from the item at HEAD, at the commit --at names, or, for the reviewer
and the implementer under push.mode pr, at the item's branch (below). Filled in: the item's id, kind and title, the
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
For a slice or a bug, the implementer's template is itos guide brief's and the
drafter's and the reviewer's ship beside it; a task's text for each role is its
own, the item's kind as next reads it choosing. A drafter is told the commit its
spec goes in under, by the config it reads: the flag the config has, --item for
a footer of the work registry, else --task for a task and a ledger footer, else
plain itos commit; and the type, docs where it may carry the spec's files, else
the first of commits.types that may and that the registry's footer stands in
for, or, with no such footer, for a task, that its ledger footer is required
for. A type may carry them when commits.types has it, commit check-paths' rules
let it touch them and the hook's footer rules call nothing missing, the staged
range commands not judged. The spec's files are the tagged scenarios' or a task's
ledger file, else a feature file under the first Gherkin kind's root or the file
of the task's phase. Where no type may, or a slice or a bug has no such kind, the
brief says so and tells the drafter to stop and report, exit 0; with no commits
section it says docs and plain itos commit. Everything is read from HEAD's
tree, the config included, never the working tree or the index; with --at <rev>, from the
tree of the commit git resolves rev to instead (the registry, the tagged
scenarios, the ledger, the reads' blobs and the notes). With no --at, under
the push.mode pr of HEAD's config, a reviewer's or an implementer's brief is
the one --at origin/<id> gives when the clone has that ref, nothing fetched,
and under HEAD's config it holds a scenario tagged for a slice or a bug, or a
ledger entry for a task, whatever its checks, as itos next reads the spec, and
HEAD's registry has the item neither done nor dropped; a ledger the ref cannot
give is an error, never HEAD's answer. A drafter's stays at HEAD, as does every
role's outside push.mode pr. It reads, writes nothing, commits nothing and asks
no forge.

Refused, exit 1: an id the registry does not have, a role it does not know,
an idea (promote it first), for the implementer and the reviewer an item
nothing is tagged for yet (brief the drafter, whose spec commit is null), and
an --at git cannot resolve to a commit (brief-unknown-revision). A missing
--role is a usage error, exit 2.

--json: {"schema":1,"ok":true,"item","role","request","base","spec":{"commit"},"reads":[{"path","blob","reason","required"}],"omitted":[{"path","reason"}],"brief"},
brief the text output; base the full SHA of the commit it was computed at;
spec.commit the last commit on base's first-parent history touching the
spec's files, null when it has none; each read's blob its git blob id at
base, required true for AGENTS.md and the spec's files;
request "sha256:" and the hex digest of these LF-terminated lines: the item,
the role, the spec commit's SHA (or "-"), then "<path> <blob>" per read in
order, so the same inputs give the same request and any change a new one.
Refused: {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Value("--role"), declare.Value("--at")}},
}
