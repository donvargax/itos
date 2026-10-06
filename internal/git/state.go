package git

// The working tree's state, as itos push reads it before it pulls and before
// it pushes (slice 39, features/push.feature): the branch and its upstream,
// whether tracked files have uncommitted changes, and whether a rebase is
// in progress or a conflict is left, the guard docs/ORCHESTRATING.md wrote
// as a one-liner before every push.

import (
	"os"
	"strings"
)

// Branch is the branch HEAD is on, its short name; "" when HEAD is detached.
func Branch() string {
	out, err := Output("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Upstream is where the branch pulls from and pushes to: its configured
// remote and the remote branch it merges (branch.<name>.remote and
// branch.<name>.merge), else origin and refs/heads/<name>; set is whether
// the branch has an upstream of its own.
func Upstream(branch string) (remote, ref string, set bool) {
	remote = strings.TrimSpace(config("branch." + branch + ".remote"))
	ref = strings.TrimSpace(config("branch." + branch + ".merge"))
	if remote != "" && ref != "" {
		return remote, ref, true
	}
	return "origin", "refs/heads/" + branch, false
}

// config is a git config value, "" when it is not set.
func config(key string) string {
	out, _ := Output("config", "--get", key)
	return out
}

// Changed are the tracked files with uncommitted changes, staged or not, as
// git status --short prints them, each path as it is named (bug 31): read
// NUL-separated, where a rename or a copy names its new path and then its
// old one, which the line puts first (`R  old -> new`); untracked files are
// not among them.
func Changed() []string {
	out, _ := Output("status", "-z", "--porcelain", "--untracked-files=no")
	entries := splitNUL(out)
	var lines []string
	for i := 0; i < len(entries); i++ {
		line := entries[i]
		if len(line) > 3 && strings.ContainsAny(line[:2], "RC") && i+1 < len(entries) {
			i++
			line = line[:3] + entries[i] + " -> " + line[3:]
		}
		lines = append(lines, line)
	}
	return lines
}

// Rebasing is whether a rebase is in progress: git keeps its state in
// rebase-merge or rebase-apply in the git folder until it ends.
func Rebasing() bool {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		dir, err := Output("rev-parse", "--git-path", name)
		if err != nil {
			continue
		}
		if info, err := os.Stat(strings.TrimSpace(dir)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// Conflicted are the paths a merge or a rebase left unmerged.
func Conflicted() []string {
	lines, _ := Paths("diff", "--name-only", "--diff-filter=U")
	return lines
}
