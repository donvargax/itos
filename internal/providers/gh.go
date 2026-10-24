package providers

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
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
// what it said on stderr, Err how it ended.
type GhFailed struct {
	Words string
	Err   error
}

func (e *GhFailed) Error() string { return e.Err.Error() }

func (e *GhFailed) Unwrap() error { return e.Err }

// Gh runs gh with the arguments in the folder dir, the one itos runs in,
// and gives its stdout. gh's stdin is the null device, so it cannot wait on
// a person; past GhCallTimeout it is killed with whatever it started
// (killTree). The error is ErrNoGh for a gh that is not on the PATH, else a
// GhFailed.
func Gh(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, GhCallTimeout)
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
		return stdout.String(), &GhFailed{Words: stderr.String(), Err: err}
	}
	return stdout.String(), nil
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
