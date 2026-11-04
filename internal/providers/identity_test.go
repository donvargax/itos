package providers

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// A gh that does not answer in time leaves the session nobody, naming --as
// as the way past it, whatever the config's hint says (bug 46).
func TestGitHubIdentityNoAnswer(t *testing.T) {
	answer := GitHubIdentity("ask someone", func() (string, error) { return "", ErrGhNoAnswer })()
	if answer.Handle != "" || !strings.Contains(answer.Problem, "--as") || !strings.Contains(answer.Problem, "did not answer") {
		t.Errorf("%+v", answer)
	}
}

// A gh that cannot run is gh's own failure: no answer in time is only the
// bound's, so the session is not told gh did not answer.
func TestGhLoginNotOnThePathIsNotNoAnswer(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	login, err := GhLogin(t.TempDir())
	if login != "" || !errors.Is(err, exec.ErrNotFound) || errors.Is(err, ErrGhNoAnswer) {
		t.Errorf("login %q, error %v", login, err)
	}
}
