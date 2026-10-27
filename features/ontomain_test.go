// The steps of itos push --onto-main (push-mode.feature) and of the pre-push
// hook's accepted rewrite (pre-push.feature), slice-193: the clone rebased
// onto, or reset to, the remote's main; a rebase left in progress and one the
// person resolves; a commit another clone puts on the remote's item branch,
// before or, with a post-rewrite hook, while itos rebases; and a remote branch
// that lacks a commit.
package features

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

func initializeOntoMainSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^itos has run "([^"]*)", exiting with code (\d+)$`, w.itosHasRunExiting)
	sc.Step(`^the clone has reset its branch to the remote's main$`, func() error {
		return w.git("reset", "-q", "--hard", "origin/main")
	})
	sc.Step(`^the clone has rebased its branch onto the remote's main$`, func() error {
		return w.git("rebase", "origin/main")
	})
	sc.Step(`^the clone has a rebase in progress$`, w.cloneIsRebasing)
	sc.Step(`^the clone resolves the conflict in "([^"]*)", keeping both sides, and continues the rebase$`, w.cloneResolvesKeepingBoth)
	sc.Step(`^the remote's branch "([^"]*)" does not have "([^"]*)"$`, w.remoteBranchLacks)
	sc.Step(`^the remote's branch "([^"]*)" has gained the commit "([^"]*)" touching "([^"]*)"$`, w.remoteBranchGains)
	sc.Step(`^another clone pushes the commit "([^"]*)" touching "([^"]*)" to the remote's branch "([^"]*)" while itos rebases$`,
		w.anotherClonePushesWhileRebasing)
}

// itos run with the command line, which must exit with the code: a Given that
// sets the scene by a run of itos whose exit is part of it.
func (w *world) itosHasRunExiting(line string, code int) error {
	args, err := shellWords(line)
	if err != nil {
		return err
	}
	if err := w.itos(args...); err != nil {
		return err
	}
	if w.exit != code {
		return fmt.Errorf("itos %s exited %d, not %d, before the run the scenario is about\n%s", line, w.exit, code, w.report())
	}
	return nil
}

// A rebase is in progress in the clone: git keeps its state in a folder of
// the git folder.
func (w *world) cloneIsRebasing() error {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		out, err := w.gitOutput("rev-parse", "--git-path", name)
		if err != nil {
			return err
		}
		dir := strings.TrimSpace(out)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(w.dir, dir)
		}
		if _, err := os.Stat(dir); err == nil {
			return nil
		}
	}
	return fmt.Errorf("the clone has no rebase in progress\n%s", w.report())
}

// The conflict markers of the file left out, so it holds both sides' lines,
// then the file staged and the rebase continued, its message editor a no-op.
func (w *world) cloneResolvesKeepingBoth(path string) error {
	full := filepath.Join(w.dir, path)
	text, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	var kept []string
	for _, line := range strings.SplitAfter(string(text), "\n") {
		bare := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(bare, "<<<<<<<") || strings.HasPrefix(bare, ">>>>>>>") || bare == "=======" {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 || len(kept) == len(strings.SplitAfter(string(text), "\n")) {
		return fmt.Errorf("%s holds no conflict to resolve:\n%s", path, text)
	}
	if err := os.WriteFile(full, []byte(strings.Join(kept, "")), 0o644); err != nil {
		return err
	}
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	return w.git("-c", "core.editor=true", "rebase", "--continue")
}

// The remote's branch of the name has no commit with the subject.
func (w *world) remoteBranchLacks(branch, subject string) error {
	subjects, _, err := w.remoteBranchHistory(branch)
	if err != nil {
		return err
	}
	if slices.Contains(subjects, subject) {
		return fmt.Errorf("the remote's branch %q has %q:\n%s", branch, subject, strings.Join(subjects, "\n"))
	}
	return nil
}

// The other clone of the remote, made if there is none yet.
func (w *world) otherClone() (string, error) {
	other := filepath.Join(w.support, "other")
	if _, err := os.Stat(other); err != nil {
		if err := w.gitIn(w.support, "clone", "-q", w.remote(), other); err != nil {
			return "", err
		}
	}
	return other, nil
}

// A commit made in the other clone on the remote's branch as the remote has it
// now, as remoteGainsCommit makes one, and pushed to that branch; the clone is
// left on its main, where remoteGainsCommit expects it.
func (w *world) remoteBranchGains(branch, subject, path string) error {
	other, err := w.otherClone()
	if err != nil {
		return err
	}
	for _, args := range [][]string{
		{"fetch", "-q", "origin"},
		{"checkout", "-q", "-B", branch, "origin/" + branch},
	} {
		if err := w.gitIn(other, args...); err != nil {
			return err
		}
	}
	if err := writeLine(other, path, subject); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"add", "--", path},
		{"commit", "-q", "--no-verify", "-m", subject},
		{"push", "-q", "--no-verify", "origin", "HEAD:refs/heads/" + branch},
		{"checkout", "-q", "main"},
	} {
		if err := w.gitIn(other, args...); err != nil {
			return err
		}
	}
	return nil
}

// A post-rewrite hook in the clone that, once a rebase has rewritten the
// commits, puts the commit on the remote's branch from the other clone: after
// itos fetched the branch and before it pushes, with no pause for the
// scenario to hold. It runs once, git ignores its exit, and the git it runs
// gets none of the hook's own repository variables.
func (w *world) anotherClonePushesWhileRebasing(subject, path, branch string) error {
	other, err := w.otherClone()
	if err != nil {
		return err
	}
	out, err := w.gitOutput("rev-parse", "--git-path", "hooks")
	if err != nil {
		return err
	}
	dir := strings.TrimSpace(out)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(w.dir, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	slash := filepath.ToSlash
	script := strings.Join([]string{
		"#!/bin/sh",
		`[ "$1" = rebase ] || exit 0`,
		"mark=" + quote(slash(filepath.Join(w.support, "post-rewrite-ran"))),
		`[ -e "$mark" ] && exit 0`,
		`: > "$mark"`,
		"unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX",
		"cd " + quote(slash(other)) + " || exit 1",
		"git fetch -q origin",
		"git checkout -q -B " + quote(branch) + " " + quote("origin/"+branch),
		"printf '%s\\n' " + quote("A line for "+subject+".") + " > " + quote(path),
		"git add -- " + quote(path),
		"git commit -q --no-verify -m " + quote(subject),
		"git push -q --no-verify origin " + quote("HEAD:refs/heads/"+branch),
		"git checkout -q main",
		"",
	}, "\n")
	return writeExecutable(filepath.Join(dir, "post-rewrite"), []byte(script))
}
