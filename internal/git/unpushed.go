package git

// The unpushed commits (slice 34, features/stealth.feature). In a repository
// that does not use itos, others' commits follow no rules of the person's,
// so under a stealth config verify and ci plan given no range judge the
// commits of HEAD that no remote branch has: HEAD --not --remotes, every
// commit of HEAD when there is no remote. A range's start is a string
// everywhere a range goes (verify, ci plan, the footers' readers), so the
// unpushed commits are the range from Unpushed, which Revs turns into the
// arguments git rev-list and git log take.

import "strings"

// Unpushed is the start of the range that holds the commits of its end that
// no remote branch has.
const Unpushed = "--remotes"

// Revs are a range's commits as git rev-list and git log take them: from..to,
// or to --not --remotes from Unpushed.
func Revs(from, to string) []string {
	if from == Unpushed {
		return []string{to, "--not", "--remotes"}
	}
	return []string{from + ".." + to}
}

// UnpushedBase is the one commit the unpushed commits of to grow from, for a
// range command's {from}: to itself when none is unpushed, the remote
// branches' commit their history starts from when there is one, else ""
// (every commit up to to, as a new branch's range has it): with no remote,
// or with the unpushed commits growing from several remote branches' tips,
// none of them an ancestor of another, which no one start can say.
func UnpushedBase(to string) string {
	lines, err := Lines(append([]string{"rev-list", "--boundary"}, Revs(Unpushed, to)...)...)
	if err != nil {
		return ""
	}
	if len(lines) == 0 {
		return to
	}
	var bounds []string
	for _, l := range lines {
		if sha, ok := strings.CutPrefix(l, "-"); ok {
			bounds = append(bounds, sha)
		}
	}
	if len(bounds) > 1 {
		if bounds, err = Lines(append([]string{"merge-base", "--independent"}, bounds...)...); err != nil {
			return ""
		}
	}
	if len(bounds) != 1 {
		return ""
	}
	return bounds[0]
}

// UnpushedPaths are the paths the unpushed commits of to touch, each once,
// in the order git log first names them: the files the range changed, as
// git diff names them for a range with one start.
func UnpushedPaths(to string) ([]string, error) {
	lines, err := Lines(append([]string{"log", "--format=", "--name-only"}, Revs(Unpushed, to)...)...)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var paths []string
	for _, p := range lines {
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths, nil
}
