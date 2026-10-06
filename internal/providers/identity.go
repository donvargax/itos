package providers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/value"
)

// Answer is what an identity provider says: the handle the session works
// for, or, with no handle, why it cannot say (Problem). Neither is an error:
// a session nobody is identified as works for nobody, and `work` still
// proposes what nobody owns.
type Answer struct {
	Handle, Problem string
}

// Identity is a work.identity provider: who a session works for when --as
// does not say.
type Identity func() Answer

// IdentityProvider is the provider itos.yaml's work.identity names
// (providers.ts's identityProvider): `github` (the account gh is signed in
// as) or `none` (only --as, with the config's hint). The `command` provider,
// which ran a repository's own command unasked, was removed in v5.0.0
// (slice 85).
func IdentityProvider(cfg *config.Loaded) Identity {
	id := cfg.Work.Identity
	if id.Provider == "none" {
		return func() Answer {
			return Answer{Problem: "work.identity is none, so this session is nobody; " + id.Hint}
		}
	}
	return GitHubIdentity(id.Hint, GhLogin)
}

// GhTimeout is how long the github provider waits for gh api user before
// giving up on it (bug 46): gh asked with no bound hung any command that
// asked, when gh waited on a keychain prompt, a sign-in or a proxy. A fixed
// bound, not a setting: --as is the way past a gh that does not answer.
const GhTimeout = 5 * time.Second

// ErrGhNoAnswer is GhLogin's error for a gh that did not answer within
// GhTimeout, and was killed.
var ErrGhNoAnswer = errors.New("gh did not answer")

// GhLogin is the login `gh api user` answers, trimmed as JavaScript trims,
// and the error of a gh that is missing, fails (signed out) or does not
// answer within GhTimeout (ErrGhNoAnswer). gh's stdin is the null device, not
// the terminal, so it cannot wait on a person, and its stderr is not shown:
// the provider's answer says what went wrong. Past the bound gh is killed
// with whatever it started (killTree), and its output is not waited for.
func GhLogin() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), GhTimeout)
	defer cancel()
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", "api", "user", "--jq", ".login")
	cmd.Stdout = &stdout
	killTree(cmd)
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", ErrGhNoAnswer
	}
	return value.Trim(stdout.String()), err
}

// GitHubIdentity is the GitHub account gh is signed in as, or why there is
// none: gh not on the PATH, gh not answering in time, or gh failing or
// answering nothing, which is gh signed out.
func GitHubIdentity(hint string, login func() (string, error)) Identity {
	return func() Answer {
		handle, err := login()
		if err == nil && handle != "" {
			return Answer{Handle: handle}
		}
		if errors.Is(err, exec.ErrNotFound) {
			return Answer{Problem: "gh is not installed, so this session is nobody; " + hint}
		}
		if errors.Is(err, ErrGhNoAnswer) {
			return Answer{Problem: fmt.Sprintf("gh did not answer within %s (gh api user), so this session is nobody; pass --as <handle>", GhTimeout)}
		}
		return Answer{Problem: "gh is not signed in (gh auth login), so this session is nobody; " + hint}
	}
}
