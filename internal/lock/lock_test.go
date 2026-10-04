package lock

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHoldMakesTheLockFileAndReleaseRemovesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "itos", "follow-ups.yaml")
	held, err := Hold(path)
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
	_, err := Hold(path)
	if err == nil || !strings.Contains(err.Error(), path+Suffix) || !strings.Contains(err.Error(), "remove") {
		t.Fatalf("a lock held past the wait gave %v, not an error naming %s", err, path+Suffix)
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		os.Remove(path + Suffix)
	}()
	held, err := Hold(path)
	if err != nil {
		t.Fatalf("a lock released during the wait gave %v", err)
	}
	held.Release()
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
			held, err := Hold(path)
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
