package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"--release"}, {""}} {
		var stdout, stderr strings.Builder
		if code := run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "usage: ") {
			t.Errorf("run(%q) = %d, stdout %q, stderr %q; want 2 and the usage", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestPublished(t *testing.T) {
	dist := t.TempDir()
	sums := "aa11  itos-1.0.0-linux-amd64.tar.gz\nbb22 *itos-1.0.0-windows-amd64.zip\ncc33  itos.schema.json\n"
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := published(dist)
	want := []string{"itos-1.0.0-linux-amd64.tar.gz", "itos-1.0.0-windows-amd64.zip", "itos.schema.json", "checksums.txt"}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("published said %q, %v; want %q", got, err, want)
	}
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte("aa11\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := published(dist); err == nil {
		t.Error("a line naming no file was read")
	}
}
