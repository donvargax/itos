package providers

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
)

// ErrNoGh is Gh's error for a gh that is not on the PATH.
var ErrNoGh = errors.New("gh is not on the PATH")

// GhCallTimeout is how long a gh call of itos push may take before it is
// killed: a pull request opened or merged is a few requests, and a gh that
// waits on a sign-in must not hold the push for good.
const GhCallTimeout = 2 * time.Minute

// GhFailed is a gh that ran and failed, or that was given up on: Words is
// what it said on stderr, Err how it ended, TimedOut whether it was killed
// for taking longer than it was given.
type GhFailed struct {
	Words    string
	Err      error
	TimedOut bool
}

func (e *GhFailed) Error() string { return e.Err.Error() }

func (e *GhFailed) Unwrap() error { return e.Err }

// Gh runs gh with the arguments in the folder dir, the one itos runs in,
// and gives its stdout. gh's stdin is the null device, so it cannot wait on
// a person; past GhCallTimeout it is killed with whatever it started
// (killTree). The error is ErrNoGh for a gh that is not on the PATH, else a
// GhFailed.
func Gh(ctx context.Context, dir string, args ...string) (string, error) {
	return ghWithin(ctx, GhCallTimeout, dir, args)
}

// GhBounded is Gh for a command that runs at every itos go, bounded by
// GhTimeout as identity's gh is (bug 46): the error is ErrGhNoAnswer for a
// gh that did not answer in time, killed as Gh's is.
func GhBounded(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := ghWithin(ctx, GhTimeout, dir, args)
	var failed *GhFailed
	if errors.As(err, &failed) && failed.TimedOut {
		return "", ErrGhNoAnswer
	}
	return out, err
}

// ghWithin is Gh with the time gh is given.
func ghWithin(ctx context.Context, timeout time.Duration, dir string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdout, cmd.Stderr, cmd.Dir = &stdout, &stderr, dir
	killTree(cmd)
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return "", ErrNoGh
	case err != nil:
		return stdout.String(), &GhFailed{Words: stderr.String(), Err: err, TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	}
	return stdout.String(), nil
}

// GhReason is why a Gh call failed, in the words a person reads: that gh is
// not on the PATH, what gh said on stderr, else the error itself.
func GhReason(err error) string {
	if errors.Is(err, ErrNoGh) {
		return ErrNoGh.Error()
	}
	var failed *GhFailed
	if errors.As(err, &failed) && strings.TrimSpace(failed.Words) != "" {
		return strings.TrimSpace(failed.Words)
	}
	return err.Error()
}

// Repository is the GitHub repository ("owner/name") the watch asks about,
// and so every gh call of itos push names: the variable
// ci.range.github.repository_env names, else the one the remote's URL
// names; "" when neither does.
func Repository(cfg *config.Loaded, env Env, remoteURL string) string {
	if repository := env(cfg.CI.Range.GitHub.RepositoryEnv); repository != "" {
		return repository
	}
	return GitHubRepository(remoteURL)
}
