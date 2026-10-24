package initcmd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// shimRun makes the offer over f with the flag and, when answers is not
// "", a terminal answering them, and gives its outcome, its exit code and
// what it said.
func shimRun(f *fake, flag ShimFlag, answers string) (shimOutcome, int, string) {
	var log strings.Builder
	s := shimOffer{flag: flag, ask: answers != "", log: &log, answers: strings.NewReader(answers), ports: f}
	got, code := s.run(context.Background())
	return got, code, log.String()
}

// The offer, each way it ends: declined by the flag or the terminal; a
// place it cannot name; a link to this itos kept; a git that is not
// itos refused for --git-shim and otherwise said; another itos
// replaced; a link it cannot make or look at; said where nobody is asked;
// and linked for --git-shim or a yes, where its folder stands said.
func TestShimOfferOutcomes(t *testing.T) {
	link, given := filepath.Join("bin", "git"), ShimFlag{Given: true, Install: true}
	there := map[string]string{link: "a git\n"}
	standing := "bin stands at -1,  at -1.\n"
	for _, c := range []struct {
		name    string
		flag    ShimFlag
		answers string
		files   map[string]string
		links   map[string][2]bool
		placed  error
		linkErr error
		action  string
		code    int
		says    string
	}{
		{"--no-git-shim", ShimFlag{Given: true}, "", nil, nil, nil, nil, "declined", 0, ""},
		{"no place", given, "", nil, nil, errors.New("no itos"), nil, "failed", exitPolicy,
			"The git shim is not linked: no itos.\n"},
		{"this itos there", given, "", there, map[string][2]bool{link: {true, false}}, nil, nil, "kept", 0,
			"The git shim is there already: " + link + " links to itos.\n" + standing},
		{"a git there, --git-shim", given, "", there, nil, nil, nil, "refused", exitPolicy,
			"The git shim is not linked: " + link + " is a git that is not itos, which init never replaces; " +
				"--git-shim-dir <folder> links it in another folder.\n"},
		{"a git there, unasked", ShimFlag{}, "", there, nil, nil, nil, "offered", 0,
			"The git shim is not linked: " + link + " is a git that is not itos, so itos init --git-shim " +
				"--git-shim-dir <folder> links itos as git in another folder, so that in a repository itos manages git commit " +
				"and git push are itos's.\n"},
		{"another itos there", given, "", there, map[string][2]bool{link: {false, true}}, nil, nil, "replaced", 0,
			"Linked the git shim: replaced " + link + ", another itos, with a link to itos.\n" + standing},
		{"unasked", ShimFlag{}, "", nil, nil, nil, nil, "offered", 0, shimUnasked},
		{"a no at the terminal", ShimFlag{}, "n\n", nil, nil, nil, nil, "declined", 0, "? [Y/n]: "},
		{"a yes at the terminal", ShimFlag{}, "\n", nil, nil, nil, nil, "linked", 0,
			"Linked the git shim: " + link + " to itos.\n" + standing},
		{"--git-shim", given, "", nil, nil, nil, nil, "linked", 0, "Linked the git shim: " + link + " to itos.\n" + standing},
		{"the link failing", given, "", nil, nil, nil, errors.New("read-only"), "failed", exitPolicy,
			"The git shim is not linked: cannot link " + link + " to itos: read-only.\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := scratch(t, "commit", c.files)
			f.links, f.placeErr, f.linkErr = c.links, c.placed, c.linkErr
			got, code, said := shimRun(f, c.flag, c.answers)
			if got.Action != c.action || code != c.code || !strings.HasSuffix(said, c.says) || (c.says == "" && said != "") {
				t.Errorf("%+v, exit %d:\n%s", got, code, said)
			}
			if c.action != "declined" && c.placed == nil && got.Link != link {
				t.Errorf("the link %v", got.Link)
			}
			if (got.Action == "failed" || got.Action == "refused") && !strings.Contains(said, got.Problem) {
				t.Errorf("the problem %q", got.Problem)
			}
		})
	}
}

// A link that cannot be looked at fails the offer with why.
func TestShimOfferWhereTheLinkCannotBeLookedAt(t *testing.T) {
	got, code, said := shimRun(scratch(t, "commit", nil), ShimFlag{Given: true, Install: true, Dir: "bad\x00dir"}, "")
	if got.Action != "failed" || code != exitPolicy || got.Problem == "" || said != "The git shim is not linked: "+got.Problem+".\n" {
		t.Errorf("%+v, exit %d:\n%s", got, code, said)
	}
}

// With no folder named, the first git on the PATH is kept when it is itos,
// made absolute, or as the PATH has it when it cannot be; one
// that does not, or none, leaves the link in the folder holding itos; and
// a folder named is where the offer says it would link.
func TestShimOfferFindsTheFirstGit(t *testing.T) {
	f := scratch(t, "commit", nil)
	first, _ := filepath.Abs(filepath.Join("path", "git"))
	f.paths = map[string]string{"git": filepath.Join("path", "git")}
	f.itos = map[string]bool{first: true}
	if got, code, said := shimRun(f, ShimFlag{}, ""); got.Link != first || got.Action != "kept" || code != 0 ||
		!strings.Contains(said, "The git shim is there already: "+first+" is itos.\n") || len(f.made) != 0 {
		t.Errorf("this itos first: %+v", got)
	}
	f.absErr = errors.New("no working folder")
	f.itos = map[string]bool{filepath.Join("path", "git"): true}
	if got, _, _ := shimRun(f, ShimFlag{}, ""); got.Link != filepath.Join("path", "git") {
		t.Errorf("not made absolute: %+v", got)
	}
	f.itos = nil
	if got, _, _ := shimRun(f, ShimFlag{}, ""); got.Link != filepath.Join("bin", "git") {
		t.Errorf("another git first: %+v", got)
	}
	got, _, said := shimRun(f, ShimFlag{Dir: "elsewhere"}, "")
	if got.Link != filepath.Join("elsewhere", "git") || !strings.Contains(said, "links itos as git in elsewhere (--git-shim-dir") {
		t.Errorf("a folder named: %+v:\n%s", got, said)
	}
}

// Where the link's folder stands on the PATH: on it before the real git,
// after it, before where there is none, and off it.
func TestShimOfferStanding(t *testing.T) {
	for _, c := range []struct {
		at, realAt        int
		onPath, beforeGit bool
	}{{0, 1, true, true}, {2, 1, true, false}, {1, 0, true, false}, {0, -1, true, true}, {-1, 0, false, false}} {
		f := scratch(t, "commit", nil)
		f.at, f.realAt = c.at, c.realAt
		got, _, _ := shimRun(f, ShimFlag{Given: true, Install: true}, "")
		if got.OnPath != c.onPath || got.BeforeGit != c.beforeGit {
			t.Errorf("at %d, git at %d: %+v", c.at, c.realAt, got)
		}
	}
}

// The recognized PATH entry is kept before looking at links, even where
// LinksItos would call it another runtime or an ordinary file.
func TestShimOfferRecognizedGitStanding(t *testing.T) {
	for _, c := range []struct {
		at, realAt        int
		onPath, beforeGit bool
	}{{0, 1, true, true}, {2, 1, true, false}, {0, -1, true, true}, {-1, 0, false, false}} {
		for _, flag := range []ShimFlag{{}, {Given: true, Install: true}} {
			f := scratch(t, "commit", nil)
			first, _ := filepath.Abs(filepath.Join("path", "git"))
			f.paths = map[string]string{"git": first}
			f.itos = map[string]bool{first: true}
			f.links = map[string][2]bool{first: {false, true}}
			f.at, f.realAt = c.at, c.realAt
			got, code, said := shimRun(f, flag, "")
			if code != 0 || got.Action != "kept" || got.Link != first || got.OnPath != c.onPath || got.BeforeGit != c.beforeGit || len(f.made) != 0 ||
				!strings.HasPrefix(said, "The git shim is there already: "+first+" is itos.\n") {
				t.Errorf("at %d, git at %d: %+v, exit %d: %s", c.at, c.realAt, got, code, said)
			}
		}
	}
}

func TestShimQuestion(t *testing.T) {
	for input, want := range map[string]string{"\n": "yes", "Y\n": "yes", "no\n": "no", "maybe\nn\n": "no", "maybe": "", "": ""} {
		var log strings.Builder
		if got := (shimOffer{log: &log, answers: strings.NewReader(input)}).question("git"); got != want {
			t.Errorf("%q: %q, want %q\n%s", input, got, want, log.String())
		}
	}
}
