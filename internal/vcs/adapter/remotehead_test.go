package adapter

import (
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// RemoteHead asks the remote itself which branch its HEAD names: main for a
// remote whose HEAD names a branch it has, trunk for one whose HEAD names a
// branch it does not (git answers the name alone), none for a remote whose
// HEAD is a commit, and git's words when there is no such remote.
func TestRemoteHead(t *testing.T) {
	dir := committed(t, "a.txt")
	remote := filepath.Join(alone(t), "origin.git")
	gitIn(t, dir, "", "clone", "-q", "--bare", dir, remote)
	g := Git{}
	if got, err := g.RemoteHead(t.Context(), remote, port.Talk{}); got != "main" || err != nil {
		t.Errorf("RemoteHead = %q, %v, want main", got, err)
	}
	gitIn(t, remote, "", "symbolic-ref", "HEAD", "refs/heads/trunk")
	if got, err := g.RemoteHead(t.Context(), remote, port.Talk{}); err != nil || got != "" && got != "trunk" {
		t.Errorf("RemoteHead of a HEAD naming a branch the remote lacks = %q, %v, want trunk or none", got, err)
	}
	gitIn(t, remote, "", "update-ref", "--no-deref", "HEAD", "refs/heads/main")
	if got, err := g.RemoteHead(t.Context(), remote, port.Talk{}); got != "" || err != nil {
		t.Errorf("RemoteHead of a detached HEAD = %q, %v, want none", got, err)
	}
	_, err := g.RemoteHead(t.Context(), filepath.Join(alone(t), "nowhere"), port.Talk{})
	if failed, ok := err.(*port.Failed); !ok || failed.Failure != port.NoRepository {
		t.Errorf("RemoteHead of no repository = %v, want a Failed with no repository", err)
	}
}
