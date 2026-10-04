package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Each -m is wrapped in its place, however it is written; -F's text goes to
// a file of its own, named where the file was; nothing changes without a
// limit or when git will refuse -m with -F.
func TestWrapBody(t *testing.T) {
	long := "chore: tidy\n\n" + strings.Repeat("word ", 30)
	wrapped := "chore: tidy\n\n" + strings.TrimSpace(strings.Repeat("word ", 20)) + "\n" + strings.TrimSpace(strings.Repeat("word ", 10))
	for _, c := range []struct{ args, want []string }{
		{[]string{"-m", long}, []string{"-m", wrapped}},
		{[]string{"--message=" + long, "-q"}, []string{"--message=" + wrapped, "-q"}},
		{[]string{"-am" + long}, []string{"-am" + wrapped}},
		{[]string{"--message", long, "--", "x"}, []string{"--message", wrapped, "--", "x"}},
	} {
		got, written, err := wrapBody(c.args, readGitArgs(c.args), 100, nil)
		if err != nil || written != "" || !slices.Equal(got, c.want) {
			t.Errorf("wrapBody(%q) = %q, %q, %v", c.args, got, written, err)
		}
	}
	for _, args := range [][]string{{"-m", "x", "-F", "y"}, {"-m", long}} {
		limit := 100
		if len(args) == 2 {
			limit = 0
		}
		if got, _, _ := wrapBody(args, readGitArgs(args), limit, nil); !slices.Equal(got, args) {
			t.Errorf("wrapBody(%q, %d) = %q", args, limit, got)
		}
	}

	file := filepath.Join(t.TempDir(), "msg.txt")
	if err := os.WriteFile(file, []byte(long+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-F", file}
	got, written, err := wrapBody(args, readGitArgs(args), 100, nil)
	if err != nil || written == "" || !slices.Equal(got, []string{"-F", written}) {
		t.Fatalf("wrapBody(%q) = %q, %q, %v", args, got, written, err)
	}
	defer os.Remove(written)
	if text, _ := os.ReadFile(written); string(text) != wrapped+"\n" {
		t.Errorf("the written message is %q", text)
	}
}
