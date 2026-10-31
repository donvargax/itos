package attempt

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "attempt",
	Rank: 38,
	Lines: `  attempt submit <id> <result.json>
                                   check an agent's result against its brief and git, and record it
`,
	Help: map[string]string{
		"": `Usage: itos attempt submit <id> <result.json>

Takes an agent's result in as data: checks it against the brief the agent
got (itos brief) and against git, and records it in the clone's journal,
.git/itos/attempts/<id>.jsonl in the git common dir, one line per result.

The result is one JSON object with exactly these keys: schema (1), item,
role (drafter or implementer), request (the "sha256:" request of itos brief
<id> --role <role> --json), base (the full lowercase SHA of the commit that
brief was computed at, its "base"), attempt (a positive integer), outcome
(done, blocked, needs_decision or spec_defect), head (a full lowercase SHA,
required with done and refused with any other outcome; null is none) and
notes (a string, optional, never judged). Any other key, a key given twice,
a missing one, a wrong type or value, or a file that is not one JSON object
is malformed.

The checks, in order, the first failing one refusing, exit 1:
attempt-malformed; attempt-stale, the item not the one named, the base no
commit of the clone once the item's branch is fetched, or the request not
the one itos brief computes at the base for the item and role;
attempt-out-of-order, the attempt not one more than the results the journal
holds for the item and role ("expected attempt <n>"); then, for done, each
claim against git, never the notes: attempt-head-not-pushed, the head not
the tip of the item's branch on the remote (fetched first);
attempt-spec-missing, a drafter's head holding no scenario tagged @<id>, or
one that is not @wip; attempt-not-red-first, an implementer's first-parent
commits past the base holding no test commit before the first feat or fix;
attempt-still-wip, an implementer's head leaving a scenario tagged @<id>
@wip. The scenarios at a head are read under the clone's config, never the
head's.

Recorded: an accepted result, verdict accepted; a malformed one naming a
role, verdict rotated, null for each key at fault and the journal's own
attempt number, so the next try is one higher (the same bytes again are
refused with nothing recorded). A stale, out-of-order or claim refusal
records nothing. A result the journal already accepted (the same role,
request, attempt, head and outcome) is already, exit 0; one of the same
attempt that differs is out of order. Each line:
{"schema":1,"role","attempt","request","base","outcome","head","notes","verdict","sha256"},
sha256 "sha256:" and the hex digest of the result file's bytes.

Text: "accepted <id>'s <role> attempt <n>, <outcome>", " at <head>" when
there is one, "already " in front of it for already; a refusal is its rule,
message and fix on stderr. An id the registry lacks is work-unknown-item,
before the result is read; a missing argument is a usage error, exit 2. It
writes nothing but the journal; its only network is fetching the item's
branch, for a done result or a base the clone lacks.

--json: {"schema":1,"ok":true,"outcome":"accepted|already","item","role","attempt","request","base","head"},
head null when the result has none; a refusal
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{"submit": {}}},
}
