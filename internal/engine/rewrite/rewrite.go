package rewrite

import (
	"context"
	"strings"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Dropped are the commits of old that main lacks, oldest first, of which the
// commits of new that main lacks hold no counterpart; an error when either
// range cannot be read. Each commit of new stands in for one commit of old
// alone, so two commits made alike (the same message in the same second) need
// two. A merge commit has no work of its own to drop: the commits it brings in
// are listed beside it, and a rebase leaves it out.
func Dropped(ctx context.Context, v vcsport.VCS, main, old, new string) ([]vcsport.Entry, error) {
	had, err := v.Log(ctx, vcsport.Range{From: main, To: old})
	if err != nil {
		return nil, err
	}
	now, err := v.Log(ctx, vcsport.Range{From: main, To: new})
	if err != nil {
		return nil, err
	}
	available := map[string]int{}
	for _, e := range now {
		if !e.Merge {
			available[counterpart(e)]++
		}
	}
	var dropped []vcsport.Entry
	for _, e := range had {
		switch key := counterpart(e); {
		case e.Merge:
		case available[key] > 0:
			available[key]--
		default:
			dropped = append(dropped, e)
		}
	}
	return dropped, nil
}

// counterpart is what a commit shares with its counterparts: its author's
// name and email, its author date and its message.
func counterpart(e vcsport.Entry) string {
	return strings.Join([]string{e.Author, e.Email, e.Date, e.Message}, "\x00")
}
