package ask

import (
	"context"

	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// Load is the questions in file, as the tracker t loads them, the file's
// text and whether it is there: none, "" and false when it is not.
func Load(ctx context.Context, t trackerport.Tracker, file string) (File, string, bool, error) {
	docs, err := t.Load(ctx, []string{file})
	if err != nil {
		return File{}, "", false, err
	}
	if !docs[0].There {
		return File{}, "", false, nil
	}
	text := docs[0].Text
	f, err := Parse(file, text)
	return f, text, true, err
}
