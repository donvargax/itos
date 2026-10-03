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

// CommitPaths are the paths a commit touches, against its parent, or every
// path it holds for a root commit.
func CommitPaths(sha string) ([]string, error) {
	return Lines("diff-tree", "--no-commit-id", "--name-only", "-r", "--root", sha)
}
