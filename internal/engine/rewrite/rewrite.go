package rewrite

import (
	"context"
	"strings"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Dropped are the commits of old that main lacks, oldest first, of which the
// commits of new that main lacks hold no counterpart; an error when either
// range cannot be read.
func Dropped(ctx context.Context, v vcsport.VCS, main, old, new string) ([]vcsport.Entry, error) {
	had, err := v.Log(ctx, vcsport.Range{From: main, To: old})
	if err != nil {
		return nil, err
	}
	now, err := v.Log(ctx, vcsport.Range{From: main, To: new})
	if err != nil {
		return nil, err
	}
	kept := map[string]bool{}
	for _, e := range now {
		kept[counterpart(e)] = true
	}
	var dropped []vcsport.Entry
	for _, e := range had {
		if !kept[counterpart(e)] {
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
