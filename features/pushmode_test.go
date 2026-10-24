// The steps of push.mode pr (push-mode.feature, slice 174): the config
// committed and pushed with a block of YAML added to it; a recording gh
// standing in for GitHub, first on the PATH, which records each call and keeps
// the pull requests it opens; the clone on a new branch, with or without
// tracking the remote's main; the GitHub repository the environment names
// (GITHUB_REPOSITORY), as the fake GitHub gives it, or none; the remote's HEAD
// naming a branch it does not have; and what a branch of the remote ends with.
package features

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

func initializePushModeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the config, committed and pushed to the remote's main, adds the YAML:$`, func(text *godog.DocString) error {
		w.config.extraYAML = append(w.config.extraYAML, strings.TrimRight(text.Content, "\n")+"\n")
		return w.pushConfig("chore: add to the config")
	})
	sc.Step(`^a recording gh on the PATH, which opens pull requests as "([^"]*)"$`, w.recordingGh)
	sc.Step(`^the recording gh has the pull request "([^"]*)" open for the branch "([^"]*)"$`, w.ghHasPullRequest)
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
// given, and a line for each open pull request, its branch and its address.
func (w *world) ghCalls() string { return filepath.Join(w.support, "gh-calls") }
func (w *world) ghPulls() string { return filepath.Join(w.support, "gh-pull-requests") }

// recordingGh is a gh first on the PATH that stands in for GitHub: it records
// each call, answers pr list with the open pull requests of its --head branch
// as gh does ([] for none), and answers pr create with the address given,
// holding that pull request open from then on; any other call succeeds
// silently. The environment names owner/scratch as the GitHub repository, as
// the fake GitHub does. A pull request's number is the end of its address.
func (w *world) recordingGh(address string) error {
	bin, err := w.binOnPath()
	if err != nil {
		return err
	}
	calls, pulls := quote(filepath.ToSlash(w.ghCalls())), quote(filepath.ToSlash(w.ghPulls()))
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %[1]s
verb="$1 $2"
head=
prev=
for a in "$@"; do
  if [ "$prev" = --head ]; then head=$a; fi
  prev=$a
done
case "$verb" in
"pr list")
  found=
  if [ -f %[2]s ]; then
    while read -r branch url; do
      if [ "$branch" = "$head" ]; then
        if [ -n "$found" ]; then found="$found,"; fi
        found="$found{\"number\":${url##*/},\"url\":\"$url\"}"
      fi
    done < %[2]s
  fi
  printf '[%%s]\n' "$found"
  ;;
"pr create")
  printf '%%s %%s\n' "$head" %[3]s >> %[2]s
  printf '%%s\n' %[3]s
  ;;
esac
`, calls, pulls, quote(address))
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
	if _, err := os.Stat(programPath(filepath.Join(w.support, "bin", "gh"))); err != nil {
		return errors.New("no recording gh: put one on the PATH first")
	}
	f, err := os.OpenFile(w.ghPulls(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s %s\n", branch, address)
	return errors.Join(err, f.Close())
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
