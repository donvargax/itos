package idcounter

import (
	"errors"
	"testing"
)

func TestReplaceFileWithPropagatesConversionAndMoveErrors(t *testing.T) {
	wantErr := errors.New("test failure")
	t.Run("source conversion", func(t *testing.T) {
		convert := func(string) (string, error) { return "", wantErr }
		move := func(string, string, uint32) error { t.Fatal("move after source conversion failed"); return nil }
		if err := replaceFileWith("source", "destination", convert, move, 0); !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
	t.Run("destination conversion", func(t *testing.T) {
		calls := 0
		convert := func(path string) (string, error) {
			calls++
			if calls == 2 {
				return "", wantErr
			}
			return path, nil
		}
		move := func(string, string, uint32) error { t.Fatal("move after destination conversion failed"); return nil }
		if err := replaceFileWith("source", "destination", convert, move, 0); !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
	t.Run("replacement", func(t *testing.T) {
		convert := func(path string) (string, error) { return path, nil }
		move := func(source, destination string, flags uint32) error {
			if source != "source" || destination != "destination" || flags != 17 {
				t.Fatalf("move(%q, %q, %d)", source, destination, flags)
			}
			return wantErr
		}
		if err := replaceFileWith("source", "destination", convert, move, 17); !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}
