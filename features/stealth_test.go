// The steps of the stealth mode (stealth.feature): the scratch repository's
// config and itos's data kept in the folder of its git folder that itos reads
// when the root has no itos.yaml, <git common dir>/itos, linked worktrees of
// it, and the hooks declared in its git config beside a project's own.
package features

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// Where the stealth mode keeps the config and its data in a repository whose
// git folder is .git: what git rev-parse --git-common-dir names, and itos/.
var stealthDir = filepath.Join(".git", "itos")

func initializeStealthSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a repository whose ledger has the task "([^"]*)", kept in its git folder$`, w.stealthRepository)
	sc.Step(`^the work registry beside the config has the item "([^"]*)" with the status "([^"]*)"$`, func(item, status string) error {
		return w.workingRegistry(w.data(startingRegistry), item, status)
	})
	sc.Step(`^the work registry beside the config has the item "([^"]*)" owned by "([^"]*)" with the status "([^"]*)"$`, func(item, owner, status string) error {
		return w.workingRegistryOwned(w.data(startingRegistry), item, owner, status)
	})
	sc.Step(`^the config in the git folder names no people file$`, func() error {
		w.config.noPeople = true
		return w.writeConfig()
	})
	sc.Step(`^a linked worktree of the repository at "([^"]*)"$`, w.linkedWorktree)
	sc.Step(`^an itos\.yaml in the root whose ledger has no task "([^"]*)"$`, w.rootConfigWithout)

	sc.Step(`^itos runs the task "([^"]*)" in the linked worktree$`, func(task string) error {
		if w.linked == "" {
			return fmt.Errorf("the scenario adds no linked worktree")
		}
		return w.itosIn(w.linked, "task", task)
	})

	sc.Step(`^git status shows nothing to commit$`, w.nothingToCommit)

	sc.Step(`^a remote that has the commit "([^"]*)" with no task$`, w.remoteWithCommit)
	sc.Step(`^itos verifies with no range$`, func() error { return w.itos("verify") })
	sc.Step(`^itos plans CI with no range$`, func() error { return w.itos("ci", "plan") })

	sc.Step(`^the project's hooks are in "([^"]*)" by core\.hooksPath, with a commit-msg hook that records it ran$`, w.projectHooks)
	sc.Step(`^itos has installed the hooks$`, w.itosHasInstalledHooks)
	sc.Step(`^the git config declares a "([^"]*)" hook that runs itos$`, w.declaresHookRunningItos)
	sc.Step(`^the git config declares a "([^"]*)" hook whose command starts with "([^"]*)"$`, w.declaresHookStartingWith)
	sc.Step(`^the project's commit-msg hook ran$`, w.projectHookRan)
	sc.Step(`^core\.hooksPath is still "([^"]*)"$`, w.hooksPathIs)
}

// Where the project's commit-msg hook records that it ran, in the scenario's
// support folder.
func (w *world) projectHookRecord() string { return filepath.Join(w.support, "project-commit-msg-ran") }

// The project's own hooks folder, committed, with a commit-msg hook that
// records it ran and passes, and core.hooksPath pointing git at it, as husky
// sets it.
func (w *world) projectHooks(dir string) error {
	hook := "#!/bin/sh\necho ran >> " + quote(w.projectHookRecord()) + "\n"
	full := filepath.Join(w.dir, dir, "commit-msg")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, []byte(hook), 0o755); err != nil {
		return err
	}
	if err := w.commit("chore: add the project's hooks"); err != nil {
		return err
	}
	return w.git("config", "core.hooksPath", dir)
}

// hooks install, which has to succeed, its hooks running the itos under
// test: hooks.bin is itos, found on the PATH git hands its hooks, as a
// global install is. It is a script that runs the itos under test, not a
// link to it, since tools/bin/itos finds its checkout from its own path; and
// it has to run there, so a hook that fails to start cannot pass for one
// that refuses.
func (w *world) itosHasInstalledHooks() error {
	bin, err := w.binOnPath()
	if err != nil {
		return err
	}
	script := "#!/bin/sh\nexec " + quote(w.bin) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "itos"), []byte(script), 0o755); err != nil {
		return err
	}
	if err := w.run(w.dir, "sh", "-c", "itos version"); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("itos on the PATH does not run\n%s", w.report())
	}
	if err := w.hooksBinIs("itos"); err != nil {
		return err
	}
	if err := w.itos("hooks", "install"); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("hooks install failed\n%s", w.report())
	}
	return nil
}

// The hook the repository's own git config declares for an event: its name
// and command, as git config reads them, and git hook list naming it, so git
// runs it.
func (w *world) declaredHook(event string) (name, command string, err error) {
	out, err := w.gitOutput("config", "--local", "--get-regexp", `^hook\..*\.event$`)
	if err != nil {
		return "", "", fmt.Errorf("the git config declares no hook: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		key, value, _ := strings.Cut(line, " ")
		if value == event {
			name = strings.TrimSuffix(strings.TrimPrefix(key, "hook."), ".event")
			break
		}
	}
	if name == "" {
		return "", "", fmt.Errorf("the git config declares no %s hook:\n%s", event, out)
	}
	command, err = w.gitOutput("config", "--local", "hook."+name+".command")
	if err != nil {
		return "", "", fmt.Errorf("the hook %s has no command: %w", name, err)
	}
	listed, err := w.gitOutput("hook", "list", event)
	if err != nil {
		return "", "", err
	}
	if !slices.Contains(strings.Fields(listed), name) {
		return "", "", fmt.Errorf("git hook list %s does not name %s:\n%s", event, name, listed)
	}
	return name, strings.TrimSpace(command), nil
}

// The declared hook's command runs itos's hook for that event.
func (w *world) declaresHookRunningItos(event string) error {
	name, command, err := w.declaredHook(event)
	if err != nil {
		return err
	}
	if !regexp.MustCompile(`(^|[ /])itos hook ` + regexp.QuoteMeta(event) + `$`).MatchString(command) {
		return fmt.Errorf("the hook %s runs %q, not itos hook %s", name, command, event)
	}
	return nil
}

// The declared hook's command starts with the text given.
func (w *world) declaresHookStartingWith(event, start string) error {
	name, command, err := w.declaredHook(event)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(command, start) {
		return fmt.Errorf("the hook %s runs %q, which does not start with %q", name, command, start)
	}
	return nil
}

func (w *world) projectHookRan() error {
	if _, err := os.Stat(w.projectHookRecord()); err != nil {
		return fmt.Errorf("the project's commit-msg hook did not run\n%s", w.report())
	}
	return nil
}

func (w *world) hooksPathIs(dir string) error {
	out, err := w.gitOutput("config", "core.hooksPath")
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(out); got != dir {
		return fmt.Errorf("core.hooksPath is %q, not %q", got, dir)
	}
	return nil
}

// A repository that does not use itos: its config, ledger, registry and
// people in the git folder, and only a README committed, its first commit
// naming no task.
func (w *world) stealthRepository(task string) error {
	w.dataDir = stealthDir
	if err := w.startingFiles(task); err != nil {
		return err
	}
	return w.commit("docs: start")
}

// A linked worktree of the scratch repository, at a path relative to it. The
// repository first moves into a folder of its own, under the scenario's
// support folder, so a path beside it is the scenario's alone and goes with
// it.
func (w *world) linkedWorktree(path string) error {
	moved := filepath.Join(w.support, "repository")
	if err := os.Rename(w.dir, moved); err != nil {
		return err
	}
	w.dir = moved
	w.linked = filepath.Join(w.dir, path)
	if !strings.HasPrefix(w.linked, w.support+string(filepath.Separator)) {
		return fmt.Errorf("the linked worktree %s would be outside the scenario's folders", path)
	}
	return w.git("worktree", "add", "-q", path)
}

// The project's own config in the root, beside the stealth one, with a
// ledger in the root that has one task, which is not the one named.
func (w *world) rootConfigWithout(task string) error {
	other := "T-002"
	if task == other {
		other = "T-003"
	}
	w.dataDir = ""
	return w.startingFiles(other)
}

// git status sees no change, staged, unstaged or untracked.
func (w *world) nothingToCommit() error {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = w.dir
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("git status shows changes:\n%s", out)
	}
	return nil
}

// A remote, a bare repository in the scenario's support folder, that has
// every commit so far and one more on top, whose type needs a task and which
// names none: someone else's, following no rules of the person's. git push
// sends the branch and leaves the itos notes behind, and sets the
// remote-tracking branch that --remotes reads.
func (w *world) remoteWithCommit(subject string) error {
	if err := w.commitOnTop("chore: " + subject); err != nil {
		return err
	}
	remote := filepath.Join(w.support, "remote.git")
	if err := w.git("init", "-q", "--bare", remote); err != nil {
		return err
	}
	if err := w.git("remote", "add", "origin", remote); err != nil {
		return err
	}
	return w.git("push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main")
}
