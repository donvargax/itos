// Package git is how itos asks the repository: it shells out to git, as the
// TypeScript does, never reading .git itself.
package git

import (
	"os/exec"
	"strings"
)

// Output is a git command's stdout, its stderr dropped; the error when git
// fails or is missing.
func Output(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return string(out), err
}

// Succeeds is whether a git command exits 0, its output dropped.
func Succeeds(args ...string) bool {
	return exec.Command("git", args...).Run() == nil
}

// HasCommit is whether the repository has the commit.
func HasCommit(sha string) bool { return Succeeds("cat-file", "-e", sha+"^{commit}") }

// Shallow is whether the repository is a shallow clone: its history stops
// short of commits the repository it was cloned from has.
func Shallow() bool {
	out, err := Output("rev-parse", "--is-shallow-repository")
	return err == nil && strings.TrimSpace(out) == "true"
}
