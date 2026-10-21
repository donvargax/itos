// Package domain is the followup slice's domain, itos followup's threads
// (features/follow.feature): following up with a person, kept as a thread
// rather than a work item. A thread has an id, the person it is with (free
// text: no people file is read), a title, a status (open or closed) and its
// notes, each a dated entry appended and never edited. They are the
// person's own: one file, FileName, in the clone's private store
// (internal/store/private), so git never commits it and every linked
// worktree of the clone reads the same threads. No config is read: any git
// repository will do.
//
// The file is itos's own, so it is read and written whole with typed structs
// rather than edited in place: Parse refuses, as *Invalid, a key it does not
// know, a thread with no id or a repeated one, a status neither open nor
// closed, and a second YAML document or none at all, since saving what it
// read would drop the rest and itos never writes an empty file. Encode is
// the text the store writes whole, its head comment first; the domain does
// no I/O.
//
// A note's time is written RFC 3339 to the second in local time and shown to
// the minute in the reader's zone (Show), so notes written from two zones
// read in order. Markdown gives a thread as the document itos followup doc
// writes.
//
// The slice (internal/slices/followup) reads the clock through its ports,
// once a run, and hands each change its time, so no corpus case reads the
// clock; it holds the file's lock across a change's read and write, so two
// itos at once keep both changes.
package domain
