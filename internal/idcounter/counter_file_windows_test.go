//go:build windows

package idcounter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceFileWindowsReplacesAtomically(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "counter")
	if err := os.WriteFile(source, []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(source, destination); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "new\n" {
		t.Fatalf("replacement = %q (%v), want new counter contents", got, err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source remains after replacement: %v", err)
	}
}

func TestReplaceFileWindowsPropagatesInvalidPathErrors(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "counter")
	for _, test := range []struct {
		name, from, to string
	}{
		{name: "source", from: source + "\x00", to: destination},
		{name: "destination", from: source, to: destination + "\x00"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := replaceFile(test.from, test.to)
			if err == nil {
				t.Fatal("replaceFile succeeded with a path containing NUL")
			}
		})
	}
}
