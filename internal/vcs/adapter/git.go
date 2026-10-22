package adapter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// Git is the VCS over git, which internal/git runs.
type Git struct{}

var _ port.VCS = Git{}

var (
	locateMu sync.Mutex
	located  = map[string]found{}
)

// found is what Locate found for a folder.
type found struct {
	l   port.Location
	err error
}

// in is git's arguments run from the folder dir.
func in(dir string, args ...string) []string {
	if dir == "" {
		return args
	}
	return append([]string{"-C", dir}, args...)
}

// Locate asks git for the top, the git folder and the common dir in one
// run; inside the git folder, where there is no top, it asks again for the
// other two. Asked once per working folder and dir.
func (Git) Locate(_ context.Context, dir string) (port.Location, error) {
	key := dir
	if cwd, err := os.Getwd(); err == nil {
		key = cwd + "\x00" + dir
	}
	locateMu.Lock()
	defer locateMu.Unlock()
	if f, ok := located[key]; ok {
		return f.l, f.err
	}
	l, err := locate(dir)
	located[key] = found{l, err}
	return l, err
}

func locate(dir string) (port.Location, error) {
	if out, err := git.Output(in(dir, "rev-parse", "--show-toplevel", "--git-dir", "--git-common-dir")...); err == nil {
		if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) == 3 {
			return port.Location{Top: lines[0], GitDir: lines[1], Common: lines[2]}, nil
		}
	}
	out, err := git.Output(in(dir, "rev-parse", "--git-dir", "--git-common-dir")...)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if err != nil || len(lines) != 2 {
		return port.Location{}, fmt.Errorf("no git repository holds the folder %q", dir)
	}
	return port.Location{GitDir: lines[0], Common: lines[1]}, nil
}

func (Git) Resolve(_ context.Context, rev string) (string, error) {
	out, err := git.Output("rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if sha := strings.TrimSpace(out); err == nil && sha != "" {
		return sha, nil
	}
	return "", errors.New(rev + " names no commit")
}

func (Git) Shallow(context.Context) bool { return git.Shallow() }

func (Git) IsAncestor(_ context.Context, ancestor, rev string) bool {
	return git.Succeeds("merge-base", "--is-ancestor", ancestor, rev)
}

func (Git) Parent(_ context.Context, rev string) string { return git.Parent(rev) }

func (Git) Parents(_ context.Context, rev string) ([]string, error) { return git.Parents(rev) }

// FirstParents is git rev-list --first-parent; its error is git's, worded
// as git.Read words it.
func (Git) FirstParents(_ context.Context, rev string, skip, most int) ([]string, error) {
	return git.Lines("rev-list", "--first-parent", fmt.Sprintf("--skip=%d", skip),
		fmt.Sprintf("--max-count=%d", most), rev, "--")
}

// revs are a range as git rev-list takes it: Since first, since --not
// would turn it round, then From..To, To alone, or To --not --remotes.
func revs(r port.Range) []string {
	var args []string
	if r.Since != "" {
		args = append(args, "^"+r.Since)
	}
	switch r.From {
	case "":
		return append(args, r.To)
	case port.Unpushed:
		return append(args, git.Revs(git.Unpushed, r.To)...)
	}
	return append(args, git.Revs(r.From, r.To)...)
}

// Commits is git rev-list --reverse; its error is git's, worded as git.Read
// words it, which verify prints.
func (Git) Commits(_ context.Context, r port.Range) ([]string, error) {
	return git.Lines(append([]string{"rev-list", "--reverse"}, revs(r)...)...)
}

func (Git) Readable(_ context.Context, r port.Range) bool {
	return git.Succeeds(append([]string{"rev-list", "--quiet"}, revs(r)...)...)
}

func (Git) UnpushedBase(_ context.Context, to string) string { return git.UnpushedBase(to) }
