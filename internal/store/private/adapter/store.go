package adapter

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/lock"
	"github.com/donvargax/itos/v8/internal/store/private/port"
)

// Store is the private store on disk: config.StealthFolder of the absolute
// git common dir of the folder itos runs in, read again on every call. OS is
// the OS itos runs on, as GOOS names it, which Hold's lock waits by.
type Store struct{ OS string }

var _ port.Store = Store{}

// Path is name in itos's folder of the git common dir (git rev-parse
// --path-format=absolute --git-common-dir); a git that fails or says
// nothing is no repository.
func (Store) Path(_ context.Context, name string) (string, error) {
	common, err := git.Output("rev-parse", "--path-format=absolute", "--git-common-dir")
	if common = strings.TrimSpace(common); err != nil || common == "" {
		return "", &port.NoRepository{}
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

// NotIgnored is whether git check-ignore exits 1 for path: in the work
// tree and not ignored. Outside the work tree or in no repository it exits
// 128, and for an ignored path 0.
func (Store) NotIgnored(_ context.Context, path string) bool {
	err := exec.Command(git.Bin(), "check-ignore", "-q", "--", path).Run()
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}
