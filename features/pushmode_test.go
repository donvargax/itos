// The steps of push.mode pr (push-mode.feature, slices 174 and 175): the
// config committed and pushed with a block of YAML added to it; a recording gh
// standing in for GitHub, first on the PATH, which records each call, keeps
// the pull requests it opens, answers a pull request's head and rebases the
// remote's branch when asked to update it (or leaves it as it is, or answers
// the head from before the rebase for a few calls, as a GitHub that has not
// caught up does); the clone on a new branch, with or without tracking the
// remote's main; the GitHub repository the environment names
// (GITHUB_REPOSITORY), as the fake GitHub gives it, or none; the remote's HEAD
// naming a branch it does not have; what a branch of the remote ends with; and
// the head of a remote's branch the fake GitHub was asked for the run of.
package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v8/internal/git"
)

func initializePushModeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the config, committed and pushed to the remote's main, adds the YAML:$`, func(text *godog.DocString) error {
		w.config.extraYAML = append(w.config.extraYAML, strings.TrimRight(text.Content, "\n")+"\n")
		return w.pushConfig("chore: add to the config")
	})
	sc.Step(`^a recording gh on the PATH, which opens pull requests as "([^"]*)"$`, w.recordingGh)
	sc.Step(`^the recording gh has the pull request "([^"]*)" open for the branch "([^"]*)"$`, w.ghHasPullRequest)
	sc.Step(`^the recording gh's pr update-branch leaves the branch as it is$`, func() error {
		return os.WriteFile(w.ghKeeps(), nil, 0o644)
	})
	sc.Step(`^the recording gh's pr view answers the pull request's previous head for its first (\d+) calls after pr update-branch$`, func(n int) error {
		return os.WriteFile(w.ghStale(), fmt.Appendf(nil, "%d\n", n), 0o644)
	})
	sc.Step(`^the fake GitHub was asked for the run of the head of the remote's branch "([^"]*)"$`, w.fakeGitHubGivenRemoteBranchHead)
	sc.Step(`^the recording gh has the pull request "([^"]*)" merged for the branch "([^"]*)"$`, func(address, branch string) error {
		return w.ghHasPullRequestIn(address, branch, "MERGED")
	})
	sc.Step(`^the recording gh has the pull request "([^"]*)" closed without merging for the branch "([^"]*)"$`, func(address, branch string) error {
		return w.ghHasPullRequestIn(address, branch, "CLOSED")
	})
	sc.Step(`^the recording gh fails every call, saying "([^"]*)"$`, func(words string) error {
		return os.WriteFile(w.ghFailure(), []byte(words+"\n"), 0o644)
	})
	sc.Step(`^the recording gh's "([^"]*)" call has "([^"]*)"$`, w.ghCallHas)
	sc.Step(`^every call of the recording gh has "([^"]*)"$`, w.ghEveryCallHas)
	sc.Step(`^the recording gh was never called$`, func() error { return w.ghNeverCalled("") })
	sc.Step(`^the recording gh was never called as "([^"]*)"$`, w.ghNeverCalled)
	sc.Step(`^the clone is on the new branch "([^"]*)"$`, func(branch string) error {
		return w.git("checkout", "-q", "-b", branch)
	})
	sc.Step(`^the clone is on the new branch "([^"]*)", tracking the remote's main$`, func(branch string) error {
		return w.git("checkout", "-q", "-b", branch, "--track", "origin/main")
	})
	sc.Step(`^the GitHub repository in the environment is "([^"]*)"$`, func(repository string) error {
		w.dropVar("GITHUB_REPOSITORY=")
		w.vars = append(w.vars, "GITHUB_REPOSITORY="+repository)
		return nil
	})
	sc.Step(`^no GitHub repository in the environment$`, func() error {
		w.dropVar("GITHUB_REPOSITORY=")
		return nil
	})
	sc.Step(`^the remote's HEAD names the branch "([^"]*)", which it does not have$`, func(branch string) error {
		return w.gitIn(w.remote(), "symbolic-ref", "HEAD", "refs/heads/"+branch)
	})
	sc.Step(`^the remote's branch "([^"]*)" ends with "([^"]*)" then "([^"]*)", with no merge commit$`, w.remoteBranchEndsWith)
}

// dropVar takes out of the variables the scenario sets those that start with
// the prefix.
func (w *world) dropVar(prefix string) {
	w.vars = slices.DeleteFunc(w.vars, func(v string) bool { return strings.HasPrefix(v, prefix) })
}

// The remote's branch of the name ends with the two commits, the older one
// before the newer, and has no merge commit.
func (w *world) remoteBranchEndsWith(branch, older, newer string) error {
	subjects, merges, err := w.remoteBranchHistory(branch)
	if err != nil {
		return err
	}
	if len(merges) > 0 {
		return fmt.Errorf("the remote's branch %q has merge commits %v:\n%s", branch, merges, strings.Join(subjects, "\n"))
	}
	if len(subjects) < 2 || subjects[0] != newer || subjects[1] != older {
		return fmt.Errorf("the remote's branch %q does not end with %q then %q:\n%s", branch, older, newer, strings.Join(subjects, "\n"))
	}
	return nil
}

// The recording gh's files: a line for each call, as its arguments were
// given, and a line for each pull request, its branch, its address and its
// state.
func (w *world) ghCalls() string { return filepath.Join(w.support, "gh-calls") }
func (w *world) ghPulls() string { return filepath.Join(w.support, "gh-pull-requests") }

// What the recording gh says on stderr when it fails every call, if it does.
func (w *world) ghFailure() string { return filepath.Join(w.support, "gh-failure") }

// The files that steer pr view and pr update-branch: the first exists when the
// branch is to be left as it is; the second holds the number of calls pr view
// answers the previous head for after pr update-branch, the third how many are
// left; the head a branch had when the recording gh was given its pull request
// is in the fourth, by the branch's name; the fifth is where pr update-branch
// rebases.
func (w *world) ghKeeps() string     { return filepath.Join(w.support, "gh-update-keeps") }
func (w *world) ghStale() string     { return filepath.Join(w.support, "gh-stale-calls") }
func (w *world) ghStaleLeft() string { return filepath.Join(w.support, "gh-stale-left") }
func (w *world) ghPrevious(branch string) string {
	return filepath.Join(w.support, "gh-previous-"+branch)
}
func (w *world) ghRebaseDir() string { return filepath.Join(w.support, "gh-rebase") }

// recordingGh is a gh first on the PATH that stands in for GitHub: it records
// each call, answers pr list with the pull requests of its --head branch that
// its --state names (open, gh's default, merged, closed or all) as gh does ([]
// for none), and answers pr create with the address given, holding that pull
// request open from then on; pr view answers the head of the remote's branch it
// names as {"headRefOid": <sha>}, and pr update-branch rebases that branch onto
// the remote's main, as GitHub does, in a clone of its own that it pushes back
// (a step can have it leave the branch as it is); any other call succeeds
// silently. It fails every call when a step says so. The environment names
// owner/scratch as the GitHub repository, as the fake GitHub does. A pull
// request's number is the end of its address.
func (w *world) recordingGh(address string) error {
	bin, err := w.binOnPath()
	if err != nil {
		return err
	}
	calls, pulls := quote(filepath.ToSlash(w.ghCalls())), quote(filepath.ToSlash(w.ghPulls()))
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %[1]s
if [ -f %[11]s ]; then cat %[11]s >&2; exit 1; fi
verb="$1 $2"
head=
state=open
prev=
for a in "$@"; do
  if [ "$prev" = --head ]; then head=$a; fi
  if [ "$prev" = --state ]; then state=$a; fi
  prev=$a
done
case "$verb" in
"pr list")
  found=
  if [ -f %[2]s ]; then
    while read -r branch url kind; do
      if [ "$branch" = "$head" ]; then
        case "$state/$kind" in all/*|open/OPEN|merged/MERGED|closed/CLOSED) ;; *) continue ;; esac
        merged=null
        if [ "$kind" = MERGED ]; then merged='"2026-10-24T10:00:00Z"'; fi
        if [ -n "$found" ]; then found="$found,"; fi
        found="$found{\"number\":${url##*/},\"url\":\"$url\",\"state\":\"$kind\",\"mergedAt\":$merged}"
      fi
    done < %[2]s
  fi
  printf '[%%s]\n' "$found"
  ;;
"pr create")
  printf '%%s %%s OPEN\n' "$head" %[3]s >> %[2]s
  printf '%%s\n' %[3]s
  ;;
"pr view")
  oid=$(%[4]s --git-dir=%[5]s rev-parse "refs/heads/$3") || exit 1
  if [ -f %[8]s ]; then
    left=$(cat %[8]s)
    if [ "$left" -gt 0 ]; then
      echo $((left - 1)) > %[8]s
      oid=$(cat %[9]s"$3")
    fi
  fi
  printf '{"headRefOid":"%%s"}\n' "$oid"
  ;;
"pr update-branch")
  if [ ! -f %[6]s ]; then
    rm -rf %[10]s
    %[4]s clone -q --branch "$3" %[5]s %[10]s &&
      %[4]s -C %[10]s rebase -q origin/main &&
      %[4]s -C %[10]s push -q --no-verify --force origin "HEAD:refs/heads/$3" || exit 1
  fi
  if [ -f %[7]s ]; then cp %[7]s %[8]s; fi
  ;;
esac
`, calls, pulls, quote(address), quote(filepath.ToSlash(git.Bin(runtime.GOOS))), quote(filepath.ToSlash(w.remote())),
		quote(filepath.ToSlash(w.ghKeeps())), quote(filepath.ToSlash(w.ghStale())), quote(filepath.ToSlash(w.ghStaleLeft())),
		quote(filepath.ToSlash(w.ghPrevious(""))), quote(filepath.ToSlash(w.ghRebaseDir())), quote(filepath.ToSlash(w.ghFailure())))
	if err := w.writeProgram(filepath.Join(bin, "gh"), script); err != nil {
		return err
	}
	if !slices.Contains(w.vars, "GITHUB_REPOSITORY="+fakeRepository) {
		w.vars = append(w.vars, "GITHUB_REPOSITORY="+fakeRepository)
	}
	return nil
}

// The recording gh holds the pull request open for the branch.
func (w *world) ghHasPullRequest(address, branch string) error {
	return w.ghHasPullRequestIn(address, branch, "OPEN")
}

// The recording gh holds the pull request for the branch, OPEN, MERGED or
// CLOSED (closed without merging), and keeps the head the remote's branch has
// now as the pull request's previous head.
func (w *world) ghHasPullRequestIn(address, branch, state string) error {
	if _, err := os.Stat(programPath(filepath.Join(w.support, "bin", "gh"))); err != nil {
		return errors.New("no recording gh: put one on the PATH first")
	}
	// A branch the remote does not have (a pull request merged and its branch
	// deleted) has no previous head to answer.
	if head, err := w.remoteBranchHead(branch); err == nil {
		if err := os.WriteFile(w.ghPrevious(branch), []byte(head+"\n"), 0o644); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(w.ghPulls(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s %s %s\n", branch, address, state)
	return errors.Join(err, f.Close())
}

// The full SHA of the remote's branch.
func (w *world) remoteBranchHead(branch string) (string, error) {
	cmd := exec.Command(git.Bin(runtime.GOOS), "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse refs/heads/%s in the remote: %w", branch, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// The fake GitHub gave a look at the watched run, and every look it gave was
// of the head the remote's branch has now, by its full SHA.
func (w *world) fakeGitHubGivenRemoteBranchHead(branch string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	head, err := w.remoteBranchHead(branch)
	if err != nil {
		return err
	}
	w.github.mu.Lock()
	given := slices.Clone(w.github.given)
	w.github.mu.Unlock()
	if len(given) == 0 {
		return fmt.Errorf("the fake GitHub was never asked for the watched run; it was asked %v\n%s", w.github.requests(), w.report())
	}
	for _, sha := range given {
		if sha != head {
			return fmt.Errorf("the fake GitHub was asked for the run of %q, not the head %s of the remote's branch %q\n%s", sha, head, branch, w.report())
		}
	}
	return nil
}

// Each call of the recording gh, as its arguments were given, in order; none
// when it was never called.
func (w *world) ghCallLines() ([]string, error) {
	text, err := os.ReadFile(w.ghCalls())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(string(text), "\n"), "\n"), nil
}

// Some call of the recording gh starts with the words of the command, and
// has the text.
func (w *world) ghCallHas(command, text string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	var named []string
	for _, call := range calls {
		if strings.HasPrefix(call, command+" ") || call == command {
			named = append(named, call)
		}
	}
	if len(named) == 0 {
		return fmt.Errorf("the recording gh was never called as %q; its calls:\n%s\n%s", command, strings.Join(calls, "\n"), w.report())
	}
	if !slices.ContainsFunc(named, func(call string) bool { return strings.Contains(call, text) }) {
		return fmt.Errorf("no %q call of the recording gh has %q:\n%s\n%s", command, text, strings.Join(named, "\n"), w.report())
	}
	return nil
}

// Every call of the recording gh has the text, and there was one.
func (w *world) ghEveryCallHas(text string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	if len(calls) == 0 {
		return fmt.Errorf("the recording gh was never called\n%s", w.report())
	}
	for _, call := range calls {
		if !strings.Contains(call, text) {
			return fmt.Errorf("the recording gh was called without %q: %s\n%s", text, call, w.report())
		}
	}
	return nil
}

// The recording gh was never called, or never as the command ("" for either).
func (w *world) ghNeverCalled(command string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	for _, call := range calls {
		if command == "" || strings.HasPrefix(call, command+" ") || call == command {
			return fmt.Errorf("the recording gh was called: %s\n%s", call, w.report())
		}
	}
	return nil
}
