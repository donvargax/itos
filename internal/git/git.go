// Package git is how itos asks the repository: it shells out to git, as the
// TypeScript does, never reading .git itself, and always to the real git
// (Bin), never a git shim that is itos.
package git

import (
	"errors"
	"os/exec"
	"strings"
)

// Output is a git command's stdout, its stderr dropped; the error when git
// fails or is missing.
func Output(args ...string) (string, error) {
	out, err := exec.Command(Bin(), args...).Output()
	return string(out), err
}

// Succeeds is whether a git command exits 0, its output dropped.
func Succeeds(args ...string) bool {
	return exec.Command(Bin(), args...).Run() == nil
}

// HasCommit is whether the repository has the commit.
func HasCommit(sha string) bool { return Succeeds("cat-file", "-e", sha+"^{commit}") }

// Shallow is whether the repository is a shallow clone: its history stops
// short of commits the repository it was cloned from has.
func Shallow() bool {
	out, err := Output("rev-parse", "--is-shallow-repository")
	return err == nil && strings.TrimSpace(out) == "true"
}

// EmptyTree is the tree a commit with no parent is compared with.
const EmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Parent is a commit's first parent, or EmptyTree for a root commit.
func Parent(sha string) string {
	out, err := Output("rev-parse", "--verify", "--quiet", sha+"^")
	if parent := strings.TrimSpace(out); err == nil && parent != "" {
		return parent
	}
	return EmptyTree
}

// Read is a git command's stdout, as Output; when git fails, an error worded
// as Node's execFileSync words it (`Command failed: git …`), its stderr
// dropped as the TypeScript's git() drops it.
func Read(args ...string) (string, error) {
	out, err := Output(args...)
	if err != nil {
		return "", errors.New("Command failed: git " + strings.Join(args, " "))
	}
	return out, nil
}

// Lines are a git command's output lines, the empty ones dropped.
func Lines(args ...string) ([]string, error) {
	out, err := Read(args...)
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
func Paths(args ...string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}
	out, err := Read(append([]string{args[0], "-z"}, args[1:]...)...)
	if err != nil {
		return nil, err
	}
	return splitNUL(out), nil
}

// splitNUL are the entries of a NUL-separated list, the empty ones dropped.
func splitNUL(out string) []string {
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
func CommitPaths(sha string) ([]string, error) {
	return Paths("diff-tree", "--no-commit-id", "--name-only", "--no-renames", "-r", "--root", sha)
}
