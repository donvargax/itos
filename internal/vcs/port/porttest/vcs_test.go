package porttest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// layout is the fake laid out as VCSContract asks, standing in /top/repo,
// naming the git folder as git does: relative at the top, absolute from a
// subfolder.
func layout(l Layout) (*VCS, string) {
	root, err := filepath.Abs(filepath.FromSlash("/top/repo"))
	if err != nil {
		panic(err)
	}
	if l.None {
		return &VCS{Root: root}, root
	}
	top := port.Location{Top: root, GitDir: ".git", Common: ".git"}
	v := &VCS{
		Root: root,
		Located: map[string]port.Location{
			"":       top,
			sub:      {Top: root, GitDir: filepath.Join(root, ".git"), Common: "../.git"},
			worktree: {Top: filepath.Join(root, worktree), GitDir: "../repo/.git/worktrees/wt", Common: "../repo/.git"},
			gitDir:   {GitDir: ".", Common: "."},
		},
		History: []Commit{
			{Name: "c-a", Tree: map[string]string{"a.txt": "a\n", "x.md": "x\n"}},
			{Name: "c-b", Parents: []string{"c-a"}, Tree: map[string]string{"a.txt": "b\n", "b.txt": "b\n", "x.md": "x\n"}},
			{Name: "c-c", Parents: []string{"c-b"}, Message: "c\nwrapped\n\nthe body of c\n", Note: "Task: T-1\n",
				Tree: map[string]string{"a.txt": "b\n", "b.txt": "b\n", "c.txt": "c\n"}},
			{Name: "c-s", Parents: []string{"c-a"}, Tree: map[string]string{"a.txt": "a\n", "x.md": "x\n", "s.txt": "s\n"}},
			{Name: "c-m", Parents: []string{"c-c", "c-s"}, Tree: map[string]string{"a.txt": "m\n", "b.txt": "b\n", "c.txt": "c\n", "s.txt": "s\n"}},
			{Name: "c-t", Parents: []string{"c-c"}, Tree: map[string]string{"a.txt": "t\n", "b.txt": "b\n", "c.txt": "c\n"}},
			{Name: "c-o", Parents: []string{"c-c"}, Tree: map[string]string{"a.txt": "o\n", "b.txt": "b\n", "c.txt": "c\n"}},
		},
		Named: map[string]string{"HEAD": "c-m", "refs/heads/main": "c-m", "refs/heads/theirs": "c-t", "refs/heads/ours": "c-o",
			"refs/heads/side": "c-s"},
		Tags:      map[string]string{"o-r": "c-c"},
		IsShallow: l.Shallow,
		OnBranch:  "main",
		Comment:   l.CommentChar,
		Ignored:   []string{"debug.log"},
	}
	for i := range v.History {
		c := &v.History[i]
		name := c.Name[2:]
		v.Named["refs/tags/"+name] = c.Name
		if c.Message == "" {
			c.Message = name + "\n\nthe body of " + name + "\n"
		}
		c.Author, c.Email, c.Date = "t", "t@t", fmt.Sprintf("%d +0000", 1767225601+i)
	}
	v.Named["refs/tags/r"] = "o-r"
	switch {
	case l.Ours:
		v.OnBranch, v.Named["HEAD"] = "ours", "c-o"
	case l.Dirty:
		v.Index = map[string]string{"a.txt": "staged\n", "c.txt": "c\n", "s.txt": "s\n", "new.txt": "new\n"}
		v.Work = map[string]string{"a.txt": "staged\n", "c.txt": "changed\n", "s.txt": "s\n", "new.txt": "new\n", "u.txt": "u\n"}
	case l.Merging:
		v.Unmerged, v.MergeHead = []string{"a.txt"}, []string{"c-t"}
	case l.Rebasing:
		v.Unmerged, v.InRebase, v.OnBranch = []string{"a.txt"}, true, ""
		v.Named["HEAD"] = "c-t"
	}
	for _, tag := range l.Remotes {
		v.Named["refs/remotes/origin/"+tag] = "c-" + tag
	}
	if l.Origin {
		v.Remote = map[string]*Remote{"down": {URL: "https://itos.invalid/x.git", Down: true}, "origin": {URL: "/top/origin.git",
			Refs: map[string]string{"refs/heads/main": "c-b", "refs/heads/theirs": "c-t"}}}
		v.Tracking = map[string][2]string{"main": {"origin", "refs/heads/main"}}
		v.Symbolic = map[string]string{"refs/remotes/origin/HEAD": "refs/remotes/origin/main"}
	}
	return v, root
}

// The fake answers as the adapter's git does.
func TestVCSKeepsTheContract(t *testing.T) {
	VCSContract(t, func(_ *testing.T, l Layout) (port.VCS, string) { return layout(l) })
}
