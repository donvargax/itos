// The steps of itos's hooks on a git that runs no hook its config declares
// (old-git.feature): the git itos runs answering as git 2.51 does, and the
// hook files a hooks folder holds.
package features

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func initializeOldGitSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the git itos runs runs no hook its config declares, as git 2\.51 does$`, w.git251)
	sc.Step(`^the hooks folder holds a commit-msg hook that records it ran$`, w.hooksFolderHoldsRecordingHook)
	sc.Step(`^the project's hooks are in "([^"]*)" by core\.hooksPath, with a commit-msg hook that calls itos's commit-msg hook$`,
		w.projectHooksCallingItos)
	sc.Step(`^the project's commit-msg hook ran once$`, w.projectHookRanOnce)
	sc.Step(`^core\.hooksPath is "([^"]*)", a folder git does not track$`, w.untrackedHooksFolder)
}

// core.hooksPath names a folder of the work tree that is there, empty, so
// git tracks nothing in it.
func (w *world) untrackedHooksFolder(dir string) error {
	if err := os.MkdirAll(filepath.Join(w.dir, dir), 0o755); err != nil {
		return err
	}
	return w.git("config", "core.hooksPath", dir)
}

// git 2.51's answers to what itos asks of a git: git --version, and git
// hook list, which 2.51 does not have (its git hook has the one subcommand
// run, and parse-options refuses any other with this error and the usage,
// exit 129). Every other command goes to the git on the PATH. Options before
// the command are skipped, -C and -c with their values, so the probe's -c
// entries are not taken for it.
const git251 = `#!/bin/sh
skip=
command=
for word in "$@"; do
  if [ -n "$skip" ]; then skip=; continue; fi
  if [ -n "$command" ]; then
    if [ "$command" = hook ] && [ "$word" = list ]; then
      echo "error: unknown subcommand: \` + "`" + `list'" >&2
      echo "usage: git hook run [--ignore-missing] [--to-stdin=<path>] <hook-name> [-- <hook-args>]" >&2
      exit 129
    fi
    break
  fi
  case $word in
    -C|-c) skip=1 ;;
    --version|version) echo "git version 2.51.0"; exit 0 ;;
    -*) ;;
    *) command=$word ;;
  esac
done
exec git "$@"
`

// The git itos runs (ITOS_GIT) is git 2.51's stand-in, a program in the
// support folder (writeProgram); the git the steps and plain git in a
// scenario run is the runner's own, which runs the hook files itos writes.
// The fixture's config entries (declareHooks) go: git 2.51 would run none
// of them, and the runner's git, which may run them, must run no hook but
// the hooks folder's.
func (w *world) git251() error {
	path := filepath.Join(w.support, "git-2.51")
	if err := w.writeProgram(path, git251); err != nil {
		return err
	}
	w.vars = append(w.vars, "ITOS_GIT="+programPath(path))
	return w.undeclareHooks(w.dir)
}

// A commit-msg hook in the git folder's hooks folder, untracked, that
// records it ran, as projectHooks's does, and passes.
func (w *world) hooksFolderHoldsRecordingHook() error {
	full := filepath.Join(w.dir, ".git", "hooks", "commit-msg")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return writeExecutable(full, []byte("#!/bin/sh\necho ran >> "+quote(w.projectHookRecord())+"\n"))
}

// The project's own hooks folder, committed, with a commit-msg hook that
// runs itos's, the line hook install names, and core.hooksPath pointing git
// at it; itos on the PATH, as a global install is (itosOnPath).
func (w *world) projectHooksCallingItos(dir string) error {
	if err := w.itosOnPath(); err != nil {
		return err
	}
	full := filepath.Join(w.dir, dir, "commit-msg")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := writeExecutable(full, []byte("#!/bin/sh\nitos hook commit-msg \"$@\"\n")); err != nil {
		return err
	}
	if err := w.commit("chore: add the project's hooks"); err != nil {
		return err
	}
	return w.git("config", "core.hooksPath", dir)
}

// The project's commit-msg hook recorded exactly one run.
func (w *world) projectHookRanOnce() error {
	text, err := os.ReadFile(w.projectHookRecord())
	if err != nil {
		return fmt.Errorf("the project's commit-msg hook did not run\n%s", w.report())
	}
	if n := strings.Count(string(text), "ran"); n != 1 {
		return fmt.Errorf("the project's commit-msg hook ran %d times\n%s", n, w.report())
	}
	return nil
}
