package adapter

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// entryFields is how many fields logArgs prints of each commit.
const entryFields = 9

// logArgs are git log's arguments that print each commit as an Entry: its
// fields, the message and the itos note last, each ended by NUL, which no
// commit message holds; a message may hold any other byte (bug-157).
func logArgs(args ...string) []string {
	return append([]string{"log", "-z", "--no-notes", "--notes=" + port.NotesRef, "--date=raw",
		"--format=%H%x00%h%x00%s%x00%an%x00%ae%x00%ad%x00%P%x00%B%x00%N"}, args...)
}

// entry is the Entry of one record's fields.
func entry(f []string) port.Entry {
	return port.Entry{SHA: f[0], Short: f[1], Subject: f[2], Author: f[3], Email: f[4], Date: f[5],
		Merge: len(strings.Fields(f[6])) > 1, Message: f[7], Note: f[8]}
}

// entries are the records logArgs printed of the commits shas names, in
// their order. A record that does not start with its commit and end where
// the next starts (a note holding NUL, or a log that is not of shas) is an
// error naming the commit, never a commit passed over.
func entries(out string, shas []string) ([]port.Entry, error) {
	f := strings.Split(out, "\x00")
	if len(shas) == 0 && out != "" {
		return nil, fmt.Errorf("git log gives a record of %s, a commit git rev-list does not name", f[0])
	}
	var found []port.Entry
	for i, sha := range shas {
		at, next := i*entryFields, (i+1)*entryFields
		if f[at] != sha || !startsAt(f, next, shas[i+1:]) {
			return nil, fmt.Errorf("git log gives %s no record itos can read", sha)
		}
		found = append(found, entry(f[at:next]))
	}
	return found, nil
}

// startsAt is whether the field at is the first of rest's records, or, with
// no record left, the empty field after the log's last NUL.
func startsAt(f []string, at int, rest []string) bool {
	if len(rest) == 0 {
		return at == len(f)-1 && f[at] == ""
	}
	return at < len(f) && f[at] == rest[0]
}

// Show is git log -1, its one record read whole, the note all that follows
// the message; its error is git's, worded as git.Read words it.
func (g Git) Show(_ context.Context, rev string) (port.Entry, error) {
	out, err := g.git().Read(logArgs("-1", rev, "--")...)
	if err != nil {
		return port.Entry{}, err
	}
	if f := strings.SplitN(strings.TrimSuffix(out, "\x00"), "\x00", entryFields); len(f) == entryFields {
		return entry(f), nil
	}
	return port.Entry{}, errors.New(rev + " names no commit")
}

// Log is git log --reverse over the range, read as the commits git rev-list
// names; its error is git's, worded as git.Read words it, or entries'.
func (g Git) Log(ctx context.Context, r port.Range) ([]port.Entry, error) {
	out, err := g.git().Read(logArgs(append(append([]string{"--reverse"}, revs(r)...), "--")...)...)
	shas, listed := g.Commits(ctx, r)
	if err = cmp.Or(err, listed); err != nil {
		return nil, err
	}
	return entries(out, shas)
}

// LastTouching is the last commit on rev's first-parent history that
// touches one of the paths, each read as the path it is, "" when none does;
// its error is git's own words (slice-204, the spec commit itos brief
// names).
func (g Git) LastTouching(ctx context.Context, rev string, paths []string) (string, error) {
	out, err := g.drafted(ctx, nil, append([]string{"rev-list", "--first-parent", "-1", rev, "--"}, paths...)...)
	return strings.TrimSpace(out), err
}

// CommentChar is core.commentChar.
func (g Git) CommentChar(context.Context) string {
	out, _ := g.git().Output("config", "core.commentChar")
	if c := value.Trim(out); c != "" {
		return c
	}
	return "#"
}
