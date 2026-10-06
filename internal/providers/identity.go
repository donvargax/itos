package providers

import (
	"bytes"
	"errors"
	"os/exec"

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

// GhLogin is the login `gh api user` answers, trimmed as JavaScript trims,
// and the error of a gh that is missing or fails (signed out). gh's stderr
// is not shown: the provider's answer says what went wrong.
func GhLogin() (string, error) {
	var stdout bytes.Buffer
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")
	cmd.Stdout = &stdout
	err := cmd.Run()
	return value.Trim(stdout.String()), err
}

// GitHubIdentity is the GitHub account gh is signed in as, or why there is
// none: gh not on the PATH, or gh failing or answering nothing, which is gh
// signed out.
func GitHubIdentity(hint string, login func() (string, error)) Identity {
	return func() Answer {
		handle, err := login()
		if err == nil && handle != "" {
			return Answer{Handle: handle}
		}
		if errors.Is(err, exec.ErrNotFound) {
			return Answer{Problem: "gh is not installed, so this session is nobody; " + hint}
		}
		return Answer{Problem: "gh is not signed in (gh auth login), so this session is nobody; " + hint}
	}
}
