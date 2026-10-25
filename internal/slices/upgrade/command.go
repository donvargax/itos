package upgrade

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "upgrade",
	Rank: 130,
	Lines: `  upgrade [<version>|latest]       move to a newer itos, listing what each release since asks
`,
	Help: map[string]string{
		"": `Usage: itos upgrade [<version>|latest]

Moves the project to a newer itos: <version> (a leading v is taken), or with
none or latest the newest, asked for as itos pin asks for it. The version it moves from
is the config's pin, or with no pin the install script's,
tools/bin/install-itos; with neither it exits 1, naming itos pin, and so does
a version older than that one, which is itos pin's to move back to.

It walks, oldest first, every release after the version it leaves: its
breaking changes, its Upgrading footers, the old scenarios and corpus cases
it changes on purpose and the config keys it adds, removes or gives another
default, read from each release's upgrading.json
(<ITOS_RELEASES>/download/v<version>/upgrading.json), walking back through
each one's previous from the new release to the old version. A release with
no upgrading.json, cut before they were published, is named by its notes,
<ITOS_RELEASES>/tag/v<version>, and the walk stops there.

It edits only what a release fixes exactly: the pin (pin.version and
pin.checksums, as itos pin sets them; none is added where there is none), the
config's first line when it is a yaml-language-server schema line naming a
release's itos.schema.json, the install script's version= line and each
platform's sum= hash, from the new release's checksums.txt, and the config
keys each release's published itos.schema.json declares: a key whose
property declares x-itos-renamed-from (<old dotted path>) takes the old key's
value and the old key goes; a key the schema's top-level x-itos-removed lists
is deleted, and the value it held printed. Releases are applied oldest first,
so a key renamed twice ends under its newest name; a release with no schema,
or one declaring nothing, edits no key. Every comment and other line of the
config is kept. A config or an install script it cannot edit so (a key in a
flow mapping, an anchor or an alias, a new key set already) exits 2, naming
the key.

It prints each release's edits as it makes them, oldest first, then By hand:
every crossed release's breaking changes and Upgrading footers, which are
what is left for the person to do, or "By hand: nothing". Nothing a commit's
text says is edited. It fetches everything, each release's schema too, and
makes every edit in memory before it writes anything, so a release the server
does not have exits 3, and a release server that cannot be reached or fails
75, every file untouched, the pin included.
A project already on the version changes nothing, exit 0; a pin on it with
other checksums is left as it is, exit 1: the release changed after it was
pinned. It commits nothing, and runs the binary called, whatever the pin says,
unless the version it moves to is newer than it: then that version is fetched,
as a pinned one is, and runs the upgrade with ITOS_UPGRADE_FROM set, so each
release applies its own steps; its exit code is upgrade's.

It names, and moves none of, each step of the workflows directly in
.github/workflows (*.yml and *.yaml, read as YAML) whose uses: is
donvargax/itos@<ref>: its file, the line of its uses: key, the ref as written
and its with.version when it has one. The action installs a launcher, the
release its ref or its version input names, and the repository's pin chooses
the itos that runs, so such a line need not move, unless the workflow sets
ITOS_VERSION or loads another config; with no pin each line runs the release
it installs, under the same two exceptions, and itos pin pins one. A workflow that cannot be
read as YAML (a syntax error, a key written twice, more than one document) is
named with why, and its steps are not. A stealth config names none,
since no workflow reads it. The lines come after the releases, before By
hand, and after "already on" when there is nothing to upgrade.

Where AGENTS.md, or a stealth config's copies, holds a block of itos's rules
for agents other than the one it writes, the itos doing the upgrade rewrites
it in its own words, only between the markers, as itos init --agent-rules
does; a file with no block gains none. Each file it rewrites is among those
it edited.

--json: {"schema":1,"action","from","to","releases","edited","applied",
        "manual","action_uses","action_scan_problems"}, action one of upgraded, already, refused; each release
        {"version","breaking","upgrading","changes","config"}, or
        {"version","notes"} for one with no upgrading.json; each applied
        {"version","kind":"renamed"|"removed","key","from"?,"value"?}; each
        manual {"version","text"}; action_uses the workflow steps named, each
        {"file","line","ref","version_input"?}, and action_scan_problems the
        workflows it could not read, each {"file","message"}, both [] when
        there are none, under a stealth config and for a refusal`,
	},
}
