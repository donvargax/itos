package decision

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "decision",
	Rank: 60,
	Lines: `  decision [--all]                 the open decisions waiting on the person the work is for, and the provisional answers awaiting them
  decision add <text> [--item <id>] [--class <class>]
                                   ask a question, and commit it
  decision answer <id> <text> [--provisional --by <handle>]
                                   answer a question, and commit it
  decision ratify <id>             make a provisional answer the decision, and commit it
  decision overturn <id> <text>    replace a provisional answer with the person's, and commit it
  decision record <id> --title <title> [--option <text>]… [--consequences <text>] [--supersedes <n>] [--none]
                                   write an answer as a decision record, and commit it
  decision show <id>               a question and its answer
`,
	Help: map[string]string{
		"": `Usage: itos decision [--all]
       itos decision add <text> [--item <id>] [--class <class>]
       itos decision answer <id> <text> [--provisional --by <handle>]
       itos decision ratify <id>
       itos decision overturn <id> <text>
       itos decision record <id> --title <title> [--option <text>]… [--consequences <text>] [--supersedes <n>]
       itos decision record <id> --none
       itos decision show <id>

The decisions waiting on the person the work is for, each asked as a
question only they can answer; anything to take up with someone else, a
question included, is itos followup. They are kept as data beside the
work registry rather than by hand: in work.asks, by default asks.yaml in the
registry's folder (tasks/asks.yaml where the ledger is in tasks/). They are
public, unlike itos followup's threads: add and answer write the file and
commit it alone, as work take commits the registry (itos help work take). A
question has an id, q-1, q-2 and so on, one past the highest the file holds,
answered or not, so an id is never given twice; its text; the registry item it
holds up, when --item names one; its class, when --class gives one; and its
answer, once given, kept beside it.
Under a stealth config the file is in the git folder beside the stealth
registry, written while the registry's lock is held, as every stealth writer
holds it, and nothing is committed.

A project may delegate decision classes to an answering agent: work.delegated
lists them, free names, none by default. A question of a listed class may be
answered provisionally in the person's place (answer --provisional --by
<handle>): work proceeds on the answer, which the person ratifies (ratify) or
overturns (overturn) in their own time. A question with no class, and a
sign-off, is the person's whatever the config lists.

An answered question is a decision: itos decision record writes it as an
architecture decision record, notes the record's number on the question and
commits both (itos help decision record), or with --none marks an answer that
concerned its item alone, recorded nowhere. A provisional answer is no
decision yet: record refuses it until it is ratified.

itos decision lists the open questions, each with the item it names, then the
provisional answers awaiting the person, oldest first, each with its class and
who gave it, then one
line naming the answered questions recorded nowhere yet, each as itos decision
record <id>, when there are any; --all adds the answered ones after the open
ones, each with its answer and its decision. Exit 1 for a refusal (no question with
the id, an item the registry does not have), 2 for a usage error or an
asks.yaml itos cannot read or did not write whole (a key it does not know, an
id not q-<n> or given twice, a decision neither a number nor none, a second
YAML document, an empty file), which it leaves as it is.

--json: {"schema":1,"questions":[{"id","item"?,"question","status","answer"?,"class"?,"by"?,"overturned"?,"decision"?}],"provisional":[{…,"provisional":true}]},
status open or answered, decision the record's number or "none", by who gave
a provisional answer, ratified or overturned since or not, overturned the
answer the person's replaced; questions leaves out the provisional answers,
which provisional holds; each
subcommand's in its help (itos help decision <subcommand>)`,

		"add": `Usage: itos decision add <text>… [--item <id>] [--class <class>]

Asks a question, open, with the next free id: the words before or after
--item joined by spaces, so the text needs no quotes unless the shell would
read it; after "--" a word that starts with "-" is text too. --item names the
registry item the question holds up. --class gives the question its decision
class, a free name, kept beside it; an answering agent may settle it
provisionally when work.delegated lists the class (itos help decision answer),
and without a class the question is the person's. Then commits the questions'
file alone,
"docs: ask q-<n>", through the hooks, leaving whatever else is staged staged.
Refused, nothing written (exit 1): an item the registry does not have, no
registry where itos looks, a file with changes no commit holds, a commit a
hook refuses (the file put back). Under a stealth config the file is written
and nothing committed.

--json: {"schema":1,"ok":true,"question":{"id","item"?,"question","status","answer"?,"class"?},"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"answer": `Usage: itos decision answer <id> <text>… [--provisional --by <handle>]

Answers the question: the words after the id joined by spaces are kept beside
it, and it is answered, left out of itos decision's list. Then commits the
questions' file alone, "docs: answer <id>", as itos decision add does. Refused,
nothing written (exit 1): no question with the id, a question already
answered, a file with changes no commit holds, a commit a hook refuses. Under
a stealth config the file is written and nothing committed.

--provisional --by <handle> answers in the person's place, as an answering
agent does: the answer is kept with provisional: true and who gave it, the
question counts as answered wherever the open questions are read, so work
proceeds on it, and it is listed among the provisional answers until the
person ratifies it (itos decision ratify) or overturns it (itos decision
overturn). The two flags go together: one without the other is a usage error
(exit 2). Refused (exit 1, ask-not-delegated): a question with no class, or
of a class work.delegated does not list, which only the person answers. A
question answered provisionally is answered: a plain answer is refused
(ask-answered), its fix naming ratify and overturn.

--json: as itos decision add's, the question as it is after, with "class"?,
"provisional"? (true) and "by"?`,

		"ratify": `Usage: itos decision ratify <id>

The person makes a provisional answer the decision: provisional is cleared,
the class and who answered kept, so itos decision stops listing it among the
provisional answers and names it as a decision to record (itos decision
record). Then commits the questions' file alone, "docs: ratify <id>", as itos
decision add does. Refused, nothing written (exit 1, ask-not-provisional): a
question with no provisional answer, open or answered by the person, since
no command replaces what the person settled; no question with the id; a
file with changes no commit holds; a commit a hook refuses. Under a stealth
config the file is written and nothing committed.

--json: as itos decision add's, the question as it is after`,

		"overturn": `Usage: itos decision overturn <id> <text>…

The person replaces a provisional answer with their own: the words after the
id joined by spaces are the answer, the agent's answer is kept beside it as
overturned, with who gave it, and provisional is cleared, so the question is
answered for good. Then commits the questions' file alone, "docs: overturn
<id>", as itos decision add does. Refused, nothing written (exit 1,
ask-not-provisional): a question with no provisional answer, open or answered
by the person, since no command replaces what the person settled; no question
with the id; a file with changes no commit holds; a commit a hook refuses.
Under a stealth config the file is written and nothing committed.

--json: as itos decision add's, the question as it is after, with
"overturned"`,

		"record": `Usage: itos decision record <id> --title <title> [--option <text>]… [--consequences <text>] [--supersedes <n>]
       itos decision record <id> --none

Writes the answered question as the next architecture decision record, in
MADR 4's format (adr/madr, its bare-minimal sections): NNNN-<slug>.md, the
slug the title lowercased with each run of other characters one dash. Its
YAML frontmatter holds "status: accepted" and the date it is recorded; then
"# <title>"; "## Context and Problem Statement", the question and "Asked as
<id>" (with the item it names); "## Considered Options", a bullet for each
--option given, else "The options are those the question names."; "## Decision
Outcome", the answer; and "### Consequences", --consequences, else "None
recorded.". The records' folder is the one work.decisions names, by default
docs/decisions; the number is one past the highest any name there starts
with. The folder's README.md is the index of the records whose status is
accepted, by number and title, between <!-- itos:decisions:begin --> and
<!-- itos:decisions:end -->, made when missing, its text outside them kept. A
record is read by its frontmatter and headings, so one edited by hand reads
the same. --supersedes <n> sets record <n>'s status to "superseded by
ADR-<nnnn>", the new record's number in four digits, so it leaves the index,
and the new record ends with "## More Information", "Supersedes ADR-<nnnn>.".
The question gains "decision: <n>". Then commits the questions' file, the
record, the one it supersedes and the index together, "docs: record <id> as
decision <n>", as itos decision add commits.

--none writes no record: the answer concerned its item alone, and the
question gains "decision: none", so itos decision stops naming it; the
questions' file is committed alone, "docs: mark <id> as recorded nowhere".

Refused, nothing written (exit 1): no question with the id, one not answered,
one answered provisionally and not yet ratified (ask-provisional),
one already recorded or marked, --supersedes naming no record in the folder,
a file with changes no commit holds, a commit a hook refuses (the files put
back). Under a stealth config the records are in the git folder beside the
stealth config, decisions/ unless its work.decisions names another, and
nothing is committed.

--json: {"schema":1,"ok":true,"question":{…,"decision"},"record":"<path>"|null,"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"show": `Usage: itos decision show <id>

Prints the question: its id and text, whether it is open or answered, the
item it names, its class, how its answer stands with the person (provisional,
ratified or overturned, and by whom) and its decision, then its answer when
it has one, and the answer the person overturned. It reads, never writes.
Exit 1 when no question has the id.

--json: {"schema":1,"ok":true,"question":{"id","item"?,"question","status","answer"?,"class"?,"provisional"?,"by"?,"overturned"?,"decision"?}}`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Switch("--all")}, Subs: map[string]declare.Spec{
		"add":      {Flags: []declare.Flag{declare.Value("--item"), declare.Value("--class")}},
		"answer":   {Flags: []declare.Flag{declare.Switch("--provisional"), declare.Value("--by")}},
		"ratify":   {},
		"overturn": {},
		"record": {Flags: []declare.Flag{declare.Value("--title"), declare.Values("--option"), declare.Value("--consequences"), declare.Value("--supersedes"),
			declare.Switch("--none")}},
		"show": {},
	}},
}
