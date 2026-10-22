package adapter

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/lock"
	"github.com/donvargax/itos/v8/internal/store/private/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Store is the private store on disk: config.StealthFolder of the absolute
// git common dir of the folder itos runs in, located on every call. OS is
// the OS itos runs on, as GOOS names it, which Hold's lock waits by, and VCS
// the repository, which locates the common dir.
type Store struct {
	OS  string
	VCS vcs.VCS
}

var _ port.Store = Store{}

// Path is name in itos's folder of the common dir as VCS locates it, made
// absolute from the working folder with its links resolved, as git names
// it; a folder in no repository is no repository.
func (s Store) Path(ctx context.Context, name string) (string, error) {
	l, err := s.VCS.Locate(ctx, "")
	if err != nil || l.Common == "" {
		return "", &port.NoRepository{}
	}
	common := l.Common
	if !filepath.IsAbs(common) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", &port.NoRepository{}
		}
		if real, err := filepath.EvalSymlinks(cwd); err == nil {
			cwd = real
		}
		common = filepath.Join(cwd, common)
	}
	return filepath.Join(common, config.StealthFolder, name), nil
}

// Read is the file's text; one not there is none.
func (s Store) Read(ctx context.Context, name string) ([]byte, bool, error) {
	path, err := s.Path(ctx, name)
	if err != nil {
		return nil, false, err
	}
	text, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return text, true, nil
}

// Write writes text to a file beside name's, mode 0600, syncs it to the
// disk and moves it over name's, its folder made (0700) when it is not
// there.
func (s Store) Write(ctx context.Context, name string, text []byte) error {
	path, err := s.Path(ctx, name)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(text)
	if err == nil {
		err = tmp.Sync()
	}
	if closed := tmp.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o600)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	return err
}

// Hold is lock.Hold of name's file: its lock file beside it, waiting up to
// lock.Wait for another itos to give it back.
func (s Store) Hold(ctx context.Context, name string) (port.Held, error) {
	path, err := s.Path(ctx, name)
	if err != nil {
		return nil, err
	}
	held, err := lock.Hold(path, s.OS)
	if err != nil {
		return nil, err
	}
	return released{held}, nil
}

// released is a lock.Held as the port's Held.
type released struct{ held *lock.Held }

// Release is lock.Held's Release.
func (r released) Release(context.Context) error { return r.held.Release() }

// NotIgnored is VCS's: whether path is in the work tree and not ignored.
func (s Store) NotIgnored(ctx context.Context, path string) bool { return s.VCS.NotIgnored(ctx, path) }
