package porttest

import (
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// layout is the fake laid out as VCSContract asks, standing in /top/repo.
func layout(l Layout) (VCS, string) {
	root := filepath.FromSlash("/top/repo")
	top := port.Location{Top: root, GitDir: ".git", Common: ".git"}
	v := VCS{
		Root: root,
		Located: map[string]port.Location{
			"":       top,
			sub:      {Top: root, GitDir: "../.git", Common: "../.git"},
			worktree: {Top: filepath.Join(root, worktree), GitDir: "../repo/.git/worktrees/wt", Common: "../repo/.git"},
			gitDir:   {GitDir: ".", Common: "."},
		},
		History: []Commit{
			{"c-a", nil},
			{"c-b", []string{"c-a"}},
			{"c-c", []string{"c-b"}},
			{"c-s", []string{"c-a"}},
			{"c-m", []string{"c-c", "c-s"}},
		},
		Refs:      map[string]string{"HEAD": "c-m", "refs/heads/main": "c-m"},
		IsShallow: l.Shallow,
	}
	for _, c := range v.History {
		v.Refs["refs/tags/"+c.Name[2:]] = c.Name
	}
	for _, tag := range l.Remotes {
		v.Refs["refs/remotes/origin/"+tag] = "c-" + tag
	}
	return v, root
}

// The fake answers as the adapter's git does.
func TestVCSKeepsTheContract(t *testing.T) {
	VCSContract(t, func(_ *testing.T, l Layout) (port.VCS, string) { return layout(l) })
}
