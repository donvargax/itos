package gitrun

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// A git that failed exits as it did, one a signal ended 1; one that did not
// start is the error, with no code.
func TestExitOf(t *testing.T) {
	plain := errors.New("no git")
	for _, c := range []struct {
		err  error
		code int
		want error
	}{
		{nil, 0, nil},
		{&vcsport.Failed{Code: 2}, 2, nil},
		{&vcsport.Failed{Code: -1}, 1, nil},
		{&vcsport.Failed{Code: 0}, 1, nil},
		{plain, 0, plain},
	} {
		if code, err := ExitOf(c.err); code != c.code || err != c.want {
			t.Errorf("ExitOf(%v) = %d, %v", c.err, code, err)
		}
	}
}

// --no-verify is the option and each start of it git takes, down to
// --no-veri; a shorter start is another option's, and a longer word none.
func TestNoVerify(t *testing.T) {
	for arg, want := range map[string]bool{"--no-verify": true, "--no-verif": true, "--no-veri": true,
		"--no-ver": false, "--no-verifyx": false, "--verify": false, "-n": false} {
		if got := NoVerify(arg); got != want {
			t.Errorf("NoVerify(%q) = %v", arg, got)
		}
	}
}

// git talks to the terminal from where the person stood, owning an
// interrupt, in the environment it is given.
func TestTalking(t *testing.T) {
	var stdout, stderr strings.Builder
	talk := Talking("sub", []string{"A=1"}, &stdout, &stderr)
	if talk.Dir != "sub" || talk.Stdin != io.Reader(os.Stdin) || talk.Stdout != &stdout || talk.Stderr != &stderr ||
		!slices.Equal(talk.Env, []string{"A=1"}) || !talk.Owns || talk.NoPrompt {
		t.Errorf("Talking = %+v", talk)
	}
}

// config is the git config in memory: its values by key, and an error to
// give an addition.
type config struct {
	values map[string][]string
	err    error
}

func (c *config) Config(_ context.Context, key string, _ bool) []string { return c.values[key] }

func (c *config) AddConfig(_ context.Context, key, value string) error {
	if c.err != nil {
		return c.err
	}
	c.values[key] = append(c.values[key], value)
	return nil
}

// notes.rewriteRef gains refs/notes/itos unless a value, a pattern among
// them, already names it; an addition git refuses is the error.
func TestRewriteNotes(t *testing.T) {
	ctx := t.Context()
	for _, c := range []struct {
		have, want []string
	}{
		{nil, []string{"refs/notes/itos"}},
		{[]string{"refs/notes/commits"}, []string{"refs/notes/commits", "refs/notes/itos"}},
		{[]string{" refs/notes/* "}, []string{" refs/notes/* "}},
		{[]string{"refs/notes/itos"}, []string{"refs/notes/itos"}},
	} {
		g := &config{values: map[string][]string{"notes.rewriteRef": slices.Clone(c.have)}}
		if err := RewriteNotes(ctx, g); err != nil || !slices.Equal(g.values["notes.rewriteRef"], c.want) {
			t.Errorf("RewriteNotes over %q = %v, %q", c.have, err, g.values["notes.rewriteRef"])
		}
	}
	refused := errors.New("refused")
	if err := RewriteNotes(ctx, &config{values: map[string][]string{}, err: refused}); err != refused {
		t.Errorf("RewriteNotes refused = %v", err)
	}
}

// refusedNote is the fake, its notes refused with git's words, or with an
// error of no git's.
type refusedNote struct {
	*porttest.VCS
	err error
}

func (r refusedNote) Note(context.Context, string, string) error { return r.err }

// The note of a commit just made is the footers, a line each; one that
// cannot be written says git's words, or the error's; none is written when
// HEAD did not move, and with no HEAD the error is the resolving's.
func TestWriteNote(t *testing.T) {
	ctx := t.Context()
	repo := func() *porttest.VCS {
		return &porttest.VCS{History: []porttest.Commit{{Name: "c-a"}}, Named: map[string]string{"HEAD": "c-a"}}
	}
	v := repo()
	if err := WriteNote(ctx, v, "", []string{"Task: T-1", "Item: a"}); err != nil || v.History[0].Note != "Task: T-1\nItem: a\n" {
		t.Errorf("WriteNote = %v, note %q", err, v.History[0].Note)
	}
	failed := refusedNote{repo(), &vcsport.Failed{Command: "git notes", Code: 1, Words: "refused\n"}}
	if err := WriteNote(ctx, failed, "", []string{"Task: T-1"}); err == nil || err.Error() != "git notes add: refused" {
		t.Errorf("WriteNote = %v", err)
	}
	plain := refusedNote{repo(), errors.New("no git")}
	if err := WriteNote(ctx, plain, "", []string{"Task: T-1"}); err == nil || err.Error() != "git notes add: no git" {
		t.Errorf("WriteNote with no git = %v", err)
	}
	if err := WriteNote(ctx, failed, "c-a", []string{"Task: T-1"}); err != nil {
		t.Errorf("WriteNote with HEAD as it was = %v", err)
	}
	if err := WriteNote(ctx, refusedNote{&porttest.VCS{}, nil}, "", nil); err == nil {
		t.Error("WriteNote with no HEAD: no error")
	}
}
