// Package attempt is the attempt command (slice-207,
// features/attempt.feature), a slice (ADR-0053): itos attempt submit <id>
// <result.json> takes an agent's result in as data, never as prose
// (docs/plans/v9-coordinator.md, the LLM steps' contracts). It checks the
// result against the brief the agent got and against git, and records it
// in the clone's journal, so that itos next can route the item from facts.
//
// The result is strict JSON (result.go): one object, each key once, exactly
// the keys of the contract, each of its type. A result naming a role it
// knows counts for that role even when it is malformed: it is recorded,
// rotated, with null for each key at fault, so the agent's next try is one
// attempt higher; the same bytes again are refused with nothing recorded.
//
// The checks run in order, the first failing one refusing: malformed;
// stale, the result's item, its base (a commit the clone has once the
// item's branch is fetched) and its request, recomputed by itos brief at
// that base, which internal/cli binds as the Request port; out of order,
// the attempt not the one after those the journal holds for the role, a
// result the journal already accepted being already instead; then, for
// outcome done, the claims, against git alone: the head the tip of the
// item's branch on the remote, fetched first; a drafter's head holding the
// item's scenarios, all @wip; an implementer's first-parent commits past
// the base holding a test commit before the first feat or fix, and leaving
// none of the item's scenarios @wip. The scenarios at a head are read under
// the clone's config, as itos work done reads them at HEAD, and a commit's
// type as the message rules read it (message.Type).
//
// The journal (journal.go) is <git common dir>/itos/attempts/<id>.jsonl, a
// line per result taken in. Beyond it the command writes nothing; its only
// network is fetching the item's branch, for a done result or a base the
// clone lacks.
package attempt
