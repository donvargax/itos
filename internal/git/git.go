package git

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/kind"
)

// Runner runs the real git of its platform (Bin), the OS whose file names
// and modes the PATH is read by, as GOOS names it, in the folder Dir, ""
// for the one itos runs in.
type Runner struct{ Platform, Dir string }

// Command is git with the arguments, ready to run under ctx.
func (r Runner) Command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, Bin(r.Platform), args...)
	cmd.Dir = r.Dir
	return cmd
}

// Output is a git command's stdout, its stderr dropped; the error when git
// fails or is missing.
func (r Runner) Output(args ...string) (string, error) {
	out, err := r.Command(context.Background(), args...).Output()
	return string(out), err
}

// Succeeds is whether a git command exits 0, its output dropped.
func (r Runner) Succeeds(args ...string) bool {
	return r.Command(context.Background(), args...).Run() == nil
}

// Shallow is whether the repository is a shallow clone: its history stops
// short of commits the repository it was cloned from has.
func (r Runner) Shallow() bool {
	out, err := r.Output("rev-parse", "--is-shallow-repository")
	return err == nil && strings.TrimSpace(out) == "true"
}

// EmptyTree is the tree a commit with no parent is compared with.
const EmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Parent is a commit's first parent, or EmptyTree for a root commit.
func (r Runner) Parent(sha string) string {
	out, err := r.Output("rev-parse", "--verify", "--quiet", sha+"^")
	if parent := strings.TrimSpace(out); err == nil && parent != "" {
		return parent
	}
	return EmptyTree
}

// Read is a git command's stdout, as Output; when git fails, an error worded
// as Node's execFileSync words it (`Command failed: git …`), its stderr
// dropped as the TypeScript's git() drops it. A git that did not run, or ran
// where there is no git repository, is a missing environment (kind.Missing,
// exit 3); any other failure has no kind.
func (r Runner) Read(args ...string) (string, error) {
	out, err := r.Output(args...)
	if err != nil {
		return "", r.failed(args, err)
	}
	return out, nil
}

// failed is Read's error for a git command that failed with err.
func (r Runner) failed(args []string, err error) error {
	failed := errors.New("Command failed: git " + strings.Join(args, " "))
	var exit *exec.ExitError
	if !errors.As(err, &exit) || !r.Succeeds("rev-parse", "--git-dir") {
		return kind.Wrap(kind.Missing, failed)
	}
	return failed
}

// Lines are a git command's output lines, the empty ones dropped.
func (r Runner) Lines(args ...string) ([]string, error) {
	out, err := r.Read(args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

// Paths are the paths a git command lists (diff --name-only, diff-tree,
// ls-files, ls-tree, log --name-only), read NUL-separated: the command runs
// with -z after its subcommand, args[0], so each path comes back as it is
// named (bug 31). Without it git C-quotes a path holding a letter outside
// ASCII, a quote, a backslash or a control character ("src/caf\303\251.js"),
// which no rule's glob then matches; core.quotePath=false alone still quotes
// all but the letters. Empty entries are dropped.
func (r Runner) Paths(args ...string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}
	out, err := r.Read(append([]string{args[0], "-z"}, args[1:]...)...)
	if err != nil {
		return nil, err
	}
	return SplitNUL(out), nil
}

// SplitNUL are the entries of a NUL-separated list, the empty ones dropped.
func SplitNUL(out string) []string {
	var entries []string
	for _, e := range strings.Split(out, "\x00") {
		if e != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

// CommitPaths are the paths a commit touches, against its parent, or every
// path it holds for a root commit; a rename is both its paths (bug 32), as
// the commit-msg hook reads the staged ones.
func (r Runner) CommitPaths(sha string) ([]string, error) {
	return r.Paths("diff-tree", "--no-commit-id", "--name-only", "--no-renames", "-r", "--root", sha)
}

// in is a path git printed, from the folder it ran in.
func (r Runner) in(printed string) string {
	p := strings.TrimSpace(printed)
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(r.Dir, p)
}
