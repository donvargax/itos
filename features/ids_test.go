package features

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

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
