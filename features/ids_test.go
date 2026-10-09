package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/cucumber/godog"
	"github.com/donvargax/itos/v7/internal/git"
)

func initializeIDSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the remote's id counter for "([^"]*)" is (\d+)$`, func(kind, want string) error {
		got, err := w.remoteCounter(kind)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("the remote's %s counter is %s, not %s", kind, got, want)
		}
		return nil
	})
	sc.Step(`^another clone of the remote has minted "(slice-\d+)"$`, func(id string) error {
		return w.seedRemoteCounter("slice", id)
	})
	sc.Step(`^the remote cannot be reached$`, func() error {
		return w.git("remote", "set-url", "origin", filepath.Join(w.support, "not-a-remote"))
	})
	sc.Step(`^the repository has a remote it pushes to$`, func() error {
		if err := w.git("init", "-q", "--bare", w.remote()); err != nil {
			return err
		}
		if err := w.git("remote", "add", "origin", w.remote()); err != nil {
			return err
		}
		return w.git("push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main")
	})
	sc.Step(`^the repository has no ref "([^"]*)"$`, func(ref string) error {
		return w.noLocalRef(ref)
	})
	sc.Step(`^the remote has no ref "([^"]*)"$`, func(ref string) error {
		return w.noRemoteRef(ref)
	})
	sc.Step(`^the clones concurrently add a slice$`, w.concurrentSliceMints)
	sc.Step(`^the concurrent clones minted "([^"]*)" and "([^"]*)"$`, w.concurrentSliceMintsAre)
}

// Both clones are held at their counter-ref push until each has built a claim
// from the same advertised parent. The counter commits therefore must differ
// even with identical author, committer and timestamp metadata.
func (w *world) concurrentSliceMints() error {
	if err := w.git("push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main"); err != nil {
		return err
	}
	second := filepath.Join(w.support, "slice-clone-two")
	if err := w.gitIn(w.support, "clone", "-q", w.remote(), second); err != nil {
		return err
	}
	if err := w.declareHooks(second); err != nil {
		return err
	}
	barrier := filepath.ToSlash(filepath.Join(w.support, "counter-push-barrier"))
	w.atOnce = make([]atOnceRun, 2)
	start := make(chan struct{})
	failed := make([]error, 2)
	var done sync.WaitGroup
	for i, clone := range []string{w.dir, second} {
		wrapper, err := w.counterGitWrapper(barrier, []string{"first", "second"}[i])
		if err != nil {
			return err
		}
		args := []string{"work", "add", "--kind", "slice", "--phase", "1", "--title", "Concurrent slice", "--why", "Two clones need distinct IDs."}
		cmd := exec.Command(w.bin, args...)
		cmd.Dir = clone
		cmd.Env = append(w.env(),
			"ITOS_GIT="+wrapper,
			"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z",
			"GIT_COMMITTER_DATE=2000-01-01T00:00:00Z",
		)
		var stdout, stderr strings.Builder
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		done.Add(1)
		go func(i int, args []string, cmd *exec.Cmd) {
			defer done.Done()
			<-start
			err := cmd.Run()
			run := atOnceRun{args: args, stdout: stdout.String(), stderr: stderr.String()}
			var exit *exec.ExitError
			switch {
			case errors.As(err, &exit):
				run.exit = exit.ExitCode()
			case err != nil:
				failed[i] = fmt.Errorf("running itos %s: %w", strings.Join(args, " "), err)
			}
			w.atOnce[i] = run
		}(i, args, cmd)
	}
	close(start)
	done.Wait()
	return errors.Join(failed...)
}

func (w *world) counterGitWrapper(barrier, name string) (string, error) {
	program := filepath.Join(w.support, "counter-git-"+name)
	marker := quote(barrier + "/" + name)
	first := quote(barrier + "/first")
	second := quote(barrier + "/second")
	native := quote(filepath.ToSlash(git.Bin()))
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = push ]; then
  mkdir -p %s
  tries=0
  while [ ! -d %s ] || [ ! -d %s ]; do
    tries=$((tries + 1))
    [ "$tries" -lt 500 ] || { echo "counter push rendezvous timed out" >&2; exit 97; }
    sleep 0.01
  done
fi
exec %s "$@"
`, marker, first, second, native)
	if err := w.writeProgram(program, script); err != nil {
		return "", err
	}
	return programPath(program), nil
}

func (w *world) concurrentSliceMintsAre(first, second string) error {
	if len(w.atOnce) != 2 {
		return fmt.Errorf("got %d concurrent mint results, want two", len(w.atOnce))
	}
	counts := map[string]int{first: 0, second: 0}
	for _, run := range w.atOnce {
		for id := range counts {
			if strings.Contains(run.stdout, id) {
				counts[id]++
			}
		}
	}
	if counts[first] != 1 || counts[second] != 1 {
		return fmt.Errorf("concurrent mint outputs do not contain each ID once: %q and %q; runs: %+v", first, second, w.atOnce)
	}
	return nil
}

// seedRemoteCounter makes a counter-tree commit in a separate repository and
// pushes only the counter ref, as a different machine's successful mint does.
func (w *world) seedRemoteCounter(kind, id string) error {
	n, err := strconv.Atoi(strings.TrimPrefix(id, kind+"-"))
	if err != nil {
		return fmt.Errorf("invalid fixture id %q: %w", id, err)
	}
	repo := filepath.Join(w.support, "counter-writer")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		return err
	}
	if err := w.gitIn(w.support, "init", "-q", repo); err != nil {
		return err
	}
	if err := w.gitIn(repo, "remote", "add", "origin", w.remote()); err != nil {
		return err
	}
	file := filepath.Join(repo, "counters", kind)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(strconv.Itoa(n)+"\n"), 0o644); err != nil {
		return err
	}
	if err := w.gitIn(repo, "add", "--", "counters/"+kind); err != nil {
		return err
	}
	if err := w.gitIn(repo, "commit", "-q", "-m", "counter: reserve "+id); err != nil {
		return err
	}
	return w.gitIn(repo, "push", "-q", "--no-verify", "origin", "HEAD:refs/itos/ids")
}

func (w *world) remoteCounter(kind string) (string, error) {
	cmd := exec.Command(git.Bin(), "show", "refs/itos/ids:counters/"+kind)
	cmd.Dir, cmd.Env = w.remote(), w.env()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("read remote id counter for %s: %w", kind, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (w *world) noLocalRef(ref string) error {
	cmd := exec.Command(git.Bin(), "show-ref", "--verify", "--quiet", ref)
	cmd.Dir, cmd.Env = w.dir, w.env()
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("repository has ref %s", ref)
	}
	return nil
}

func (w *world) noRemoteRef(ref string) error {
	cmd := exec.Command(git.Bin(), "show-ref", "--verify", "--quiet", ref)
	cmd.Dir, cmd.Env = w.remote(), w.env()
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("remote has ref %s", ref)
	}
	return nil
}
