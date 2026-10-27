package commit

import (
	"github.com/donvargax/itos/v8/internal/declare"
	"github.com/donvargax/itos/v8/internal/engine/hooks"
)

// configHooksSince is the first git that runs the hooks its config declares.
const configHooksSince = hooks.Since

var Command = declare.Command{
	Name: "commit",
	Rank: 90,
	Lines: `  commit [--task <id>] [--item <id>] [--scenarios <ids>] [--<footer> <text>] [<git commit args>…]
                                   git commit, with the footers itos writes
  commit check-message <file|->    header lint and footer rules on one message
  commit check-paths --type <t> <path>…
                                   the commit type's path rules, for planning a split
  commit footers <name> <from> <to>
                                   a range's free-text footers, for the release notes
`,
	Help: map[string]string{
		"": `Usage: itos commit [--task <id>] [--item <id>] [--scenarios <ids>]
                   [--<footer> <text|none>] [--breaking <text>] [<git commit args>…]
       itos commit check-message <file|-> [--at <sha>]
       itos commit check-paths --type <type> <path>…
       itos commit footers <name> <from> <to>

Runs git commit with every argument but its own flags, and writes the footers
from them as git's --trailer, so they land whether the message comes from -m,
-F or the editor. --task writes the footer whose source is the ledger (Task:
here), --item the one whose source is the work registry (Item: here; in
place of Task: for the types its in_place_of names) and --scenarios the one
whose source is a kind of named tests, the first of commits.footers each,
their IDs separated by commas or spaces and written on lines of 100
characters at most, the key on each. Each footer of free text (source: text)
has a flag of its name in lower case (--upgrading for Upgrading:), unless
--task, --item, --scenarios or --breaking has that name, and --breaking
writes BREAKING-CHANGE:, the form git reads as a trailer; their value is the
next argument, whatever it is. Every flag may repeat. Where itos reads the
message (-m, -F <file>, the commit's for -C <commit>, or -c <commit> with
--no-edit, HEAD's for --amend --no-edit), a footer it already
gives with the same value, read as the hook reads it, is not written again, so
an amend keeps one, and one it gives with another value is refused, naming the
footer and both values, nothing committed; a message it does not read (the
editor's, -F -) gets each line unless it has that very line (trailer.ifExists
addIfDifferent).

Before git runs, a commit whose type requires a footer that neither the flags
nor the message give is refused, naming the flag, and nothing is committed;
the type is read from -m, from -F <file>, from the commit's message for -C
<commit>, or -c <commit> with --no-edit, or for --amend --no-edit from HEAD's
message. A message from the editor (-c without --no-edit, -C with --edit), or
-F -, is left to the hooks, which judge
the commit as any other, so a commit they refuse is not made; ITOS_AMEND tells
them whether --amend is among git's arguments, so they know an amend rather
than guess one. Under a stealth config (<git common dir>/itos/itos.yaml) the
links, the footers --task, --item and --scenarios write, are never written
into the message: they are handed to the hook in ITOS_FOOTERS and written as
the new commit's note in refs/notes/itos, and notes.rewriteRef is set to that
ref, so an amend or a rebase carries the note; footers of free text and
BREAKING-CHANGE stay in the message.
-q is git's --quiet. A first argument that is not a flag is a subcommand or a
usage error, never a path: paths for git go after a flag or after --, as in
itos commit --task T-1 -m '…' -- README.md.

It refuses before anything else, exit 2, what would commit past the gates:
--no-verify and -n, which skip git's hooks; a core.hooksPath set by git -c,
which moves them; and -a and --all, which stage every change to a tracked
file, someone else's work in progress among them (stage your own with git add
<path>). git's arguments are read as git reads them: -m's value, a cluster
of short flags (-am) and anything after -- are git's.

Where itos is set up (a config is there), it refuses before git runs, exit 3,
when git would run none of its checks: the git config declares no commit-msg
hook of itos's (run itos hook install once in the clone), or, on a git that
runs no hook its config declares (older than ` + configHooksSince + `), no commit-msg hook
file in the folder git runs hooks from calls itos's (hook install writes one).

Exit: git's own code; 1 when a required footer is missing or a footer the
message gives differs from its flag's; 2 when the first word is no subcommand,
a flag has no value or the config no footer for it, or an argument is
refused; 3 when itos's commit-msg hook would not run.

--json: git's output on stderr; {"schema":1,"ok","commit"?}, or when a footer
        is missing or differs
        {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"check-message": `Usage: itos commit check-message <file|-> [--at <sha>]

Runs the header lint (commits.header_lint: itos's own with use: builtin, else
the stdin delegate), if any, and itos's footer rules (commits.footers),
always, on one message, read from <file> or stdin (-), and reports both. --at
reads the footers' IDs at that commit. Under a stealth config the footers of
IDs are the ones ITOS_FOOTERS gives, never the message's, and a footer of
free text is the message's. Exit 1 when either rejects the message; the
built-in lint's warnings alone do not.

--json: {"schema":1,"ok","problems":[{"rule","message","fix"?,"level"}]}`,

		"check-paths": `Usage: itos commit check-paths --type <type> <path>…

Applies the type's path rules (commits.scopes) to the paths and prints what
they reject, without committing. A path is read from the folder itos is run
in, as git reads one, and named from the repository's top. Exit 1 when a path
is rejected.

--json: {"schema":1,"type","files","ok","problems":[{"rule","message","fix"}]}`,

		"footers": `Usage: itos commit footers <name> <from> <to>

Lists the footers <name> that the range's non-merge commits carry (from may be
empty or all zeros: every commit up to <to>), where commits.footers.<name> is
free text (source: text): what each commit asks of a consumer, gathered for a
release's notes. One line per footer, oldest commit first, its commit's short
SHA and its text; a commit with several gives a line for each. A footer that
says none (the word alone, in lower case) asks nothing and is left out, as is
an empty one. The footers are each commit's message's, under a stealth config
too. Exit 2 when <name> is not a footer of free text.

--json: {"schema":1,"footer","range","footers":[{"sha","subject","text"}]}`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Values("--task"), declare.Values("--item"), declare.Values("--scenarios"), declare.Values("--breaking")},
		More: declare.Footers, Others: true, Subs: map[string]declare.Spec{
			"check-message": {Flags: []declare.Flag{declare.Value("--at")}},
			"check-paths":   {Flags: []declare.Flag{declare.Value("--type")}},
			"footers":       {},
		}},
}
