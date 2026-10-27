package status

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "status",
	Rank: 30,
	Lines: `  status [--as <handle>]           where things stand: head, CI, release, work, questions
`,
	Help: map[string]string{
		"": `Usage: itos status [--as <handle>]

Prints where things stand, for the person the work is for (--as, else the
identity provider's, as itos work): the remote branch's head, its short SHA and
header, asked of the remote (the branch's upstream, else origin), or as last
fetched when the remote cannot be reached, and the repository's visibility in
brackets after the branch, public, private or internal, asked once of ci.watch's
provider (for github the repository's visibility, else its private flag) and
left out when it cannot tell; its CI run, looked at once through
ci.watch's provider and never waited for, a run still going said to be going;
while it is going or has not passed, the commit main last proved, "Last green:"
and its short SHA and header (the SHA alone when it is not fetched here), as
ci.range's provider names it: for github the nearest of the branch's first
parents as fetched, its head included and 100 at most, with a successful run of
ci.range.github's workflow on any branch (its repository, token and API address
found as ci.watch's), read once, and left out for none; the last nightly's run,
looked at once the same way, "Nightly:" and its result and address, read from
ci.watch.github.nightly_workflow's newest run on the branch, and left out when
that key is not set; the newest release, the highest tag
v<semver> on the remote (a prerelease below its release), asked of it as the
head is, or as last fetched, or no release yet; the commits since it the next
release would carry, the feat, fix and breaking ones, by header, oldest first,
read from the commits as fetched here, with a line saying they may be behind
when the head is not fetched; the version of itos the repository runs (its pin,
else tools/bin/install-itos's version= line, as itos upgrade reads it) and the
newest stable release of itos, asked of ITOS_RELEASES once within 5 seconds as
itos pin asks, naming itos upgrade only when that is a newer major, and left
out with neither a pin nor an install script, when the server does not answer,
and in CI or with ITOS_NO_UPDATE=1, as the launcher asks nothing there
(ITOS_NO_UPDATE_NOTICE=1 does not leave it out); beside it, a line naming
itos init --agent-rules when AGENTS.md, or under a stealth config the copies
init writes there, holds a block of itos's rules for agents other than the one
this itos writes (none for no block, or a file it cannot read); the person's
items in progress; the next items they can start, theirs and the unowned, in the queue's order and the unqueued
after, five at most, each its id, its tags in brackets when it carries any, and
its title; the open questions (itos decision); and the drafts waiting
for itos draft promote, when there are any. It reads, and writes nothing
but the remote-tracking ref of the one branch it reads, which it fetches
first, quietly, never prompting, within 20 seconds (git fetch --quiet
--no-tags of that ref; from a detached HEAD the remote's own refspecs): not
with no remote, under a stealth config, or when the remote answers without
the branch. After a fetch that worked, a clone whose HEAD lacks commits of that
branch is told so before the work lists, "<branch> is <n> commits behind
<remote>/<branch>: git pull before reading the work below." (from a detached
HEAD, "switch to a branch and pull" instead), the items being read from the
working tree. A fetch that fails is a line, "<remote>/<branch> cannot be
fetched: " and git's first line of why, and the rest is read as last fetched.
Under push.mode pr, an item in progress whose branch (named for it) has a
merged pull request and no open one ends its line with "merged (#<n>): itos
work done <id>", n the newest merged pull request: the work is on main and only
the close is left. One gh api graphql call asks for every item in progress,
within 5 seconds, unless nothing is in progress, push.mode is trunk, the config
is the stealth one or the remote names no GitHub repository (ci.range.github's
repository_env variable, else its URL); with no gh on the PATH nothing is said.
What cannot be reached (no remote, ci.watch.provider none, a
provider that fails, for the head or the nightly, a ci.range provider that
fails, a gh that fails, answers wrongly or gives no answer within 5 seconds, for
the pull requests) is one line naming it, and the rest still prints, exit 0.
Exit 1 when the work registry is not there or not sound, 3 when --as is not
among the people. itos go prints it last.

--json: {"schema":1,"person","every_item"?,"head":{"remote","branch","commit",
"header","last_fetched","visibility"?}|null,"behind"?,"ci":{"result","run"?}|null,"last_green":
{"commit","header"}|null,"nightly":{"url","status","conclusion"?,"jobs"}|null,
"release":{"tag","commit","last_fetched"}|null,"unreleased":["<header>"]|null,
"itos":{"runs","newest","new_major"}|null,"agent_rules":{"stale":["<path>"]}|null,
"doing":[<item>, plus "merged_pull_request":<n> on one marked as above],"next","more","questions":
[{"id","item"?,"question","status"}],
"drafts":[{"id","kind","header"?,"paths"?,"command"?}],"unread"}, result success, failure (or another conclusion), going, or none for
no run yet; last_green null when the head's run passed or was not read,
ci.range's provider is none, it names no commit or it cannot be read, header ""
when not fetched; nightly null when no nightly is named, it has no run yet or it
cannot be read; unreleased null with no release or when they cannot be listed;
itos null when its line is left out; agent_rules the files holding a stale
block, null with none; behind how many commits of the fetched branch HEAD lacks,
0 when it is up to date, left out when no fetch worked; unread the lines of what
could not be reached`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Value("--as")}},
}
