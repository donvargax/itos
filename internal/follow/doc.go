// Package follow is itos followup's threads (features/follow.feature):
// following up with a person, kept as a thread rather than a work item. A
// thread has an id, the person it is with (free text: no people file is
// read), a title, a status (open or closed) and its notes, each a dated
// entry appended and never edited. They are the person's own: one file,
// follow-ups.yaml, in itos's folder (config.StealthFolder) of the absolute
// git common dir, where the stealth mode keeps its data, so git never
// commits it and every linked worktree of the clone reads the same threads.
// No config is read: any git repository will do.
//
// The file is itos's own, so it is read and written whole with typed structs
// rather than edited in place: Load refuses a key it does not know, a thread
// with no id or a repeated one, a status neither open nor closed, and a
// second YAML document or none at all, since saving what it read would drop
// the rest and itos never writes an empty file. Save writes a file beside
// it, mode 0600, syncs it and moves it over, so a reader in another worktree
// never sees half of it and a crash leaves the old file or the new, and
// makes the folder (0700) where there is none.
//
// A note's time is written RFC 3339 to the second in local time and shown to
// the minute in the reader's zone (Show), so notes written from two zones
// read in order. Markdown gives a thread as the document itos followup doc
// writes.
//
// The command line (internal/cli/follow.go) reads the clock, once a run
// (followNow), and hands each change its time, so no corpus case reads the
// clock; it holds the file's lock (internal/lock) across a change's load and
// save, so two itos at once keep both changes.
package follow
