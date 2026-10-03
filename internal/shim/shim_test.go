package shim

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestNamed(t *testing.T) {
	for arg0, want := range map[string]bool{
		"git": true, "/usr/local/bin/git": true, "itos": false, "/x/git/itos": false, "git-shim": false,
	} {
		if got := Named(arg0); got != want {
			t.Errorf("Named(%q) = %v, want %v", arg0, got, want)
		}
	}
}

// git's options before the command: -C folders joined as git joins them (an
// empty one changing nothing), each -c kept, --no-pager and -P passed over;
// any other option, a missing value or no command is not the shim's to read.
func TestParse(t *testing.T) {
	l, ok := parse([]string{"-C", "a", "-C", "", "-c", "user.name=x", "--no-pager", "-C", "b", "-P", "commit", "-m", "-C"})
	if !ok || l.dir != filepath.Join("a", "b") || !slices.Equal(l.configs, []string{"user.name=x"}) ||
		l.name != "commit" || !slices.Equal(l.rest, []string{"-m", "-C"}) {
		t.Errorf("parse = %+v, %v", l, ok)
	}
	if l, ok := parse([]string{"-C", "/abs", "push"}); !ok || l.dir != "/abs" || l.name != "push" {
		t.Errorf("an absolute -C: %+v, %v", l, ok)
	}
	for _, args := range [][]string{
		{"--git-dir=x", "commit"}, {"--work-tree", "x", "commit"}, {"-C"}, {"-c"}, {}, {"--no-pager"},
	} {
		if l, ok := parse(args); ok {
			t.Errorf("parse(%q) = %+v, want not the shim's", args, l)
		}
	}
}
