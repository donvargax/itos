package cli

// itos init's offer of the git shim (slice 50, features/init.feature; PLAN.md
// §7 and §10, "Adoption"), made as the plugin's is (initplugin.go) and
// installed as git-shim install installs it (gitshim.go): itos linked as git,
// by default in the folder holding the itos running.
//
// The offer is opt-in everywhere (the user's call, 2026-10-03). --git-shim
// answers it, --git-shim-dir <folder> being git-shim install's --dir, and
// --no-git-shim declines it. On a terminal with neither, the first run asks,
// its default yes; anywhere else (an agent, CI) it links nothing and says
// how to. Run again where a config is, it never asks, and a --git-shim given
// there links it all the same, the flag being the ask; without one the report
// says how to, as it does of the plugin, never counting it as missing
// (initReport).
//
// A shim already in effect is kept: the link where it would go already
// linking this itos, or, with no --git-shim-dir, the first git on the PATH
// being itos, wherever its folder is. A git there that is no link to itos is
// never replaced, as git-shim install never replaces one: that is a refusal,
// exit 1, the rest of init's work done; a link to another binary named itos,
// an older install's, is replaced.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// shimFlag is how init's command line answered the offer: given is whether
// it has --git-shim or --no-git-shim, install which, dir --git-shim-dir's
// folder made absolute where it was typed, "" for the folder holding itos.
type shimFlag struct {
	given   bool
	install bool
	dir     string
}

// parseShimFlag reads --git-shim, --no-git-shim or --git-shim-dir <folder>
// (--git-shim-dir=<folder>) out of init's arguments at i, and gives how many
// arguments it read, 0 when args[i] is none of them.
func parseShimFlag(args []string, i int, f *shimFlag) (int, error) {
	a := args[i]
	switch a {
	case "--git-shim", "--no-git-shim":
		install := a == "--git-shim"
		if f.given && f.install != install {
			return 0, usage("init takes --git-shim or --no-git-shim, not both")
		}
		f.given, f.install = true, install
		return 1, nil
	}
	folder, ok := strings.CutPrefix(a, "--git-shim-dir=")
	n := 1
	if !ok {
		if a != "--git-shim-dir" {
			return 0, nil
		}
		if i+1 >= len(args) {
			return 0, usage("--git-shim-dir needs <folder>")
		}
		folder, n = args[i+1], 2
	}
	if folder == "" {
		return 0, usage("--git-shim-dir needs <folder>")
	}
	dir, err := filepath.Abs(typed(folder))
	if err != nil {
		return 0, err
	}
	f.dir = dir
	return n, nil
}

// shimOutcome is what came of the offer, init's --json "git_shim": action
// one of linked, replaced (a link to another itos), kept (a link to this
// itos was there), offered (nothing linked, and nobody asked), declined,
// refused (a git that is no link to itos is there; exit 1) or failed (exit
// 1); link the path of the link, made or that would be, null when declined;
// on_path and before_git where its folder stands on the PATH, as git-shim
// install --json says them; problem why it was refused or failed.
type shimOutcome struct {
	Action    string `json:"action"`
	Link      any    `json:"link"`
	OnPath    bool   `json:"on_path"`
	BeforeGit bool   `json:"before_git"`
	Problem   string `json:"problem,omitempty"`
}

// shimOffer is the offer's context, as pluginOffer's: the flag, whether a
// terminal may be asked, where it says what it did, and where an answer is
// read from.
type shimOffer struct {
	flag    shimFlag
	ask     bool
	log     io.Writer
	answers io.Reader
}

// run makes the offer, and gives its outcome and its exit code: 1 when the
// link asked for could not be made, else 0.
func (s shimOffer) run() (shimOutcome, int) {
	say := func(format string, a ...any) { fmt.Fprintf(s.log, format+"\n", a...) }
	if s.flag.given && !s.flag.install {
		return shimOutcome{Action: "declined"}, 0
	}
	self, link, err := shimPlace(s.flag.dir)
	if err != nil {
		say("The git shim is not linked: %s.", err)
		return shimOutcome{Action: "failed", Problem: err.Error()}, ExitPolicy
	}
	if s.flag.dir == "" {
		if first := firstGit(); first != "" {
			if mine, _ := linksItos(first); mine {
				link = first
			}
		}
	}
	outcome := func(action string) shimOutcome {
		at, _, realAt := pathStanding(filepath.Dir(link))
		return shimOutcome{Action: action, Link: link, OnPath: at >= 0, BeforeGit: at >= 0 && (realAt < 0 || at < realAt)}
	}
	standing := func() {
		at, realGit, realAt := pathStanding(filepath.Dir(link))
		say("%s", standingLine(filepath.Dir(link), at, realGit, realAt))
	}

	replace := false
	if _, err := os.Lstat(link); err == nil {
		mine, other := linksItos(link)
		switch {
		case mine:
			say("The git shim is there already: %s links to %s.", link, self)
			standing()
			return outcome("kept"), 0
		case !other && s.flag.given:
			problem := link + " is a git that is not a link to itos, which init never replaces"
			say("The git shim is not linked: %s; --git-shim-dir <folder> links it in another folder.", problem)
			o := outcome("refused")
			o.Problem = problem
			return o, ExitPolicy
		case !other:
			say("%s", s.howTo(link))
			return outcome("offered"), 0
		}
		replace = other
	} else if !errors.Is(err, fs.ErrNotExist) {
		say("The git shim is not linked: %s.", err)
		o := outcome("failed")
		o.Problem = err.Error()
		return o, ExitPolicy
	}

	if !s.flag.given {
		answer := ""
		if s.ask {
			answer = s.question(link)
		}
		switch answer {
		case "":
			say("%s", s.howTo(link))
			return outcome("offered"), 0
		case "no":
			return shimOutcome{Action: "declined"}, 0
		}
	}
	if err := makeLink(self, link, replace); err != nil {
		problem := fmt.Sprintf("cannot link %s to %s: %s", link, self, err)
		say("The git shim is not linked: %s.", problem)
		o := outcome("failed")
		o.Problem = problem
		return o, ExitPolicy
	}
	action := "linked"
	if replace {
		action = "replaced"
		say("Linked the git shim: replaced %s, a link to another itos, with a link to %s.", link, self)
	} else {
		say("Linked the git shim: %s to %s.", link, self)
	}
	standing()
	return outcome(action), 0
}

// firstGit is the git the PATH finds first, made absolute, "" for none.
func firstGit() string {
	p, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// howTo is what the offer says where nobody was asked.
func (s shimOffer) howTo(link string) string {
	where := "in " + filepath.Dir(link) + ", the folder holding itos"
	if s.flag.dir != "" {
		where = "in " + filepath.Dir(link)
	}
	text := "The git shim is not linked: itos init --git-shim links itos as git " + where +
		" (--git-shim-dir <folder> for another), so that in a repository itos manages git commit and git push are itos's."
	if _, err := os.Lstat(link); err == nil {
		if mine, other := linksItos(link); !mine && !other {
			text = "The git shim is not linked: " + link + " is a git that is not a link to itos, so " +
				"itos init --git-shim --git-shim-dir <folder> links itos as git in another folder, " +
				"so that in a repository itos manages git commit and git push are itos's."
		}
	}
	return text
}

// question asks the terminal whether to link the shim, until it answers:
// "yes", "no", or "" when its input ends first.
func (s shimOffer) question(link string) string {
	lines := bufio.NewReader(s.answers)
	for {
		fmt.Fprintf(s.log, "Link itos as git at %s, so that in a repository itos manages git commit and git push are itos's? [Y/n]: ", link)
		line, err := lines.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		switch {
		case answer == "" && err != nil:
			fmt.Fprintln(s.log)
			return ""
		case answer == "" || answer == "y" || answer == "yes":
			return "yes"
		case answer == "n" || answer == "no":
			return "no"
		}
		fmt.Fprintln(s.log, "Answer yes or no.")
		if err != nil {
			return ""
		}
	}
}
