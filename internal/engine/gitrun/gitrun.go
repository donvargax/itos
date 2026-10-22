package gitrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/donvargax/itos/v8/internal/message"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// signalled is the exit code of a git a signal ended, internal/cli's
// ExitPolicy.
const signalled = 1

// Config is the git config the notes' rewriting is set in, which the VCS
// port leaves to the adapter (internal/vcs/adapter's Git is one): a key's
// values, and a value added to it in the repository's own config.
type Config interface {
	Config(ctx context.Context, key string, local bool) []string
	AddConfig(ctx context.Context, key, value string) error
}

// abbreviates is whether arg is the long option full, or an abbreviation of
// it git takes, at least min characters long: git reads any unambiguous start
// of a long option as the option, so --no-veri is --no-verify.
func abbreviates(arg, full string, min int) bool {
	return len(arg) >= min && strings.HasPrefix(full, arg)
}

// NoVerify is whether the argument is --no-verify, as git commit and git
// push read it: the option, or its start down to --no-veri, the shortest
// that is no other option's.
func NoVerify(arg string) bool { return abbreviates(arg, "--no-verify", len("--no-veri")) }

// Talking is how git runs for a command the person typed: with the
// terminal's stdin, so an editor can open, its output to stdout and its
// words to stderr, an interrupt git's and the editor's to act on while it
// runs, not a reason for itos to leave first. It runs in origin, the folder
// the person stood in, so the paths and the files they gave git mean what
// they mean there, though itos moved to the repository's top to read its
// config (slice 40); in env, itos's own when nil.
func Talking(origin string, env []string, stdout, stderr io.Writer) vcsport.Talk {
	return vcsport.Talk{Dir: origin, Stdin: os.Stdin, Stdout: stdout, Stderr: stderr, Env: env, Owns: true}
}

// ExitOf is the exit code of a git command that failed (err a
// vcsport.Failed), 1 for one a signal ended; 0 when err is nil, and the
// error itself, with no code, when git could not start.
func ExitOf(err error) (int, error) {
	var failed *vcsport.Failed
	switch {
	case err == nil:
		return 0, nil
	case !errors.As(err, &failed):
		return 0, err
	case failed.Code > 0:
		return failed.Code, nil
	}
	return signalled, nil
}

// RewriteNotes adds refs/notes/itos to notes.rewriteRef in the repository's
// own config unless a value already names it, so that git commit --amend and
// git rebase, which copy the notes of the refs it names to the commits they
// make, carry a commit's footers along.
func RewriteNotes(ctx context.Context, g Config) error {
	for _, ref := range g.Config(ctx, "notes.rewriteRef", false) {
		if ok, _ := path.Match(strings.TrimSpace(ref), message.NotesRef); ok {
			return nil
		}
	}
	return g.AddConfig(ctx, "notes.rewriteRef", message.NotesRef)
}

// WriteNote writes the footers as the itos note of the commit git just made,
// replacing the one an amend carried over; nothing when HEAD is still the
// commit it was before (a dry run made none).
func WriteNote(ctx context.Context, v vcsport.VCS, before string, lines []string) error {
	head, err := v.Resolve(ctx, "HEAD")
	if err != nil {
		return err
	}
	if head == before {
		return nil
	}
	if err := v.Note(ctx, "HEAD", strings.Join(lines, "\n")); err != nil {
		return fmt.Errorf("git notes add: %s", wordsOf(err))
	}
	return nil
}

// wordsOf is what git said of a failure on stderr, when err is a command
// that failed, else err's own words (internal/cli's wordsOf).
func wordsOf(err error) string {
	var failed *vcsport.Failed
	if errors.As(err, &failed) {
		return strings.TrimSpace(failed.Words)
	}
	return err.Error()
}
