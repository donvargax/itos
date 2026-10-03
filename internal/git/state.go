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
// git status --short prints them; untracked files are not among them.
func Changed() []string {
	lines, _ := Lines("status", "--porcelain", "--untracked-files=no")
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
	lines, _ := Lines("diff", "--name-only", "--diff-filter=U")
	return lines
}
