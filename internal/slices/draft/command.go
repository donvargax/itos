package draft

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "draft",
	Rank: 80,
	Lines: `  draft                            the drafts waiting, in the order promote applies them
  draft add <id> -m <message> <path>…
                                   keep a change to files for later, and put the files back
  draft add <id> -- <itos args>…   keep an itos command line to run later
  draft edit <id> -m <message> <path>…
                                   keep a change as copies to edit, off the work tree
  draft promote                    apply and commit the drafts, once the checkout is clean
  draft drop <id>                  drop a draft, applying nothing
`,
	Help: map[string]string{
		"": `Usage: itos draft
       itos draft add <id> -m <message> <path>…
       itos draft add <id> -- <itos args>…
       itos draft edit <id> -m <message> <path>…
       itos draft promote
       itos draft drop <id>

The changes a coordinator writes while an agent holds the checkout, kept
until no work is going on: a commit there would take the agent's staged
files with it, and a file left changed would stop the agent's push. A draft
is a change to files with the message to commit it with, or an itos command
line to run later (a work add, a work queue, a task add), which commits
itself. They are the clone's own: drafts.yaml in the drafts folder of
itos's folder in the git common dir (git rev-parse --git-common-dir), each
change's patch beside it as <id>.patch, or an edit draft's copies in
edits/<id>/, readable by you only, never committed, and the same from every
linked worktree of the clone. A subcommand that changes them holds
drafts.yaml.lock from before it reads them to after it writes them, as itos
followup does; one that finds the lock held waits, and gives up after 10
seconds naming it.

itos draft lists the drafts in the order they were added, which is the order
itos draft promote applies them in: each id with its message's header and
its paths, or its command line. itos status names them too. Exit 1 for a
refusal, 2 for a usage error, a lock still held, or a drafts.yaml itos cannot
read or did not write whole, 3 outside a git repository.

--json: {"schema":1,"drafts":[{"id","kind","header"?,"paths"?,"command"?,"minted"?}]},
kind change or command; each subcommand's in its help (itos help draft <subcommand>)`,

		"add": `Usage: itos draft add <id> -m <message> <path>…
       itos draft add <id> -- <itos args>…

With -m (--message), keeps the change of the paths against HEAD, as a binary
git patch, files git does not track yet included, and the message to commit
it with, footers and all; then puts those paths, and only those, back as HEAD
has them, in the index and the work tree, a file HEAD lacks removed. With --,
keeps the itos arguments after it exactly as given, to be run later by the
same itos binary, never through a shell; a draft cannot run itos draft. A
command that mints an item's id (work add --kind slice|task|bug, task add, work
promote) has it minted now, through the id counter, as the command would mint
it: the draft keeps it, the line printed ends "(makes <id>)", and promote
creates the item under it, so a later draft, a spec or a brief can name it at
once. The number is never given back: a mint meanwhile takes the next, and a
draft dropped leaves a gap. An id is letters, digits, '.', '_' and '-', a
letter or digit first. Refused, nothing written (exit 1): an id a draft
already has, a path that names nothing in the work tree or HEAD, or one git
ignores, paths with no change against HEAD, a repository with no commit, a
registry the command would refuse; a counter that cannot be reached, exit 3.

--json: {"schema":1,"ok":true,"draft":{"id","kind","header"?,"paths"?,"command"?,"minted"?}},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"edit": `Usage: itos draft edit <id> -m <message> <path>…

Keeps a change to files as copies to edit, with the message to commit it
with, footers and all, never reading or writing the work tree, so it may run
while an agent holds the checkout: a copy of each path, HEAD's content or
empty for a file HEAD lacks, at that path in edits/<id>/ of the drafts folder.
It prints each copy's path, one per line, to open and edit there. The draft
records HEAD as the commit its copies were taken from; itos draft promote
makes its change from them against that commit and applies it as any change
draft's, so a file that commit had and HEAD has changed since stops it. What
a copy holds when promoted is what the commit holds, except that an empty
copy of a file the commit lacked adds nothing; a copy removed removes its
file. Refused, nothing written (exit 1): an id a draft already has, a path
that is a folder or a submodule in HEAD, a repository with no commit.

--json: as itos draft add's, with "copies", each copy's path, absolute`,

		"promote": `Usage: itos draft promote

Applies the drafts in the order they were added, while the checkout is
clean: refused, nothing applied (exit 1), while any tracked file has a change
no commit holds, staged or not, naming each, or a rebase or a merge is in
progress. An item's status is no gate: items stay doing for the coordinator's
own work and for agents that have gone. A change is applied with git apply --index, which takes
the whole patch or none of it, and committed with its message through the
hooks, as any commit; an edit draft's patch is made from its copies first; a
command line is run by this itos and commits itself, its output on stderr, one
that minted its item's id when drafted writing the item under that id.
Each draft promoted leaves the list. The first that cannot be applied stops
it, exit 1 naming it: a change that no longer applies to the tree, an edit
draft whose copies changed nothing, a command that fails, a commit a hook
refuses. The tree is left as HEAD has it, that draft and the ones after it
are kept, and the ones before it stay committed.

--json: {"schema":1,"ok","promoted":["<id>"],"problems"?:[{"rule","message","fix"}]}`,

		"drop": `Usage: itos draft drop <id>

Takes the draft out of the list and removes its patch or its copies,
applying nothing. An id the draft minted is not given back: it leaves a gap.
Refused (exit 1): no draft with the id.

--json: as itos draft add's, the draft dropped`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"add":     {Flags: []declare.Flag{declare.Value("-m"), declare.Value("--message")}},
		"edit":    {Flags: []declare.Flag{declare.Value("-m"), declare.Value("--message")}},
		"promote": {},
		"drop":    {},
	}},
}
