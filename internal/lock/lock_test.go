package lock

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The lock file there is busy on every OS; access denied, a lock file being
// removed, only on windows; any other error never.
func TestBusy(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for err, want := range map[error]bool{
			fs.ErrExist:                       true,
			&fs.PathError{Err: fs.ErrExist}:   true,
			fs.ErrPermission:                  goos == "windows",
			&fs.PathError{Err: os.ErrInvalid}: false,
		} {
			if got := busy(err, goos); got != want {
				t.Errorf("busy(%v, %s) = %v", err, goos, got)
			}
		}
	}
}

func TestHoldMakesTheLockFileAndReleaseRemovesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "itos", "follow-ups.yaml")
	held, err := Hold(path, runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + Suffix); err != nil {
		t.Fatalf("no lock file: %v", err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + Suffix); !os.IsNotExist(err) {
		t.Fatalf("the lock file is still there: %v", err)
	}
}

func TestHoldWaitsForTheLockAndRefusesOneStillHeld(t *testing.T) {
	was := Wait
	Wait = 100 * time.Millisecond
	t.Cleanup(func() { Wait = was })
	path := filepath.Join(t.TempDir(), "data.yaml")
	if err := os.WriteFile(path+Suffix, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Hold(path, runtime.GOOS)
	if err == nil || !strings.Contains(err.Error(), path+Suffix) || !strings.Contains(err.Error(), "remove") {
		t.Fatalf("a lock held past the wait gave %v, not an error naming %s", err, path+Suffix)
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		os.Remove(path + Suffix)
	}()
	held, err := Hold(path, runtime.GOOS)
	if err != nil {
		t.Fatalf("a lock released during the wait gave %v", err)
	}
	held.Release()
}

// Hold backs off while the lock is held: each pause doubles the last, from
// a millisecond, up to 50ms, so a long wait does not spin.
func TestHoldBacksOffBetweenTries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.yaml")
	if err := os.WriteFile(path+Suffix, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var pauses []time.Duration
	was := sleep
	sleep = func(d time.Duration) {
		if pauses = append(pauses, d); len(pauses) == 8 {
			os.Remove(path + Suffix)
		}
	}
	t.Cleanup(func() { sleep = was })
	held, err := Hold(path, runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	held.Release()
	ms := time.Millisecond
	want := []time.Duration{ms, 2 * ms, 4 * ms, 8 * ms, 16 * ms, 32 * ms, 50 * ms, 50 * ms}
	if !slices.Equal(pauses, want) {
		t.Errorf("pauses %v, want %v", pauses, want)
	}
}

// Many holders at once each get the lock alone: a count read and written
// back under it loses no increment.
func TestHoldLetsOneHolderInAtATime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "count")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			held, err := Hold(path, runtime.GOOS)
			if err != nil {
				t.Error(err)
				return
			}
			defer held.Release()
			text, _ := os.ReadFile(path)
			os.WriteFile(path, append(text, 'x'), 0o600)
		}()
	}
	wg.Wait()
	if text, _ := os.ReadFile(path); len(text) != 20 {
		t.Errorf("%d of 20 writes kept", len(text))
	}
}
