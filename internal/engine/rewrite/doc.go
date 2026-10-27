// Package rewrite is the engine that judges a branch's own rewrite (slice-193,
// features/pre-push.feature), which the pre-push hook accepts and itos push
// --onto-main makes: one rule, shared, so what the push makes is what the hook
// accepts.
//
// A commit's counterpart is a commit with the same author (name and email),
// the same author date and the same message. git rebase keeps all three, and
// neither a resolved conflict nor a change main made to the lines around the
// commit's own changes any of them; a patch-id, which hashes the diff with its
// context lines, changes with both, so a conflict resolved during the rebase
// would read as a dropped commit. A squash or a reword changes the message, so
// the old commit has no counterpart and the rewrite is one that drops it.
//
// Dropped are the commits of the old head that main lacks and the new head's
// commits that main lacks hold no counterpart of. Each commit of the new head
// is the counterpart of one old commit alone: two commits made alike (the same
// message in the same second) need two. A merge commit is set aside in both
// heads, having no work of its own: a rebase leaves it out, and the commits it
// brought in are matched. Main is a revision the caller resolved, "" to count
// each head's commits as all of its history.
package rewrite
