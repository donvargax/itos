package providers

import (
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
