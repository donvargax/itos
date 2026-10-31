package git

// A merge commit's own changes (bugs 30, 158 and 159): the commit rules
// judge a merge by what it changes itself, never by what its parents bring,
// which their own commits were judged by. A two-parent merge's own changes
// are the paths where its tree differs from the tree git's automatic merge
// of its parents writes (`git merge-tree --write-tree`, conflict markers and
// all), compared with --no-renames, and every path that merge reports
// conflicted, whatever the merge's tree holds: what git's merge did not
// write, or could not settle, is the merge's own, whatever made it. A merge
// that commits merge-tree's conflict tree as it is, markers and all, or the
// changed side of a modify/delete conflict it kept, changes those paths. A
// conflict merge-tree lists no path for (a directory rename split) is read
// by the trees alone. A clean merge has none, even where both sides
// changed neighbouring lines of one file; a conflict resolved, by new lines
// or by taking a side, is the merge's own, and so is all a merge made with
// -s ours, -X ours or -X theirs changed from git's merge, or one that relied
// on a merge driver, merge.renormalize or merge.renames set in one clone.
// The paths of the dense combined diff (`git diff-tree --cc`), which bug 30
// read, count a clean merge's hunk holding both sides' lines. A merge of
// more than two parents, which merge-tree cannot merge, and one of two with
// no merge base (an --allow-unrelated-histories merge, or one read in a
// shallow clone whose base was cut off), which it refuses, are still read by
// --cc: the paths with a hunk whose lines in the merge are no parent's.

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// Parents are a commit's parents, the first first; none for a root commit.
func (r Runner) Parents(sha string) ([]string, error) {
	out, err := r.Read("rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return nil, nil
	}
	return fields[1:], nil
}

// MergeHeads are the commits a merge in progress brings into HEAD, as git
// keeps them in MERGE_HEAD until the merge is committed or aborted; none
// when no merge is in progress.
func (r Runner) MergeHeads() []string {
	file, err := r.Output("rev-parse", "--git-path", "MERGE_HEAD")
	if err != nil {
		return nil
	}
	text, err := os.ReadFile(r.in(file))
	if err != nil {
		return nil
	}
	return strings.Fields(string(text))
}

// combinedHeader starts each path's part of a dense combined diff.
const combinedHeader = "diff --cc "

// OwnPaths are the paths a merge commit changes of its own, in git's order:
// for a merge of two parents with a merge base, those where its tree differs
// from git's merge of them and those git's merge reports conflicted; else
// those its dense combined diff has a hunk in.
func (r Runner) OwnPaths(merge string) ([]string, error) {
	parents, err := r.Parents(merge)
	if err != nil {
		return nil, err
	}
	if len(parents) != 2 || !r.Succeeds("merge-base", parents[0], parents[1]) {
		return r.combinedPaths(merge)
	}
	tree, conflicted, err := r.mergedTree(parents[0], parents[1])
	if err != nil {
		return nil, err
	}
	paths, err := r.Paths("diff-tree", "--no-commit-id", "--name-only", "--no-renames", "-r", tree, merge)
	if err != nil {
		return nil, err
	}
	paths = append(paths, conflicted...)
	slices.Sort(paths)
	return slices.Compact(paths), nil
}

// mergedTree is the tree git's automatic merge of the two commits writes,
// with conflict markers where it exits 1 on a conflict, and the paths it
// reports conflicted, each once: with --name-only and -z merge-tree prints
// the tree and then each conflicted path, every one ended by a NUL.
func (r Runner) mergedTree(first, second string) (string, []string, error) {
	args := []string{"merge-tree", "--write-tree", "--name-only", "--no-messages", "-z", first, second}
	out, err := r.Output(args...)
	if exit := (*exec.ExitError)(nil); errors.As(err, &exit) && exit.ExitCode() == 1 {
		err = nil
	}
	if err != nil {
		return "", nil, r.failed(args, err)
	}
	tree, conflicted, _ := strings.Cut(out, "\x00")
	return tree, SplitNUL(conflicted), nil
}

// combinedPaths are the paths a merge's dense combined diff has a hunk in,
// in git's order. A path git quotes (a quote, a backslash, a control
// character or a byte above ASCII in it) is unquoted.
func (r Runner) combinedPaths(merge string) ([]string, error) {
	out, err := r.Read("diff-tree", "--cc", "--no-commit-id", merge)
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, line := range strings.Split(out, "\n") {
		path, ok := strings.CutPrefix(line, combinedHeader)
		if !ok {
			continue
		}
		if strings.HasPrefix(path, `"`) {
			if unquoted, err := strconv.Unquote(path); err == nil {
				path = unquoted
			}
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// StagedOwnPaths are the paths the commit being made, a merge of the
// parents, changes of its own: the index written as a tree and committed on
// the parents, an object no ref points to and git collects in time, then
// that commit's own paths, so that the hook and verify judge a merge by one
// reading. The commit, which nothing keeps, is made as itos (where git's
// environment names no one) and never signed.
func (r Runner) StagedOwnPaths(parents []string) ([]string, error) {
	tree, err := r.Read("write-tree")
	if err != nil {
		return nil, err
	}
	args := []string{"-c", "user.name=itos", "-c", "user.email=itos@localhost",
		"commit-tree", "--no-gpg-sign", strings.TrimSpace(tree), "-m", "the merge being made"}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	sha, err := r.Read(args...)
	if err != nil {
		return nil, err
	}
	return r.OwnPaths(strings.TrimSpace(sha))
}
