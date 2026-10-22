package gocmd

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/donvargax/itos/v8/internal/engine/guides"
	"github.com/donvargax/itos/v8/internal/guide"
	"github.com/donvargax/itos/v8/internal/out"
)

// Run is the go command over h: the coordinator's guide and the notes on
// stdout, then, after a line of ---, where things stand; under --json one
// object, its guide, text, the notes' files and status. It always exits 0;
// an error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a := h.Handle(ctx, Out{Stderr: stderr})
	g, st := a.Guide, a.Standing
	if c.JSON {
		fields := []out.Field{{Key: "guide", Value: guide.Coordinate}, {Key: "text", Value: g.Text}}
		if g.Notes != "" {
			fields = append(fields, out.Field{Key: "notes", Value: filepath.ToSlash(g.Notes)})
		}
		if g.Local != "" {
			fields = append(fields, out.Field{Key: "local_notes", Value: filepath.ToSlash(g.Local)})
		}
		if st != nil {
			fields = append(fields, out.Field{Key: "status", Value: st})
		}
		return 0, out.Emit(stdout, fields...)
	}
	if _, err := fmt.Fprint(stdout, g.Text); err != nil || st == nil {
		return 0, err
	}
	fmt.Fprint(stdout, guides.Separator)
	st.Print(stdout)
	return 0, nil
}
