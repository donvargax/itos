package adapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// Commit is git commit: --only and the paths for Only, each paragraph a -m,
// else, with no Args, the message read from stdin (-F -). With Args,
// trailer.ifExists is addIfDifferent, so that a trailer they give that the
// message has is not written twice.
func (g Git) Commit(ctx context.Context, c port.Commit, talk port.Talk) error {
	args := []string{"commit"}
	if len(c.Args) > 0 {
		args = []string{"-c", "trailer.ifExists=addIfDifferent", "commit"}
	}
	if len(c.Only) > 0 {
		args = append(args, "--only")
	}
	for _, p := range c.Paragraphs {
		args = append(args, "-m", p)
	}
	if len(c.Paragraphs) == 0 && len(c.Args) == 0 {
		args = append(args, "-F", "-")
		talk.Stdin = strings.NewReader(c.Message)
	}
	if c.Quiet {
		args = append(args, "--quiet")
	}
	args = append(args, c.Args...)
	if len(c.Only) > 0 {
		args = append(append(args, "--"), c.Only...)
	}
	return g.run(ctx, talk, nil, false, args...)
}

// Add is git add.
func (g Git) Add(ctx context.Context, paths []string, talk port.Talk) error {
	return g.run(ctx, talk, nil, false, append([]string{"add", "--"}, paths...)...)
}

// Unstage is git reset -q.
func (g Git) Unstage(ctx context.Context, paths []string) error {
	return g.run(ctx, literal(port.Talk{}), nil, false, append([]string{"reset", "-q", "--"}, paths...)...)
}

// Untrack is git rm -q --cached --ignore-unmatch.
func (g Git) Untrack(ctx context.Context, paths []string) error {
	return g.run(ctx, literal(port.Talk{}), nil, false, append([]string{"rm", "-q", "--cached", "--ignore-unmatch", "--"}, paths...)...)
}

// Restore is git checkout HEAD.
func (g Git) Restore(ctx context.Context, paths []string) error {
	return g.run(ctx, literal(port.Talk{}), nil, false, append([]string{"checkout", "HEAD", "--"}, paths...)...)
}

// Note is git notes add -f on the itos notes' ref.
func (g Git) Note(ctx context.Context, rev, text string) error {
	return g.run(ctx, port.Talk{}, nil, false, "notes", "--ref="+port.NotesRef, "add", "-f", "-m", text, rev)
}

// CherryPick merges the commit's change onto onto in memory (git merge-tree
// --write-tree, the commit's first parent the merge base), then commits the
// tree it wrote on onto with git commit-tree as the commit's author, at the
// commit's author date, with its message: no ref, index or work tree is
// touched. git merge-tree exits 1 for a merge with conflicts, and --name-only
// prints the tree and then the conflicted paths up to a blank line.
func (g Git) CherryPick(ctx context.Context, commit, onto string) (string, error) {
	shown, err := g.Show(ctx, commit)
	if err != nil {
		return "", err
	}
	merged, err := g.output(ctx, port.Talk{}, false, "merge-tree", "--write-tree", "--name-only",
		"--merge-base="+g.Parent(ctx, commit), onto, commit)
	lines := strings.Split(merged, "\n")
	var failed *port.Failed
	if errors.As(err, &failed) && failed.Code == 1 {
		conflicts := &port.Conflicts{}
		for _, line := range lines[1:] {
			if line == "" {
				break
			}
			conflicts.Paths = append(conflicts.Paths, line)
		}
		return "", conflicts
	}
	if err != nil {
		return "", err
	}
	as := port.Talk{Stdin: strings.NewReader(strings.TrimRight(shown.Message, "\n") + "\n"),
		Env: append(os.Environ(), "GIT_AUTHOR_NAME="+shown.Author, "GIT_AUTHOR_EMAIL="+shown.Email, "GIT_AUTHOR_DATE="+shown.Date)}
	made, err := g.output(ctx, as, false, "commit-tree", lines[0], "-p", onto, "-F", "-")
	return strings.TrimSpace(made), err
}

// Reset is git reset --hard.
func (g Git) Reset(ctx context.Context, rev string) error {
	return g.run(ctx, port.Talk{}, nil, false, "reset", "--hard", "--quiet", rev)
}

// CommitFile builds the commit through an index of its own, in a folder of
// its own so git makes it, and its lock beside it, where nothing else
// writes: parent's tree read into it (none for no parent), the file's blob
// written and set at path, the tree written and committed by the author.
func (g Git) CommitFile(ctx context.Context, parent, path, text, message, name, email string) (string, error) {
	scratch, err := os.MkdirTemp("", "itos-id-index-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	index := port.Talk{Env: append(os.Environ(), "GIT_INDEX_FILE="+filepath.Join(scratch, "index"))}
	tree := "--empty"
	if parent != "" {
		tree = parent + "^{tree}"
	}
	if err := g.run(ctx, index, nil, false, "read-tree", tree); err != nil {
		return "", err
	}
	blob, err := g.output(ctx, port.Talk{Stdin: strings.NewReader(text)}, false, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	entry := "100644," + strings.TrimSpace(blob) + "," + path
	if err := g.run(ctx, index, nil, false, "update-index", "--add", "--cacheinfo", entry); err != nil {
		return "", err
	}
	written, err := g.output(ctx, index, false, "write-tree")
	if err != nil {
		return "", err
	}
	args := []string{"commit-tree", strings.TrimSpace(written)}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	by := port.Talk{Env: append(os.Environ(), "GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL="+email)}
	commit, err := g.output(ctx, by, false, append(args, "-m", message)...)
	return strings.TrimSpace(commit), err
}
