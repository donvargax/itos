package guide

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/donvargax/itos/v8/internal/out"
)

// Run is the guide command over h: the guide or procedure on stdout, as
// written, or as --json's one object, its guide and text and the notes'
// file appended. It always exits 0; an error is the command line's to
// report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a := h.Handle(ctx, c.Query, Out{Stderr: stderr})
	if c.JSON {
		fields := []out.Field{{Key: "guide", Value: c.Name}, {Key: "text", Value: a.Text}}
		if a.Notes != "" {
			fields = append(fields, out.Field{Key: "notes", Value: filepath.ToSlash(a.Notes)})
		}
		return 0, out.Emit(stdout, fields...)
	}
	_, err := fmt.Fprint(stdout, a.Text)
	return 0, err
}
