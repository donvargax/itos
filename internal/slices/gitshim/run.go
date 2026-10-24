package gitshim

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/donvargax/itos/v8/internal/out"
)

// The exit codes git-shim gives itself (itos --help): a git left alone, and
// a link that cannot be placed, made or removed.
const (
	policy  = 1
	missing = 3
)

// Run does what c asks and says so, or under --json the one object. run
// exits as the command it runs; a git left alone is refused, exit 1; a link
// that cannot be placed, made or removed is a line on stderr, exit 3. An
// error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query)
	switch {
	case c.Sub == run:
		return a.Code, err
	case err != nil:
		return 0, err
	case a.Failed != nil:
		fmt.Fprintf(stderr, "itos: %s\n", a.Failed)
		return missing, nil
	case a.Action == refused:
		return c.refused(stdout, stderr, a.Link)
	case c.Sub == uninstall:
		return c.uninstalled(stdout, a)
	}
	if c.JSON {
		fields := []out.Field{{Key: "link", Value: a.Link}, {Key: "target", Value: a.Target}, {Key: "action", Value: a.Action},
			{Key: "on_path", Value: a.At >= 0}, {Key: "before_git", Value: a.At >= 0 && (a.GitAt < 0 || a.At < a.GitAt)}}
		if a.Git != "" {
			fields = append(fields, out.Field{Key: "git", Value: a.Git})
		}
		return 0, out.Emit(stdout, fields...)
	}
	switch a.Action {
	case linked:
		fmt.Fprintf(stdout, "Linked %s to %s.\n", a.Link, a.Target)
	case replaced:
		fmt.Fprintf(stdout, "Replaced %s, another itos, with a link to %s.\n", a.Link, a.Target)
	default:
		fmt.Fprintf(stdout, "%s already links to %s.\n", a.Link, a.Target)
	}
	fmt.Fprintln(stdout, standingLine(filepath.Dir(a.Link), a.At, a.Git, a.GitAt))
	return 0, nil
}

// uninstalled says the link removed, or that there was none.
func (c CLI) uninstalled(stdout io.Writer, a Answer) (int, error) {
	if c.JSON {
		return 0, out.Emit(stdout, out.Field{Key: "link", Value: a.Link}, out.Field{Key: "action", Value: a.Action})
	}
	if a.Action == absent {
		fmt.Fprintf(stdout, "No git shim at %s: nothing to remove.\n", a.Link)
	} else {
		fmt.Fprintf(stdout, "Removed %s.\n", a.Link)
	}
	return 0, nil
}

// refused reports a git left alone, exit 1.
func (c CLI) refused(stdout, stderr io.Writer, link string) (int, error) {
	if c.JSON {
		return policy, out.Emit(stdout, out.Field{Key: "link", Value: link}, out.Field{Key: "action", Value: refused})
	}
	why := "is a git that is not itos; git-shim install never replaces one"
	if c.Sub == uninstall {
		why = "is a git that is not itos; git-shim uninstall never removes one"
	}
	fmt.Fprintf(stderr, "itos: %s %s; left alone\n", link, why)
	return policy, nil
}

// standingLine says where the link's folder stands on the PATH, and what to
// do when git does not run the shim yet.
func standingLine(dir string, at int, realGit string, realAt int) string {
	switch {
	case at < 0 && realGit != "":
		return fmt.Sprintf("%s is not on the PATH: put it before %s, the real git's folder, for git to run the shim.",
			dir, filepath.Dir(realGit))
	case at < 0:
		return fmt.Sprintf("%s is not on the PATH, and the PATH has no other git for the shim to run.", dir)
	case realGit == "":
		return fmt.Sprintf("%s is on the PATH, but the PATH has no other git for the shim to run: install git.", dir)
	case at < realAt:
		return fmt.Sprintf("%s comes before the real git (%s) on the PATH: git runs the shim, and in a repository "+
			"itos manages git commit and git push are itos's.", dir, realGit)
	}
	return fmt.Sprintf("%s comes after the real git (%s) on the PATH: put it first for git to run the shim.", dir, realGit)
}
