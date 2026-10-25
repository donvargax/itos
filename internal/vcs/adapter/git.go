package adapter

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// Git is the VCS over git, which internal/git runs. Platform is the OS whose
// PATH the real git is found on, as GOOS names it, when ITOS_GIT names
// none: the assembly sets it, and exports the git it finds (Export) before
// anything else runs, so a Git made later without it runs that git. Dir is
// the folder git runs in, "" for the one itos runs in: one clone of
// several a test runs side by side.
type Git struct{ Platform, Dir string }

var _ port.VCS = Git{}

// git is internal/git's runner on the platform.
func (g Git) git() git.Runner { return git.Runner{Platform: g.Platform, Dir: g.Dir} }

// Export sets ITOS_GIT to the real git, when there is one, for everything
// the run starts, so that none of it runs the git shim.
func (g Git) Export() { git.Export(g.Platform) }

// Real is the first git on the PATH that is no itos (internal/git's Real);
// an error, exit 3, when there is none.
func (g Git) Real() (string, error) { return git.Real(g.Platform) }

// Inherited is the git ITOS_GIT names, "" when it is unset or names an
// itos.
func (Git) Inherited() string { return git.Inherited() }

// IsSelf is whether the file at p, its links followed, is this binary.
func (Git) IsSelf(p string) bool { return git.IsSelf(p) }

// IsItos identifies an itos executable across runtime versions without running it.
func (Git) IsItos(p string) bool { return git.IsItos(p) }

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

// Folder is Dir.
func (g Git) Folder(context.Context) string { return g.Dir }

// Locate asks git for the top, the git folder and the common dir in one
// run; inside the git folder, where there is no top, it asks again for the
// other two. Asked once per folder and git, the folder dir from Dir; asked
// every time for a relative folder from the process's.
func (g Git) Locate(_ context.Context, dir string) (port.Location, error) {
	at := dir
	if !filepath.IsAbs(at) {
		if g.Dir == "" {
			return g.locate(dir)
		}
		at = filepath.Join(g.Dir, dir)
	}
	key := filepath.Clean(at) + "\x00" + git.Bin(g.Platform)
	locateMu.Lock()
	defer locateMu.Unlock()
	if f, ok := located[key]; ok {
		return f.l, f.err
	}
	l, err := g.locate(dir)
	located[key] = found{l, err}
	return l, err
}

func (g Git) locate(dir string) (port.Location, error) {
	if out, err := g.git().Output(in(dir, "rev-parse", "--show-toplevel", "--git-dir", "--git-common-dir")...); err == nil {
		if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) == 3 {
			return port.Location{Top: native(lines[0]), GitDir: native(lines[1]), Common: native(lines[2])}, nil
		}
	}
	out, err := g.git().Read(in(dir, "rev-parse", "--git-common-dir", "--git-dir")...)
	if err != nil {
		return port.Location{}, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		return port.Location{}, fmt.Errorf("no git repository holds the folder %q", dir)
	}
	return port.Location{GitDir: native(lines[1]), Common: native(lines[0])}, nil
}

// native is a path git printed in the system's form, cleaned: git prints
// forward slashes on Windows too.
func native(p string) string { return filepath.Clean(strings.TrimSpace(p)) }

func (g Git) Resolve(_ context.Context, rev string) (string, error) {
	out, err := g.git().Output("rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if sha := strings.TrimSpace(out); err == nil && sha != "" {
		return sha, nil
	}
	return "", errors.New(rev + " names no commit")
}

func (g Git) Shallow(context.Context) bool { return g.git().Shallow() }

func (g Git) IsAncestor(_ context.Context, ancestor, rev string) bool {
	return g.git().Succeeds("merge-base", "--is-ancestor", ancestor, rev)
}

func (g Git) Parent(_ context.Context, rev string) string { return g.git().Parent(rev) }

func (g Git) Parents(_ context.Context, rev string) ([]string, error) { return g.git().Parents(rev) }

// FirstParents is git rev-list --first-parent; its error is git's, worded
// as git.Read words it.
func (g Git) FirstParents(_ context.Context, rev string, skip, most int) ([]string, error) {
	return g.git().Lines("rev-list", "--first-parent", fmt.Sprintf("--skip=%d", skip),
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
func (g Git) Commits(_ context.Context, r port.Range) ([]string, error) {
	return g.git().Lines(append([]string{"rev-list", "--reverse"}, revs(r)...)...)
}

func (g Git) Readable(_ context.Context, r port.Range) bool {
	return g.git().Succeeds(append([]string{"rev-list", "--quiet"}, revs(r)...)...)
}

func (g Git) UnpushedBase(_ context.Context, to string) string { return g.git().UnpushedBase(to) }

func (g Git) UnpushedBases(_ context.Context, to string) ([]string, bool) {
	return g.git().UnpushedBases(to)
}

// EnvGit names the real git, absolute: set by itos for every program it
// starts, read by every itos those start.
const EnvGit = git.EnvGit
