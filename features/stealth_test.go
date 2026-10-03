// The steps of the stealth mode (stealth.feature): the scratch repository's
// config and itos's data kept in the folder of its git folder that itos reads
// when the root has no itos.yaml, <git common dir>/itos, and linked worktrees
// of it.
package features

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	sc.Step(`^a linked worktree of the repository at "([^"]*)"$`, w.linkedWorktree)
	sc.Step(`^an itos\.yaml in the root whose ledger has no task "([^"]*)"$`, w.rootConfigWithout)

	sc.Step(`^itos runs the task "([^"]*)" in the linked worktree$`, func(task string) error {
		if w.linked == "" {
			return fmt.Errorf("the scenario adds no linked worktree")
		}
		return w.itosIn(w.linked, "task", task)
	})

	sc.Step(`^git status shows nothing to commit$`, w.nothingToCommit)
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
