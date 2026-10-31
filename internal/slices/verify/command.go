package verify

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "verify",
	Rank: 150,
	Lines: `  verify <from> <to>               re-check every commit of a range
`,
	Help: map[string]string{
		"": `Usage: itos verify <from> <to>
       itos verify

Re-checks every commit of the range (from may be empty or all zeros: every
commit up to <to>): its message, with the footers read at that commit, its
paths, and, by a kind's built-in moves rule, its feature files against its
parent's, unless the rule's except_types names its type (a scenario going
live judged by the commit's own footers and paths, as the hook judged it);
then each kind's
range command. A merge commit is judged by its own changes, the paths where
its tree differs from git's automatic merge of its two parents (git
merge-tree --write-tree), or, for more parents or two with no merge base,
the paths of its dense combined diff (git diff-tree --cc): with none it passes whatever its
message, with some it is judged as its first line's type, and refused when
that is none of the commit types. The commit commits.since names and its ancestors are left out,
and the range commands start there; a footer's own since leaves that commit
and its ancestors out of the footer's required_for. Under a stealth config a
commit's footers of IDs are read from its note in refs/notes/itos, and with
no range the commits are HEAD's on no remote branch (every one with no
remote), the range's from "--remotes": the person's own, not yet pushed.
Prints "<n>/<m> commits pass the commit rules". Exit 1 on any failure, 2 when
commits.since or a footer's since is not a commit of the repository.

--json: {"schema":1,"range","since"?,"commits":[{"sha","header","ok"}],"passed","total","range_checks"}`,
	},
}
