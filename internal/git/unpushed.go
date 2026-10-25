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

// UnpushedBases are the commits the unpushed commits of to grow from: the
// boundary of git rev-list --boundary over them, less any that is an
// ancestor of another (git merge-base --independent), each a commit some
// remote branch has; adds is whether any commit is unpushed. A range git
// cannot read adds with no base, which asks for everything to be judged.
func (r Runner) UnpushedBases(to string) (bases []string, adds bool) {
	lines, err := r.Lines(append([]string{"rev-list", "--boundary"}, Revs(Unpushed, to)...)...)
	if err != nil {
		return nil, true
	}
	for _, l := range lines {
		if sha, ok := strings.CutPrefix(l, "-"); ok {
			bases = append(bases, sha)
		} else {
			adds = true
		}
	}
	if len(bases) > 1 {
		if bases, err = r.Lines(append([]string{"merge-base", "--independent"}, bases...)...); err != nil {
			return nil, true
		}
	}
	return bases, adds
}

// UnpushedBase is the one commit the unpushed commits of to grow from, for a
// range command's {from}: to itself when none is unpushed, the remote
// branches' commit their history starts from when there is one, else ""
// (every commit up to to, as a new branch's range has it): with no remote,
// or with the unpushed commits growing from several remote branches' tips,
// none of them an ancestor of another, which no one start can say.
func (r Runner) UnpushedBase(to string) string {
	bases, adds := r.UnpushedBases(to)
	switch {
	case !adds:
		return to
	case len(bases) == 1:
		return bases[0]
	}
	return ""
}
