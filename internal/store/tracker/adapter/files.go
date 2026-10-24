package adapter

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/donvargax/itos/v8/internal/idcounter"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/lock"
	"github.com/donvargax/itos/v8/internal/store/tracker/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Files is the tracker on disk, each document the file its name is a path
// of, relative to the folder itos runs in (the VCS's Folder). VCS is the
// repository its commits and its ID counter go to; OS the OS
// itos runs on, as GOOS names it, which Hold's lock and the local ID
// counter's wait by; Stealth a config that commits nothing, its counter
// local; Lock the file whose lock Hold takes under it; HooksReady the
// error of the hooks that check a commit not ready, nil when they are.
type Files struct {
	VCS        vcs.VCS
	OS         string
	Stealth    bool
	Lock       string
	HooksReady func(ctx context.Context) error
}

var _ port.Tracker = Files{}

// Load reads each file; one not there is a Doc not There.
func (f Files) Load(ctx context.Context, names []string) ([]port.Doc, error) {
	docs := make([]port.Doc, len(names))
	for i, name := range names {
		text, err := os.ReadFile(f.path(ctx, name))
		if errors.Is(err, fs.ErrNotExist) {
			docs[i] = port.Doc{Name: name}
			continue
		}
		if err != nil {
			return nil, as(err, name)
		}
		docs[i] = port.Doc{Name: name, Text: string(text), There: true}
	}
	return docs, nil
}

// List is os.ReadDir's names; none when the folder is not there.
func (f Files) List(ctx context.Context, dir string) ([]string, error) {
	entries, err := os.ReadDir(f.path(ctx, dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, as(err, dir)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, nil
}

// Hold is lock.Hold of Lock under Stealth, waiting up to lock.Wait for
// another itos to give it back; else at once.
func (f Files) Hold(ctx context.Context) (port.Held, error) {
	if !f.Stealth {
		return free{}, nil
	}
	held, err := lock.Hold(f.path(ctx, f.Lock), f.OS)
	if err != nil {
		return nil, err
	}
	return released{held}, nil
}

// free is the Held of a Hold that orders nothing.
type free struct{}

func (free) Release(context.Context) error { return nil }

// released is a lock.Held as the port's Held.
type released struct{ held *lock.Held }

func (r released) Release(context.Context) error { return r.held.Release() }

// Ready is HooksReady's error, else the first file with changes no commit
// holds (uncommitted), else a git that cannot say whether there are any, an
// error of kind.Unknown (exit 70) holding git's words; nil under Stealth.
func (f Files) Ready(ctx context.Context, names []string) error {
	if f.Stealth {
		return nil
	}
	if err := f.HooksReady(ctx); err != nil {
		return err
	}
	for _, name := range names {
		dirty, err := f.uncommitted(ctx, name)
		if err != nil {
			return kind.Wrap(kind.Unknown, fmt.Errorf("cannot read git's status of %s: %w", name, err))
		}
		if dirty {
			return &port.Dirty{Name: name}
		}
	}
	return nil
}

// uncommitted is whether git does not hold the file as HEAD has it, which a
// commit of it would sweep in beside the transaction's change. A file in
// the working tree is clean when git tracks it and reports no change to
// it; a file not there, which the transaction makes, when git knows nothing
// of it either: one deleted and the deletion not committed, staged or not,
// is a change no commit holds (bug 13), which a commit of the file written
// afresh would undo. A git that fails is the error, never a change: the
// repository was located and its config loaded by then, so it is a git
// failing, which no kind fits (bug-130).
func (f Files) uncommitted(ctx context.Context, name string) (bool, error) {
	_, err := os.Lstat(f.path(ctx, name))
	there := !errors.Is(err, fs.ErrNotExist)
	tracked := f.VCS.Tracked(ctx, name)
	status, err := f.VCS.PathStatus(ctx, name)
	if err != nil {
		return false, err
	}
	return len(status) != 0 || tracked != there, nil
}

// Transact writes the changes and, unless Stealth, commits them alone, in
// the order the package's doc gives.
func (f Files) Transact(ctx context.Context, t port.Transaction) (string, error) {
	files := t.Changes
	paths := make([]string, len(files))
	for i, c := range files {
		paths[i] = c.Name
	}
	if err := f.Ready(ctx, paths); err != nil {
		return "", err
	}
	named := and(paths)
	modes := make([]fs.FileMode, len(files))
	for i, c := range files {
		modes[i] = fs.FileMode(0o644)
		if info, err := os.Stat(f.path(ctx, c.Name)); err == nil {
			modes[i] = info.Mode().Perm()
		}
		if touched, err := put(f.path(ctx, c.Name), c.Text, modes[i]); err != nil {
			if !touched {
				i--
			}
			return "", f.restored(ctx, files[:i+1], modes, fmt.Errorf("%s cannot be written: %w", c.Name, as(err, c.Name)))
		}
	}
	if f.Stealth {
		return "", nil
	}
	failed := func(step string, exit *vcs.Failed) (string, error) {
		return "", &port.Refused{Step: step, Code: exit.Code, Restore: f.restored(ctx, files, modes, nil)}
	}
	talk := vcs.Talk{Stdin: t.Stdin, Stdout: t.Words, Stderr: t.Words}
	var exit *vcs.Failed
	for _, c := range files {
		// A path git does not know is no pathspec for --only until it is
		// added; what git add says goes to Words, as the commit's words do.
		if !c.Created {
			continue
		}
		if err := f.VCS.Add(ctx, []string{c.Name}, talk); errors.As(err, &exit) {
			return failed("git add of "+c.Name, exit)
		} else if err != nil {
			return "", f.restored(ctx, files, modes, fmt.Errorf("cannot run git: %w", err))
		}
	}
	talk.Env = t.Env
	commit := vcs.Commit{Paragraphs: t.Paragraphs, Only: paths, Quiet: t.Quiet}
	if err := f.VCS.Commit(ctx, commit, talk); errors.As(err, &exit) {
		return failed("the commit of "+named, exit)
	} else if err != nil {
		return "", f.restored(ctx, files, modes, fmt.Errorf("cannot run git: %w", err))
	}
	// git commit --only stages the files in the real index before the hooks
	// run on a temporary one, so a hook that rewrites them, a formatter,
	// puts its version in the commit and the working tree while the real
	// index keeps itos's (bug 17). Their entries are set to the commit's:
	// they had no change of the person's, which Ready refused above.
	if err := f.VCS.Unstage(ctx, paths); err != nil {
		return "", fmt.Errorf("the commit of %s was made, but git's index of them cannot be set to it: %w", named, err)
	}
	return f.VCS.Resolve(ctx, "HEAD")
}

// Reserve is idcounter.Claim of the run at the counter's store (counter),
// located first, as the counter has always been.
func (f Files) Reserve(ctx context.Context, series string, floor, count int) (int, error) {
	store, err := f.counter(ctx)
	if err != nil {
		return 0, err
	}
	return idcounter.Claim(store, series, floor, count)
}

// counter is where this repository's ID counter is: the remote itos push
// pushes to (the branch's upstream, else origin, as the VCS's Upstream
// gives it) when there is one, else, and always under Stealth, the file in
// the git common dir. The repository is located and its config loaded by
// then, so a locate that fails here is a git failing, which no kind fits
// (exit 70), not the missing repository internal/git's Read takes it for
// (bug 119).
func (f Files) counter(ctx context.Context) (idcounter.Store, error) {
	l, err := f.VCS.Locate(ctx, "")
	if err != nil {
		return idcounter.Store{}, kind.Wrap(kind.Unknown, err)
	}
	root := f.VCS.Folder(ctx)
	common := f.path(ctx, l.Common)
	remote := ""
	if !f.Stealth {
		remote, _, _ = f.VCS.Upstream(ctx, f.VCS.Branch(ctx))
		if _, err := f.VCS.RemoteURL(ctx, remote); err != nil {
			remote = ""
		}
	}
	return idcounter.Store{Root: root, CommonDir: common, Remote: remote, OS: f.OS, VCS: f.VCS, LocalOnly: f.Stealth}, nil
}

// path is the file name names, read from the folder itos runs in.
func (f Files) path(ctx context.Context, name string) string { return vcs.Within(ctx, f.VCS, name) }

// as is err naming the file as it was named, not as it was read from the
// folder itos runs in.
func as(err error, name string) error {
	var failed *fs.PathError
	if errors.As(err, &failed) {
		failed.Path = name
	}
	return err
}

// put writes the text to the file in the mode, as os.WriteFile does, and
// says whether it touched the file: false when the file could not be opened,
// so it is as it was.
func put(path, text string, mode fs.FileMode) (bool, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return false, err
	}
	_, err = f.WriteString(text)
	return true, errors.Join(err, f.Close())
}

// restore puts the files back as they were before the transaction wrote
// them, and git's index for them as HEAD has it (they were clean before): a
// file that was there gets its old text back, in its mode, where it
// differs; a created one is unstaged and removed. Each file is put back
// whatever failed with another, and what failed is the error, naming the
// file.
func (f Files) restore(ctx context.Context, files []port.Change, modes []fs.FileMode) error {
	var failed error
	for i, c := range files {
		var err error
		if c.Created {
			if !f.Stealth {
				err = f.VCS.Untrack(ctx, []string{c.Name})
			}
			if rm := os.Remove(f.path(ctx, c.Name)); rm != nil && !errors.Is(rm, fs.ErrNotExist) {
				err = errors.Join(err, as(rm, c.Name))
			}
		} else {
			if now, read := os.ReadFile(f.path(ctx, c.Name)); read != nil || string(now) != c.Old {
				err = as(os.WriteFile(f.path(ctx, c.Name), []byte(c.Old), modes[i]), c.Name)
			}
			if !f.Stealth {
				err = errors.Join(err, f.VCS.Unstage(ctx, []string{c.Name}))
			}
		}
		if err != nil {
			failed = errors.Join(failed, fmt.Errorf("%s cannot be put back: %w", c.Name, err))
		}
	}
	return failed
}

// restored is cause, after restore has put the files back; when that fails
// too, an error that says both.
func (f Files) restored(ctx context.Context, files []port.Change, modes []fs.FileMode, cause error) error {
	err := f.restore(ctx, files, modes)
	switch {
	case err == nil:
		return cause
	case cause == nil:
		return err
	}
	return fmt.Errorf("%w, and %w", cause, err)
}

// and is the names joined as a list is said: "a", "a and b", "a, b and c".
func and(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
