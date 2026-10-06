package git

// A merge commit's own changes (bug 30): the commit rules judge a merge by
// what it changes itself, never by what its parents bring, which their own
// commits were judged by. A merge's own changes are the paths of its dense
// combined diff (`git diff-tree --cc`, what git show shows of a merge): the
// paths with a hunk whose lines in the merge are no parent's there. A clean
// merge has none, even of a file both sides changed, since each hunk of it is
// one side's; a conflict resolved by taking one side's lines has none either;
// a line the merge adds, or a resolution that writes new lines, is its own.
// The paths whose whole content matches no parent (`--cc --name-only`, or
// `-c`) would count every file both sides changed, so a plain merge of two
// lines of work would be judged as if its author had written them all; and
// the diff against git's own re-merge of the parents (`--remerge-diff`) would
// count a merge made with another strategy or option, or any conflict
// resolved by taking a side, and needs a git of 2.36 or later.

import (
	"os"
	"strconv"
	"strings"
)

// Parents are a commit's parents, the first first; none for a root commit.
func Parents(sha string) ([]string, error) {
	out, err := Read("rev-list", "--parents", "-n", "1", sha)
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
func MergeHeads() []string {
	file, err := Output("rev-parse", "--git-path", "MERGE_HEAD")
	if err != nil {
		return nil
	}
	text, err := os.ReadFile(strings.TrimSpace(file))
	if err != nil {
		return nil
	}
	return strings.Fields(string(text))
}

// combinedHeader starts each path's part of a dense combined diff.
const combinedHeader = "diff --cc "

// OwnPaths are the paths a merge commit changes of its own: those its dense
// combined diff has a hunk in, in git's order. A path git quotes (a quote, a
// backslash, a control character or a byte above ASCII in it) is unquoted.
func OwnPaths(merge string) ([]string, error) {
	out, err := Read("diff-tree", "--cc", "--no-commit-id", merge)
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
func StagedOwnPaths(parents []string) ([]string, error) {
	tree, err := Read("write-tree")
	if err != nil {
		return nil, err
	}
	args := []string{"-c", "user.name=itos", "-c", "user.email=itos@localhost",
		"commit-tree", "--no-gpg-sign", strings.TrimSpace(tree), "-m", "the merge being made"}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	sha, err := Read(args...)
	if err != nil {
		return nil, err
	}
	return OwnPaths(strings.TrimSpace(sha))
}
