package idcounter

import (
	"context"
	"maps"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// remoteFake is the VCS port's in-memory fake with the remote the counter is
// claimed on made to misbehave as a test says: beforePush runs before each
// push, and may land another clone's claim first or refuse the push; and a
// push the remote refuses because it moved reads as git words it, which the
// claim retries on, the fake's own words being no git's.
type remoteFake struct {
	*porttest.VCS
	beforePush func() error
}

const origin = "origin"

func newRemoteFake() *remoteFake {
	return &remoteFake{VCS: &porttest.VCS{
		Root:   "repository",
		Named:  map[string]string{},
		Remote: map[string]*porttest.Remote{origin: {URL: "remote", Refs: map[string]string{}}},
	}}
}

func (r *remoteFake) Push(ctx context.Context, remote string, specs []string, p port.Pushing, talk port.Talk) error {
	if r.beforePush != nil {
		if err := r.beforePush(); err != nil {
			return err
		}
	}
	err := r.VCS.Push(ctx, remote, specs, p, talk)
	if failed, ok := err.(*port.Failed); ok && failed.Failure == port.Refused {
		failed.Words = " ! [rejected] " + remoteRef + " -> " + remoteRef + " (fetch first)\nerror: failed to push some refs\n"
	}
	return err
}

// refuse is a push the remote refuses for no race: a hook declining it.
func refuse() error {
	return &port.Failed{Command: "git push " + origin, Code: 1, Failure: port.Refused, Words: "remote: error: declined by a hook\n"}
}

// advance is another clone's claim landing on the remote: a counter commit on
// the counter's ref, fast-forward, that holds the category's counter at n.
func (r *remoteFake) advance(ctx context.Context, category string, n int) error {
	remote := r.Remote[origin]
	commit, err := r.CommitFile(ctx, remote.Refs[remoteRef], "counters/"+category, strconv.Itoa(n)+"\n", "another clone", "another", "another@localhost")
	if err != nil {
		return err
	}
	remote.Refs = maps.Clone(remote.Refs)
	remote.Refs[remoteRef] = commit
	return nil
}

// stored is the category's counter on the remote, 0 where it has none.
func (r *remoteFake) stored(ctx context.Context, category string) (int, error) {
	tip, ok := r.Remote[origin].Refs[remoteRef]
	if !ok {
		return 0, nil
	}
	text, ok, err := r.File(ctx, tip, "counters/"+category)
	if err != nil || !ok {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(text))
}
