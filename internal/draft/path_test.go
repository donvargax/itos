package draft

import (
	"os"
	"path/filepath"
	"testing"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// Path is the list in itos's folder of the common dir the VCS locates from
// the folder itos runs in: an absolute one as it is, a relative one from the
// folder, its links resolved; "" where the VCS locates none or no common
// dir.
func TestPath(t *testing.T) {
	real := t.TempDir()
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	// A system that makes no symbolic link runs from the folder itself.
	link := filepath.Join(t.TempDir(), "link")
	if os.Symlink(real, link) != nil {
		link = real
	}
	t.Chdir(t.TempDir())
	at := func(common string) *vcsfake.VCS {
		return &vcsfake.VCS{Root: link, Located: map[string]vcsport.Location{"": {Top: ".", GitDir: ".git", Common: common}}}
	}
	abs := filepath.Join(t.TempDir(), "common")
	for name, c := range map[string]struct {
		v    vcsport.VCS
		want string
	}{
		"absolute":   {at(abs), filepath.Join(abs, "itos", Folder, FileName)},
		"relative":   {at(".git"), filepath.Join(resolved, ".git", "itos", Folder, FileName)},
		"no common":  {at(""), ""},
		"no located": {&vcsfake.VCS{}, ""},
	} {
		if got := Path(t.Context(), c.v); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
}
