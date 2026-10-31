package adapter

import (
	"context"
	"errors"
	"strings"

	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// entryFields is how many fields logArgs prints of each commit.
const entryFields = 9

// logArgs are git log's arguments that print each commit as an Entry: its
// fields NUL-separated, the message and the itos note last, each commit's
// ended by a record separator.
func logArgs(args ...string) []string {
	return append([]string{"log", "--no-notes", "--notes=" + port.NotesRef, "--date=raw",
		"--format=%H%x00%h%x00%s%x00%an%x00%ae%x00%ad%x00%P%x00%B%x00%N%x1e"}, args...)
}

// entries are the commits logArgs printed.
func entries(out string) []port.Entry {
	var found []port.Entry
	for _, record := range strings.Split(out, "\x1e") {
		f := strings.SplitN(strings.TrimLeft(record, "\n"), "\x00", entryFields)
		if len(f) < entryFields {
			continue
		}
		found = append(found, port.Entry{SHA: f[0], Short: f[1], Subject: f[2], Author: f[3], Email: f[4], Date: f[5],
			Merge: len(strings.Fields(f[6])) > 1, Message: f[7], Note: f[8]})
	}
	return found
}

// Show is git log -1; its error is git's, worded as git.Read words it.
func (g Git) Show(_ context.Context, rev string) (port.Entry, error) {
	out, err := g.git().Read(logArgs("-1", rev, "--")...)
	if err != nil {
		return port.Entry{}, err
	}
	if found := entries(out); len(found) == 1 {
		return found[0], nil
	}
	return port.Entry{}, errors.New(rev + " names no commit")
}

// Log is git log --reverse over the range; its error is git's, worded as
// git.Read words it.
func (g Git) Log(_ context.Context, r port.Range) ([]port.Entry, error) {
	out, err := g.git().Read(logArgs(append(append([]string{"--reverse"}, revs(r)...), "--")...)...)
	if err != nil {
		return nil, err
	}
	return entries(out), nil
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
