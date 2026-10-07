package lock

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/donvargax/itos/v7/internal/kind"
)

// Suffix is what the lock file's name adds to the data's.
const Suffix = ".lock"

// Wait is how long Hold waits for a lock another itos holds.
var Wait = 10 * time.Second

// Held is a lock Hold took; Release gives it back.
type Held struct{ path string }

// Hold takes the lock of the file at path, its folder made (0700) when it
// is not there: it makes path+Suffix, which no other process may then make,
// waiting up to Wait while one has it. A lock still held after the wait is
// an error naming the lock file, kind.Temporary (exit 75): the itos holding
// it is likely still at work, and the command may pass when run again.
func Hold(path string) (*Held, error) {
	file := path + Suffix
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(Wait)
	pause := time.Millisecond
	for {
		f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, err = fmt.Fprintf(f, "%d\n", os.Getpid())
			if err = errors.Join(err, f.Close()); err != nil {
				os.Remove(file)
				return nil, err
			}
			return &Held{file}, nil
		}
		if !busy(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, kind.Wrap(kind.Temporary, fmt.Errorf("%s is held by another itos, still after %s; if no itos is running, one that stopped left it: remove %s and run the command again", file, Wait, file))
		}
		time.Sleep(pause)
		pause = min(pause*2, 50*time.Millisecond)
	}
}

// busy is whether making the lock file failed because another process has
// it: it is there, or, on Windows, it is being removed, which Windows
// reports as access denied until the last handle on it closes.
func busy(err error) bool {
	return errors.Is(err, fs.ErrExist) || runtime.GOOS == "windows" && errors.Is(err, fs.ErrPermission)
}

// Release removes the lock file, so the next writer may take it.
func (h *Held) Release() error {
	return os.Remove(h.path)
}
