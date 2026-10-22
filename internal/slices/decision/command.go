package decision

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "decision",
	Rank: 60,
	Lines: `  decision [--all]                 the open decisions waiting on the person the work is for
  decision add <text> [--item <id>]
                                   ask a question, and commit it
  decision answer <id> <text>      answer a question, and commit it
  decision record <id> --title <title> [--option <text>]… [--consequences <text>] [--supersedes <n>] [--none]
                                   write an answer as a decision record, and commit it
  decision show <id>               a question and its answer
`,
	Help: map[string]string{
		"": `Usage: itos decision [--all]
       itos decision add <text> [--item <id>]
       itos decision answer <id> <text>
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
holds up, when --item names one; and its answer, once given, kept beside it.
Under a stealth config the file is in the git folder beside the stealth
registry, written while the registry's lock is held, as every stealth writer
holds it, and nothing is committed.

An answered question is a decision: itos decision record writes it as an
architecture decision record, notes the record's number on the question and
commits both (itos help decision record), or with --none marks an answer that
concerned its item alone, recorded nowhere.

itos decision lists the open questions, each with the item it names, then one
line naming the answered questions recorded nowhere yet, each as itos decision
record <id>, when there are any; --all adds the answered ones after the open
ones, each with its answer and its decision. Exit 1 for a refusal (no question with
the id, an item the registry does not have), 2 for a usage error or an
asks.yaml itos cannot read or did not write whole (a key it does not know, an
id not q-<n> or given twice, a decision neither a number nor none, a second
YAML document, an empty file), which it leaves as it is.

--json: {"schema":1,"questions":[{"id","item"?,"question","status","answer"?,"decision"?}]},
status open or answered, decision the record's number or "none"; each
subcommand's in its help (itos help decision <subcommand>)`,

		"add": `Usage: itos decision add <text>… [--item <id>]

Asks a question, open, with the next free id: the words before or after
--item joined by spaces, so the text needs no quotes unless the shell would
read it; after "--" a word that starts with "-" is text too. --item names the
registry item the question holds up. Then commits the questions' file alone,
"docs: ask q-<n>", through the hooks, leaving whatever else is staged staged.
Refused, nothing written (exit 1): an item the registry does not have, no
registry where itos looks, a file with changes no commit holds, a commit a
hook refuses (the file put back). Under a stealth config the file is written
and nothing committed.

--json: {"schema":1,"ok":true,"question":{"id","item"?,"question","status","answer"?},"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"answer": `Usage: itos decision answer <id> <text>…

Answers the question: the words after the id joined by spaces are kept beside
it, and it is answered, left out of itos decision's list. Then commits the
questions' file alone, "docs: answer <id>", as itos decision add does. Refused,
nothing written (exit 1): no question with the id, a question already
answered, a file with changes no commit holds, a commit a hook refuses. Under
a stealth config the file is written and nothing committed.

--json: as itos decision add's, the question as it is after`,

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
one already recorded or marked, --supersedes naming no record in the folder,
a file with changes no commit holds, a commit a hook refuses (the files put
back). Under a stealth config the records are in the git folder beside the
stealth config, decisions/ unless its work.decisions names another, and
nothing is committed.

--json: {"schema":1,"ok":true,"question":{…,"decision"},"record":"<path>"|null,"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"show": `Usage: itos decision show <id>

Prints the question: its id and text, whether it is open or answered, the
item it names and its decision, then its answer when it has one. It reads, never writes. Exit 1
when no question has the id.

--json: {"schema":1,"ok":true,"question":{"id","item"?,"question","status","answer"?,"decision"?}}`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Switch("--all")}, Subs: map[string]declare.Spec{
		"add":    {Flags: []declare.Flag{declare.Value("--item")}},
		"answer": {},
		"record": {Flags: []declare.Flag{declare.Value("--title"), declare.Values("--option"), declare.Value("--consequences"), declare.Value("--supersedes"),
			declare.Switch("--none")}},
		"show": {},
	}},
}
