package adapter

import (
	"strings"
	"testing"
)

// LastTouching is the newest first-parent commit touching one of the paths,
// each a literal path: a later commit touching another file is passed over,
// a path with a glob character is the file it names, none touching gives
// "", and a revision git cannot read is git's error.
func TestLastTouching(t *testing.T) {
	dir := committed(t, "a.feature", "b[1].feature")
	g := Git{Dir: dir}
	ctx := t.Context()
	start, err := g.LastTouching(ctx, "HEAD", []string{"a.feature"})
	if err != nil || len(start) != 40 {
		t.Fatalf("LastTouching = %q, %v", start, err)
	}
	write(t, dir, "b[1].feature", "changed")
	gitIn(t, dir, "", "commit", "-q", "-am", "change b")
	write(t, dir, "README.md", "readme")
	gitIn(t, dir, "", "add", "--", "README.md")
	gitIn(t, dir, "", "commit", "-q", "-m", "readme")
	head, _ := g.Resolve(ctx, "HEAD")
	b, _ := g.Resolve(ctx, "HEAD~1")
	for _, c := range []struct {
		paths []string
		want  string
	}{
		{[]string{"a.feature"}, start},
		{[]string{"b[1].feature"}, b},
		{[]string{"a.feature", "b[1].feature"}, b},
		{[]string{"README.md"}, head},
		{[]string{"missing.feature"}, ""},
	} {
		got, err := g.LastTouching(ctx, "HEAD", c.paths)
		if err != nil || got != c.want {
			t.Errorf("LastTouching(%v) = %q, %v, want %q", c.paths, got, err, c.want)
		}
	}
	if _, err := g.LastTouching(ctx, "nosuchrev", []string{"a.feature"}); err == nil || !strings.Contains(err.Error(), "nosuchrev") {
		t.Errorf("a revision git cannot read: %v", err)
	}
}
