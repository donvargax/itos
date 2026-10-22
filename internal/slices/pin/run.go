package pin

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
)

// The exit codes pin gives itself (itos --help): a pin on the release with
// other checksums, and a release that cannot be fetched, by its kind.
const (
	policy    = 1
	usage     = 2
	missing   = 3
	software  = 70
	temporary = 75
)

// Run moves the pin and says so, from what version, with the release's
// notes, or under --json the one object; a pin already there says so, exit
// 0, and one with other checksums is refused, exit 1. A release that
// cannot be fetched is a line on stderr, the config left as it was, exit
// by its kind. An error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query)
	switch {
	case err != nil:
		return 0, err
	case a.Unfetched != nil:
		fmt.Fprintf(stderr, "itos: %s; %s is left as it was\n", a.Unfetched, a.File)
		return exitCode(a.Unfetched), nil
	case a.Action == refused:
		if !c.JSON {
			fmt.Fprintf(stderr, "itos: %s pins itos %s with the checksums %s, but its checksums.txt is %s now: "+
				"the release changed after it was pinned; the pin is left as it was\n", a.File, a.Version, a.WasSums, a.Checksums)
			return policy, nil
		}
		return report(stdout, a, policy)
	case a.Action == already:
		if c.JSON {
			return report(stdout, a, 0)
		}
		fmt.Fprintf(stdout, "%s already pins itos %s: nothing to change.\n", a.File, a.Version)
		return 0, nil
	}
	if c.JSON {
		return report(stdout, a, 0)
	}
	if a.Was != "" {
		fmt.Fprintf(stdout, "Pinned itos %s in %s, from %s.\n", a.Version, a.File, a.Was)
	} else {
		fmt.Fprintf(stdout, "Pinned itos %s in %s, which pinned none.\n", a.Version, a.File)
	}
	fmt.Fprintf(stdout, "Read its notes, their Upgrading section above all, before you commit the change: %s\n",
		release.NotesURL(a.Version))
	return 0, nil
}

// report is --json's one object, exiting code: the config, the action, the
// release and its checksums, the version pinned before if any, and the
// notes of a release pinned.
func report(stdout io.Writer, a Answer, code int) (int, error) {
	fields := []out.Field{{Key: "config", Value: a.File}, {Key: "action", Value: a.Action},
		{Key: "version", Value: a.Version}, {Key: "checksums", Value: a.Checksums}}
	if a.Was != "" {
		fields = append(fields, out.Field{Key: "previous", Value: a.Was})
	}
	if a.Action == pinned {
		fields = append(fields, out.Field{Key: "notes", Value: release.NotesURL(a.Version)})
	}
	return code, out.Emit(stdout, fields...)
}

// exitCode is the exit code of a release that cannot be fetched, by the
// error's kind, as internal/cli's ExitCode gives it.
func exitCode(err error) int {
	switch kind.Of(err) {
	case kind.Usage:
		return usage
	case kind.Missing:
		return missing
	case kind.Temporary:
		return temporary
	}
	return software
}
