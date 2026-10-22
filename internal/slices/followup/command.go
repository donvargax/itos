package followup

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "followup",
	Rank: 70,
	Lines: `  followup [--all]                 your open threads with people; private, never committed
  followup add <id> --with <who> --title <title> --note <text>
                                   open a thread with someone
  followup note <id> <text>        add a dated note to a thread
  followup close <id> [--note <text>]
                                   close a thread
  followup show <id>               a thread and all its notes
  followup doc <id> <path> [--force]
                                   write a thread out as Markdown
`,
	Help: map[string]string{
		"": `Usage: itos followup [--all]
       itos followup add <id> --with <who> --title <title> --note <text>
       itos followup note <id> <text>
       itos followup close <id> [--note <text>]
       itos followup show <id>
       itos followup doc <id> <path> [--force]

Your threads with people: following up with someone on something, a thread
of dated notes rather than a work item. A thread has an id, the person it is
with (any text: no people file is read), a title, a status, open or closed,
and its notes, each dated when written and never edited. They are yours
alone: kept in follow-ups.yaml of itos's folder in the git common dir (git
rev-parse --git-common-dir, where the stealth mode keeps its data), readable
by you only, never committed, and the same from every linked worktree of the
clone. No config is needed, only a git repository: exit 3 outside one.

A subcommand that changes the threads holds follow-ups.yaml.lock beside the
file from before it reads them to after it writes them, so two at once, in
two worktrees say, take turns and both changes are kept; one that finds the
lock held waits, and gives up after 10 seconds naming it. An itos stopped
while it held the lock leaves the file behind: remove it when no itos runs.
Times print to the minute in your time zone.

itos followup lists the open threads, each with its title, who it is with and
when it last had a note; --all adds the closed ones after them. Exit 1 for a
refusal (an id a thread has, no thread with the id), 2 for a usage error, a
lock still held, or a follow-ups.yaml itos cannot read or did not write
whole (a second YAML document, an empty file), which it leaves as it is.

--json: {"schema":1,"threads":[{"id","with","title","status","opened","last","notes"}]},
notes the count; each subcommand's in its help (itos help followup <subcommand>)`,

		"add": `Usage: itos followup add <id> --with <who> --title <title> --note <text>

Opens a thread with <who>, its first note <text> dated now. An id is letters,
digits, '.', '_' and '-', a letter or digit first. Refused, nothing written
(exit 1): an id a thread already has, open or closed.

--json: {"schema":1,"ok":true,"thread":{"id","with","title","status","closed"?,"notes":[{"at","text"}],"docs"?}},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"note": `Usage: itos followup note <id> <text>…

Appends a note to the thread, dated now: the words after the id joined by
spaces, so the text needs no quotes unless the shell would read it; after
"--" a word that starts with "-" is text too. A closed thread takes notes and
stays closed. Refused (exit 1): no thread with the id.

--json: as itos followup add's, the thread as it is after`,

		"close": `Usage: itos followup close <id> [--note <text>]

Closes the thread, dated now, with a last note when --note gives one. A closed
thread is left out of itos followup's list and still shows and takes notes.
Refused, nothing written (exit 1): no thread with the id, a thread already
closed.

--json: as itos followup add's, the thread as it is after`,

		"show": `Usage: itos followup show <id>

Prints the whole thread: its id and title, who it is with and its status,
then every note in the order written, each with its date and time, and the
files itos followup doc wrote it to. It reads, never writes. Exit 1 when no
thread has the id.

--json: as itos followup add's`,

		"doc": `Usage: itos followup doc <id> <path> [--force]
       itos followup doc <id> -

Writes the whole thread at <path> as Markdown, in the repository or not, the
folders it needs made: its title, who it is with and its status, then each
note under its date, in order, the raw material for a document someone
refines. The thread records the file, as itos followup show prints it. A file
already at <path> is refused, nothing written (exit 1), unless --force writes
over it; so is a folder, or no thread with the id.

The thread is private, so the file is readable by you alone (0600, the
folders made 0700), and a <path> in the work tree that git does not ignore
is written with a warning on stderr: a git add would commit it. With - for
<path> the Markdown is printed instead, and no file written or recorded.

--json: as itos followup add's, with "path", the file written, absolute; with
-, as itos followup add's with "markdown", the text`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Switch("--all")}, Subs: map[string]declare.Spec{
		"add":   {Flags: []declare.Flag{declare.Value("--with"), declare.Value("--title"), declare.Value("--note")}},
		"note":  {},
		"close": {Flags: []declare.Flag{declare.Value("--note")}},
		"show":  {},
		"doc":   {Flags: []declare.Flag{declare.Switch("--force")}},
	}},
}
