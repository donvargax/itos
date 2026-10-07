// Package work is the work registry: work.registry, work-items.yaml in the
// ledger's folder by default (in the git folder under a stealth config),
// which says who owns each group and each work item, its kind (slice, task,
// idea), its status, what it waits on (depends_on), its tags, and the queue.
// The registry is written by commands, never by hand; this package judges
// and edits its text, and internal/cli writes and commits it.
//
// # Reading and judging
//
// Load reads the registry and the people beside it, none under a stealth
// config; a people file that is missing or cannot be read leaves
// Registry.People false, silently, and then no owner is checked and any
// handle is listed. Only config check says why, as a warning (PeopleProblem)
// that never sets its exit code and is no finding of the commit-msg hook's
// check of staged data. Issues and Problems are the registry's problems,
// which config check and itos work check report (ProblemsAt, for a file
// named on the command line, whose missing-file fix differs: Missing). An
// item's tags are held to those work.tags declares, none when it declares
// none. ItemStatuses reads the statuses alone, without the people, so itos
// task list, the CI plan's waiting tasks and the commit-msg hook's "done"
// read work over a registry with problems; IDsAt reads the item ids at a
// tree, for a footer whose source is the registry.
//
// Dropped is a status itos knows whatever work.statuses lists, since only
// itos sets it: an item is live while neither done nor dropped (Open lists
// those, so blocked and any status a project adds are open), and a dropped
// idea or deferred item is exempt from the started-item problems; an item
// still to do that depends on a dropped one is work-dropped-dependency.
//
// # The proposal
//
// Whoami gives the session's person: --as, which must be among the people
// when there are any (exit 3), else who the people make it: one login is the
// session, and with no one listed (Registry.Nobody) the session is nobody
// and no one is asked. Only among several people is the work.identity
// provider asked. A session with no handle is nobody, and one the people do
// not list owns nothing yet; both still see what nobody owns. Under a
// stealth config with no --as nobody is asked: ProposeEvery is the proposal
// of a session that owns every item, whatever owner it or its group names.
// Ownership is the item's owner, else its group's (ownerOf). Propose keeps
// each item the mapping as written (value.Map, its keys in the registry's
// order, depends_on and tags added last when absent), so every --json item
// has both lists, and walks the items in the queue's order, the unqueued
// after in the registry's (inQueueOrder); what another person owns stays out
// of each list, so every person sees their part. Startable is what may
// start, for itos status. PrintList is itos work list's text, a deferred
// item marked (deferred) after its title.
//
// # Writing
//
// Every writer starts from a sound registry and its text, finds the item by
// its id, and gives a Change (the text after, the item as it reads after,
// the items whose depends_on were renamed, the commit's header and body, or
// Unchanged) or a problem that refuses, nothing written. The text is edited
// with value.Doc, every comment and quote kept. Add, Edit, Done, Drop, Defer
// and Resume run Issues on the registry with the item as it would be (sound)
// rather than restate the check: an owner not among the people, a dependency
// on no item, a cycle, an item done before its dependency.
//
//   - Take sets an item doing for a person (the owner left as it is when
//     nobody is asked); Promote turns an idea into a slice or a task, renames
//     it in every depends_on and in the queue, and replaces the title when
//     --title gives one.
//   - Done closes an item (an idea, a deferred item, one not doing and one
//     whose dependency is not done refused, one already done Unchanged);
//     Unqueue then takes it out of the queue, a commit of its own.
//   - Add puts a new item, todo, at the end of the items (value.Doc's Append,
//     keys in the registry's own order, its why folded); its group is
//     --phase, else the one a p<n>- id names, else the registry's only one.
//   - Edit replaces a title, a depends_on or refs and adds a paragraph to the
//     why; a list the item lacks compares as an empty one. A note on a slice
//     or a task (SpecKind) is refused, rule work-note-spec, naming where its
//     why lives (WhereWhy): the registry keeps an idea's why alone
//     (docs/decisions/0034-the-work-registry-is-an-index-and-a-why-lives-in-the-spec.md).
//     Edit changes no owner, kind or status: the other writers own those.
//   - Queue puts an item first, just before or after one the queue holds, or
//     out of it, the queue (a top-level list of ids, one for the repository,
//     ideas included) written whole as a block list before items:, a shape
//     no formatter rewraps however long it grows. queueIssues refuses a
//     queue that is not a list, an id no item has and one named twice.
//   - Drop sets an item dropped, drops its why as Done does and takes it out
//     of the queue; it refuses an item done and one a live item depends on
//     (dependants). Take, Done, Promote and Queue refuse a dropped item.
//   - Defer writes the reason as the item's deferred, a folded block in a
//     block mapping and a quoted text in a flow one, leaving status, owner,
//     tags and the queue as they are; it refuses an item already deferred,
//     one done or dropped, and one of any status but todo. Resume drops the
//     key and nothing else.
//
// RegistryHeader reads back the headers the writers commit with (docs: take
// <id>, docs: queue <id> and the rest), so itos work show finds an item's
// registry commits; its unit test reads the writers' headers. Clashes gives,
// for itos push's rebase conflict, each item both sides changed over the
// registry at the base, theirs and mine. Show is an item with the ids of the
// items whose depends_on name it.
package work
