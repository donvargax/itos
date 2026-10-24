package tests

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "tests",
	Rank: 160,
	Lines: `  tests list <kind> [--at <tree>] [--tsv]
                                   the kind's named tests
  tests smoke check|ids|run <kind> the smoke rule, the smoke IDs, the smoke run
  tests moves <kind>               the staged feature files by the moves rule
  tests next-id <kind> <stem> [--count <n>]
                                   the kind's next free tag of the stem, claimed for an area
`,
	Help: map[string]string{
		"": `Usage: itos tests list <kind> [--at <tree>] [--tsv]
       itos tests smoke check|ids|run <kind>
       itos tests moves <kind>
       itos tests next-id <kind> <stem> [--count <n>]`,

		"list": `Usage: itos tests list <kind> [--at <tree>] [--tsv]

The kind's adapter's listing: each named test's ID, file and whether it is
live, at the working tree (default), the index or a commit. A command
adapter may give a test a title too. Under --json a Gherkin kind gives each
scenario a fingerprint, the SHA-256 of what it runs (its steps, tables, doc
strings, Examples, the Background steps before it and its tags, its own and
its feature's): a comment, its name, the feature's description, another
scenario, or a move to a file with the same Background and feature tags
leaves it as it was.

--tsv prints the live tests alone, one a line in the listing's order: its
ID, a tab and its file from the project's top (the kind's root joined to it,
slash separated), then for a Gherkin kind a tab and its fingerprint; no
heading, each line ending in LF. A live test no such row can hold (an empty
id or file, a tab, carriage return, newline or NUL in one, an id two live
tests have, a file outside the project) refuses the whole listing: nothing
on stdout, each such test named on stderr, exit 1. Warnings and diagnostics
stay on stderr. --tsv and --json together are a usage error (exit 2).

--json: {"schema":1,"protocol":1,"tests":[{"id","file","live","title"?,"fingerprint"?}],"files"}`,

		"smoke": `Usage: itos tests smoke check <kind> [--features <dir>]
       itos tests smoke ids <kind>
       itos tests smoke run <kind> [--] [<runner args>…]

check: every file with a live test has a smoke test (unless the kind's
smoke.every_file is false), and every smoke ID is live (exit 1 when not). ids:
the smoke set's IDs, one a line. run: the kind's run of exactly the smoke set,
everything after <kind> added to it as given (a -- right after <kind> is taken,
to pass what looks like an itos flag, and not passed on); its exit code is the
runner's.

--json (check): {"schema":1,"kind","ok","ids","problems":[{"rule","message","fix"}]}
--json (ids):   {"schema":1,"kind","ids"}`,

		"next-id": `Usage: itos tests next-id <kind> <stem> [--count <n>]

Prints the next free tag of the kind with the stem, <tag_prefix><stem>-<n>
(@ID-A-04, @slice-10): n is one past the highest that any tag
<tag_prefix><stem>-<n> of the kind's files holds at the working tree, live or
@wip, and any work registry item id <stem>-<n> (an item such as slice-9 is
named before its scenarios are written). A kind behind a command adapter
counts its listing's IDs instead of tags. The number keeps the width the stem
already has; a stem nothing uses starts at 1, padded to two digits where the
kind's ID pattern is an ID- one and the tag is one of its IDs (@ID-NEW-01).

A stem that is an area of that ID pattern (ID-<AREA>) is claimed through the
id counter that work add mints item ids through: n is one past the higher of
the area's counter and the highest the files hold, and the counter is raised
to it, so a number printed is never printed again, used or not, and one a
removed scenario held is not given again. --count <n> claims n numbers in a
row and prints each, one a line. Any other stem (slice, bug) only reads: the
commands that create those items mint their ids, and what it prints for one
is a preview, not a scenario ID. A bare stem that would be an area with ID-
in front, itos tests next-id scenario SEC where the pattern takes ID-SEC, is
refused with the command that claims, itos tests next-id scenario ID-SEC
(--count kept), where it would print @SEC-1, an ID the pattern refuses. That
holds when the pattern's text begins with ID- and no tag or item id of the
kind or the registry is <stem>-<n> (a series such as @batch-7 stays a
preview), and slice and bug stay previews too. A <kind> the config does not
have, a --count that is no number from 1 up, a --count for a stem that only
reads and a stem refused for its correction are usage errors (exit 2); a
counter that cannot be reached is exit 3.

--json: {"schema":1,"kind","stem","id","ids","claimed"}`,

		"moves": `Usage: itos tests moves <kind>

Judges the staged feature files of a Gherkin kind against HEAD's by the moves
rule, as the commit-msg hook does for a type the rule does not exempt: live
scenarios keep their ID, name, tags and steps exactly, none is lost or turned
@wip, and a file with a live scenario keeps its header and Background; @wip
ones may come, go or change, and comment lines and the feature's description (the free
text under Feature:) are never compared. A rename the kind's
range check { builtin: moves } lists in allowed_renames passes. A scenario
going live (a new live one, or one losing its @wip) is reported: there is no
message here, and it is the commit's footer that allows it, when the check
sets code, the footer for the kind (Scenarios:) names it and the commit
changes none of that code. Exit 1 with each problem.

--json: {"schema":1,"kind","ok","problems":[{"rule","message"}]}`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"list":    {Flags: []declare.Flag{declare.Value("--at"), declare.Switch("--tsv")}},
		"moves":   {},
		"next-id": {Flags: []declare.Flag{declare.Value("--count")}},
		"smoke": {Subs: map[string]declare.Spec{
			"check": {Flags: []declare.Flag{declare.Value("--features")}},
			"ids":   {},
			"run":   {Unread: true, After: 1},
		}},
	}},
}
